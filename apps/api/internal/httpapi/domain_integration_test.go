//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/analytics"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/database"
)

func TestWorkspaceSessionsAndJourneyPersistence(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Open(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(New(slog.New(slog.NewTextHandler(io.Discard, nil)), func(context.Context) error { return nil }, pool))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	csrf := ""
	request := func(method, path string, body any, want int) []byte {
		t.Helper()
		var b []byte
		if body != nil {
			b, _ = json.Marshal(body)
		}
		req, _ := http.NewRequest(method, server.URL+"/api/v1"+path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", csrf)
		r, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		out, _ := io.ReadAll(r.Body)
		if r.StatusCode != want {
			t.Fatalf("%s %s: want %d got %d: %s", method, path, want, r.StatusCode, out)
		}
		return out
	}
	request("GET", "/journeys", nil, 401)
	password := randomToken()
	email := randomToken()[:12] + "@example.invalid"
	var identity Identity
	if err := json.Unmarshal(request("POST", "/auth/register", map[string]any{"name": "Test user", "workspace": "Test workspace", "email": email, "password": password}, 200), &identity); err != nil {
		t.Fatal(err)
	}
	csrf = identity.CSRF
	if identity.Role != "Admin" || csrf == "" {
		t.Fatal("incomplete identity")
	}
	request("GET", "/auth/me", nil, 200)
	var journey Journey
	if err := json.Unmarshal(request("POST", "/journeys", map[string]any{"name": "Cart recovery", "description": "Persistent draft", "tags": []string{"cart"}}, 200), &journey); err != nil {
		t.Fatal(err)
	}
	if journey.ID == "" {
		t.Fatal("missing journey id")
	}
	body := map[string]any{"name": journey.Name, "description": journey.Description, "tags": journey.Tags, "revision": journey.Revision, "draft": map[string]any{"schema_version": 1, "nodes": []any{}, "edges": []any{}}}
	request("PUT", "/journeys/"+journey.ID+"/draft", body, 200)
	request("PUT", "/journeys/"+journey.ID+"/draft", body, 409)
	csrf = ""
	request("POST", "/journeys", map[string]any{"name": "No CSRF"}, 403)
	csrf = identity.CSRF
	_, err = pool.Exec(ctx, "UPDATE users SET role='Viewer' WHERE id=$1", identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	request("POST", "/journeys", map[string]any{"name": "Forbidden"}, 403)
	request("GET", "/journeys/"+journey.ID, nil, 200)
	request("POST", "/auth/logout", nil, 200)
	request("GET", "/auth/me", nil, 401)
	request("POST", "/auth/login", map[string]any{"email": email, "password": randomToken()}, 401)
	json.Unmarshal(request("POST", "/auth/login", map[string]any{"email": email, "password": password}, 200), &identity)
	csrf = identity.CSRF
	var restored Journey
	json.Unmarshal(request("GET", "/journeys/"+journey.ID, nil, 200), &restored)
	if restored.Revision != 2 {
		t.Fatal("draft was not persisted across sessions")
	}
	request("POST", "/auth/logout", nil, 200)
	json.Unmarshal(request("POST", "/auth/register", map[string]any{"name": "Other", "workspace": "Other workspace", "email": randomToken()[:12] + "@example.invalid", "password": randomToken()}, 200), &identity)
	csrf = identity.CSRF
	request("GET", "/journeys/"+journey.ID, nil, 404)
	request("DELETE", "/journeys/"+journey.ID, nil, 404)
	var isolated []Journey
	json.Unmarshal(request("GET", "/journeys", nil, 200), &isolated)
	if len(isolated) != 0 {
		t.Fatal("workspace isolation violated")
	}
	// Concurrent writes against the same revision must admit exactly one writer.
	json.Unmarshal(request("POST", "/journeys", map[string]any{"name": "Concurrent"}, 200), &journey)
	body["revision"] = journey.Revision
	data, _ := json.Marshal(body)
	codes := make(chan int, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() {
			req, _ := http.NewRequest("PUT", server.URL+"/api/v1/journeys/"+journey.ID+"/draft", bytes.NewReader(data))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-CSRF-Token", csrf)
			r, err := client.Do(req)
			if err != nil {
				codes <- 0
				return
			}
			defer r.Body.Close()
			codes <- r.StatusCode
		})
	}
	wg.Wait()
	close(codes)
	counts := map[int]int{}
	for c := range codes {
		counts[c]++
	}
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatalf("concurrent writers: %v", counts)
	}
	for {
		n, e := analytics.Drain(ctx, pool)
		if e != nil {
			t.Fatal(e)
		}
		if n == 0 {
			break
		}
	}
	var overview struct {
		Metrics []struct {
			Dimension string  `json:"dimension"`
			Metric    string  `json:"metric"`
			Value     float64 `json:"value"`
		} `json:"metrics"`
	}
	json.Unmarshal(request("GET", "/overview", nil, 200), &overview)
	found := false
	for _, m := range overview.Metrics {
		if m.Dimension == "journey" && m.Metric == "total" {
			found = m.Value == 1
		}
	}
	if !found {
		t.Fatal("workspace overview not scoped", overview)
	}
	request("GET", "/analytics", nil, 200)
	request("GET", "/analytics/comparison", nil, 200)
	request("GET", "/overview/activity", nil, 200)
	request("GET", "/overview/entries-24h", nil, 200)
	request("GET", "/journeys/"+journey.ID+"/stats", nil, 200)

}

func TestWorkflowValidationAndImmutablePublishing(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Open(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(New(slog.New(slog.NewTextHandler(io.Discard, nil)), func(context.Context) error { return nil }, pool))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	csrf := ""
	call := func(method, path string, body any, status int) []byte {
		t.Helper()
		data, _ := json.Marshal(body)
		req, _ := http.NewRequest(method, server.URL+"/api/v1"+path, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", csrf)
		r, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		b, _ := io.ReadAll(r.Body)
		if r.StatusCode != status {
			t.Fatalf("%s: %d %s", path, r.StatusCode, b)
		}
		return b
	}
	var user Identity
	json.Unmarshal(call("POST", "/auth/register", map[string]any{"email": randomToken()[:12] + "@example.invalid", "password": randomToken(), "name": "Publisher", "workspace": "Version tests"}, 200), &user)
	csrf = user.CSRF
	var j Journey
	json.Unmarshal(call("POST", "/journeys", map[string]any{"name": "Versioned journey"}, 200), &j)
	call("POST", "/journeys/"+j.ID+"/publish", map[string]int{"revision": 1}, 422)
	w := map[string]any{"schema_version": 1, "nodes": []any{map[string]any{"id": "start", "type": "event_trigger", "label": "Cart", "position": map[string]int{"x": 0, "y": 0}, "config": map[string]any{"event_name": "cart_updated"}}, map[string]any{"id": "done", "type": "exit", "label": "Done", "position": map[string]int{"x": 0, "y": 200}, "config": map[string]any{"reason": "completed"}}}, "edges": []any{map[string]any{"id": "first", "source": "start", "target": "done", "port": "started"}}}
	call("PUT", "/journeys/"+j.ID+"/draft", map[string]any{"name": j.Name, "draft": w, "revision": 1}, 200)
	var validation Validation
	json.Unmarshal(call("POST", "/journeys/"+j.ID+"/validate", nil, 200), &validation)
	if !validation.Valid {
		t.Fatal(validation.Issues)
	}
	call("POST", "/journeys/"+j.ID+"/publish", map[string]int{"revision": 2}, 422)
	var gateRun TestRun
	json.Unmarshal(call("POST", "/test-runs", map[string]any{"journey_id": j.ID, "scenario": map[string]any{"name": "Publish check", "entry_event": map[string]any{"event_name": "cart_updated"}}}, 200), &gateRun)
	call("POST", "/test-runs/"+gateRun.ID+"/advance", map[string]any{"operation": "complete", "revision": gateRun.Revision}, 200)
	var version Version
	json.Unmarshal(call("POST", "/journeys/"+j.ID+"/publish", map[string]int{"revision": 2}, 200), &version)
	if version.Version != 1 || version.ID == "" {
		t.Fatalf("invalid published version: %+v", version)
	}
	for _, query := range []string{"UPDATE journey_versions SET version=2 WHERE id=$1", "DELETE FROM journey_versions WHERE id=$1"} {
		if _, err := pool.Exec(ctx, query, version.ID); err == nil {
			t.Fatal("published version mutation must fail")
		}
	}
	call("POST", "/journeys/"+j.ID+"/publish", map[string]int{"revision": 2}, 409)
	call("POST", "/journeys/"+j.ID+"/publish", map[string]int{"revision": 3}, 200)
	var versions []Version
	json.Unmarshal(call("GET", "/journeys/"+j.ID+"/versions", nil, 200), &versions)
	if len(versions) != 2 || versions[0].Version != 2 || versions[1].Version != 1 {
		t.Fatalf("version history: %+v", versions)
	}
	var current Journey
	json.Unmarshal(call("GET", "/journeys/"+j.ID, nil, 200), &current)
	for _, action := range []string{"pause", "resume", "sunset", "pause", "resume", "complete", "archive"} {
		json.Unmarshal(call("POST", "/journeys/"+j.ID+"/"+action, map[string]int{"revision": current.Revision}, 200), &current)
		if action == "pause" {
			call("POST", "/journeys/"+j.ID+"/pause", map[string]int{"revision": 1}, 200)
		}
	}
	if current.Status != "Archived" {
		t.Fatal(current.Status)
	}
	var audits []map[string]any
	json.Unmarshal(call("GET", "/audit-logs", nil, 200), &audits)
	if len(audits) < 9 {
		t.Fatal("missing atomic lifecycle audit", len(audits))
	}
	call("PUT", "/users/"+user.ID+"/role", map[string]string{"role": "Viewer"}, 409)
	call("POST", "/users", map[string]string{"name": "Viewer", "email": randomToken()[:12] + "@example.invalid", "password": randomToken(), "role": "Viewer"}, 200)
	call("GET", "/users", nil, 200)

}

func TestScopedEventAPIKeys(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Open(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(New(slog.New(slog.NewTextHandler(io.Discard, nil)), func(context.Context) error { return nil }, pool))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	csrf := ""
	call := func(method, path string, body any, want int) []byte {
		t.Helper()
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest(method, server.URL+"/api/v1"+path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", csrf)
		r, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		out, _ := io.ReadAll(r.Body)
		if r.StatusCode != want {
			t.Fatalf("%s: %d %s", path, r.StatusCode, out)
		}
		return out
	}
	var p Identity
	json.Unmarshal(call("POST", "/auth/register", map[string]any{"name": "Owner", "workspace": "Key tests", "email": randomToken()[:12] + "@example.invalid", "password": randomToken()}, 200), &p)
	csrf = p.CSRF
	call("POST", "/customers", map[string]any{"customer_id": "key_customer", "attributes": map[string]any{"first_name": "Ada"}, "consents": map[string]bool{"email": false}}, 200)
	var key map[string]string
	json.Unmarshal(call("POST", "/settings/api-keys", map[string]any{"name": "Integration key"}, 200), &key)
	bearer := func(method, path string, body any, want int) {
		t.Helper()
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest(method, server.URL+"/api/v1"+path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+key["token"])
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		if r.StatusCode != want {
			b, _ := io.ReadAll(r.Body)
			t.Fatalf("bearer %s: %d %s", path, r.StatusCode, b)
		}
	}
	event := map[string]any{"event_id": randomToken(), "customer_id": "key_customer", "event_name": "cart_updated", "occurred_at": time.Now().UTC().Format(time.RFC3339Nano), "properties": map[string]any{"amount": 42}}
	bearer("POST", "/events", event, 200)
	bearer("POST", "/events", event, 200)
	bearer("GET", "/customers", nil, 401)
	listed := string(call("GET", "/settings/api-keys", nil, 200))
	if strings.Contains(listed, key["token"]) {
		t.Fatal("API key plaintext leaked in listing")
	}
	call("DELETE", "/settings/api-keys/"+key["id"], nil, 200)
	bearer("POST", "/events", event, 401)
}

func TestMessagePreviewIsolationAndAllowlistedTestSend(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Open(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(New(slog.New(slog.NewTextHandler(io.Discard, nil)), func(context.Context) error { return nil }, pool))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	csrf := ""
	key := ""
	call := func(method, path string, body any, want int) []byte {
		t.Helper()
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest(method, server.URL+"/api/v1"+path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", csrf)
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		r, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		out, _ := io.ReadAll(r.Body)
		if r.StatusCode != want {
			t.Fatalf("%s: %d %s", path, r.StatusCode, out)
		}
		return out
	}
	var p Identity
	json.Unmarshal(call("POST", "/auth/register", map[string]any{"name": "Message editor", "workspace": "Message tests", "email": randomToken()[:12] + "@example.invalid", "password": randomToken()}, 200), &p)
	csrf = p.CSRF
	call("POST", "/customers", map[string]any{"customer_id": "preview_customer", "email": "original@example.invalid", "attributes": map[string]any{"first_name": "Original"}, "consents": map[string]bool{"email": false}}, 200)
	preview := map[string]any{"channel": "email", "config": map[string]any{"subject": "Hello {{ customer.first_name }}", "html": "<p>Hello {{ customer.first_name }}</p><script>alert(1)</script>", "text": "Hello {{ customer.first_name }}"}, "customer_id": "preview_customer", "customer": map[string]any{"attributes": map[string]any{"first_name": "Preview only"}}}
	var rendered map[string]any
	json.Unmarshal(call("POST", "/messages/preview", preview, 200), &rendered)
	if rendered["subject"] != "Hello Preview only" || strings.Contains(fmt.Sprint(rendered["html"]), "<script") {
		t.Fatal(rendered)
	}
	var original string
	pool.QueryRow(ctx, `SELECT attributes->>'first_name' FROM customers WHERE workspace_id=$1 AND customer_id='preview_customer'`, p.WorkspaceID).Scan(&original)
	if original != "Original" {
		t.Fatal("preview mutated live customer")
	}
	key = randomToken()
	body := map[string]any{"preview": preview, "recipient": "test@example.invalid"}
	call("POST", "/messages/test-send", body, 403)
	call("PUT", "/settings/messaging", map[string]any{"frequency_limit": 3, "frequency_window_seconds": 86400, "test_allowlist": []string{"test@example.invalid"}}, 200)
	var first, second map[string]any
	json.Unmarshal(call("POST", "/messages/test-send", body, 200), &first)
	json.Unmarshal(call("POST", "/messages/test-send", body, 200), &second)
	if first["id"] != second["id"] || first["mode"] != "TEST" {
		t.Fatal(first, second)
	}
	var inbox []map[string]any
	json.Unmarshal(call("GET", "/test-inbox?status=TEST_PENDING", nil, 200), &inbox)
	if len(inbox) != 1 || inbox[0]["mode"] != "REAL_TEST" || inbox[0]["delivery_id"] != first["id"] {
		t.Fatal(inbox)
	}
	preview["config"] = map[string]any{"subject": "Changed", "text": "Changed"}
	call("POST", "/messages/test-send", body, 409)
	var live int
	pool.QueryRow(ctx, `SELECT count(*) FROM delivery_jobs WHERE workspace_id=$1 AND mode='LIVE'`, p.WorkspaceID).Scan(&live)
	if live != 0 {
		t.Fatal("test send contaminated live deliveries")
	}
	_, err = pool.Exec(ctx, `UPDATE users SET role='Viewer' WHERE id=$1`, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	call("POST", "/messages/test-send", body, 403)
	call("POST", "/messages/preview", preview, 200)
}

func TestDisabledAIHasNoDraftSideEffects(t *testing.T) {
	t.Setenv("AI_PROVIDER", "disabled")
	ctx := context.Background()
	pool, err := database.Open(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(New(slog.New(slog.NewTextHandler(io.Discard, nil)), func(context.Context) error { return nil }, pool))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	csrf := ""
	call := func(path string, body any, want int) []byte {
		t.Helper()
		data, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", server.URL+"/api/v1"+path, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", csrf)
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		out, _ := io.ReadAll(response.Body)
		if response.StatusCode != want {
			t.Fatalf("%s: %d %s", path, response.StatusCode, out)
		}
		return out
	}
	var identity Identity
	json.Unmarshal(call("/auth/register", map[string]any{"name": "AI review", "workspace": "AI scope", "email": randomToken()[:12] + "@example.invalid", "password": randomToken()}, 200), &identity)
	csrf = identity.CSRF
	var journey Journey
	json.Unmarshal(call("/journeys", map[string]any{"name": "Human draft"}, 200), &journey)
	call("/ai/journeys/generate", map[string]any{"provider": "claude", "action": "generate", "prompt": "Create a cart journey", "name": journey.Name, "workflow": map[string]any{"schema_version": 1, "nodes": []any{}, "edges": []any{}}}, 503)
	var revision int
	var status string
	if err := pool.QueryRow(ctx, `SELECT revision,status FROM journeys WHERE id=$1`, journey.ID).Scan(&revision, &status); err != nil {
		t.Fatal(err)
	}
	if revision != 1 || status != "Draft" {
		t.Fatal("AI changed or published a draft without approval")
	}
}
