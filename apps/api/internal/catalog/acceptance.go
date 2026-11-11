package catalog

import (
	"fmt"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/simulation"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

const RecoveryEURContract = "cart-recovery-eur.v1"

// RecoveryEURAcceptanceCases are the frozen behavior contract, not model-generated tests.
func RecoveryEURAcceptanceCases() []simulation.Scenario {
	cases := RecoveryEURScenarios()
	fresh := func(name string) simulation.Scenario { s := RecoveryEURScenarios()[2]; s.Name = name; return s }
	base := func() []simulation.Assertion {
		return []simulation.Assertion{{Type: "no_duplicate_delivery", Expected: true}}
	}
	below := fresh("Below EUR 50")
	below.Entry.Properties["amount"] = 49.99
	below.Entry.Properties["amount_display"] = "49.99"
	below.Assertions = append(base(), simulation.Assertion{Type: "enrollment_count", Expected: 0}, simulation.Assertion{Type: "no_message"})
	cases = append(cases, below)
	boundary := fresh("Exactly EUR 50")
	boundary.Entry.Properties["amount"] = 50.0
	boundary.Entry.Properties["amount_display"] = "50.00"
	cases = append(cases, boundary)
	none := fresh("No channel consent")
	none.Customer["consents"] = map[string]any{"email": false, "sms": false}
	none.Assertions = append(base(), simulation.Assertion{Type: "no_message"}, simulation.Assertion{Type: "exit_reason", Expected: "condition_not_met"})
	cases = append(cases, none)
	purchased := fresh("Already purchased in profile")
	purchased.Customer["attributes"].(map[string]any)["cart_purchased"] = true
	purchased.Assertions = append(base(), simulation.Assertion{Type: "no_message"}, simulation.Assertion{Type: "exit_reason", Expected: "converted"})
	cases = append(cases, purchased)
	final := fresh("Profile purchase at 72h")
	final.Changes = []simulation.Change{{AfterSeconds: 72 * 3600, Attributes: map[string]any{"cart_purchased": true}}}
	final.Assertions = append(base(), simulation.Assertion{Type: "message_count", Expected: 2}, simulation.Assertion{Type: "exit_reason", Expected: "converted"})
	cases = append(cases, final)
	currency := fresh("Reject non-EUR cart")
	currency.Entry.Properties["currency"] = "USD"
	currency.Assertions = append(base(), simulation.Assertion{Type: "enrollment_count", Expected: 0}, simulation.Assertion{Type: "no_message"})
	cases = append(cases, currency)
	sms := fresh("SMS only consent")
	sms.Customer["consents"] = map[string]any{"email": false, "sms": true}
	sms.Assertions = append(base(), simulation.Assertion{Type: "message_count", Channel: "email", Expected: 0}, simulation.Assertion{Type: "message_count", Channel: "sms", Expected: 1}, simulation.Assertion{Type: "exit_reason", Expected: "completed"})
	cases = append(cases, sms)
	again := RecoveryEURScenarios()[4]
	again.Name = "Re-entry after eight days"
	again.Events[1].AfterSeconds = 8 * 86400
	again.Assertions = append(base(), simulation.Assertion{Type: "enrollment_count", Expected: 2}, simulation.Assertion{Type: "message_count", Expected: 2})
	cases = append(cases, again)
	return cases
}

// ValidateAcceptance is used at proposal review/validation/publication, never in
// the live worker. Arbitrary workflows without a selected contract stay editable.
func ValidateAcceptance(w workflow.Workflow) []workflow.Issue {
	if w.AcceptanceContract == "" {
		return nil
	}
	if w.AcceptanceContract != RecoveryEURContract {
		return []workflow.Issue{{Field: "acceptance_contract", Message: "Unknown behavior contract"}}
	}
	if len(workflow.Validate(w)) > 0 {
		return nil
	} // Graph validator supplies actionable issues first.
	issues := []workflow.Issue{}
	for i, scenario := range RecoveryEURAcceptanceCases() {
		state, spec, input, err := simulation.New(w, scenario)
		if err == nil {
			err = simulation.Advance(spec, input, &state, "complete", 0)
		}
		if err != nil {
			issues = append(issues, workflow.Issue{Field: "acceptance_contract", Message: fmt.Sprintf("%s: simulation could not complete", scenario.Name)})
			continue
		}
		for _, result := range state.Assertions {
			if !result.Pass {
				issues = append(issues, workflow.Issue{Field: "acceptance_contract", Message: fmt.Sprintf("%s: %s expected %v, got %v", scenario.Name, result.Description, result.Assertion.Expected, result.Actual)})
			}
		}
		if i == 7 && !state.Now.Equal(scenario.StartTime) {
			issues = append(issues, workflow.Issue{Field: "acceptance_contract", Message: "No channel consent: exit immediately before any wait"})
		}
	}
	return issues
}
