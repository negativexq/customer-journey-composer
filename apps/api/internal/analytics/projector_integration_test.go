//go:build integration

package analytics

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"github.com/ofk/customer-journey-composer/apps/api/internal/database"
	engine "github.com/ofk/customer-journey-composer/apps/api/internal/runtime"
	"github.com/ofk/customer-journey-composer/apps/api/internal/workflow"
	"os"
	"sync"
	"testing"
	"time"
)

func TestAtomicProjectionCountsAndIsolation(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Open(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	bytes := make([]byte, 12)
	rand.Read(bytes)
	suffix := hex.EncodeToString(bytes)
	var workspace, user, journey, version string
	if err = pool.QueryRow(ctx, `INSERT INTO workspaces(name,slug) VALUES('Metric test',$1) RETURNING id::text`, suffix).Scan(&workspace); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO users(workspace_id,email,name,password_hash) VALUES($1,$2,'Metrics','!disabled') RETURNING id::text`, workspace, suffix+"@example.invalid").Scan(&user); err != nil {
		t.Fatal(err)
	}
	spec := workflow.Workflow{SchemaVersion: 1, Nodes: []workflow.Node{{ID: "start", Type: "event_trigger", Label: "Start", Config: workflow.Config{EventName: "cart_updated"}}, {ID: "done", Type: "exit", Label: "Done", Config: workflow.Config{Reason: "completed"}}}, Edges: []workflow.Edge{{ID: "a", Source: "start", Target: "done", Port: "started"}}}
	data, _ := json.Marshal(spec)
	if err = pool.QueryRow(ctx, `INSERT INTO journeys(workspace_id,name,draft) VALUES($1,'Metrics',$2) RETURNING id::text`, workspace, data).Scan(&journey); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO journey_versions(workspace_id,journey_id,version,workflow,published_by) VALUES($1,$2,1,$3,$4) RETURNING id::text`, workspace, journey, data, user).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE journeys SET status='Active',entry_event_name='cart_updated',current_version_id=$2 WHERE id=$1`, journey, version); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO customers(workspace_id,customer_id) VALUES($1,'metrics_customer')`, workspace); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for i, id := range []string{"first", "second"} {
		out, err := engine.Ingest(ctx, pool, workspace, engine.Event{EventID: id, CustomerID: "metrics_customer", EventName: "cart_updated", OccurredAt: now}, now)
		if err != nil || len(out.Executions) != 1 {
			t.Fatal(out, err)
		}
		state, reason := "COMPLETED", "completed"
		if i == 1 {
			state, reason = "EXITED", "converted"
		}
		if _, err = pool.Exec(ctx, `UPDATE journey_executions SET state=$2,exit_reason=$3,completed_at=started_at+interval '2 seconds' WHERE id=$1`, out.Executions[0], state, reason); err != nil {
			t.Fatal(err)
		}
	}
	// Two replicas must not double-project or lose deltas; a later drain is a no-op.
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Go(func() { _, e := Drain(ctx, pool); errs <- e })
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	for {
		n, e := Drain(ctx, pool)
		if e != nil {
			t.Fatal(e)
		}
		if n == 0 {
			break
		}
	}
	get := func(jid, dimension, key, metric string) float64 {
		t.Helper()
		var v float64
		err := pool.QueryRow(ctx, `SELECT COALESCE(sum(value),0) FROM metric_totals WHERE workspace_id=$1 AND journey_id=$2 AND version_id=$3 AND dimension=$4 AND dimension_key=$5 AND metric=$6`, workspace, jid, ZeroID, dimension, key, metric).Scan(&v)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	for _, jid := range []string{journey, ZeroID} {
		for metric, want := range map[string]float64{"entries": 2, "unique_customers": 1, "COMPLETED": 1, "EXITED": 1, "RUNNING": 0, "converted": 1, "duration_ms": 4000, "finished_count": 2} {
			if got := get(jid, "execution", "", metric); got != want {
				t.Fatalf("%s %s=%v want %v", jid, metric, got, want)
			}
		}
	}
	if get(ZeroID, "journey", "", "total") != 1 || get(ZeroID, "journey", "", "Active") != 1 || get(ZeroID, "journey", "", "Draft") != 0 {
		t.Fatal("journey counters")
	}
	var daily float64
	if err = pool.QueryRow(ctx, `SELECT sum(value) FROM metric_hourly WHERE workspace_id=$1 AND journey_id=$2 AND version_id=$3 AND metric='entries'`, workspace, journey, ZeroID).Scan(&daily); err != nil || daily != 2 {
		t.Fatal(daily, err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO delivery_jobs(workspace_id,mode,channel,provider,idempotent_provider,logical_key,message,decision,status,due_at,created_at) VALUES($1,'TEST','sms','mock_sms',true,$2,'{}','{}','ACCEPTED',now(),now())`, workspace, suffix); err != nil {
		t.Fatal(err)
	}
	if _, err = Drain(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if get(ZeroID, "channel", "sms", "ACCEPTED") != 0 {
		t.Fatal("test send counted as live")
	}
	var bad int64
	if err = pool.QueryRow(ctx, `INSERT INTO metric_queue(workspace_id,journey_id,version_id,cohort_at,deltas) VALUES($1,$2,$3,now(),'[{"dimension":"execution","key":"","metric":"rollback_probe","value":"invalid"}]') RETURNING id`, workspace, journey, version).Scan(&bad); err != nil {
		t.Fatal(err)
	}
	if _, err = Drain(ctx, pool); err == nil {
		t.Fatal("malformed projection unexpectedly committed")
	}
	var retained bool
	pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM metric_queue WHERE id=$1)`, bad).Scan(&retained)
	if !retained || get(ZeroID, "execution", "", "rollback_probe") != 0 {
		t.Fatal("failed transaction lost queue or committed summary")
	}
	if _, err = pool.Exec(ctx, `UPDATE metric_queue SET deltas='[{"dimension":"execution","key":"","metric":"rollback_probe","value":1}]' WHERE id=$1`, bad); err != nil {
		t.Fatal(err)
	}
	if _, err = Drain(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if get(ZeroID, "execution", "", "rollback_probe") != 1 {
		t.Fatal("recovery lost delta")
	}

}
