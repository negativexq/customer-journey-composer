package runtime

import (
	"testing"
	"time"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

func TestPureDecisionsUseProvidedClockAndPorts(t *testing.T) {
	now := time.Date(2026, 11, 10, 10, 0, 0, 0, time.UTC)
	data := map[string]any{"customer": map[string]any{"attributes": map[string]any{"orders": 2.0}}}
	wait := workflow.Node{Type: "wait", Config: workflow.Config{DurationSeconds: 7200}}
	d, err := Decide(wait, data, now, "")
	if err != nil || d.WaitUntil == nil || !d.WaitUntil.Equal(now.Add(2*time.Hour)) {
		t.Fatal(d, err)
	}
	d, err = Decide(wait, data, now, "completed")
	if err != nil || d.WaitUntil != nil || d.Port != "completed" {
		t.Fatal(d, err)
	}
	condition := workflow.Node{Type: "condition", Config: workflow.Config{Rule: &workflow.Rule{Path: "customer.attributes.orders", Operator: "greater_than", Value: 1.0}}}
	d, err = Decide(condition, data, now, "")
	if err != nil || d.Port != "true" || d.Condition == nil || !*d.Condition {
		t.Fatal(d, err)
	}
	waiter := workflow.Node{Type: "wait_event", Config: workflow.Config{EventName: "purchase_completed", TimeoutSeconds: 60}}
	d, err = Decide(waiter, data, now, "")
	if err != nil || d.WaitUntil == nil || d.EventName != "purchase_completed" {
		t.Fatal(d, err)
	}
	for _, port := range []string{"matched", "timeout"} {
		d, err = Decide(waiter, data, now, port)
		if err != nil || d.Port != port || d.WaitUntil != nil {
			t.Fatal(d, err)
		}
	}
	if _, err := Decide(workflow.Node{Type: "arbitrary_code"}, data, now, ""); err == nil {
		t.Fatal("unregistered node must be rejected")
	}
}
