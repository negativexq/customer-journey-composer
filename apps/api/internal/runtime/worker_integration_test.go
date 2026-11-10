//go:build integration

package runtime

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

type testClock struct {
	mu  sync.RWMutex
	now time.Time
}

func (c *testClock) Now() time.Time          { c.mu.RLock(); defer c.mu.RUnlock(); return c.now }
func (c *testClock) Advance(d time.Duration) { c.mu.Lock(); defer c.mu.Unlock(); c.now = c.now.Add(d) }
func publishTest(t *testing.T, ctx context.Context, pool *pgxpool.Pool, workspace, journey string, w workflow.Workflow) string {
	t.Helper()
	if issues := workflow.Validate(w); len(issues) > 0 {
		t.Fatal(issues)
	}
	data, _ := json.Marshal(w)
	var version string
	err := pool.QueryRow(ctx, `INSERT INTO journey_versions(workspace_id,journey_id,version,workflow,published_by) SELECT $1,$2,max(version)+1,$3,(array_agg(published_by))[1] FROM journey_versions WHERE workspace_id=$1 AND journey_id=$2 RETURNING id::text`, workspace, journey, data).Scan(&version)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE journeys SET current_version_id=$2,entry_event_name=$3 WHERE id=$1`, journey, version, w.Start().Config.EventName); err != nil {
		t.Fatal(err)
	}
	return version
}
func drain(t *testing.T, ctx context.Context, w *Worker) {
	t.Helper()
	for range 100 {
		jobs, err := w.Claim(ctx, 32)
		if err != nil {
			t.Fatal(err)
		}
		if len(jobs) == 0 {
			return
		}
		for _, job := range jobs {
			if err := w.Process(ctx, job); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Fatal("worker did not become idle")
}
func entry(t *testing.T, ctx context.Context, pool *pgxpool.Pool, workspace, customer string, now time.Time) string {
	t.Helper()
	out, err := Ingest(ctx, pool, workspace, Event{EventID: testID(), CustomerID: customer, EventName: "cart_updated", OccurredAt: now, Properties: map[string]any{"amount": 2450.0}}, now)
	if err != nil || len(out.Executions) != 1 {
		t.Fatalf("entry: %v %v", out, err)
	}
	return out.Executions[0]
}
func executionState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id string) string {
	t.Helper()
	var state string
	if err := pool.QueryRow(ctx, "SELECT state FROM journey_executions WHERE id=$1", id).Scan(&state); err != nil {
		t.Fatal(err)
	}
	return state
}
func TestDurableWaitClaimRecoveryAndIdempotentSteps(t *testing.T) {
	ctx, pool, workspace, journey, _, spec := fixture(t)
	clock := &testClock{now: time.Now().UTC().Truncate(time.Microsecond)}
	spec.Nodes = append(spec.Nodes, workflow.Node{ID: "delay", Type: "wait", Label: "Wait 2 hours", Config: workflow.Config{DurationSeconds: 7200}}, workflow.Node{ID: "check", Type: "condition", Label: "Cart amount", Config: workflow.Config{Rule: &workflow.Rule{Path: "event.properties.amount", Operator: "greater_than", Value: 1000.0}}}, workflow.Node{ID: "update", Type: "update_attribute", Label: "Count", Config: workflow.Config{Attribute: "visits", Operation: "increment", Value: 1.0}})
	spec.Edges = []workflow.Edge{{ID: "a", Source: "start", Target: "delay", Port: "started"}, {ID: "b", Source: "delay", Target: "check", Port: "completed"}, {ID: "c", Source: "check", Target: "update", Port: "true"}, {ID: "d", Source: "check", Target: "done", Port: "false"}, {ID: "e", Source: "update", Target: "done", Port: "next"}}
	publishTest(t, ctx, pool, workspace, journey, spec)
	id := entry(t, ctx, pool, workspace, "cust_test", clock.Now())
	worker := NewWorker(pool, clock)
	jobs, err := worker.Claim(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	var lost Job
	for _, j := range jobs {
		if j.ExecutionID == id {
			lost = j
		} else {
			if err := worker.Process(ctx, j); err != nil {
				t.Fatal(err)
			}
		}
	}
	if lost.ID == "" {
		t.Fatal("job not claimed")
	}
	clock.Advance(31 * time.Second)
	restarted := NewWorker(pool, clock)
	reclaimed, err := restarted.Claim(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if err := worker.Process(ctx, lost); err != nil {
		t.Fatal(err)
	}
	for _, j := range reclaimed {
		if j.ID == lost.ID && j.Token == lost.Token {
			t.Fatal("claim token was not fenced")
		}
		if err := restarted.Process(ctx, j); err != nil {
			t.Fatal(err)
		}
	}
	drain(t, ctx, restarted)
	if got := executionState(t, ctx, pool, id); got != "WAITING" {
		t.Fatal(got)
	}
	clock.Advance(2*time.Hour - time.Second)
	drain(t, ctx, NewWorker(pool, clock))
	if got := executionState(t, ctx, pool, id); got != "WAITING" {
		t.Fatal("timer resumed too early", got)
	}
	clock.Advance(time.Second)
	drain(t, ctx, NewWorker(pool, clock))
	if got := executionState(t, ctx, pool, id); got != "COMPLETED" {
		t.Fatal(got)
	}
	var visits int
	if err := pool.QueryRow(ctx, `SELECT (attributes->>'visits')::int FROM customers WHERE workspace_id=$1 AND customer_id='cust_test'`, workspace).Scan(&visits); err != nil || visits != 1 {
		t.Fatal(visits, err)
	}
	if err := worker.Process(ctx, lost); err != nil {
		t.Fatal(err)
	}
	var steps int
	pool.QueryRow(ctx, `SELECT count(*) FROM step_executions WHERE execution_id=$1`, id).Scan(&steps)
	if steps != 5 {
		t.Fatalf("want 5 logical steps got %d", steps)
	}
}
func TestGlobalExitCancelsDurableWaitAndPendingActions(t *testing.T) {
	ctx, pool, workspace, journey, _, spec := fixture(t)
	clock := &testClock{now: time.Now().UTC().Truncate(time.Microsecond)}
	spec.GlobalExit = &workflow.ExitCriteria{EventName: "purchase_completed"}
	spec.Nodes = append(spec.Nodes, workflow.Node{ID: "delay", Type: "wait", Label: "Wait", Config: workflow.Config{DurationSeconds: 7200}}, workflow.Node{ID: "mail", Type: "email", Label: "Reminder", Config: workflow.Config{Subject: "Cart", Text: "Reminder"}})
	spec.Edges = []workflow.Edge{{ID: "a", Source: "start", Target: "delay", Port: "started"}, {ID: "b", Source: "delay", Target: "mail", Port: "completed"}, {ID: "c", Source: "mail", Target: "done", Port: "next"}}
	publishTest(t, ctx, pool, workspace, journey, spec)
	id := entry(t, ctx, pool, workspace, "cust_test", clock.Now())
	worker := NewWorker(pool, clock)
	drain(t, ctx, worker)
	clock.Advance(time.Hour)
	_, err := Ingest(ctx, pool, workspace, Event{EventID: testID(), CustomerID: "cust_test", EventName: "purchase_completed", OccurredAt: clock.Now()}, clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	clock.Advance(48 * time.Hour)
	drain(t, ctx, worker)
	if got := executionState(t, ctx, pool, id); got != "EXITED" {
		t.Fatal(got)
	}
	var actions int
	pool.QueryRow(ctx, `SELECT count(*) FROM action_outbox WHERE execution_id=$1`, id).Scan(&actions)
	if actions != 0 {
		t.Fatal("action created after exit")
	}
}
func TestEventTimeoutRaceHasOneWinner(t *testing.T) {
	ctx, pool, workspace, journey, _, spec := fixture(t)
	clock := &testClock{now: time.Now().UTC().Truncate(time.Microsecond)}
	spec.Nodes = append(spec.Nodes, workflow.Node{ID: "await", Type: "wait_event", Label: "Wait for purchase", Config: workflow.Config{EventName: "purchase_completed", TimeoutSeconds: 3600}}, workflow.Node{ID: "matched", Type: "update_attribute", Label: "Matched", Config: workflow.Config{Attribute: "winner", Operation: "set", Value: "matched"}}, workflow.Node{ID: "timeout", Type: "update_attribute", Label: "Timeout", Config: workflow.Config{Attribute: "winner", Operation: "set", Value: "timeout"}})
	spec.Edges = []workflow.Edge{{ID: "a", Source: "start", Target: "await", Port: "started"}, {ID: "b", Source: "await", Target: "matched", Port: "matched"}, {ID: "c", Source: "await", Target: "timeout", Port: "timeout"}, {ID: "d", Source: "matched", Target: "done", Port: "next"}, {ID: "e", Source: "timeout", Target: "done", Port: "next"}}
	publishTest(t, ctx, pool, workspace, journey, spec)
	for iteration := range 12 {
		customer := "race_" + testID()
		if _, err := pool.Exec(ctx, `INSERT INTO customers(workspace_id,customer_id) VALUES($1,$2)`, workspace, customer); err != nil {
			t.Fatal(err)
		}
		id := entry(t, ctx, pool, workspace, customer, clock.Now())
		worker := NewWorker(pool, clock)
		drain(t, ctx, worker)
		clock.Advance(time.Hour)
		jobs, err := worker.Claim(ctx, 100)
		if err != nil {
			t.Fatal(err)
		}
		var timer Job
		for _, j := range jobs {
			if j.ExecutionID == id {
				timer = j
			} else {
				if err := worker.Process(ctx, j); err != nil {
					t.Fatal(err)
				}
			}
		}
		if timer.ID == "" {
			t.Fatal("timer missing")
		}
		var group sync.WaitGroup
		errs := make(chan error, 2)
		group.Go(func() { errs <- worker.Process(ctx, timer) })
		group.Go(func() {
			_, err := Ingest(ctx, pool, workspace, Event{EventID: testID(), CustomerID: customer, EventName: "purchase_completed", OccurredAt: clock.Now()}, clock.Now())
			errs <- err
		})
		group.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatal(iteration, err)
			}
		}
		drain(t, ctx, worker)
		var branches int
		pool.QueryRow(ctx, `SELECT count(*) FROM step_executions WHERE execution_id=$1 AND node_id IN ('matched','timeout')`, id).Scan(&branches)
		if branches != 1 || executionState(t, ctx, pool, id) != "COMPLETED" {
			t.Fatalf("race produced %d branches", branches)
		}
	}
}
func TestVersionIsolationAndPauseSunset(t *testing.T) {
	ctx, pool, workspace, journey, _, spec := fixture(t)
	clock := &testClock{now: time.Now().UTC().Truncate(time.Microsecond)}
	spec.Nodes = append(spec.Nodes, workflow.Node{ID: "delay", Type: "wait", Label: "Wait", Config: workflow.Config{DurationSeconds: 10}})
	spec.Edges = []workflow.Edge{{ID: "a", Source: "start", Target: "delay", Port: "started"}, {ID: "b", Source: "delay", Target: "done", Port: "completed"}}
	firstVersion := publishTest(t, ctx, pool, workspace, journey, spec)
	id := entry(t, ctx, pool, workspace, "cust_test", clock.Now())
	worker := NewWorker(pool, clock)
	drain(t, ctx, worker)
	spec.Nodes[2].Config.DurationSeconds = 20
	secondVersion := publishTest(t, ctx, pool, workspace, journey, spec)
	if firstVersion == secondVersion {
		t.Fatal("snapshot not created")
	}
	pool.Exec(ctx, `UPDATE journeys SET status='Paused' WHERE id=$1`, journey)
	clock.Advance(time.Hour)
	drain(t, ctx, worker)
	if executionState(t, ctx, pool, id) != "WAITING" {
		t.Fatal("paused action advanced")
	}
	pool.Exec(ctx, `UPDATE journeys SET status='Sunset' WHERE id=$1`, journey)
	drain(t, ctx, worker)
	if executionState(t, ctx, pool, id) != "COMPLETED" {
		t.Fatal("sunset did not drain")
	}
	var actual string
	pool.QueryRow(ctx, `SELECT version_id::text FROM journey_executions WHERE id=$1`, id).Scan(&actual)
	if actual != firstVersion {
		t.Fatal("execution switched versions")
	}
	event := Event{EventID: testID(), CustomerID: "cust_test", EventName: "cart_updated", OccurredAt: clock.Now()}
	out, err := Ingest(ctx, pool, workspace, event, clock.Now())
	if err != nil || len(out.Executions) != 0 {
		t.Fatal("sunset accepted entry", out, err)
	}
}

func TestClaimSkipsOlderPausedAndLockedJobsBeforeApplyingLimit(t *testing.T) {
	ctx, pool, pausedWorkspace, pausedJourney, _, _ := fixture(t)
	_, _, lockedWorkspace, _, _, _ := fixture(t)
	_, _, readyWorkspace, _, _, _ := fixture(t)
	now := time.Now().UTC()
	paused := entry(t, ctx, pool, pausedWorkspace, "cust_test", now)
	locked := entry(t, ctx, pool, lockedWorkspace, "cust_test", now)
	ready := entry(t, ctx, pool, readyWorkspace, "cust_test", now)
	t.Cleanup(func() {
		for _, workspace := range []string{pausedWorkspace, lockedWorkspace, readyWorkspace} {
			_, _ = pool.Exec(context.Background(), `UPDATE scheduled_jobs SET state='CANCELLED',claim_token=NULL,lease_until=NULL WHERE workspace_id=$1 AND state IN ('SCHEDULED','CLAIMED')`, workspace)
			_, _ = pool.Exec(context.Background(), `UPDATE journey_executions SET state='CANCELLED',completed_at=now(),updated_at=now() WHERE workspace_id=$1 AND state IN ('RUNNING','WAITING')`, workspace)
		}
	})
	for i, id := range []string{paused, locked, ready} {
		if _, err := pool.Exec(ctx, `UPDATE scheduled_jobs SET due_at=$2 WHERE execution_id=$1`, id, time.Date(2000, 1, 1, 0, 0, i, 0, time.UTC)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `UPDATE journeys SET status='Paused' WHERE id=$1`, pausedJourney); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var id string
	if err = tx.QueryRow(ctx, `SELECT id::text FROM scheduled_jobs WHERE execution_id=$1 FOR UPDATE`, locked).Scan(&id); err != nil {
		t.Fatal(err)
	}
	claimCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	jobs, err := NewWorker(pool, nil).Claim(claimCtx, 1)
	if err != nil || len(jobs) != 1 || jobs[0].ExecutionID != ready {
		t.Fatalf("eligible work behind paused/locked rows was not claimed: %v %v", jobs, err)
	}
	var state string
	if err = pool.QueryRow(ctx, `SELECT state FROM scheduled_jobs WHERE execution_id=$1`, paused).Scan(&state); err != nil || state != "SCHEDULED" {
		t.Fatal("paused work changed", state, err)
	}
}
