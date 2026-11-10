//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/ofk/customer-journey-composer/apps/api/internal/database"
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
}
