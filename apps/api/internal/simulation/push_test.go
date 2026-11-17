package simulation

import (
	"testing"
	"time"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

func pushJourney() workflow.Workflow {
	return workflow.Workflow{SchemaVersion: 1, Nodes: []workflow.Node{
		{ID: "start", Type: "event_trigger", Label: "Cart updated", Config: workflow.Config{EventName: "cart_updated"}},
		{ID: "push", Type: "push", Label: "Cart push", Config: workflow.Config{Subject: "Your cart", Text: "Hello {{ customer.attributes.first_name }}", URL: "https://example.com/cart"}},
		{ID: "done", Type: "exit", Label: "Complete", Config: workflow.Config{Reason: "completed"}},
	}, Edges: []workflow.Edge{{ID: "a", Source: "start", Target: "push", Port: "started"}, {ID: "b", Source: "push", Target: "done", Port: "next"}}}
}

func TestTheSandboxSendsAPushOnlyWithConsentAndAToken(t *testing.T) {
	run := func(consent bool, token string) State {
		customer := map[string]any{"customer_id": "push_customer", "attributes": map[string]any{"first_name": "Ada"}, "consents": map[string]any{"push": consent}}
		if token != "" {
			customer["push_token"] = token
		}
		return complete(t, pushJourney(), Scenario{Name: "Push", StartTime: time.Date(2026, 11, 10, 10, 0, 0, 0, time.UTC), Customer: customer, Entry: Event{EventID: "e1", EventName: "cart_updated"}, Assertions: []Assertion{{Type: "message_count", Channel: "push", Expected: 1}}})
	}
	sent := run(true, "device-token-1")
	if sent.Result != "PASS" || len(sent.Messages) != 1 || sent.Messages[0].Channel != "push" || sent.Messages[0].Rendered.To != "device-token-1" || sent.Messages[0].Rendered.Text != "Hello Ada" {
		t.Fatalf("%s %+v", sent.Result, sent.Messages)
	}
	if denied := run(false, "device-token-1"); denied.Result == "PASS" || denied.Messages[0].Decision.Reason != "consent_denied" {
		t.Fatalf("without consent: %s %+v", denied.Result, denied.Messages)
	}
	if missing := run(true, ""); missing.Result == "PASS" || missing.Messages[0].Decision.Reason != "invalid_recipient" {
		t.Fatalf("without a token: %s %+v", missing.Result, missing.Messages)
	}
}
