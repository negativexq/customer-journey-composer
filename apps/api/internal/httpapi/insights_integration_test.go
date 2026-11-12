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

	"github.com/negativexq/customer-journey-composer/apps/api/internal/analytics"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/database"
	engine "github.com/negativexq/customer-journey-composer/apps/api/internal/runtime"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

func TestJourneyInsightsReconcileVersionCohortBranchesAndMessages(t *testing.T) {
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
		request, _ := http.NewRequest(method, server.URL+"/api/v1"+path, bytes.NewReader(raw))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-CSRF-Token", csrf)
		response, e := client.Do(request)
		if e != nil {
			t.Fatal(e)
		}
		defer response.Body.Close()
		data, _ := io.ReadAll(response.Body)
		if response.StatusCode != want {
			t.Fatalf("%s: %d %s", path, response.StatusCode, data)
		}
		return data
	}
	var user Identity
	json.Unmarshal(call("POST", "/auth/register", map[string]any{"name": "Insight test", "workspace": "Cohorts", "email": randomToken()[:12] + "@example.invalid", "password": randomToken()}, 200), &user)
	csrf = user.CSRF
	var j Journey
	json.Unmarshal(call("POST", "/journeys", map[string]any{"name": "Cohort journey"}, 200), &j)
	spec := workflow.Workflow{SchemaVersion: 1, Nodes: []workflow.Node{{ID: "start", Type: "event_trigger", Label: "Entered", Config: workflow.Config{EventName: "cart_updated"}}, {ID: "branch", Type: "condition", Label: "Large cart", Config: workflow.Config{Rule: &workflow.Rule{Path: "event.properties.amount", Operator: "greater_than", Value: 50}}}, {ID: "email", Type: "email", Label: "Reminder", Config: workflow.Config{Subject: "Hello", Text: "A reminder"}}, {ID: "done", Type: "exit", Label: "Completed", Config: workflow.Config{Reason: "completed"}}}, Edges: []workflow.Edge{{ID: "a", Source: "start", Target: "branch", Port: "started"}, {ID: "b", Source: "branch", Target: "email", Port: "true"}, {ID: "c", Source: "branch", Target: "done", Port: "false"}, {ID: "d", Source: "email", Target: "done", Port: "next"}}}
	publish := func(number int) string {
		t.Helper()
		raw, _ := json.Marshal(spec)
		var version string
		if e := pool.QueryRow(ctx, `INSERT INTO journey_versions(workspace_id,journey_id,version,workflow,published_by) VALUES($1,$2,$3,$4,$5) RETURNING id::text`, user.WorkspaceID, j.ID, number, raw, user.ID).Scan(&version); e != nil {
			t.Fatal(e)
		}
		if _, e := pool.Exec(ctx, `UPDATE journeys SET status='Active',current_version_id=$2,entry_event_name='cart_updated' WHERE id=$1`, j.ID, version); e != nil {
			t.Fatal(e)
		}
		return version
	}
	project := func() {
		t.Helper()
		for i := 0; i < 100; i++ {
			n, e := analytics.Drain(ctx, pool)
			if e != nil {
				t.Fatal(e)
			}
			var done bool
			if e = pool.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM metric_snapshot_backfill WHERE NOT complete)`).Scan(&done); e != nil {
				t.Fatal(e)
			}
			if n == 0 && done {
				return
			}
		}
		t.Fatal("projection did not drain")
	}
	enroll := func(customer string, now time.Time, amount float64) string {
		t.Helper()
		if _, e := pool.Exec(ctx, `INSERT INTO customers(workspace_id,customer_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, user.WorkspaceID, customer); e != nil {
			t.Fatal(e)
		}
		out, e := engine.Ingest(ctx, pool, user.WorkspaceID, engine.Event{EventID: randomToken()[:20], CustomerID: customer, EventName: "cart_updated", OccurredAt: now, Properties: map[string]any{"amount": amount}}, now)
		if e != nil || len(out.Executions) != 1 {
			t.Fatal(out, e)
		}
		w := engine.NewWorker(pool, nil)
		for i := 0; i < 30; i++ {
			jobs, e := w.Claim(ctx, 100)
			if e != nil {
				t.Fatal(e)
			}
			if len(jobs) == 0 {
				break
			}
			for _, job := range jobs {
				if e = w.Process(ctx, job); e != nil {
					t.Fatal(e)
				}
			}
		}
		return out.Executions[0]
	}
	publish(1)
	enroll("old_customer", time.Now().AddDate(0, 0, -40), 10)
	live := enroll("repeat_customer", time.Now(), 80)
	var outbox, delivery string
	if err = pool.QueryRow(ctx, `SELECT id::text FROM action_outbox WHERE execution_id=$1`, live).Scan(&outbox); err != nil {
		t.Fatal(err)
	}
	err = pool.QueryRow(ctx, `INSERT INTO delivery_jobs(workspace_id,execution_id,outbox_id,node_id,customer_id,channel,provider,idempotent_provider,logical_key,message,decision,status,due_at,created_at) VALUES($1,$2,$3,'email','repeat_customer','email','mock_email',true,'insight-message','{}','{"eligible":false,"reason":"consent_denied"}','BLOCKED',now(),now()+interval '5 days') RETURNING id::text`, user.WorkspaceID, live, outbox).Scan(&delivery)
	if err != nil {
		t.Fatal(err)
	}
	publish(2)
	enroll("repeat_customer", time.Now(), 10)
	project()
	type m struct {
		Dimension string  `json:"dimension"`
		Key       string  `json:"key"`
		Metric    string  `json:"metric"`
		Value     float64 `json:"value"`
	}
	type response struct {
		Metrics  []m  `json:"metrics"`
		Unique   int  `json:"unique_customers"`
		Complete bool `json:"coverage_complete"`
	}
	read := func(version, days string) response {
		var result response
		json.Unmarshal(call("GET", "/journeys/"+j.ID+"/insights?version="+version+"&days="+days, nil, 200), &result)
		return result
	}
	value := func(r response, dimension, key, metric string) float64 {
		for _, m := range r.Metrics {
			if m.Dimension == dimension && m.Key == key && m.Metric == metric {
				return m.Value
			}
		}
		return 0
	}
	one := read("1", "7")
	two := read("2", "7")
	all := read("0", "7")
	lifetime := read("1", "0")
	if !one.Complete || value(one, "execution", "", "entries") != 1 || value(two, "execution", "", "entries") != 1 || value(lifetime, "execution", "", "entries") != 2 || all.Unique != 1 {
		t.Fatal("cohort/version/member mismatch", one, two, lifetime, all)
	}
	if value(one, "edge", "branch:true", "traversed") != 1 || value(one, "node_delivery", "email", "BLOCKED") != 1 || value(one, "node_reason", "email", "consent_denied") != 1 || value(two, "edge", "branch:false", "traversed") != 1 {
		t.Fatal("breakdowns do not reconcile", one, two)
	}
	var rows []map[string]any
	json.Unmarshal(call("GET", "/journeys/"+j.ID+"/executions?version=1&node_id=branch&port=true", nil, 200), &rows)
	if len(rows) != 1 || rows[0]["id"] != live {
		t.Fatal("branch drilldown mismatch", rows)
	}
	json.Unmarshal(call("GET", "/journeys/"+j.ID+"/messages?version=1&node_id=email&status=BLOCKED&reason=consent_denied", nil, 200), &rows)
	if len(rows) != 1 || rows[0]["id"] != delivery {
		t.Fatal("message drilldown mismatch", rows)
	}
	json.Unmarshal(call("GET", "/journeys/"+j.ID+"/messages?version=2", nil, 200), &rows)
	if len(rows) != 0 {
		t.Fatal("version leaked", rows)
	}
	if _, err = pool.Exec(ctx, `UPDATE delivery_jobs SET attempts=2,status='ACCEPTED',decision='{"eligible":true,"reason":"allowed"}' WHERE id=$1`, delivery); err != nil {
		t.Fatal(err)
	}
	project()
	updated := read("1", "7")
	if value(updated, "channel", "email", "scheduled") != 1 || value(updated, "node_delivery", "email", "attempted") != 1 || value(updated, "node_delivery", "email", "attempts") != 2 || value(updated, "node_reason", "email", "consent_denied") != 0 {
		t.Fatal("retry/old reason inflated", updated)
	}
	// A late baseline must never replace a newer transition, and duplicate delivery
	// snapshots must not create another logical send.
	if _, err = pool.Exec(ctx, `INSERT INTO metric_queue(workspace_id,journey_id,version_id,cohort_at,deltas,snapshot,baseline) SELECT workspace_id,(snapshot->>'journey_id')::uuid,(snapshot->>'version_id')::uuid,now(),'[]',snapshot,true FROM metric_snapshot_facts WHERE source_id=$1`, delivery); err != nil {
		t.Fatal(err)
	}
	project()
	if value(read("1", "7"), "channel", "email", "scheduled") != 1 {
		t.Fatal("baseline doubled send")
	}
	call("GET", "/journeys/"+j.ID+"/insights?version=99", nil, 404)
	var other Identity
	json.Unmarshal(call("POST", "/auth/register", map[string]any{"name": "Other", "workspace": "Other", "email": randomToken()[:12] + "@example.invalid", "password": randomToken()}, 200), &other)
	csrf = other.CSRF
	call("GET", "/journeys/"+j.ID+"/insights?version=1", nil, 404)
	json.Unmarshal(call("GET", "/journeys/"+j.ID+"/messages?version=1", nil, 200), &rows)
	if len(rows) != 0 {
		t.Fatal("workspace leaked")
	}
}
