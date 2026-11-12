package catalog

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/simulation"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

func setInput(data map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	m := data
	for _, p := range parts[:len(parts)-1] {
		next, ok := m[p].(map[string]any)
		if !ok {
			next = map[string]any{}
			m[p] = next
		}
		m = next
	}
	m[parts[len(parts)-1]] = value
}
func BehaviorExample(c *workflow.BehaviorChecks) simulation.Scenario {
	data := map[string]any{"customer": map[string]any{"customer_id": "behavior_check", "email": "preview@example.invalid", "phone": "+15005550006", "attributes": map[string]any{"first_name": "Sample"}, "consents": map[string]any{"email": true, "sms": true}}, "event": map[string]any{"properties": map[string]any{}}}
	for p, v := range c.Inputs {
		setInput(data, p, v)
	}
	return simulation.Scenario{Name: "Independent eligible example", StartTime: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Customer: data["customer"].(map[string]any), Entry: simulation.Event{EventID: "behavior_entry", EventName: c.EntryEvent, Properties: data["event"].(map[string]any)["properties"].(map[string]any)}, FrequencyLimit: 1000, FrequencyWindowSeconds: 86400}
}

// ValidateBehavior is bounded, pure simulation. Expected values come from a
// human-confirmed request, not from inspecting the candidate's nodes or labels.
func ValidateBehavior(w workflow.Workflow) []workflow.Issue {
	c := w.BehaviorChecks
	if c == nil {
		return nil
	}
	issue := func(field, message string) workflow.Issue {
		return workflow.Issue{Field: "behavior_checks." + field, Message: message}
	}
	if err := workflow.ValidateBehaviorChecks(c); err != nil {
		return []workflow.Issue{issue("input", err.Error())}
	}
	if len(workflow.Validate(w)) > 0 {
		return nil
	}
	if w.Start().Config.EventName != c.EntryEvent {
		return []workflow.Issue{issue("entry_event", "Entry event differs from the reviewed expectation")}
	}
	base := BehaviorExample(c)
	for _, threshold := range c.Thresholds {
		data := map[string]any{"customer": base.Customer, "event": map[string]any{"properties": base.Entry.Properties}}
		if !workflow.Evaluate(workflow.Rule{Path: threshold.Path, Operator: threshold.Operator, Value: threshold.Value}, data) {
			return []workflow.Issue{issue("inputs", "The synthetic example must satisfy every reviewed eligibility threshold")}
		}
	}
	candidate := w
	candidate.BehaviorChecks = nil // avoid validating metadata inside each simulation
	issues := []workflow.Issue{}
	run := func(input simulation.Scenario, eligible bool, label string) {
		state, spec, in, err := simulation.New(candidate, input)
		if err == nil {
			err = simulation.Advance(spec, in, &state, "complete", 0)
		}
		if err != nil || state.Status == "FAILED" {
			issues = append(issues, issue("scenario", label+": simulation did not complete safely"))
			return
		}
		email, sms := 0, 0
		for _, m := range state.Messages {
			if m.AcceptedAt != nil {
				if m.Channel == "email" {
					email++
				}
				if m.Channel == "sms" {
					sms++
				}
			}
		}
		if !eligible {
			if email+sms > 0 || state.ExitReason == "converted" {
				issues = append(issues, issue("threshold", label+": ineligible input produced a message or conversion"))
			}
			return
		}
		if email != *c.EmailCount {
			issues = append(issues, issue("email_count", fmt.Sprintf("%s: expected %d accepted email messages, got %d", label, *c.EmailCount, email)))
		}
		if sms != *c.SMSCount {
			issues = append(issues, issue("sms_count", fmt.Sprintf("%s: expected %d accepted SMS messages, got %d", label, *c.SMSCount, sms)))
		}
		if state.ExitReason != c.Outcome {
			issues = append(issues, issue("outcome", fmt.Sprintf("%s: expected %s, got %s", label, c.Outcome, state.ExitReason)))
		}
		if c.DurationSeconds != nil && state.Now.Sub(in.StartTime) != time.Duration(*c.DurationSeconds)*time.Second {
			issues = append(issues, issue("duration", "Virtual completion time differs from the reviewed expectation"))
		}
	}
	run(base, true, "Eligible example")
	for _, t := range c.Thresholds {
		for _, value := range []float64{math.Nextafter(t.Value, math.Inf(-1)), t.Value, math.Nextafter(t.Value, math.Inf(1))} {
			raw, _ := json.Marshal(base)
			var input simulation.Scenario
			_ = json.Unmarshal(raw, &input)
			data := map[string]any{"customer": input.Customer, "event": map[string]any{"properties": input.Entry.Properties}}
			setInput(data, t.Path, value)
			eligible := workflow.Evaluate(workflow.Rule{Path: t.Path, Operator: t.Operator, Value: t.Value}, data)
			run(input, eligible, fmt.Sprintf("Threshold %s %s %g", t.Path, t.Operator, t.Value))
		}
	}
	return issues[:min(20, len(issues))]
}
