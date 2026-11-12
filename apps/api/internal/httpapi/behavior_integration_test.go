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
	"testing"

	composer "github.com/negativexq/customer-journey-composer/apps/api/internal/ai"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/database"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

func TestIndependentBehaviorBlocksApprovalAndPublishAfterManualEdits(t *testing.T) {
	for _, version := range []int{1, 2} {
		t.Run(fmt.Sprintf("contract_v%d", version), func(t *testing.T) {
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
			var identity Identity
			json.Unmarshal(call("POST", "/auth/register", map[string]any{"name": "Checks", "workspace": "Checks", "email": randomToken()[:12] + "@example.invalid", "password": randomToken()}, 200), &identity)
			csrf = identity.CSRF
			call("PUT", "/data-catalog", map[string]any{"entries": []map[string]any{{"kind": "event", "event_name": "registered", "label": "Registered", "data_type": "event", "aliases": []string{}, "enabled": true}}}, 200)
			var j Journey
			json.Unmarshal(call("POST", "/journeys", map[string]any{"name": "Independent checks", "creation_step": "method"}, 200), &j)
			zero, two := 0, 2
			checks := &workflow.BehaviorChecks{Version: version, Confirmed: true, EntryEvent: "registered", EmailCount: &zero, SMSCount: &zero, Outcome: "completed"}
			if version == 2 {
				checks.CaseName = "No follow-up event"
			}
			w := workflow.Workflow{SchemaVersion: 1, BehaviorChecks: checks, Nodes: []workflow.Node{{ID: "start", Type: "event_trigger", Label: "Registered", Config: workflow.Config{EventName: "registered"}}, {ID: "end", Type: "exit", Label: "Complete", Config: workflow.Config{Reason: "completed"}}}, Edges: []workflow.Edge{{ID: "a", Source: "start", Target: "end", Port: "started"}}}
			state := CompositionState{Step: "review", Method: "ai", BehaviorChecks: checks, Proposal: &composer.Proposal{SchemaVersion: composer.SchemaVersion, Workflow: w, Name: j.Name, Warnings: []string{}}}
			var record CompositionRecord
			save := func(want int) {
				t.Helper()
				data := call("PUT", "/journeys/"+j.ID+"/composition", map[string]any{"name": j.Name, "revision": j.Revision, "state": state}, want)
				if want == 200 {
					json.Unmarshal(data, &record)
					j = record.Journey
				}
			}
			state.Proposal.Workflow.BehaviorChecks = nil
			state.BehaviorChecks = nil
			save(200)
			call("POST", "/journeys/"+j.ID+"/composition/approve", map[string]any{"revision": j.Revision}, 422)
			state.Proposal.Workflow.BehaviorChecks = checks
			stricter := *checks
			if version == 1 {
				stricter.SMSCount = &two
			} else {
				stricter.Alternatives = []workflow.BehaviorCase{{Name: "Alternate event", Events: []workflow.BehaviorEvent{{EventName: "ready", AfterSeconds: 1}}, EmailCount: &zero, SMSCount: &two, Outcome: "completed"}}
			}
			state.BehaviorChecks = &stricter
			save(200)
			call("POST", "/journeys/"+j.ID+"/composition/approve", map[string]any{"revision": j.Revision}, 422)
			state.BehaviorChecks = checks
			save(200)
			json.Unmarshal(call("POST", "/journeys/"+j.ID+"/composition/approve", map[string]any{"revision": j.Revision}, 200), &j)
			w.Nodes = append(w.Nodes, workflow.Node{ID: "sms", Type: "sms", Label: "SMS", Config: workflow.Config{Text: "A message"}})
			w.Edges = []workflow.Edge{{ID: "a", Source: "start", Target: "sms", Port: "started"}, {ID: "b", Source: "sms", Target: "end", Port: "next"}}
			json.Unmarshal(call("PUT", "/journeys/"+j.ID+"/draft", map[string]any{"name": j.Name, "revision": j.Revision, "draft": w}, 200), &j)
			var validation Validation
			json.Unmarshal(call("POST", "/journeys/"+j.ID+"/validate", map[string]any{}, 200), &validation)
			if validation.Valid {
				t.Fatal("manual edit bypassed expectations")
			}
			gate := call("GET", "/journeys/"+j.ID+"/publish-check", nil, 200)
			if !bytes.Contains(gate, []byte("expected 0 accepted SMS")) {
				t.Fatal(string(gate))
			}
			call("POST", "/journeys/"+j.ID+"/publish", map[string]any{"revision": j.Revision}, 422)
			var versions int
			if err = pool.QueryRow(ctx, `SELECT count(*) FROM journey_versions WHERE journey_id=$1`, j.ID).Scan(&versions); err != nil || versions != 0 {
				t.Fatal("failed checks produced a published version", versions, err)
			}

		})
	}
}
