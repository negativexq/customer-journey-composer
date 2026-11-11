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
	"time"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/database"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/simulation"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

func TestPublicationRequiresLatestAcceptanceResults(t *testing.T) {
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
	var identity Identity
	json.Unmarshal(call("POST", "/auth/register", map[string]any{"name": "Owner", "workspace": "Acceptance tests", "email": randomToken()[:12] + "@example.invalid", "password": randomToken()}, 200), &identity)
	csrf = identity.CSRF
	spec := workflow.Workflow{SchemaVersion: 1, Nodes: []workflow.Node{{ID: "start", Type: "event_trigger", Label: "Start", Config: workflow.Config{EventName: "cart_updated"}}, {ID: "done", Type: "exit", Label: "Complete", Config: workflow.Config{Reason: "completed"}}}, Edges: []workflow.Edge{{ID: "edge", Source: "start", Target: "done", Port: "started"}}}
	var j Journey
	json.Unmarshal(call("POST", "/journeys", map[string]any{"name": "Acceptance gate"}, 200), &j)
	json.Unmarshal(call("PUT", "/journeys/"+j.ID+"/draft", map[string]any{"name": j.Name, "draft": spec, "revision": j.Revision}, 200), &j)
	scene := func(name, expected string) simulation.Scenario {
		return simulation.Scenario{Name: name, StartTime: time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC), Entry: simulation.Event{EventName: "cart_updated"}, Assertions: []simulation.Assertion{{Type: "state", Expected: expected}}}
	}
	run := func(input testInput) TestRun {
		t.Helper()
		var r TestRun
		json.Unmarshal(call("POST", "/test-runs", input, 200), &r)
		json.Unmarshal(call("POST", "/test-runs/"+r.ID+"/advance", map[string]any{"operation": "complete", "revision": r.Revision}, 200), &r)
		return r
	}
	check := func(want bool, code string) {
		t.Helper()
		var g struct {
			Ready  bool        `json:"ready"`
			Issues []gateIssue `json:"issues"`
		}
		json.Unmarshal(call("GET", "/journeys/"+j.ID+"/publish-check", nil, 200), &g)
		if g.Ready != want {
			t.Fatalf("ready=%v want=%v: %+v", g.Ready, want, g.Issues)
		}
		if code != "" {
			for _, i := range g.Issues {
				if i.Code == code {
					return
				}
			}
			t.Fatalf("missing gate %s: %+v", code, g.Issues)
		}
	}
	check(false, "test_required")
	good := scene("good", "COMPLETED")
	r := run(testInput{JourneyID: j.ID, Scenario: &good})
	if r.State.Result != "PASS" {
		t.Fatal(r.State.Result)
	}
	check(true, "")
	// An old failure must be superseded by the same case's new result, not hidden by an unrelated PASS.
	if _, err = pool.Exec(ctx, `UPDATE test_runs SET result='FAIL' WHERE workspace_id=$1 AND id=$2`, identity.WorkspaceID, r.ID); err != nil {
		t.Fatal(err)
	}
	check(false, "test_failed")
	call("POST", "/journeys/"+j.ID+"/publish", map[string]any{"revision": j.Revision}, 422)
	var replay TestRun
	json.Unmarshal(call("POST", "/test-runs/"+r.ID+"/replay", nil, 200), &replay)
	check(false, "test_incomplete")
	call("POST", "/test-runs/"+replay.ID+"/advance", map[string]any{"operation": "complete", "revision": replay.Revision}, 200)
	check(true, "")
	bad := scene("exploration", "EXITED")
	run(testInput{JourneyID: j.ID, Scenario: &bad, Exploratory: true})
	check(true, "")
	// Saved required scenarios block publication until tested against the current workflow.
	var saved map[string]any
	json.Unmarshal(call("POST", "/test-scenarios", testInput{JourneyID: j.ID, Scenario: &good}, 200), &saved)
	sid := saved["id"].(string)
	check(false, "test_required")
	// Overriding a saved scenario cannot fulfill its immutable requirement.
	run(testInput{ScenarioID: sid, Scenario: &good})
	check(false, "test_required")
	run(testInput{ScenarioID: sid})
	check(true, "")
	spec.Nodes[1].Label = "New workflow snapshot"
	json.Unmarshal(call("PUT", "/journeys/"+j.ID+"/draft", map[string]any{"name": j.Name, "draft": spec, "revision": j.Revision}, 200), &j)
	check(false, "test_required")
	run(testInput{ScenarioID: sid, UseCurrentDraft: true})
	check(true, "")
	// Optional saved cases are explicit and auditable, not silently ignored failures.
	json.Unmarshal(call("POST", "/test-scenarios", testInput{JourneyID: j.ID, Scenario: &bad}, 200), &saved)
	optional := saved["id"].(string)
	check(false, "test_required")
	run(testInput{ScenarioID: optional})
	check(false, "test_failed")
	call("PATCH", "/test-scenarios/"+optional+"/requirement", map[string]bool{"required": false}, 200)
	check(true, "")
	var audits int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE workspace_id=$1 AND action='test_scenario.requirement'`, identity.WorkspaceID).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("missing requirement audit: %d %v", audits, err)
	}
}
