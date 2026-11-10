//go:build integration

package runtime

import (
	"context"
	"github.com/ofk/customer-journey-composer/apps/api/internal/workflow"
	"testing"
	"time"
)

func TestCancelFencesClaimAndFailedJobRecoversWithoutDuplicate(t *testing.T) {
	ctx, pool, workspace, journey, _, spec := fixture(t)
	clock := &testClock{now: time.Now().UTC()}
	spec.Nodes = append(spec.Nodes, workflow.Node{ID: "increment", Type: "update_attribute", Label: "Increment", Config: workflow.Config{Attribute: "count", Operation: "increment", Value: 1.0}})
	spec.Edges = []workflow.Edge{{ID: "a", Source: "start", Target: "increment", Port: "started"}, {ID: "b", Source: "increment", Target: "done", Port: "next"}}
	publishTest(t, ctx, pool, workspace, journey, spec)
	id := entry(t, ctx, pool, workspace, "cust_test", clock.Now())
	worker := NewWorker(pool, clock)
	jobs, err := worker.Claim(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Control(ctx, tx, workspace, id, "cancel", clock.Now()); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	for _, job := range jobs {
		if err = worker.Process(ctx, job); err != nil {
			t.Fatal(err)
		}
	}
	if executionState(t, ctx, pool, id) != "CANCELLED" {
		t.Fatal("stale claim advanced cancelled execution")
	}
	if _, err = pool.Exec(ctx, `UPDATE customers SET attributes='{"count":"invalid"}' WHERE workspace_id=$1 AND customer_id='cust_test'`, workspace); err != nil {
		t.Fatal(err)
	}
	failed := entry(t, ctx, pool, workspace, "cust_test", clock.Now())
	drain(t, ctx, worker)
	if executionState(t, ctx, pool, failed) != "FAILED" {
		t.Fatal("expected failed attribute step")
	}
	if _, err = pool.Exec(ctx, `UPDATE customers SET attributes='{"count":0}' WHERE workspace_id=$1 AND customer_id='cust_test'`, workspace); err != nil {
		t.Fatal(err)
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Control(ctx, tx, workspace, failed, "retry", clock.Now()); err != nil {
		_ = tx.Rollback(context.Background())
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	drain(t, ctx, worker)
	if executionState(t, ctx, pool, failed) != "COMPLETED" {
		t.Fatal("recovery did not complete")
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT (attributes->>'count')::int FROM customers WHERE workspace_id=$1 AND customer_id='cust_test'`, workspace).Scan(&count); err != nil || count != 1 {
		t.Fatal("attribute duplicated", count, err)
	}
}
