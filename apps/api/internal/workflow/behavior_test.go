package workflow

import (
	"strings"
	"testing"
)

func TestScopedContractRejectsContradictoryAndUnboundedClauses(t *testing.T) {
	zero := 0
	fresh := func() *BehaviorChecks {
		return &BehaviorChecks{Version: 2, Confirmed: true, EntryEvent: "entry", CaseName: "Reviewed", Inputs: map[string]any{"customer.attributes.enabled": false}, EmailCount: &zero, SMSCount: &zero, Outcome: "completed"}
	}
	tests := []struct {
		name   string
		mutate func(*BehaviorChecks)
	}{
		{"contradictory outcomes", func(c *BehaviorChecks) {
			one := 1
			c.Events = []BehaviorEvent{}
			c.Alternatives = []BehaviorCase{{Name: "Same scope", EmailCount: &one, SMSCount: &zero, Outcome: "completed"}}
		}},
		{"contradictory condition", func(c *BehaviorChecks) {
			c.Conditions = []Rule{{Path: "customer.attributes.enabled", Operator: "equals", Value: true}}
		}},
		{"unnamed scope", func(c *BehaviorChecks) { c.CaseName = "" }},
		{"unbounded clauses", func(c *BehaviorChecks) { c.Alternatives = make([]BehaviorCase, 8) }},
		{"invalid order", func(c *BehaviorChecks) {
			c.Events = []BehaviorEvent{{EventName: "ready", AfterSeconds: 20}, {EventName: "ready", AfterSeconds: 10}}
		}},
		{"unsafe input", func(c *BehaviorChecks) { c.Inputs["customer.attributes.__proto__.x"] = true }},
		{"unknown consent", func(c *BehaviorChecks) { c.Consents = map[string]bool{"unknown": true} }},
		{"scope version", func(c *BehaviorChecks) { c.Version = 1 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fresh()
			tt.mutate(c)
			if ValidateBehaviorChecks(c) == nil {
				t.Fatal("invalid scoped contract accepted")
			}
		})
	}
	if err := ValidateBehaviorChecks(fresh()); err != nil {
		t.Fatal(err)
	}
	// A clause may state consent for every message channel, push included.
	all := fresh()
	all.Consents = map[string]bool{"email": true, "sms": true, "push": true}
	if err := ValidateBehaviorChecks(all); err != nil {
		t.Fatal(err)
	}
}

// A bound event whose example lacks the entry's identifier is refused with the
// situation's name and the field to set, not a clause number.
func TestBoundEventExampleErrorNamesTheSituationAndField(t *testing.T) {
	zero, one := 0, 1
	c := &BehaviorChecks{Version: 2, Confirmed: true, EntryEvent: "cart_abandoned", CaseName: "No order", Inputs: map[string]any{"event.properties.cart_id": "c-1"}, EmailCount: &one, SMSCount: &zero, Outcome: "completed"}
	c.Alternatives = []BehaviorCase{{Name: "Same cart ordered", EmailCount: &zero, SMSCount: &zero, Outcome: "converted", Events: []BehaviorEvent{{EventName: "order_completed", AfterSeconds: 3600, EntryBinding: &BehaviorBinding{EntryPath: "event.properties.cart_id", EventPath: "event.properties.cart_id", Operator: "equals"}}}}}
	err := ValidateBehaviorChecks(c)
	if err == nil || !strings.Contains(err.Error(), `Situation "Same cart ordered"`) || !strings.Contains(err.Error(), "the same cart_id") {
		t.Fatal(err)
	}
	c.Alternatives[0].Events[0].Properties = map[string]any{"cart_id": "c-1"}
	if err := ValidateBehaviorChecks(c); err != nil {
		t.Fatal(err)
	}
}
