package catalog

import (
	"bytes"
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
	return behaviorScenario(c, c.Cases()[0])
}
func behaviorScenario(c *workflow.BehaviorChecks, clause workflow.BehaviorCase) simulation.Scenario {
	data := workflow.BehaviorCaseData(c, clause)
	scenario := simulation.Scenario{Name: clause.Name, StartTime: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Customer: data["customer"].(map[string]any), Entry: simulation.Event{EventID: "behavior_entry", EventName: c.EntryEvent, Properties: data["event"].(map[string]any)["properties"].(map[string]any)}, FrequencyLimit: 1000, FrequencyWindowSeconds: 86400}
	if scenario.Name == "" {
		scenario.Name = "Private representative example"
	}
	for i, e := range clause.Events {
		scenario.Events = append(scenario.Events, simulation.TimedEvent{AfterSeconds: e.AfterSeconds, Event: simulation.Event{EventID: fmt.Sprintf("behavior_followup_%d", i), EventName: e.EventName, Properties: e.Payload()}})
	}
	return scenario
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
	run := func(input simulation.Scenario, expected workflow.BehaviorCase, eligible bool, field, label string) {
		add := func(key, message string) { issues = append(issues, issue(field+key, label+": "+message)) }
		state, spec, in, err := simulation.New(candidate, input)
		if err == nil {
			err = simulation.Advance(spec, in, &state, "complete", 0)
		}
		if err != nil || state.Status == "FAILED" {
			add("scenario", "simulation did not complete safely")
			return
		}
		email, sms, push := 0, 0, 0
		for _, m := range state.Messages {
			if m.AcceptedAt != nil {
				switch m.Channel {
				case "email":
					email++
				case "sms":
					sms++
				case "push":
					push++
				}
			}
		}
		if !eligible {
			if email+sms+push > 0 || state.ExitReason == "converted" {
				add("threshold", "ineligible input produced a message or conversion")
			}
			if c.Version == 2 {
				if state.ExitReason != c.IneligibleOutcome {
					add("ineligible_outcome", "ineligible outcome differs from the reviewed contract")
				}
				before, _ := json.Marshal(input.Customer["attributes"])
				after, _ := json.Marshal(state.Customer["attributes"])
				if !bytes.Equal(before, after) {
					add("threshold", "ineligible input changed customer attributes")
				}
			}
			return
		}
		if email != *expected.EmailCount {
			add("email_count", fmt.Sprintf("expected %d accepted email messages, got %d", *expected.EmailCount, email))
		}
		if sms != *expected.SMSCount {
			add("sms_count", fmt.Sprintf("expected %d accepted SMS messages, got %d", *expected.SMSCount, sms))
		}
		if expected.PushCount != nil && push != *expected.PushCount {
			add("push_count", fmt.Sprintf("expected %d accepted push notifications, got %d", *expected.PushCount, push))
		}
		if state.ExitReason != expected.Outcome {
			add("outcome", fmt.Sprintf("expected %s, got %s", expected.Outcome, state.ExitReason))
		}
		if expected.DurationSeconds != nil && state.Now.Sub(in.StartTime) != time.Duration(*expected.DurationSeconds)*time.Second {
			add("duration", "virtual completion time differs from the reviewed expectation")
		}
		for path, want := range expected.ExpectedAttributes {
			got, _ := workflow.Lookup(map[string]any{"customer": state.Customer}, "customer.attributes."+path)
			a, _ := json.Marshal(got)
			b, _ := json.Marshal(want)
			if !bytes.Equal(a, b) {
				add("attributes", "expected customer attribute update did not occur")
			}
		}
	}
	for i, clause := range c.Cases() {
		field := ""
		if i > 0 {
			field = fmt.Sprintf("alternatives.%d.", i-1)
		}
		run(behaviorScenario(c, clause), clause, true, field, fmt.Sprintf("Clause %d", i+1))
	}
	for _, t := range c.Thresholds {
		for _, value := range []float64{math.Nextafter(t.Value, math.Inf(-1)), t.Value, math.Nextafter(t.Value, math.Inf(1))} {
			raw, _ := json.Marshal(base)
			var input simulation.Scenario
			_ = json.Unmarshal(raw, &input)
			data := map[string]any{"customer": input.Customer, "event": map[string]any{"properties": input.Entry.Properties}}
			setInput(data, t.Path, value)
			eligible := true
			for _, bound := range c.Thresholds {
				eligible = eligible && workflow.Evaluate(workflow.Rule{Path: bound.Path, Operator: bound.Operator, Value: bound.Value}, data)
			}
			run(input, c.Cases()[0], eligible, "", fmt.Sprintf("Threshold %s %s %g", t.Path, t.Operator, t.Value))
		}
	}
	return issues[:min(20, len(issues))]
}
