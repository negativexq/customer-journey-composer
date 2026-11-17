//go:build integration

package delivery

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/database"
	engine "github.com/negativexq/customer-journey-composer/apps/api/internal/runtime"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

// A published push step goes from the outbox to the mock push provider with the
// customer's device token, its tap link tracked, and is blocked once the token
// changes before dispatch.
func TestAPushStepReachesTheMockProviderWithTheDeviceToken(t *testing.T) {
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
	if err := pool.QueryRow(ctx, `INSERT INTO workspaces(name,slug) VALUES('Push tests',$1) RETURNING id::text`, "test-"+randomID()).Scan(&workspace); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO users(workspace_id,email,name,password_hash) VALUES($1,$2,'Test owner','!disabled') RETURNING id::text`, workspace, randomID()+"@example.invalid").Scan(&user); err != nil {
		t.Fatal(err)
	}
	spec := workflow.Workflow{SchemaVersion: 1, Nodes: []workflow.Node{
		{ID: "start", Type: "event_trigger", Label: "Cart", Config: workflow.Config{EventName: "cart_updated"}},
		{ID: "push", Type: "push", Label: "Push", Config: workflow.Config{Subject: "{{ customer.attributes.first_name }}, your cart", Text: "Still waiting for you.", URL: "https://example.com/cart"}},
		{ID: "done", Type: "exit", Label: "Complete", Config: workflow.Config{Reason: "completed"}},
	}, Edges: []workflow.Edge{{ID: "a", Source: "start", Target: "push", Port: "started"}, {ID: "b", Source: "push", Target: "done", Port: "next"}}}
	if issues := workflow.Validate(spec); len(issues) > 0 {
		t.Fatal(issues)
	}
	data, _ := json.Marshal(spec)
	if err := pool.QueryRow(ctx, `INSERT INTO journeys(workspace_id,name,draft) VALUES($1,'Push journey',$2) RETURNING id::text`, workspace, data).Scan(&journey); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO journey_versions(workspace_id,journey_id,version,workflow,published_by) VALUES($1,$2,1,$3,$4) RETURNING id::text`, workspace, journey, data, user).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE journeys SET current_version_id=$2,entry_event_name='cart_updated',status='Active' WHERE id=$1`, journey, version); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"tapper", "moved"} {
		if _, err := pool.Exec(ctx, `INSERT INTO customers(workspace_id,customer_id,push_token,attributes,consents) VALUES($1,$2,$3,'{"first_name":"Ada"}','{"push":true}')`, workspace, id, "device-"+id); err != nil {
			t.Fatal(err)
		}
		if _, err := engine.Ingest(ctx, pool, workspace, engine.Event{EventID: randomID(), CustomerID: id, EventName: "cart_updated", OccurredAt: now.Now()}, now.Now()); err != nil {
			t.Fatal(err)
		}
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
	sender := New(pool, now)
	provider := &Mock{}
	sender.Providers["mock_push"] = provider
	if _, err := sender.Schedule(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := sender.Schedule(ctx); err != nil {
		t.Fatal(err)
	}
	// The second customer reinstalls the app before the message goes out.
	if _, err := pool.Exec(ctx, `UPDATE customers SET push_token='device-new' WHERE workspace_id=$1 AND customer_id='moved'`, workspace); err != nil {
		t.Fatal(err)
	}
	for range 5 {
		job, err := sender.Claim(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if job == nil {
			break
		}
		if job.Provider != "mock_push" || job.Message.Channel != "push" {
			t.Fatalf("job %+v", job)
		}
		if err := sender.Dispatch(ctx, *job); err != nil {
			t.Fatal(err)
		}
	}
	status := map[string]string{}
	rows, err := pool.Query(ctx, `SELECT customer_id,status,message->>'to',message->>'subject',message->>'url',(SELECT count(*) FROM tracking_codes c WHERE c.delivery_id=d.id AND c.kind='click' AND c.url='https://example.com/cart') FROM delivery_jobs d WHERE workspace_id=$1`, workspace)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var customer, state, to, subject, url string
		var tracked int
		if err := rows.Scan(&customer, &state, &to, &subject, &url, &tracked); err != nil {
			t.Fatal(err)
		}
		status[customer] = state
		if to != "device-"+customer || subject != "Ada, your cart" || tracked != 1 || url == "https://example.com/cart" {
			t.Fatalf("%s: to=%s subject=%q url=%s tracked=%d", customer, to, subject, url, tracked)
		}
	}
	if status["tapper"] != "ACCEPTED" || status["moved"] != "BLOCKED" || provider.calls != 1 {
		t.Fatalf("statuses %v, provider calls %d", status, provider.calls)
	}
}
