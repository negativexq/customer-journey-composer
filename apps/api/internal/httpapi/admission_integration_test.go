//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/database"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"
)

func TestBlockedIngestionLeavesConnectionsForInteractiveReads(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Open(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	blocker, err := database.Open(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Close()
	server := httptest.NewServer(New(slog.New(slog.NewTextHandler(io.Discard, nil)), func(context.Context) error { return nil }, pool))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	raw, _ := json.Marshal(map[string]string{"name": "Admission editor", "workspace": "Admission fixture", "email": randomToken() + "@example.invalid", "password": randomToken()})
	response, err := client.Post(server.URL+"/api/v1/auth/register", "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	var identity Identity
	if err = json.NewDecoder(response.Body).Decode(&identity); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 || identity.WorkspaceID == "" {
		t.Fatal("fixture registration failed")
	}
	if _, err = pool.Exec(ctx, `INSERT INTO customers(workspace_id,customer_id) SELECT $1,'blocked_'||s FROM generate_series(1,20)s`, identity.WorkspaceID); err != nil {
		t.Fatal(err)
	}
	token := "cjc_" + randomToken()
	if _, err = pool.Exec(ctx, `INSERT INTO api_keys(workspace_id,created_by,name,token_hash,prefix) VALUES($1,$2,'Fixture',$3,$4)`, identity.WorkspaceID, identity.ID, digest(token), token[:12]); err != nil {
		t.Fatal(err)
	}
	tx, err := blocker.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := tx.Query(ctx, `SELECT customer_id FROM customers WHERE workspace_id=$1 FOR UPDATE`, identity.WorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
	}
	rows.Close()
	ingestCtx, cancel := context.WithCancel(ctx)
	var callers sync.WaitGroup
	defer func() { cancel(); _ = tx.Rollback(ctx); callers.Wait() }()
	for i := 1; i <= 20; i++ {
		callers.Go(func() {
			body, _ := json.Marshal(map[string]any{"event_id": randomToken(), "customer_id": fmt.Sprintf("blocked_%d", i), "event_name": "cart_updated", "occurred_at": time.Now().UTC(), "properties": map[string]any{}})
			r, _ := http.NewRequestWithContext(ingestCtx, "POST", server.URL+"/api/v1/events", bytes.NewReader(body))
			r.Header.Set("Authorization", "Bearer "+token)
			r.Header.Set("Content-Type", "application/json")
			if res, e := client.Do(r); e == nil {
				_, _ = io.Copy(io.Discard, res.Body)
				res.Body.Close()
			}
		})
	}
	until := time.Now().Add(3 * time.Second)
	for pool.Stat().AcquiredConns() < 16 && time.Now().Before(until) {
		time.Sleep(10 * time.Millisecond)
	}
	if pool.Stat().AcquiredConns() < 16 {
		t.Fatal("ingestion fixture did not occupy expected slots")
	}
	readCtx, stop := context.WithTimeout(ctx, 750*time.Millisecond)
	defer stop()
	request, _ := http.NewRequestWithContext(readCtx, "GET", server.URL+"/api/v1/overview", nil)
	result, err := client.Do(request)
	if err != nil {
		t.Fatal("interactive read starved behind ingestion", err)
	}
	defer result.Body.Close()
	if result.StatusCode != 200 {
		t.Fatal("interactive read lost database capacity", result.StatusCode)
	}
}
