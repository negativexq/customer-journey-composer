package catalog

import (
	"strings"
	"testing"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

func TestBehaviorChecksCountPushNotificationsWhenAsked(t *testing.T) {
	n := func(v int) *int { return &v }
	flow := func(push *int) workflow.Workflow {
		return workflow.Workflow{SchemaVersion: 1, Nodes: []workflow.Node{
			{ID: "start", Type: "event_trigger", Label: "Start", Config: workflow.Config{EventName: "cart_updated"}},
			{ID: "push", Type: "push", Label: "Push", Config: workflow.Config{Subject: "Your cart", Text: "Still waiting"}},
			{ID: "done", Type: "exit", Label: "Done", Config: workflow.Config{Reason: "completed"}},
		}, Edges: []workflow.Edge{{ID: "a", Source: "start", Target: "push", Port: "started"}, {ID: "b", Source: "push", Target: "done", Port: "next"}},
			BehaviorChecks: &workflow.BehaviorChecks{Version: 1, Confirmed: true, EntryEvent: "cart_updated", EmailCount: n(0), SMSCount: n(0), PushCount: push, Outcome: "completed", Inputs: map[string]any{}}}
	}
	if issues := ValidateBehavior(flow(n(1))); len(issues) != 0 {
		t.Fatalf("one expected push: %v", issues)
	}
	// Checks written before push existed leave it unchecked.
	if issues := ValidateBehavior(flow(nil)); len(issues) != 0 {
		t.Fatalf("no push expectation: %v", issues)
	}
	issues := ValidateBehavior(flow(n(0)))
	found := false
	for _, issue := range issues {
		found = found || strings.Contains(issue.Message, "push")
	}
	if !found {
		t.Fatalf("an unexpected push was not reported: %v", issues)
	}
}
