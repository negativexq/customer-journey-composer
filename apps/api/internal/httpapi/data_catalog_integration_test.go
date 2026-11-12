//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/database"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestWorkspaceCatalogDiscoveryIsolationAndCuration(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Open(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(New(slog.New(slog.NewTextHandler(io.Discard, nil)), func(context.Context) error { return nil }, pool))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	csrf := ""
	call := func(method, path string, body any, want int) []byte {
		t.Helper()
		raw, _ := json.Marshal(body)
		req, _ := http.NewRequest(method, server.URL+"/api/v1"+path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", csrf)
		res, e := client.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		data, _ := io.ReadAll(res.Body)
		if res.StatusCode != want {
			t.Fatalf("%s: %d %s", path, res.StatusCode, data)
		}
		return data
	}
	register := func() Identity {
		var user Identity
		json.Unmarshal(call("POST", "/auth/register", map[string]any{"name": "Catalog owner", "workspace": "Catalog tests", "email": randomToken()[:12] + "@example.invalid", "password": randomToken()}, 200), &user)
		csrf = user.CSRF
		return user
	}
	owner := register()
	// Catalog serialization must not block inserts that check the workspace FK.
	holder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var locked string
	if err = holder.QueryRow(ctx, `SELECT id FROM workspaces WHERE id=$1 FOR NO KEY UPDATE`, owner.WorkspaceID).Scan(&locked); err != nil {
		t.Fatal(err)
	}
	fkContext, cancel := context.WithTimeout(ctx, 2*time.Second)
	_, err = pool.Exec(fkContext, `INSERT INTO customers(workspace_id,customer_id) VALUES($1,'fk-lock-probe')`, owner.WorkspaceID)
	cancel()
	_ = holder.Rollback(ctx)
	if err != nil {
		t.Fatal("catalog lock blocked runtime foreign-key check", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO customers(workspace_id,customer_id,email,attributes) VALUES($1,'catalog-customer','private@example.invalid','{"activated":true,"nested":{"amount":42},"secret_value":"PRIVATE_VALUE_DO_NOT_EXPORT"}')`, owner.WorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO customer_events(workspace_id,event_id,customer_id,event_name,occurred_at,properties,payload_hash,status) VALUES($1,'catalog-event','catalog-customer','account_activated',now(),'{"amount":42,"secret":"PRIVATE_PAYLOAD_VALUE"}','catalog-hash','ACCEPTED')`, owner.WorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	data := call("POST", "/data-catalog/discover", map[string]any{}, 200)
	for _, value := range []string{"PRIVATE_VALUE_DO_NOT_EXPORT", "PRIVATE_PAYLOAD_VALUE", "private@example.invalid"} {
		if bytes.Contains(data, []byte(value)) {
			t.Fatal("PII/value leaked")
		}
	}
	if !bytes.Contains(data, []byte("account_activated")) || !bytes.Contains(data, []byte("customer.attributes.nested.amount")) {
		t.Fatal("discovery missed schema")
	}
	call("PUT", "/data-catalog", map[string]any{"entries": []map[string]any{{"kind": "event", "event_name": "account_activated", "label": "Account enabled", "description": "A reviewed definition", "data_type": "event", "aliases": []string{"Activation"}, "enabled": false}}}, 200)
	data = call("POST", "/data-catalog/discover", map[string]any{}, 200)
	if !strings.Contains(string(data), "Account enabled") || !strings.Contains(string(data), "A reviewed definition") {
		t.Fatal("discovery overwrote curation")
	}
	var rows []map[string]any
	json.Unmarshal(data, &rows)
	for _, row := range rows {
		if row["event_name"] == "account_activated" && row["kind"] == "event" && row["enabled"] != false {
			t.Fatal("discovery reenabled disabled event")
		}
	}
	viewer := register()
	other := call("GET", "/data-catalog", nil, 200)
	if bytes.Contains(other, []byte("account_activated")) || bytes.Contains(other, []byte("customer.attributes.activated")) {
		t.Fatal("workspace boundary crossed")
	}
	_, err = pool.Exec(ctx, `UPDATE users SET role='Viewer' WHERE id=$1`, viewer.ID)
	if err != nil {
		t.Fatal(err)
	}
	call("PUT", "/data-catalog", map[string]any{"entries": []map[string]any{{"kind": "event", "event_name": "blocked", "label": "Blocked", "data_type": "event", "aliases": []string{}, "enabled": true}}}, 403)
	call("POST", "/data-catalog/discover", map[string]any{}, 403)
}
