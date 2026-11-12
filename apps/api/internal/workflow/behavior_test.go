package workflow

import "testing"

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
}
