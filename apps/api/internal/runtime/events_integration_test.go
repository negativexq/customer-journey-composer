//go:build integration

package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ofk/customer-journey-composer/apps/api/internal/database"
	"github.com/ofk/customer-journey-composer/apps/api/internal/workflow"
)

func testID() string { b := make([]byte, 12); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func fixture(t *testing.T) (context.Context, *pgxpool.Pool, string, string, string, workflow.Workflow) {
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
	var workspace, user, journey, version string
	if err := pool.QueryRow(ctx, `INSERT INTO workspaces(name,slug) VALUES('Runtime tests',$1) RETURNING id::text`, "test-"+testID()).Scan(&workspace); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO users(workspace_id,email,name,password_hash) VALUES($1,$2,'Test owner','!disabled') RETURNING id::text`, workspace, testID()+"@example.invalid").Scan(&user); err != nil {
		t.Fatal(err)
	}
	w := workflow.Workflow{SchemaVersion: 1, Nodes: []workflow.Node{{ID: "start", Type: "event_trigger", Label: "Cart", Config: workflow.Config{EventName: "cart_updated"}}, {ID: "done", Type: "exit", Label: "Done", Config: workflow.Config{Reason: "completed"}}}, Edges: []workflow.Edge{{ID: "start-done", Source: "start", Target: "done", Port: "started"}}}
	raw, _ := json.Marshal(w)
	if err := pool.QueryRow(ctx, `INSERT INTO journeys(workspace_id,name,draft) VALUES($1,'Test journey',$2) RETURNING id::text`, workspace, raw).Scan(&journey); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO journey_versions(workspace_id,journey_id,version,workflow,published_by) VALUES($1,$2,1,$3,$4) RETURNING id::text`, workspace, journey, raw, user).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE journeys SET current_version_id=$2,entry_event_name='cart_updated',status='Active' WHERE id=$1`, journey, version); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO customers(workspace_id,customer_id,attributes,consents,email,phone) VALUES($1,'cust_test','{"first_name":"Ada"}','{"email":true,"sms":true}','ada@example.invalid','+905551234567')`, workspace); err != nil {
		t.Fatal(err)
	}
	return ctx, pool, workspace, journey, version, w
}
func TestConcurrentEventDeduplicationAndEnrollment(t *testing.T) {
	ctx, pool, workspace, journey, _, _ := fixture(t)
	now := time.Now().UTC()
	event := Event{EventID: testID(), CustomerID: "cust_test", EventName: "cart_updated", OccurredAt: now, Properties: map[string]any{"amount": 2450.0}}
	var wg sync.WaitGroup
	results := make(chan Ingestion, 8)
	errs := make(chan error, 8)
	for range 8 {
		wg.Go(func() { out, err := Ingest(ctx, pool, workspace, event, now); results <- out; errs <- err })
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	accepted, duplicates := 0, 0
	for result := range results {
		if result.Status == "ACCEPTED" {
			accepted++
			if len(result.Executions) != 1 {
				t.Fatal(result)
			}
		}
		if result.Status == "DUPLICATE" {
			duplicates++
		}
	}
	if accepted != 1 || duplicates != 7 {
		t.Fatalf("accepted %d duplicate %d", accepted, duplicates)
	}
	var executions, jobs int
	pool.QueryRow(ctx, `SELECT count(*) FROM journey_executions WHERE workspace_id=$1 AND journey_id=$2`, workspace, journey).Scan(&executions)
	pool.QueryRow(ctx, `SELECT count(*) FROM scheduled_jobs WHERE workspace_id=$1`, workspace).Scan(&jobs)
	if executions != 1 || jobs != 1 {
		t.Fatalf("%d executions, %d jobs", executions, jobs)
	}
	conflict := event
	conflict.Properties = map[string]any{"amount": 100.0}
	_, err := Ingest(ctx, pool, workspace, conflict, now)
	var input *InputError
	if !errors.As(err, &input) || !input.Conflict {
		t.Fatalf("expected ID conflict, got %v", err)
	}
	event.EventID = testID()
	out, err := Ingest(ctx, pool, workspace, event, now)
	if err != nil || len(out.Executions) != 0 {
		t.Fatal("must prevent simultaneous active enrollment", out, err)
	}
	event.EventID = testID()
	event.OccurredAt = now.Add(-48 * time.Hour)
	out, err = Ingest(ctx, pool, workspace, event, now)
	if err != nil || out.Status != "LATE" || len(out.Executions) != 0 {
		t.Fatal(out, err)
	}
	event.EventID = testID()
	event.OccurredAt = now.Add(time.Hour)
	if _, err := Ingest(ctx, pool, workspace, event, now); err == nil {
		t.Fatal("future event must be rejected")
	}
	event.OccurredAt = now
	event.CustomerID = "outside-workspace"
	if _, err := Ingest(ctx, pool, workspace, event, now); err == nil {
		t.Fatal("unknown customer must be rejected")
	}
}
