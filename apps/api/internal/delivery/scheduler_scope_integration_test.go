//go:build integration

package delivery

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// Hold the first insert at a known barrier. Customers beyond the scheduler
// transaction budget must remain available to runtime/global-exit transactions.
func TestSchedulingDoesNotLockCustomersBeyondItsTransactionBudget(t *testing.T) {
	ctx, pool, now, first, _ := seeded(t, true)
	workspaces := []string{first}
	for range max(1, scheduleBatchSize/2) {
		_, _, _, workspace, _ := seeded(t, true)
		workspaces = append(workspaces, workspace)
	}
	holder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Rollback(context.Background()) }()
	if _, err = holder.Exec(ctx, `SELECT pg_advisory_xact_lock(883311)`); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `CREATE FUNCTION hold_scheduler_insert() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_advisory_xact_lock(883311); RETURN NEW; END $$; CREATE TRIGGER hold_scheduler_insert BEFORE INSERT ON delivery_jobs FOR EACH ROW EXECUTE FUNCTION hold_scheduler_insert()`); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), `DROP TRIGGER hold_scheduler_insert ON delivery_jobs; DROP FUNCTION hold_scheduler_insert()`)
	}()
	running, cancel := context.WithTimeout(ctx, 10*time.Second)
	done := make(chan struct{})
	sender := New(pool, now)
	var count int
	var scheduleErr error
	go func() { defer close(done); count, scheduleErr = sender.scheduleOne(running) }()
	defer func() { cancel(); _ = holder.Rollback(context.Background()); <-done }()
	deadline := time.Now().Add(3 * time.Second)
	blocked := false
	for time.Now().Before(deadline) {
		if err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND NOT granted AND objid=883311)`).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !blocked {
		t.Fatal("scheduler did not reach the insert barrier")
	}
	outside, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = outside.Rollback(context.Background()) }()
	if _, err = outside.Exec(ctx, `SELECT 1 FROM customers WHERE workspace_id=$1 AND customer_id='test_customer' FOR UPDATE NOWAIT`, workspaces[len(workspaces)-1]); err != nil {
		t.Fatalf("scheduler held a customer beyond its transaction budget: %v", err)
	}
	_ = outside.Rollback(ctx)
	if err = holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	<-done
	if scheduleErr != nil || count != scheduleBatchSize {
		t.Fatalf("scheduled %d: %v", count, scheduleErr)
	}
	for {
		n, err := sender.Schedule(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			break
		}
	}
	var logical, unique int
	if err = pool.QueryRow(ctx, `SELECT count(*),count(DISTINCT logical_key) FROM delivery_jobs WHERE workspace_id=ANY($1::uuid[])`, workspaces).Scan(&logical, &unique); err != nil {
		t.Fatal(err)
	}
	if logical != 2*len(workspaces) || unique != logical {
		t.Fatalf("lost or duplicated actions: %d logical, %d unique", logical, unique)
	}
}

func TestSchedulerPlanBoundsPendingQueueWork(t *testing.T) {
	ctx, pool, _, workspace, execution := seeded(t, true)
	if _, err := pool.Exec(ctx, `INSERT INTO action_outbox(workspace_id,execution_id,node_id,channel,payload,created_at) SELECT $1,$2,'plan_'||n,'sms','{}',now() FROM generate_series(1,10000) n`, workspace, execution); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `ANALYZE action_outbox; ANALYZE customers; ANALYZE journey_executions`); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var raw []byte
	if err = tx.QueryRow(ctx, "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "+ScheduleQuery, scheduleBatchSize).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var plan []struct{ Plan map[string]any }
	if err = json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	queueWork := float64(0)
	indexCondition := ""
	var walk func(map[string]any)
	walk = func(node map[string]any) {
		if node["Relation Name"] == "action_outbox" {
			rows, _ := node["Actual Rows"].(float64)
			loops, _ := node["Actual Loops"].(float64)
			queueWork += rows * loops
			if condition, ok := node["Index Cond"].(string); ok {
				indexCondition += condition
			}
		}
		if children, ok := node["Plans"].([]any); ok {
			for _, child := range children {
				if child, ok := child.(map[string]any); ok {
					walk(child)
				}
			}
		}
	}
	walk(plan[0].Plan)
	if queueWork == 0 || queueWork > scheduleBatchSize {
		t.Fatalf("pending queue visits exceed the transaction budget: %v", queueWork)
	}
	t.Logf("10,002 pending rows: outbox row visits %.0f for budget %d", queueWork, scheduleBatchSize)
	var position scheduleCursor
	if err = pool.QueryRow(ctx, `SELECT created_at,id::text FROM action_outbox WHERE state='PENDING' ORDER BY created_at,id OFFSET 5000 LIMIT 1`).Scan(&position.createdAt, &position.id); err != nil {
		t.Fatal(err)
	}
	if err = tx.QueryRow(ctx, "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "+ScheduleAfterQuery, scheduleBatchSize, position.createdAt, position.id).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	queueWork = 0
	walk(plan[0].Plan)
	if queueWork != scheduleBatchSize || !strings.Contains(indexCondition, "created_at") || !strings.Contains(indexCondition, "id") {
		t.Fatalf("continuation did not seek the pending index: visits %v, condition %s", queueWork, indexCondition)
	}
}

// A blocked later action must not keep earlier scheduled work uncommitted.
// Retrying after cancellation resumes the remaining outbox without duplicates.
func TestSchedulerCommitsProgressBeforeTheNextAction(t *testing.T) {
	ctx, pool, now, workspace, _ := seeded(t, true)
	var second string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM action_outbox WHERE workspace_id=$1 ORDER BY created_at,id OFFSET 1 LIMIT 1`, workspace).Scan(&second); err != nil {
		t.Fatal(err)
	}
	holder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Rollback(context.Background()) }()
	if _, err = holder.Exec(ctx, `SELECT pg_advisory_xact_lock(883312)`); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `CREATE FUNCTION hold_second_action() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.outbox_id::text=TG_ARGV[0] THEN PERFORM pg_advisory_xact_lock(883312); END IF; RETURN NEW; END $$`); err != nil {
		t.Fatal(err)
	}
	// Only a database-generated UUID enters this isolated trigger fixture.
	if _, err = pool.Exec(ctx, `CREATE TRIGGER hold_second_action BEFORE INSERT ON delivery_jobs FOR EACH ROW EXECUTE FUNCTION hold_second_action('`+second+`')`); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), `DROP TRIGGER hold_second_action ON delivery_jobs; DROP FUNCTION hold_second_action()`)
	}()
	running, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	sender := New(pool, now)
	var count int
	var scheduleErr error
	go func() { defer close(done); count, scheduleErr = sender.Schedule(running) }()
	defer func() { cancel(); _ = holder.Rollback(context.Background()); <-done }()
	deadline := time.Now().Add(3 * time.Second)
	blocked := false
	for time.Now().Before(deadline) {
		if err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND NOT granted AND objid=883312)`).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !blocked {
		t.Fatal("second action did not reach barrier")
	}
	var visible int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM delivery_jobs WHERE workspace_id=$1`, workspace).Scan(&visible); err != nil {
		t.Fatal(err)
	}
	if visible != 1 {
		t.Fatalf("earlier progress was held by later work: %d committed actions", visible)
	}
	cancel()
	<-done
	if scheduleErr == nil || count != 1 {
		t.Fatalf("partial progress not reported: %d, %v", count, scheduleErr)
	}
	if err = holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	// Cancelling a pgx request can close its connection before PostgreSQL finishes
	// releasing its locks. Like Run, retry SKIP LOCKED work after rollback.
	deadline = time.Now().Add(3 * time.Second)
	for visible < 2 && time.Now().Before(deadline) {
		if _, err = sender.Schedule(ctx); err != nil {
			t.Fatal(err)
		}
		if err = pool.QueryRow(ctx, `SELECT count(*) FROM delivery_jobs WHERE workspace_id=$1`, workspace).Scan(&visible); err != nil {
			t.Fatal(err)
		}
		if visible < 2 {
			time.Sleep(10 * time.Millisecond)
		}
	}
	if visible != 2 {
		t.Fatalf("retry duplicated or lost progress: %d", visible)
	}
}

// A cursor may skip an earlier locked customer only within the current bounded
// drain. Releasing that lock must make its actions eligible on the next call.
func TestSchedulerCursorRevisitsEarlierLockedCustomers(t *testing.T) {
	ctx, pool, now, first, _ := seeded(t, true)
	_, _, _, second, _ := seeded(t, true)
	holder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Rollback(context.Background()) }()
	if _, err = holder.Exec(ctx, `SELECT 1 FROM customers WHERE workspace_id=$1 FOR UPDATE`, first); err != nil {
		t.Fatal(err)
	}
	sender := New(pool, now)
	n, err := sender.Schedule(ctx)
	if err != nil || n != 2 {
		t.Fatalf("later customer did not progress: %d, %v", n, err)
	}
	var earlier, later int
	if err = pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE workspace_id=$1),count(*) FILTER (WHERE workspace_id=$2) FROM delivery_jobs`, first, second).Scan(&earlier, &later); err != nil {
		t.Fatal(err)
	}
	if earlier != 0 || later != 2 {
		t.Fatalf("unexpected progress: first %d, second %d", earlier, later)
	}
	if err = holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	n, err = sender.Schedule(ctx)
	if err != nil || n != 2 {
		t.Fatalf("earlier customer lost after cursor reset: %d, %v", n, err)
	}
	if n, err = sender.Schedule(ctx); err != nil || n != 0 {
		t.Fatalf("duplicate progress: %d, %v", n, err)
	}
}

func TestSchedulerCursorUsesTupleOrderAndResetsForBackdatedActions(t *testing.T) {
	ctx, pool, now, workspace, execution := seeded(t, true)
	// Equal timestamps require the UUID tie breaker; the seed's two nodes are
	// both scheduled exactly once across separate committed transactions.
	if _, err := pool.Exec(ctx, `UPDATE action_outbox SET created_at=$2 WHERE workspace_id=$1`, workspace, now.Now()); err != nil {
		t.Fatal(err)
	}
	sender := New(pool, now)
	cursor := &scheduleCursor{}
	for range 2 {
		n, err := sender.scheduleAfter(ctx, cursor)
		if err != nil || n != 1 {
			t.Fatalf("tuple continuation: %d, %v", n, err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO action_outbox(workspace_id,execution_id,node_id,channel,payload,created_at) VALUES($1,$2,'late_backdated','sms','{}',$3)`, workspace, execution, now.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if n, err := sender.scheduleAfter(ctx, cursor); err != nil || n != 0 {
		t.Fatalf("cursor unexpectedly restarted: %d, %v", n, err)
	}
	if n, err := sender.Schedule(ctx); err != nil || n != 1 {
		t.Fatalf("next drain lost backdated action: %d, %v", n, err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(DISTINCT logical_key) FROM delivery_jobs WHERE workspace_id=$1`, workspace).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("lost/duplicate logical actions: %d", count)
	}
}
