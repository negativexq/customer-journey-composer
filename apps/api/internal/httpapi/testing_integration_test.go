//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/database"
	sim "github.com/negativexq/customer-journey-composer/apps/api/internal/simulation"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestSandboxPersistenceReplayAndProductionIsolation(t *testing.T) {
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
		data, _ := json.Marshal(body)
		req, _ := http.NewRequest(method, server.URL+"/api/v1"+path, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", csrf)
		r, e := client.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer r.Body.Close()
		b, _ := io.ReadAll(r.Body)
		if r.StatusCode != want {
			t.Fatalf("%s: %d %s", path, r.StatusCode, b)
		}
		return b
	}
	var identity Identity
	json.Unmarshal(call("POST", "/auth/register", map[string]any{"name": "Sandbox", "workspace": "Sandbox", "email": randomToken()[:12] + "@example.invalid", "password": randomToken()}, 200), &identity)
	csrf = identity.CSRF
	call("POST", "/customers", map[string]any{"customer_id": "sandbox_source", "email": "original@example.invalid", "attributes": map[string]any{"first_name": "Original"}, "consents": map[string]bool{"email": true}}, 200)
	var journey Journey
	json.Unmarshal(call("POST", "/journeys", map[string]any{"name": "Sandbox cart"}, 200), &journey)
	spec := workflow.Workflow{SchemaVersion: 1, GlobalExit: &workflow.ExitCriteria{EventName: "purchase_completed"}, Nodes: []workflow.Node{{ID: "start", Type: "event_trigger", Label: "Cart", Config: workflow.Config{EventName: "cart_updated"}}, {ID: "wait", Type: "wait", Label: "Wait", Config: workflow.Config{DurationSeconds: 7200}}, {ID: "email", Type: "email", Label: "Reminder", Config: workflow.Config{Subject: "Hello {{ customer.first_name }}", HTML: "<p>Original body {{ customer.first_name }}</p>", Text: "Hi {{ customer.first_name }}"}}, {ID: "done", Type: "exit", Label: "Done", Config: workflow.Config{Reason: "completed"}}}, Edges: []workflow.Edge{{ID: "a", Source: "start", Target: "wait", Port: "started"}, {ID: "b", Source: "wait", Target: "email", Port: "completed"}, {ID: "c", Source: "email", Target: "done", Port: "next"}}}
	call("PUT", "/journeys/"+journey.ID+"/draft", map[string]any{"name": journey.Name, "description": "", "tags": []string{}, "draft": spec, "revision": journey.Revision}, 200)
	scenario := sim.Scenario{Name: "Stored test", StartTime: time.Date(2026, 11, 10, 10, 0, 0, 0, time.UTC), CustomerID: "sandbox_source", Customer: map[string]any{"attributes": map[string]any{"first_name": "Sandbox"}}, Entry: sim.Event{EventID: "cart", EventName: "cart_updated"}, Faults: []int{429, 500, 200}, Assertions: []sim.Assertion{{Type: "state", Expected: "COMPLETED"}, {Type: "message_sent", Channel: "email"}}}
	input := testInput{JourneyID: journey.ID, Scenario: &scenario}
	var saved map[string]any
	json.Unmarshal(call("POST", "/test-scenarios", input, 200), &saved)
	var run TestRun
	json.Unmarshal(call("POST", "/test-runs", testInput{ScenarioID: saved["id"].(string)}, 200), &run)
	advance := func(op string, seconds int64) {
		t.Helper()
		json.Unmarshal(call("POST", "/test-runs/"+run.ID+"/advance", map[string]any{"operation": op, "seconds": seconds, "revision": run.Revision}, 200), &run)
	}
	advance("run", 0)
	if run.State.Status != "WAITING" {
		t.Fatal(run.State.Status)
	}
	call("POST", "/test-runs/"+run.ID+"/advance", map[string]any{"operation": "complete", "revision": 1}, 409)
	advance("complete", 0)
	if run.State.Result != "PASS" || len(run.State.Messages) != 1 || len(run.State.Messages[0].Attempts) != 3 {
		t.Fatal(run.State.Result, run.State.Messages)
	}
	var body string
	var messages int
	pool.QueryRow(ctx, `SELECT count(*),max(message->>'html') FROM test_messages WHERE workspace_id=$1 AND test_run_id=$2`, identity.WorkspaceID, run.ID).Scan(&messages, &body)
	if messages != 1 || body != "<p>Original body Sandbox</p>" || run.State.Messages[0].Rendered.HTML != "" {
		t.Fatal(messages, body, run.State.Messages)
	}
	var live int
	pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM journey_executions WHERE workspace_id=$1)+(SELECT count(*) FROM customer_events WHERE workspace_id=$1)+(SELECT count(*) FROM delivery_jobs WHERE workspace_id=$1)`, identity.WorkspaceID).Scan(&live)
	if live != 0 {
		t.Fatal("sandbox affected live records", live)
	}
	var original string
	pool.QueryRow(ctx, `SELECT attributes->>'first_name' FROM customers WHERE workspace_id=$1 AND customer_id='sandbox_source'`, identity.WorkspaceID).Scan(&original)
	if original != "Original" {
		t.Fatal(original)
	}
	originalRun := run
	json.Unmarshal(call("POST", "/test-runs/"+run.ID+"/replay", nil, 200), &run)
	if run.ID == originalRun.ID || run.ParentID == nil || *run.ParentID != originalRun.ID {
		t.Fatal("replay did not preserve original")
	}
	advance("complete", 0)
	if run.WorkflowHash != originalRun.WorkflowHash || run.State.Result != "PASS" {
		t.Fatal("replay drift")
	}
	// List endpoints project metadata; details hydrate one rendered body and trace.
	var inbox []map[string]any
	json.Unmarshal(call("GET", "/test-inbox?test_run_id="+run.ID+"&channel=email&status=SIMULATED", nil, 200), &inbox)
	if len(inbox) != 1 || inbox[0]["message"] != nil {
		t.Fatal("inbox metadata", inbox)
	}
	var detail map[string]any
	json.Unmarshal(call("GET", "/test-messages/"+inbox[0]["id"].(string), nil, 200), &detail)
	if detail["trace_id"] != run.ID || detail["message"].(map[string]any)["html"] != "<p>Original body Sandbox</p>" {
		t.Fatal(detail)
	}
	call("GET", "/test-messages/"+inbox[0]["id"].(string)+"/previous", nil, 200)
	// Exact deadline purchase wins deterministically; no logical message is produced.
	scenario.Events = []sim.TimedEvent{{AfterSeconds: 7200, Event: sim.Event{EventName: "purchase_completed"}}}
	scenario.Assertions = []sim.Assertion{{Type: "state", Expected: "EXITED"}, {Type: "no_message", Channel: "email"}}
	var preview sim.State
	json.Unmarshal(call("POST", "/preview/journey", testInput{JourneyID: journey.ID, Scenario: &scenario}, 200), &preview)
	if preview.Result != "PASS" || len(preview.Messages) != 0 {
		t.Fatal(preview.Result, preview.Messages)
	}
	var snapshots int
	pool.QueryRow(ctx, `SELECT count(*) FROM sandbox_workflows WHERE workspace_id=$1`, identity.WorkspaceID).Scan(&snapshots)
	if snapshots != 1 {
		t.Fatal("snapshot duplicated", snapshots)
	}
	// Cross-workspace source IDs and scenario/run IDs must not resolve.
	json.Unmarshal(call("POST", "/auth/register", map[string]any{"name": "Other", "workspace": "Other", "email": randomToken()[:12] + "@example.invalid", "password": randomToken()}, 200), &identity)
	csrf = identity.CSRF
	call("GET", "/test-runs/"+run.ID, nil, 404)
	call("GET", "/test-messages/"+inbox[0]["id"].(string), nil, 404)
	call("POST", "/test-runs", testInput{ScenarioID: saved["id"].(string)}, 404)
}
