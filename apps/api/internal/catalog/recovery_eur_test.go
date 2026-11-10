package catalog

import (
	"fmt"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/simulation"
	"strings"
	"testing"
)

func runRecovery(t *testing.T, scenario simulation.Scenario) simulation.State {
	t.Helper()
	state, spec, input, err := simulation.New(recoveryEUR().Workflow, scenario)
	if err != nil {
		t.Fatal(err)
	}
	if err = simulation.Advance(spec, input, &state, "complete", 0); err != nil {
		t.Fatal(err)
	}
	if state.Result != "PASS" {
		t.Fatalf("%s: %s, assertions=%+v", scenario.Name, state.Result, state.Assertions)
	}
	return state
}
func TestRecoveryEURBusinessScenarios(t *testing.T) {
	for _, scenario := range RecoveryEURScenarios() {
		t.Run(scenario.Name, func(t *testing.T) {
			state := runRecovery(t, scenario)
			for _, m := range state.Messages {
				if m.Status == "SIMULATED" {
					if !strings.Contains(m.Rendered.Text, "Alex") || !strings.Contains(m.Rendered.Text, "89.90") {
						t.Fatal("personalization missing")
					}
				}
			}
		})
	}
}
func TestRecoveryEURPriceCurrencyAndConsentBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name       string
		amount     float64
		currency   string
		email, sms bool
		count      int
	}{
		{"exactly 50 EUR", 50, "EUR", true, true, 2}, {"below threshold", 49.99, "EUR", true, true, 0}, {"different currency", 89.9, "USD", true, true, 0}, {"no permitted channel", 89.9, "EUR", false, false, 0}, {"SMS only", 89.9, "EUR", false, true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := RecoveryEURScenarios()[2]
			s.Name = tc.name
			s.Entry.Properties["amount"] = tc.amount
			s.Entry.Properties["amount_display"] = fmt.Sprintf("%.2f", tc.amount)
			s.Entry.Properties["currency"] = tc.currency
			s.Customer["consents"] = map[string]any{"email": tc.email, "sms": tc.sms}
			s.Assertions = []simulation.Assertion{{Type: "message_count", Expected: tc.count}, {Type: "no_duplicate_delivery", Expected: true}}
			runRecovery(t, s)
		})
	}
}
func TestRecoveryEURAllowsReentryAfterSevenDaysAndFinalProfileConversion(t *testing.T) {
	t.Run("cooldown expires", func(t *testing.T) {
		s := RecoveryEURScenarios()[4]
		s.Events[1].AfterSeconds = 8 * 86400
		s.Assertions = []simulation.Assertion{{Type: "enrollment_count", Expected: 2}, {Type: "message_count", Expected: 2}, {Type: "no_duplicate_delivery", Expected: true}}
		runRecovery(t, s)
	})
	t.Run("current cart profile converts at final check", func(t *testing.T) {
		s := RecoveryEURScenarios()[2]
		s.Changes = []simulation.Change{{AfterSeconds: 72 * 3600, Attributes: map[string]any{"cart_purchased": true}}}
		s.Assertions = []simulation.Assertion{{Type: "message_count", Expected: 2}, {Type: "exit_reason", Expected: "converted"}}
		runRecovery(t, s)
	})
}
