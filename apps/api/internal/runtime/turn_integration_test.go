//go:build integration

package runtime

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ofk/customer-journey-composer/apps/api/internal/workflow"
)

func turnCondition(id string) workflow.Node {
	return workflow.Node{ID: id, Type: "condition", Label: id, Config: workflow.Config{Rule: &workflow.Rule{Path: "event.properties.amount", Operator: "exists"}}}
}
func singleClaim(t *testing.T, ctx context.Context, w *Worker, id string) Job {
	t.Helper()
	jobs, err := w.Claim(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, j := range jobs {
		if j.ExecutionID == id {
			return j
		}
	}
	t.Fatal("expected durable continuation")
	return Job{}
}
func turnCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql, id string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, sql, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
func TestShortTurnPreservesNodeHistoryAndActionBoundary(t *testing.T) {
	ctx, pool, ws, journey, _, spec := fixture(t)
	spec.Nodes = append(spec.Nodes, turnCondition("check"), workflow.Node{ID: "sms", Type: "sms", Label: "SMS", Config: workflow.Config{Text: "Hello {{ customer.first_name }}"}})
	spec.Edges = []workflow.Edge{{ID: "a", Source: "start", Target: "check", Port: "started"}, {ID: "b", Source: "check", Target: "sms", Port: "true"}, {ID: "c", Source: "check", Target: "done", Port: "false"}, {ID: "d", Source: "sms", Target: "done", Port: "next"}}
	publishTest(t, ctx, pool, ws, journey, spec)
	w := NewWorker(pool, nil)
	id := entry(t, ctx, pool, ws, "cust_test", time.Now())
	first := singleClaim(t, ctx, w, id)
	job := first
	for range 5 {
		if err := w.Process(ctx, job); err != nil {
			t.Fatal(err)
		}
		if turnCount(t, ctx, pool, `SELECT count(*) FROM action_outbox WHERE execution_id=$1`, id) == 1 {
			break
		}
		job = singleClaim(t, ctx, w, id)
	}
	var current, state string
	if err := pool.QueryRow(ctx, `SELECT current_node_id,state FROM journey_executions WHERE id=$1`, id).Scan(&current, &state); err != nil {
		t.Fatal(err)
	}
	if current != "done" || state != "RUNNING" {
		t.Fatalf("message must commit before following node: %s/%s", current, state)
	}
	if n := turnCount(t, ctx, pool, `SELECT count(*) FROM step_executions WHERE execution_id=$1 AND node_id='done'`, id); n != 0 {
		t.Fatal("action boundary was crossed")
	}
	drain(t, ctx, w)
	if err := w.Process(ctx, first); err != nil {
		t.Fatal(err)
	}
	if n := turnCount(t, ctx, pool, `SELECT count(*) FROM step_executions WHERE execution_id=$1 AND state='SUCCEEDED'`, id); n != 4 {
		t.Fatal("node history missing", n)
	}
	if n := turnCount(t, ctx, pool, `SELECT count(*) FROM action_outbox WHERE execution_id=$1`, id); n != 1 {
		t.Fatal("duplicate intent", n)
	}
}
func TestLongPurePathYieldsDurablyBeforeNodeBudget(t *testing.T) {
	ctx, pool, ws, journey, _, spec := fixture(t)
	previous := "start"
	for i := range 20 {
		node := fmt.Sprintf("check_%02d", i)
		spec.Nodes = append(spec.Nodes, turnCondition(node))
		if i == 0 {
			spec.Edges = []workflow.Edge{{ID: "start-check", Source: previous, Target: node, Port: "started"}}
		} else {
			spec.Edges = append(spec.Edges, workflow.Edge{ID: previous + "t", Source: previous, Target: node, Port: "true"}, workflow.Edge{ID: previous + "f", Source: previous, Target: node, Port: "false"})
		}
		previous = node
	}
	spec.Edges = append(spec.Edges, workflow.Edge{ID: "last-t", Source: previous, Target: "done", Port: "true"}, workflow.Edge{ID: "last-f", Source: previous, Target: "done", Port: "false"})
	publishTest(t, ctx, pool, ws, journey, spec)
	w := NewWorker(pool, nil)
	id := entry(t, ctx, pool, ws, "cust_test", time.Now())
	if err := w.Process(ctx, singleClaim(t, ctx, w, id)); err != nil {
		t.Fatal(err)
	}
	n := turnCount(t, ctx, pool, `SELECT count(*) FROM step_executions WHERE execution_id=$1`, id)
	if n < 1 || n > maxTurnNodes || executionState(t, ctx, pool, id) != "RUNNING" {
		t.Fatalf("unbounded turn: %d steps", n)
	}
	if n := turnCount(t, ctx, pool, `SELECT count(*) FROM scheduled_jobs WHERE execution_id=$1 AND state='SCHEDULED'`, id); n != 1 {
		t.Fatal("missing durable yield", n)
	}
	drain(t, ctx, NewWorker(pool, nil))
	if n := turnCount(t, ctx, pool, `SELECT count(*) FROM step_executions WHERE execution_id=$1 AND state='SUCCEEDED'`, id); n != 22 || executionState(t, ctx, pool, id) != "COMPLETED" {
		t.Fatal("yield/restart lost steps", n)
	}
}
func TestInlineFailureMaterializesRecoverableNodeWithoutRepeatingMutation(t *testing.T) {
	ctx, pool, ws, journey, _, spec := fixture(t)
	spec.Nodes = append(spec.Nodes, workflow.Node{ID: "increment", Type: "update_attribute", Label: "Increment", Config: workflow.Config{Attribute: "visits", Operation: "increment", Value: 1.0}}, turnCondition("check"), workflow.Node{ID: "sms", Type: "sms", Label: "SMS", Config: workflow.Config{Text: "Hello {{ customer.missing }}"}})
	spec.Edges = []workflow.Edge{{ID: "a", Source: "start", Target: "increment", Port: "started"}, {ID: "b", Source: "increment", Target: "check", Port: "next"}, {ID: "c", Source: "check", Target: "sms", Port: "true"}, {ID: "d", Source: "check", Target: "done", Port: "false"}, {ID: "e", Source: "sms", Target: "done", Port: "next"}}
	publishTest(t, ctx, pool, ws, journey, spec)
	w := NewWorker(pool, nil)
	id := entry(t, ctx, pool, ws, "cust_test", time.Now())
	drain(t, ctx, w)
	var node string
	if err := pool.QueryRow(ctx, `SELECT current_node_id FROM journey_executions WHERE id=$1`, id).Scan(&node); err != nil {
		t.Fatal(err)
	}
	if node != "sms" || executionState(t, ctx, pool, id) != "FAILED" {
		t.Fatal("wrong failure node", node)
	}
	if n := turnCount(t, ctx, pool, `SELECT count(*) FROM scheduled_jobs WHERE execution_id=$1 AND node_id='sms' AND state='FAILED'`, id); n != 1 {
		t.Fatal("failure is not recoverable", n)
	}
	if _, err := pool.Exec(ctx, `UPDATE customers SET attributes=attributes||'{"missing":"Ada"}' WHERE workspace_id=$1 AND customer_id='cust_test'`, ws); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Control(ctx, tx, ws, id, "retry", time.Now()); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	drain(t, ctx, NewWorker(pool, nil))
	var visits int
	if err = pool.QueryRow(ctx, `SELECT (attributes->>'visits')::int FROM customers WHERE workspace_id=$1 AND customer_id='cust_test'`, ws).Scan(&visits); err != nil {
		t.Fatal(err)
	}
	if visits != 1 || executionState(t, ctx, pool, id) != "COMPLETED" {
		t.Fatal("mutation replayed", visits)
	}
	if n := turnCount(t, ctx, pool, `SELECT count(*) FROM action_outbox WHERE execution_id=$1`, id); n != 1 {
		t.Fatal("message intent missing or repeated", n)
	}
}
func TestSQLFailureRollsBackWholeTurnAndSameClaimCanRetry(t *testing.T) {
	ctx, pool, ws, journey, _, spec := fixture(t)
	spec.Nodes = append(spec.Nodes, turnCondition("check"), workflow.Node{ID: "fault_sms", Type: "sms", Label: "SMS", Config: workflow.Config{Text: "Hello"}})
	spec.Edges = []workflow.Edge{{ID: "a", Source: "start", Target: "check", Port: "started"}, {ID: "b", Source: "check", Target: "fault_sms", Port: "true"}, {ID: "c", Source: "check", Target: "done", Port: "false"}, {ID: "d", Source: "fault_sms", Target: "done", Port: "next"}}
	publishTest(t, ctx, pool, ws, journey, spec)
	if _, err := pool.Exec(ctx, `CREATE FUNCTION test_turn_fault() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected turn failure'; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `CREATE TRIGGER test_turn_fault BEFORE INSERT ON action_outbox FOR EACH ROW WHEN (NEW.node_id='fault_sms') EXECUTE FUNCTION test_turn_fault()`); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), `DROP TRIGGER IF EXISTS test_turn_fault ON action_outbox`)
		_, _ = pool.Exec(context.Background(), `DROP FUNCTION IF EXISTS test_turn_fault()`)
	}()
	w := NewWorker(pool, nil)
	id := entry(t, ctx, pool, ws, "cust_test", time.Now())
	var failed Job
	for range 5 {
		job := singleClaim(t, ctx, w, id)
		before := turnCount(t, ctx, pool, `SELECT count(*) FROM step_executions WHERE execution_id=$1`, id)
		if err := w.Process(ctx, job); err != nil {
			failed = job
			if after := turnCount(t, ctx, pool, `SELECT count(*) FROM step_executions WHERE execution_id=$1`, id); after != before {
				t.Fatal("partial turn committed")
			}
			break
		}
	}
	if failed.ID == "" {
		t.Fatal("injected SQL failure was not reached")
	}
	if n := turnCount(t, ctx, pool, `SELECT count(*) FROM action_outbox WHERE execution_id=$1`, id); n != 0 {
		t.Fatal("outbox escaped rollback")
	}
	if _, err := pool.Exec(ctx, `DROP TRIGGER test_turn_fault ON action_outbox`); err != nil {
		t.Fatal(err)
	}
	if err := w.Process(ctx, failed); err != nil {
		t.Fatal(err)
	}
	drain(t, ctx, w)
	if n := turnCount(t, ctx, pool, `SELECT count(*) FROM action_outbox WHERE execution_id=$1`, id); n != 1 || executionState(t, ctx, pool, id) != "COMPLETED" {
		t.Fatal("retry duplicated or lost intent", n)
	}
}
