package delivery

import "github.com/negativexq/customer-journey-composer/apps/api/internal/messaging"

// EvaluateEligibility is pure and shared by production delivery and Test Lab.
// Reservations include pending/unknown sends and recent accepted logical sends.
func EvaluateEligibility(message messaging.Message, consents map[string]bool, reservations, limit int, state string) Eligibility {
	decision := Eligibility{Eligible: true, Consent: consents[message.Channel], FrequencyCap: reservations < limit, Reason: "eligible"}
	switch {
	case state == "EXITED" || state == "FAILED" || state == "CANCELLED":
		decision.Eligible = false
		decision.Reason = "execution_terminated"
	case len(message.Missing) > 0:
		decision.Eligible = false
		decision.Reason = "missing_personalization"
	case !decision.Consent:
		decision.Eligible = false
		decision.Reason = "consent_denied"
	case !validRecipient(message.Channel, message.To):
		decision.Eligible = false
		decision.Reason = "invalid_recipient"
	case !decision.FrequencyCap:
		decision.Eligible = false
		decision.Reason = "frequency_cap"
	}
	return decision
}
func RetryDelay(attempt int) int64 { return int64(1 << min(attempt, 8)) }
