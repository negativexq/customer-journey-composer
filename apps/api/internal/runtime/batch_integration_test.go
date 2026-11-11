//go:build integration

package runtime

import (
	"context"
	"testing"
	"time"
)

// Transport batching must retain fresh statement snapshots after the customer
// lock, and must evaluate lease time only after that wait has finished.
func TestBatchedReadsRespectPauseAndLeaseExpiryDuringCustomerLockWait(t *testing.T) {
	for _, mode := range []string{"pause", "lease_expiry"} {
		t.Run(mode, func(t *testing.T) {
			ctx, pool, workspace, journey, _, _ := fixture(t)
			clock := &testClock{now: time.Now().UTC().Truncate(time.Microsecond)}
			worker := NewWorker(pool, clock)
			id := entry(t, ctx, pool, workspace, "cust_test", clock.Now())
			job := singleClaim(t, ctx, worker, id)
			holder, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = holder.Rollback(context.Background()) }()
			var holderPID int
			if err = holder.QueryRow(ctx, `SELECT pg_backend_pid() FROM customers WHERE workspace_id=$1 AND customer_id='cust_test' FOR UPDATE`, workspace).Scan(&holderPID); err != nil {
				t.Fatal(err)
			}
			running, cancel := context.WithTimeout(ctx, 5*time.Second)
			done := make(chan struct{})
			var processErr error
			go func() { defer close(done); processErr = worker.Process(running, job) }()
			defer func() { cancel(); _ = holder.Rollback(context.Background()); <-done }()
			deadline := time.Now().Add(2 * time.Second)
			blocked := false
			for time.Now().Before(deadline) {
				if err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND $1::integer=ANY(pg_blocking_pids(pid)))`, holderPID).Scan(&blocked); err != nil {
					t.Fatal(err)
				}
				if blocked {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if !blocked {
				t.Fatal("worker did not reach customer lock")
			}
			if mode == "pause" {
				if _, err = holder.Exec(ctx, `UPDATE journeys SET status='Paused' WHERE id=$1`, journey); err != nil {
					t.Fatal(err)
				}
			} else {
				clock.Advance(31 * time.Second)
			}
			if err = holder.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			<-done
			if processErr != nil {
				t.Fatal(processErr)
			}
			if n := turnCount(t, ctx, pool, `SELECT count(*) FROM step_executions WHERE execution_id=$1`, id); n != 0 {
				t.Fatal("stale batched read advanced workflow", n)
			}
			if executionState(t, ctx, pool, id) != "RUNNING" {
				t.Fatal("blocked workflow changed state")
			}
			if mode == "pause" {
				if _, err = pool.Exec(ctx, `UPDATE journeys SET status='Active' WHERE id=$1`, journey); err != nil {
					t.Fatal(err)
				}
			}
			fresh := singleClaim(t, ctx, worker, id)
			if fresh.Token == job.Token {
				t.Fatal("resume/recovery reused stale token")
			}
			if err = worker.Process(ctx, fresh); err != nil {
				t.Fatal(err)
			}
			if executionState(t, ctx, pool, id) != "COMPLETED" {
				t.Fatal("fresh claim did not recover")
			}
		})
	}
}
