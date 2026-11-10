//go:build integration

package runtime

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestSlowCustomerDoesNotBlockAvailableRuntimeSlots(t *testing.T) {
	ctx, pool, workspace, _, _, _ := fixture(t)
	now := time.Now().UTC()
	if _, err := pool.Exec(ctx, `INSERT INTO customers(workspace_id,customer_id) SELECT $1,'parallel_'||s FROM generate_series(1,40)s`, workspace); err != nil {
		t.Fatal(err)
	}
	locked := entry(t, ctx, pool, workspace, "cust_test", now)
	for i := 1; i <= 40; i++ {
		entry(t, ctx, pool, workspace, fmt.Sprintf("parallel_%d", i), now)
	}
	if _, err := pool.Exec(ctx, `UPDATE scheduled_jobs SET due_at='2000-01-01' WHERE execution_id=$1`, locked); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var customer string
	if err = tx.QueryRow(ctx, `SELECT customer_id FROM customers WHERE workspace_id=$1 AND customer_id='cust_test' FOR UPDATE`, workspace).Scan(&customer); err != nil {
		t.Fatal(err)
	}
	running, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); _ = NewWorker(pool, nil).Run(running) }()
	defer func() {
		cancel()
		_ = tx.Rollback(ctx)
		<-done
		_, _ = pool.Exec(ctx, `UPDATE scheduled_jobs SET state='CANCELLED',claim_token=NULL,lease_until=NULL WHERE workspace_id=$1 AND state IN ('SCHEDULED','CLAIMED')`, workspace)
	}()
	deadline := time.Now().Add(2 * time.Second)
	completed := 0
	for time.Now().Before(deadline) {
		if err = pool.QueryRow(ctx, `SELECT count(*) FROM journey_executions WHERE workspace_id=$1 AND state='COMPLETED'`, workspace).Scan(&completed); err != nil {
			t.Fatal(err)
		}
		if completed >= 10 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("one locked customer blocked available processors: only %d executions completed", completed)
}
