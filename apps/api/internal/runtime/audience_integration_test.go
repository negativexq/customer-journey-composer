//go:build integration

package runtime

import (
	"errors"
	"testing"
	"time"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

func TestAudienceEntryEnrollsMatchingCustomersOncePerSlot(t *testing.T) {
	ctx, pool, workspace, journey, _, spec := fixture(t)
	spec.Nodes[0] = workflow.Node{ID: "start", Type: "event_trigger", Label: "Gold members", Config: workflow.Config{
		EntryMode: "audience", EventName: workflow.AudienceEvent, AtTime: "09:00",
		Filters: &workflow.Rule{Path: "customer.attributes.tier", Operator: "equals", Value: "gold"},
	}}
	version := publishTest(t, ctx, pool, workspace, journey, spec)
	if _, err := pool.Exec(ctx, `UPDATE journeys SET status='Active' WHERE id=$1`, journey); err != nil {
		t.Fatal(err)
	}
	for id, tier := range map[string]string{"aud_gold_1": "gold", "aud_gold_2": "gold", "aud_silver": "silver"} {
		if _, err := pool.Exec(ctx, `INSERT INTO customers(workspace_id,customer_id,attributes) VALUES($1,$2,jsonb_build_object('tier',$3::text))`, workspace, id, tier); err != nil {
			t.Fatal(err)
		}
	}
	count := func(query string, args ...any) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, query, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	executions := func() int {
		return count(`SELECT count(*) FROM journey_executions WHERE journey_id=$1`, journey)
	}

	// A customer event with the internal name never starts an audience journey.
	now := time.Now().UTC()
	sent, err := Ingest(ctx, pool, workspace, Event{EventID: testID(), CustomerID: "aud_gold_1", EventName: workflow.AudienceEvent, OccurredAt: now}, now)
	if err != nil || len(sent.Executions) != 0 {
		t.Fatal("audience journeys must not enroll on sent events", sent, err)
	}

	// Nothing is due before the first slot after publishing.
	scheduler := &AudienceScheduler{Pool: pool, Now: func() time.Time { return now.Add(-time.Hour) }}
	if _, err := scheduler.Tick(ctx); err != nil || executions() != 0 {
		t.Fatal("no audience run is due yet", executions(), err)
	}
	scheduler.Now = func() time.Time { return now.Add(26 * time.Hour) }
	if _, err := scheduler.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if got := executions(); got != 2 {
		t.Fatal("only matching customers enter", got)
	}
	if count(`SELECT count(*) FROM journey_executions e JOIN customer_events c ON c.workspace_id=e.workspace_id AND c.event_id=e.entry_event_id WHERE e.journey_id=$1 AND c.event_name='audience_entry'`, journey) != 2 {
		t.Fatal("each audience entry is recorded as an audience_entry event")
	}
	// The same slot is claimed once, however often the scheduler ticks.
	if _, err := scheduler.Tick(ctx); err != nil || executions() != 2 {
		t.Fatal("a slot runs once", executions(), err)
	}
	slot, _ := workflow.LatestAudienceSlot(spec.Start().Config, now, now.Add(26*time.Hour))
	if _, err := RunAudience(ctx, pool, workspace, journey, version, spec, slot, "SCHEDULE", now); !errors.Is(err, ErrAudienceRunExists) {
		t.Fatal("a claimed slot cannot run again", err)
	}

	// Customers still inside the journey are not enrolled twice.
	manual, err := RunAudience(ctx, pool, workspace, journey, version, spec, now.Add(27*time.Hour), "MANUAL", now)
	if err != nil || manual.Checked < 3 || manual.Matched != 2 || manual.Enrolled != 0 {
		t.Fatal("active customers keep their execution", manual, err)
	}
	// Scheduled entries started at the scheduler's clock.
	drain(t, ctx, NewWorker(pool, &testClock{now: now.Add(30 * time.Hour)}))
	again, err := RunAudience(ctx, pool, workspace, journey, version, spec, now.Add(28*time.Hour), "MANUAL", now)
	if err != nil || again.Enrolled != 2 || executions() != 4 {
		t.Fatal("finished customers re-enter on the next run", again, executions(), err)
	}

	// After a long stop only the latest missed slot runs.
	scheduler.Now = func() time.Time { return now.Add(10 * 24 * time.Hour) }
	if _, err := scheduler.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if got := count(`SELECT count(*) FROM audience_runs WHERE version_id=$1 AND trigger='SCHEDULE'`, version); got != 2 {
		t.Fatal("missed slots collapse into one run", got)
	}
}
