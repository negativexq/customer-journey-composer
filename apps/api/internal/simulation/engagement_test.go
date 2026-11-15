package simulation

import (
	"testing"
	"time"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

// Two messages, each followed by a wait for a click on that message.
func clickFlow() workflow.Workflow {
	return workflow.Workflow{SchemaVersion: 1, Nodes: []workflow.Node{
		{ID: "start", Type: "event_trigger", Label: "Signed up", Config: workflow.Config{EventName: "user_signed_up"}},
		{ID: "welcome", Type: "email", Label: "Welcome", Config: workflow.Config{Subject: "Welcome", Text: "Hello"}},
		{ID: "click", Type: "wait_event", Label: "Clicked welcome?", Config: workflow.Config{EventName: "message_clicked", MessageNode: "welcome", TimeoutSeconds: 86400}},
		{ID: "nudge", Type: "sms", Label: "Nudge", Config: workflow.Config{Text: "Come back"}},
		{ID: "click2", Type: "wait_event", Label: "Clicked nudge?", Config: workflow.Config{EventName: "message_clicked", MessageNode: "nudge", TimeoutSeconds: 86400}},
		{ID: "engaged", Type: "exit", Label: "Engaged", Config: workflow.Config{Reason: "converted"}},
		{ID: "quiet", Type: "exit", Label: "Quiet", Config: workflow.Config{Reason: "completed"}},
	}, Edges: []workflow.Edge{
		{ID: "a", Source: "start", Target: "welcome", Port: "started"},
		{ID: "b", Source: "welcome", Target: "click", Port: "next"},
		{ID: "c", Source: "click", Target: "engaged", Port: "matched"},
		{ID: "d", Source: "click", Target: "nudge", Port: "timeout"},
		{ID: "e", Source: "nudge", Target: "click2", Port: "next"},
		{ID: "f", Source: "click2", Target: "engaged", Port: "matched"},
		{ID: "g", Source: "click2", Target: "quiet", Port: "timeout"},
	}}
}

func clickScenario(events ...TimedEvent) Scenario {
	return Scenario{Name: "Clicks", StartTime: time.Date(2026, 11, 10, 10, 0, 0, 0, time.UTC), Customer: map[string]any{"customer_id": "c", "email": "c@example.invalid", "phone": "+905551234567", "attributes": map[string]any{}, "consents": map[string]any{"email": true, "sms": true}}, Entry: Event{EventID: "signup", EventName: "user_signed_up"}, Events: events}
}

func TestAnUnnamedClickBelongsToTheLatestMessage(t *testing.T) {
	click := func(after int64) TimedEvent {
		return TimedEvent{AfterSeconds: after, Event: Event{EventID: "click", EventName: "message_clicked"}}
	}
	if s := complete(t, clickFlow(), clickScenario(click(3600))); s.ExitReason != "converted" || len(s.Messages) != 1 {
		t.Fatal("a click after the welcome email is a click on it", s.Status, s.ExitReason, len(s.Messages))
	}
	// After the welcome wait times out, the latest message is the SMS nudge.
	if s := complete(t, clickFlow(), clickScenario(click(90000))); s.ExitReason != "converted" || len(s.Messages) != 2 {
		t.Fatal("a later click is a click on the nudge", s.Status, s.ExitReason, len(s.Messages))
	}
	// A click that names another message does not match.
	named := click(3600)
	named.Event.Properties = map[string]any{"node_id": "nudge"}
	if s := complete(t, clickFlow(), clickScenario(named)); s.ExitReason != "completed" {
		t.Fatal("a click named for another message is not this one", s.ExitReason)
	}
}
