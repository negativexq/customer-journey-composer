//go:build integration

package delivery

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/analytics"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/database"
	engine "github.com/negativexq/customer-journey-composer/apps/api/internal/runtime"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

type clock struct{ now time.Time }

func (c *clock) Now() time.Time { return c.now }
func randomID() string          { b := make([]byte, 12); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func seeded(t *testing.T, consent bool) (context.Context, *pgxpool.Pool, *clock, string, string) {
	t.Helper()
	ctx := context.Background()
	pool, err := database.Open(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	now := &clock{now: time.Now().UTC().Truncate(time.Microsecond)}
	var workspace, user, journey, version string
	if err := pool.QueryRow(ctx, `INSERT INTO workspaces(name,slug) VALUES('Delivery tests',$1) RETURNING id::text`, "test-"+randomID()).Scan(&workspace); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO users(workspace_id,email,name,password_hash) VALUES($1,$2,'Test owner','!disabled') RETURNING id::text`, workspace, randomID()+"@example.invalid").Scan(&user); err != nil {
		t.Fatal(err)
	}
	spec := workflow.Workflow{SchemaVersion: 1, GlobalExit: &workflow.ExitCriteria{EventName: "purchase_completed"}, Nodes: []workflow.Node{{ID: "start", Type: "event_trigger", Label: "Cart", Config: workflow.Config{EventName: "cart_updated"}}, {ID: "email", Type: "email", Label: "Email", Config: workflow.Config{Subject: "Hello {{ customer.attributes.first_name }}", Text: "Your cart is waiting.", HTML: "<p>Your cart is waiting.</p>"}}, {ID: "sms", Type: "sms", Label: "SMS", Config: workflow.Config{Text: "Your cart is waiting."}}, {ID: "done", Type: "exit", Label: "Complete", Config: workflow.Config{Reason: "completed"}}}, Edges: []workflow.Edge{{ID: "a", Source: "start", Target: "email", Port: "started"}, {ID: "b", Source: "email", Target: "sms", Port: "next"}, {ID: "c", Source: "sms", Target: "done", Port: "next"}}}
	data, _ := json.Marshal(spec)
	if err := pool.QueryRow(ctx, `INSERT INTO journeys(workspace_id,name,draft) VALUES($1,'Messaging journey',$2) RETURNING id::text`, workspace, data).Scan(&journey); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO journey_versions(workspace_id,journey_id,version,workflow,published_by) VALUES($1,$2,1,$3,$4) RETURNING id::text`, workspace, journey, data, user).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE journeys SET current_version_id=$2,entry_event_name='cart_updated',status='Active' WHERE id=$1`, journey, version); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO customers(workspace_id,customer_id,email,phone,attributes,consents) VALUES($1,'test_customer','recipient@example.invalid','+905551234567','{"first_name":"Ada"}',jsonb_build_object('email',$2::bool,'sms',$2::bool))`, workspace, consent); err != nil {
		t.Fatal(err)
	}
	out, err := engine.Ingest(ctx, pool, workspace, engine.Event{EventID: randomID(), CustomerID: "test_customer", EventName: "cart_updated", OccurredAt: now.Now()}, now.Now())
	if err != nil || len(out.Executions) != 1 {
		t.Fatal(out, err)
	}
	worker := engine.NewWorker(pool, now)
	for range 20 {
		jobs, err := worker.Claim(ctx, 100)
		if err != nil {
			t.Fatal(err)
		}
		if len(jobs) == 0 {
			break
		}
		for _, j := range jobs {
			if err := worker.Process(ctx, j); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `UPDATE delivery_jobs SET status='CANCELLED',claim_token=NULL,lease_until=NULL WHERE workspace_id=$1 AND status IN ('SCHEDULED','CLAIMED','DISPATCHING')`, workspace)
		_, _ = pool.Exec(context.Background(), `UPDATE action_outbox SET state='CANCELLED' WHERE workspace_id=$1 AND state='PENDING'`, workspace)
	})
	return ctx, pool, now, workspace, out.Executions[0]
}
func TestDeliveryRetryBackoffAndLogicalIdempotency(t *testing.T) {
	ctx, pool, now, workspace, execution := seeded(t, true)
	sender := New(pool, now)
	email := &Mock{Codes: []int{429, 500, 200}}
	sender.Providers["smtp"] = email
	var group sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		group.Go(func() { _, err := sender.Schedule(ctx); errs <- err })
	}
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	for range 10 {
		job, err := sender.Claim(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if job != nil {
			if err := sender.Dispatch(ctx, *job); err != nil {
				t.Fatal(err)
			}
			if err := sender.Dispatch(ctx, *job); err != nil {
				t.Fatal(err)
			}
		} else {
			now.now = now.now.Add(10 * time.Second)
		}
	}
	var sends, accepted, attempts int
	pool.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE status='ACCEPTED') FROM delivery_jobs WHERE workspace_id=$1 AND execution_id=$2`, workspace, execution).Scan(&sends, &accepted)
	pool.QueryRow(ctx, `SELECT count(*) FROM delivery_attempts WHERE workspace_id=$1`, workspace).Scan(&attempts)
	if sends != 2 || accepted != 2 || attempts != 4 || email.calls != 3 {
		t.Fatalf("sends=%d accepted=%d attempts=%d provider_calls=%d", sends, accepted, attempts, email.calls)
	}
	for {
		n, e := analytics.Drain(ctx, pool)
		if e != nil {
			t.Fatal(e)
		}
		if n == 0 {
			break
		}
	}
	var logical float64
	if err := pool.QueryRow(ctx, `SELECT sum(value) FROM metric_totals WHERE workspace_id=$1 AND journey_id=$2 AND version_id=$2 AND dimension='channel' AND metric='attempted'`, workspace, analytics.ZeroID).Scan(&logical); err != nil || logical != 2 {
		t.Fatal("retries counted as logical sends", logical, err)
	}

}
func TestConsentAndWorkspaceFrequencyReservations(t *testing.T) {
	t.Run("consent", func(t *testing.T) {
		ctx, pool, now, workspace, _ := seeded(t, false)
		sender := New(pool, now)
		mock := &Mock{}
		sender.Providers["smtp"] = mock
		if _, err := sender.Schedule(ctx); err != nil {
			t.Fatal(err)
		}
		job, err := sender.Claim(ctx)
		if err != nil || job != nil {
			t.Fatal("blocked messages must not be claimed", job, err)
		}
		var blocked int
		pool.QueryRow(ctx, `SELECT count(*) FROM delivery_jobs WHERE workspace_id=$1 AND status='BLOCKED' AND decision->>'reason'='consent_denied'`, workspace).Scan(&blocked)
		if blocked != 2 || mock.calls != 0 {
			t.Fatal(blocked, mock.calls)
		}
	})
	t.Run("cap", func(t *testing.T) {
		ctx, pool, now, workspace, _ := seeded(t, true)
		if _, err := pool.Exec(ctx, `INSERT INTO workspace_settings(workspace_id,frequency_limit) VALUES($1,1)`, workspace); err != nil {
			t.Fatal(err)
		}
		sender := New(pool, now)
		if _, err := sender.Schedule(ctx); err != nil {
			t.Fatal(err)
		}
		var reserved, blocked int
		pool.QueryRow(ctx, `SELECT count(*) FILTER(WHERE status='SCHEDULED'),count(*) FILTER(WHERE status='BLOCKED' AND decision->>'reason'='frequency_cap') FROM delivery_jobs WHERE workspace_id=$1`, workspace).Scan(&reserved, &blocked)
		if reserved != 1 || blocked != 1 {
			t.Fatal(reserved, blocked)
		}
	})
}
func TestConsentRevocationAndGlobalExitBeforeDispatch(t *testing.T) {
	for _, exit := range []bool{false, true} {
		t.Run(map[bool]string{false: "revoked", true: "exit"}[exit], func(t *testing.T) {
			ctx, pool, now, workspace, execution := seeded(t, true)
			sender := New(pool, now)
			mock := &Mock{}
			sender.Providers["smtp"] = mock
			sender.Providers["mock_sms"] = mock
			if _, err := sender.Schedule(ctx); err != nil {
				t.Fatal(err)
			}
			job, err := sender.Claim(ctx)
			if err != nil || job == nil {
				t.Fatal(job, err)
			}
			if exit { // Restore a waiting execution to model an exit after messages were planned.
				pool.Exec(ctx, `UPDATE journey_executions SET state='WAITING' WHERE id=$1`, execution)
				now.now = now.now.Add(time.Second)
				if _, err := engine.Ingest(ctx, pool, workspace, engine.Event{EventID: randomID(), CustomerID: "test_customer", EventName: "purchase_completed", OccurredAt: now.Now()}, now.Now()); err != nil {
					t.Fatal(err)
				}
			} else {
				pool.Exec(ctx, `UPDATE customers SET consents='{"email":false,"sms":false}' WHERE workspace_id=$1`, workspace)
			}
			if err := sender.Dispatch(ctx, *job); err != nil {
				t.Fatal(err)
			}
			if mock.calls != 0 {
				t.Fatal("provider called after consent revocation or exit")
			}
		})
	}
}
func TestAmbiguousSMTPLeaseIsNeverAutomaticallyRetried(t *testing.T) {
	ctx, pool, now, workspace, _ := seeded(t, true)
	sender := New(pool, now)
	if _, err := sender.Schedule(ctx); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM delivery_jobs WHERE workspace_id=$1 AND provider='smtp'`, workspace).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE delivery_jobs SET status='DISPATCHING',attempts=1,claim_token=gen_random_uuid(),lease_until=$2 WHERE id=$1`, id, now.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO delivery_attempts(workspace_id,delivery_id,attempt,status,started_at) VALUES($1,$2,1,'STARTED',$3)`, workspace, id, now.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := sender.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	var status string
	pool.QueryRow(ctx, `SELECT status FROM delivery_jobs WHERE id=$1`, id).Scan(&status)
	if status != "UNKNOWN" {
		t.Fatal(status)
	}
}

func TestDeliveryClaimSkipsPausedAndLockedCustomers(t *testing.T) {
	ctx, pool, now, pausedWorkspace, pausedExecution := seeded(t, true)
	_, _, _, lockedWorkspace, lockedExecution := seeded(t, true)
	_, _, _, readyWorkspace, readyExecution := seeded(t, true)
	sender := New(pool, now)
	if _, err := sender.Schedule(ctx); err != nil {
		t.Fatal(err)
	}
	for i, execution := range []string{pausedExecution, lockedExecution, readyExecution} {
		if _, err := pool.Exec(ctx, `UPDATE delivery_jobs SET due_at=$2 WHERE execution_id=$1`, execution, time.Date(2000, 1, 1, 0, 0, i, 0, time.UTC)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `UPDATE journeys SET status='Paused' WHERE workspace_id=$1`, pausedWorkspace); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var customer string
	if err = tx.QueryRow(ctx, `SELECT customer_id FROM customers WHERE workspace_id=$1 FOR UPDATE`, lockedWorkspace).Scan(&customer); err != nil {
		t.Fatal(err)
	}
	claimCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	job, err := sender.Claim(claimCtx)
	if err != nil || job == nil || job.WorkspaceID != readyWorkspace {
		t.Fatalf("eligible delivery behind paused/locked customers was not claimed: %v %v", job, err)
	}
	var changed int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM delivery_jobs WHERE workspace_id=$1 AND status<>'SCHEDULED'`, pausedWorkspace).Scan(&changed); err != nil || changed != 0 {
		t.Fatal("paused deliveries changed", changed, err)
	}
}

// Parallel scheduling loops share pages and customers: every action is decided
// once, and the frequency cap still counts the reservation another loop made.
func TestParallelSchedulingShardsKeepTheCapAndDecideOnce(t *testing.T) {
	ctx, pool, now, workspace, _ := seeded(t, true)
	if _, err := pool.Exec(ctx, `INSERT INTO workspace_settings(workspace_id,frequency_limit) VALUES($1,1)`, workspace); err != nil {
		t.Fatal(err)
	}
	sender := New(pool, now)
	for round := 0; ; round++ {
		var group sync.WaitGroup
		errs := make(chan error, 8)
		for _, shards := range []int{4, 1} {
			for shard := range shards {
				group.Go(func() { _, err := sender.scheduleShard(ctx, shard, shards); errs <- err })
			}
		}
		group.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
		var pending int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM action_outbox WHERE workspace_id=$1 AND state='PENDING'`, workspace).Scan(&pending); err != nil {
			t.Fatal(err)
		}
		if pending == 0 {
			break
		}
		if round == 10 {
			t.Fatalf("%d actions still pending", pending)
		}
	}
	var decided, reserved, blocked int
	pool.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE status='SCHEDULED'),count(*) FILTER(WHERE status='BLOCKED' AND decision->>'reason'='frequency_cap') FROM delivery_jobs WHERE workspace_id=$1`, workspace).Scan(&decided, &reserved, &blocked)
	if decided != 2 || reserved != 1 || blocked != 1 {
		t.Fatalf("decided=%d reserved=%d blocked=%d", decided, reserved, blocked)
	}
}
