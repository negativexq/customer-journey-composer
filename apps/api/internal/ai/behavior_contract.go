package ai

import (
	"encoding/json"
	"strings"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

// Private examples and event payloads never cross the provider boundary. Only
// explicitly reviewed rule predicates and event timing describe a clause's scope.
func behaviorContractSummary(c *workflow.BehaviorChecks) string {
	public := map[string]any{"version": c.Version, "entry_event": c.EntryEvent, "eligibility": c.Thresholds}
	if c.Version == 2 {
		clauses := []map[string]any{}
		for _, clause := range c.Cases() {
			events := []map[string]any{}
			for _, e := range clause.Events {
				events = append(events, map[string]any{"event_name": e.EventName, "after_seconds": e.AfterSeconds, "entry_binding": e.EntryBinding})
			}
			consents := map[string]bool{"email": true, "sms": true}
			for k, v := range c.Consents {
				consents[k] = v
			}
			for k, v := range clause.Consents {
				consents[k] = v
			}
			clauses = append(clauses, map[string]any{"name": clause.Name, "initial_conditions": clause.Conditions, "channel_consents": consents, "subsequent_events": events, "email_count": clause.EmailCount, "sms_count": clause.SMSCount, "outcome": clause.Outcome, "duration_seconds": clause.DurationSeconds})
		}
		public["clauses"] = clauses
		public["ineligible_outcome"] = c.IneligibleOutcome
	} else {
		// Legacy counters have no declared public scope. Do not present them as
		// unconditional business requirements or reveal their private fixture values.
		public["verification_scope"] = "Legacy private representative example; counts and outcome are server-side checks, not journey-wide requirements."
	}
	raw, _ := json.Marshal(public)
	return string(raw)
}

func validationSummary(issues []workflow.Issue) *ValidationSummary {
	if len(issues) == 0 {
		return nil
	}
	result := &ValidationSummary{}
	for _, issue := range issues {
		switch {
		case strings.HasPrefix(issue.Field, "behavior_checks"), issue.Field == "acceptance_contract":
			result.Behavior++
		case issue.Field == "schema" || issue.Field == "fallbacks":
			result.Schema++
		case issue.Field == "content" || issue.Field == "subject" || issue.Field == "html" || issue.Field == "text":
			result.Message++
		default:
			result.Workflow++
		}
	}
	return result
}
