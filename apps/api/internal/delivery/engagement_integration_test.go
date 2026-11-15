//go:build integration

package delivery

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/analytics"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/database"
	engine "github.com/negativexq/customer-journey-composer/apps/api/internal/runtime"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

// clickJourney sends one email and waits a day for its link to be clicked.
func clickJourney(t *testing.T) (context.Context, *pgxpool.Pool, *clock, string, string, string) {
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
	if err := pool.QueryRow(ctx, `INSERT INTO workspaces(name,slug) VALUES('Engagement tests',$1) RETURNING id::text`, "test-"+randomID()).Scan(&workspace); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO users(workspace_id,email,name,password_hash) VALUES($1,$2,'Test owner','!disabled') RETURNING id::text`, workspace, randomID()+"@example.invalid").Scan(&user); err != nil {
		t.Fatal(err)
	}
	spec := workflow.Workflow{SchemaVersion: 1, Nodes: []workflow.Node{
		{ID: "start", Type: "event_trigger", Label: "Cart", Config: workflow.Config{EventName: "cart_updated"}},
		{ID: "email", Type: "email", Label: "Reminder", Config: workflow.Config{Subject: "Your cart", Text: "Finish at https://shop.example/cart", HTML: `<p><a href="https://shop.example/cart">Finish your order</a></p>`}},
		{ID: "click", Type: "wait_event", Label: "Wait for a click", Config: workflow.Config{EventName: "message_clicked", MessageNode: "email", TimeoutSeconds: 86400}},
		{ID: "clicked", Type: "exit", Label: "Clicked", Config: workflow.Config{Reason: "converted"}},
		{ID: "ignored", Type: "exit", Label: "No click", Config: workflow.Config{Reason: "completed"}},
	}, Edges: []workflow.Edge{
		{ID: "a", Source: "start", Target: "email", Port: "started"},
		{ID: "b", Source: "email", Target: "click", Port: "next"},
		{ID: "c", Source: "click", Target: "clicked", Port: "matched"},
		{ID: "d", Source: "click", Target: "ignored", Port: "timeout"},
	}}
	if issues := workflow.Validate(spec); len(issues) > 0 {
		t.Fatal(issues)
	}
	data, _ := json.Marshal(spec)
	if err := pool.QueryRow(ctx, `INSERT INTO journeys(workspace_id,name,draft) VALUES($1,'Click journey',$2) RETURNING id::text`, workspace, data).Scan(&journey); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO journey_versions(workspace_id,journey_id,version,workflow,published_by) VALUES($1,$2,1,$3,$4) RETURNING id::text`, workspace, journey, data, user).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE journeys SET current_version_id=$2,entry_event_name='cart_updated',status='Active' WHERE id=$1`, journey, version); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO customers(workspace_id,customer_id,email,consents) VALUES($1,'reader','reader@example.invalid','{"email":true,"sms":false}')`, workspace); err != nil {
		t.Fatal(err)
	}
	out, err := engine.Ingest(ctx, pool, workspace, engine.Event{EventID: randomID(), CustomerID: "reader", EventName: "cart_updated", OccurredAt: now.Now()}, now.Now())
	if err != nil || len(out.Executions) != 1 {
		t.Fatal(out, err)
	}
	// Workers claim across workspaces, so nothing may be left for the next test.
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `UPDATE delivery_jobs SET status='CANCELLED',claim_token=NULL,lease_until=NULL WHERE workspace_id=$1 AND status IN ('SCHEDULED','CLAIMED','DISPATCHING')`, workspace)
		_, _ = pool.Exec(context.Background(), `UPDATE action_outbox SET state='CANCELLED' WHERE workspace_id=$1 AND state='PENDING'`, workspace)
		_, _ = pool.Exec(context.Background(), `UPDATE scheduled_jobs SET state='CANCELLED',claim_token=NULL,lease_until=NULL WHERE workspace_id=$1 AND state IN ('SCHEDULED','CLAIMED')`, workspace)
	})
	return ctx, pool, now, workspace, journey, out.Executions[0]
}

func runJourney(t *testing.T, ctx context.Context, pool *pgxpool.Pool, now *clock) {
	t.Helper()
	worker := engine.NewWorker(pool, now)
	for range 20 {
		jobs, err := worker.Claim(ctx, 100)
		if err != nil {
			t.Fatal(err)
		}
		if len(jobs) == 0 {
			return
		}
		for _, j := range jobs {
			if err := worker.Process(ctx, j); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestAClickedLinkMovesTheStepWaitingForThatMessage(t *testing.T) {
	ctx, pool, now, workspace, journey, execution := clickJourney(t)
	runJourney(t, ctx, pool, now)
	sender := New(pool, now)
	sender.Providers["smtp"] = &Mock{}
	if _, err := sender.Schedule(ctx); err != nil {
		t.Fatal(err)
	}
	job, err := sender.Claim(ctx)
	if err != nil || job == nil {
		t.Fatal(job, err)
	}
	// What is stored, and so sent, carries tracked links only.
	if strings.Contains(job.Message.HTML, "shop.example") || strings.Contains(job.Message.Text, "shop.example") || job.Message.Unsubscribe == "" {
		t.Fatalf("message was not tracked: %+v", job.Message)
	}
	if err := sender.Dispatch(ctx, *job); err != nil {
		t.Fatal(err)
	}
	var click, kinds string
	if err := pool.QueryRow(ctx, `SELECT max(code) FILTER(WHERE kind='click' AND url='https://shop.example/cart'),string_agg(kind,',' ORDER BY kind) FROM tracking_codes WHERE delivery_id=$1`, job.ID).Scan(&click, &kinds); err != nil || click == "" || kinds != "click,open,unsubscribe" {
		t.Fatal(click, kinds, err)
	}
	if !strings.Contains(job.Message.HTML, TrackingPath+click) {
		t.Fatal("stored code is not the one in the message")
	}
	// The mock reports delivery; a click then moves the waiting step.
	var delivered string
	if err := pool.QueryRow(ctx, `SELECT source FROM delivery_events WHERE delivery_id=$1 AND kind='delivered'`, job.ID).Scan(&delivered); err != nil || delivered != "simulated" {
		t.Fatal(delivered, err)
	}
	runJourney(t, ctx, pool, now)
	now.now = now.now.Add(time.Minute)
	for range 2 {
		if _, err := RecordEngagement(ctx, pool, workspace, job.ID, "clicked", "tracking", map[string]any{"url": "https://shop.example/cart"}, now.Now(), now.Now()); err != nil {
			t.Fatal(err)
		}
		now.now = now.now.Add(time.Second)
	}
	runJourney(t, ctx, pool, now)
	var state, reason string
	if err := pool.QueryRow(ctx, `SELECT state,COALESCE(exit_reason,'') FROM journey_executions WHERE id=$1`, execution).Scan(&state, &reason); err != nil {
		t.Fatal(err)
	}
	if state != "COMPLETED" && state != "EXITED" || reason != "converted" {
		t.Fatalf("the click did not take the matched path: %s %s", state, reason)
	}
	var events int
	var url string
	if err := pool.QueryRow(ctx, `SELECT count(*),max(properties->>'url') FROM customer_events WHERE workspace_id=$1 AND event_name='message_clicked'`, workspace).Scan(&events, &url); err != nil || events != 1 || url != "https://shop.example/cart" {
		t.Fatal("a repeated click must reach the journey once", events, url, err)
	}
	for {
		n, err := analytics.Drain(ctx, pool)
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			break
		}
	}
	var clicked float64
	if err := pool.QueryRow(ctx, `SELECT COALESCE(sum(value),0) FROM journey_cohort_metrics WHERE workspace_id=$1 AND journey_id=$2 AND dimension='node_delivery' AND dimension_key='email' AND metric='clicked'`, workspace, journey).Scan(&clicked); err != nil || clicked != 1 {
		t.Fatal("insights count a clicked message once", clicked, err)
	}
}

func TestEngagementWithAnotherMessageDoesNotMoveTheStep(t *testing.T) {
	ctx, pool, now, workspace, _, execution := clickJourney(t)
	runJourney(t, ctx, pool, now)
	event := engine.Event{EventID: randomID(), CustomerID: "reader", EventName: "message_clicked", OccurredAt: now.Now(), Properties: map[string]any{"execution_id": execution, "node_id": "another_email"}}
	if _, err := engine.Ingest(ctx, pool, workspace, event, now.Now()); err != nil {
		t.Fatal(err)
	}
	runJourney(t, ctx, pool, now)
	var state string
	if err := pool.QueryRow(ctx, `SELECT state FROM journey_executions WHERE id=$1`, execution).Scan(&state); err != nil || state != "WAITING" {
		t.Fatal("a click on another message moved the step", state, err)
	}
}

func TestUnsubscribingWithdrawsEmailConsentAndTellsTheJourney(t *testing.T) {
	ctx, pool, now, workspace, _, _ := clickJourney(t)
	runJourney(t, ctx, pool, now)
	sender := New(pool, now)
	sender.Providers["smtp"] = &Mock{}
	if _, err := sender.Schedule(ctx); err != nil {
		t.Fatal(err)
	}
	job, err := sender.Claim(ctx)
	if err != nil || job == nil {
		t.Fatal(job, err)
	}
	if err := sender.Dispatch(ctx, *job); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := Unsubscribe(ctx, pool, workspace, job.ID, now.Now()); err != nil {
			t.Fatal(err)
		}
	}
	var email, sms bool
	var events int
	pool.QueryRow(ctx, `SELECT (consents->>'email')::boolean,(consents->>'sms')::boolean FROM customers WHERE workspace_id=$1 AND customer_id='reader'`, workspace).Scan(&email, &sms)
	pool.QueryRow(ctx, `SELECT count(*) FROM customer_events WHERE workspace_id=$1 AND event_name='message_unsubscribed'`, workspace).Scan(&events)
	if email || sms || events != 1 {
		t.Fatal("email consent withdrawn once, SMS untouched", email, sms, events)
	}
}
