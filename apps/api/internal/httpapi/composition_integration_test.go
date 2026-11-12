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
	"testing"

	composer "github.com/negativexq/customer-journey-composer/apps/api/internal/ai"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/database"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

func TestCompositionPersistsReviewAndApprovesAtomically(t *testing.T) {
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
		out, _ := io.ReadAll(res.Body)
		if res.StatusCode != want {
			t.Fatalf("%s %s: %d %s", method, path, res.StatusCode, out)
		}
		return out
	}
	var owner Identity
	json.Unmarshal(call("POST", "/auth/register", map[string]any{"name": "Owner", "workspace": "Creation tests", "email": randomToken()[:12] + "@example.invalid", "password": randomToken()}, 200), &owner)
	csrf = owner.CSRF
	var j Journey
	json.Unmarshal(call("POST", "/journeys", map[string]any{"name": "Resumable plan", "creation_step": "method"}, 200), &j)
	var record CompositionRecord
	json.Unmarshal(call("GET", "/journeys/"+j.ID+"/composition", nil, 200), &record)
	if record.State.Step != "method" {
		t.Fatal(record.State.Step)
	}
	save := func(state CompositionState, revision int, want int) []byte {
		return call("PUT", "/journeys/"+j.ID+"/composition", map[string]any{"name": j.Name, "revision": revision, "state": state}, want)
	}
	state := CompositionState{Step: "brief", Method: "ai", Provider: "openai", Brief: "Welcome users"}
	json.Unmarshal(save(state, j.Revision, 200), &record)
	save(state, j.Revision, 409) // Optimistic fencing, including other tabs.
	call("POST", "/journeys/"+j.ID+"/composition/approve", map[string]any{"revision": record.Journey.Revision}, 422)
	spec := workflow.Workflow{SchemaVersion: 1, Nodes: []workflow.Node{{ID: "start", Type: "event_trigger", Label: "Start", Config: workflow.Config{EventName: "user_registered"}}, {ID: "end", Type: "exit", Label: "Complete", Config: workflow.Config{Reason: "completed"}}}, Edges: []workflow.Edge{{ID: "e", Source: "start", Target: "end", Port: "started"}}}
	state.Step = "review"
	state.Proposal = &composer.Proposal{SchemaVersion: "composer.v2", Name: "Plan", Workflow: spec, Explanation: "Welcome flow", Warnings: []string{}}
	json.Unmarshal(save(state, record.Journey.Revision, 200), &record)
	var persisted CompositionRecord
	json.Unmarshal(call("GET", "/journeys/"+j.ID+"/composition", nil, 200), &persisted)
	if persisted.State.Proposal == nil || persisted.State.Step != "review" || len(persisted.Journey.Draft) == 0 {
		t.Fatal("review was not restored")
	}
	var empty workflow.Workflow
	json.Unmarshal(persisted.Journey.Draft, &empty)
	if len(empty.Nodes) != 0 {
		t.Fatal("proposal applied without approval")
	}
	gate := call("GET", "/journeys/"+j.ID+"/publish-check", nil, 200)
	if !bytes.Contains(gate, []byte("creation_review")) {
		t.Fatal(string(gate))
	}
	state.Editing = true
	json.Unmarshal(save(state, record.Journey.Revision, 200), &record)
	call("POST", "/journeys/"+j.ID+"/composition/approve", map[string]any{"revision": record.Journey.Revision}, 422)
	state.Editing = false
	json.Unmarshal(save(state, record.Journey.Revision, 200), &record)
	before := record.Journey.Revision
	json.Unmarshal(call("POST", "/journeys/"+j.ID+"/composition/approve", map[string]any{"revision": before}, 200), &j)
	var applied workflow.Workflow
	json.Unmarshal(j.Draft, &applied)
	if j.CreationStep != "builder" || j.Status != "Draft" || len(applied.Nodes) != 2 {
		t.Fatal("approval did not atomically open draft")
	}
	call("POST", "/journeys/"+j.ID+"/composition/approve", map[string]any{"revision": before}, 409)
	var versions int
	pool.QueryRow(ctx, `SELECT count(*) FROM journey_versions WHERE workspace_id=$1 AND journey_id=$2`, owner.WorkspaceID, j.ID).Scan(&versions)
	if versions != 0 {
		t.Fatal("approval published")
	}
	pool.Exec(ctx, `UPDATE users SET role='Viewer' WHERE id=$1`, owner.ID)
	save(state, j.Revision, 403)
	var other Identity
	json.Unmarshal(call("POST", "/auth/register", map[string]any{"name": "Other", "workspace": "Other", "email": randomToken()[:12] + "@example.invalid", "password": randomToken()}, 200), &other)
	csrf = other.CSRF
	call("GET", "/journeys/"+j.ID+"/composition", nil, 404)
	save(state, j.Revision, 404)
}
