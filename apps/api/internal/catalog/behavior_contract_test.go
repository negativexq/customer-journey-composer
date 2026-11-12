package catalog

import (
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
	"testing"
)

func intp(n int) *int        { return &n }
func seconds(n int64) *int64 { return &n }
func scopedContractFixture(channel string) workflow.Workflow {
	c := &workflow.BehaviorChecks{Version: 2, Confirmed: true, EntryEvent: "request_received", CaseName: "No subsequent event", Inputs: map[string]any{"customer.attributes.ready": false}, Conditions: []workflow.Rule{{Path: "customer.attributes.ready", Operator: "equals", Value: false}}, EmailCount: intp(0), SMSCount: intp(0), Outcome: "completed", DurationSeconds: seconds(3600)}
	ready := workflow.BehaviorCase{Name: "Matching event arrives", Conditions: c.Conditions, Events: []workflow.BehaviorEvent{{EventName: "resource_ready", AfterSeconds: 600}}, EmailCount: intp(0), SMSCount: intp(0), Outcome: "completed", DurationSeconds: seconds(600)}
	if channel == "email" {
		ready.EmailCount = intp(1)
	} else {
		ready.SMSCount = intp(1)
	}
	c.Alternatives = []workflow.BehaviorCase{ready, {Name: "Initially ready", Conditions: []workflow.Rule{{Path: "customer.attributes.ready", Operator: "equals", Value: true}}, Inputs: map[string]any{"customer.attributes.ready": true}, EmailCount: ready.EmailCount, SMSCount: ready.SMSCount, Outcome: "completed", DurationSeconds: seconds(0)}}
	return workflow.Workflow{SchemaVersion: 1, BehaviorChecks: c, Nodes: []workflow.Node{{ID: "a", Type: "event_trigger", Label: "Entry", Config: workflow.Config{EventName: c.EntryEvent}}, {ID: "b", Type: "condition", Label: "Ready", Config: workflow.Config{Rule: &workflow.Rule{Path: "customer.attributes.ready", Operator: "equals", Value: true}}}, {ID: "c", Type: "wait_event", Label: "Wait", Config: workflow.Config{EventName: "resource_ready", TimeoutSeconds: 3600}}, {ID: "d", Type: channel, Label: "Message", Config: workflow.Config{Subject: "Notification", Text: "Ready"}}, {ID: "e", Type: "exit", Label: "Complete", Config: workflow.Config{Reason: "completed"}}}, Edges: []workflow.Edge{{ID: "1", Source: "a", Target: "b", Port: "started"}, {ID: "2", Source: "b", Target: "d", Port: "true"}, {ID: "3", Source: "b", Target: "c", Port: "false"}, {ID: "4", Source: "c", Target: "d", Port: "matched"}, {ID: "5", Source: "c", Target: "e", Port: "timeout"}, {ID: "6", Source: "d", Target: "e", Port: "next"}}}
}
func TestScopedContractDistinguishesInitialStateMatchedEventAndTimeout(t *testing.T) {
	for _, channel := range []string{"email", "sms"} {
		t.Run(channel, func(t *testing.T) {
			w := scopedContractFixture(channel)
			if issues := ValidateAcceptance(w); len(issues) > 0 {
				t.Fatal(issues)
			}
			// A structurally valid alternate-path omission must fail its own obligation,
			// even though the timeout example still passes. No domain-specific matching.
			for i := range w.Edges {
				if w.Edges[i].Source == "c" && w.Edges[i].Port == "matched" {
					w.Edges[i].Target = "e"
				}
			}
			if issues := ValidateAcceptance(w); len(issues) == 0 {
				t.Fatal("omitted matched-event effect accepted")
			}
		})
	}
}
func TestIneligibleOutcomeAndNoAttributeEffectsAreContractRules(t *testing.T) {
	for _, threshold := range []workflow.BehaviorThreshold{{Path: "event.properties.level", Operator: "greater_than_or_equal", Value: 7}, {Path: "event.properties.level", Operator: "less_than_or_equal", Value: 7}} {
		c := &workflow.BehaviorChecks{Version: 2, Confirmed: true, EntryEvent: "level_changed", CaseName: "Eligible", Inputs: map[string]any{"event.properties.level": 7}, EmailCount: intp(0), SMSCount: intp(0), Outcome: "completed", Thresholds: []workflow.BehaviorThreshold{threshold}, IneligibleOutcome: "condition_not_met"}
		w := workflow.Workflow{SchemaVersion: 1, BehaviorChecks: c, Nodes: []workflow.Node{{ID: "a", Type: "event_trigger", Label: "Entry", Config: workflow.Config{EventName: c.EntryEvent}}, {ID: "b", Type: "condition", Label: "Eligible", Config: workflow.Config{Rule: &workflow.Rule{Path: threshold.Path, Operator: threshold.Operator, Value: threshold.Value}}}, {ID: "c", Type: "exit", Label: "Normal", Config: workflow.Config{Reason: "completed"}}, {ID: "d", Type: "exit", Label: "Ineligible", Config: workflow.Config{Reason: "condition_not_met"}}}, Edges: []workflow.Edge{{ID: "1", Source: "a", Target: "b", Port: "started"}, {ID: "2", Source: "b", Target: "c", Port: "true"}, {ID: "3", Source: "b", Target: "d", Port: "false"}}}
		if issues := ValidateAcceptance(w); len(issues) > 0 {
			t.Fatal(issues)
		}
		w.Nodes[3].Config.Reason = "completed"
		if len(ValidateAcceptance(w)) == 0 {
			t.Fatal("wrong ineligible reason accepted")
		}
		w.Nodes[3].Config.Reason = "condition_not_met"
		w.Nodes = append(w.Nodes, workflow.Node{ID: "mutation", Type: "update_attribute", Label: "Mutation", Config: workflow.Config{Attribute: "flag", Operation: "set", Value: true}})
		w.Edges[2].Target = "mutation"
		w.Edges = append(w.Edges, workflow.Edge{ID: "4", Source: "mutation", Target: "d", Port: "next"})
		if len(ValidateAcceptance(w)) == 0 {
			t.Fatal("ineligible attribute mutation accepted")
		}
	}
}

func TestContractEventScopeDistinguishesMatchingAndUnrelatedIdentities(t *testing.T) {
	c := &workflow.BehaviorChecks{Version: 2, Confirmed: true, EntryEvent: "process_started", CaseName: "No follow-up", Inputs: map[string]any{"event.properties.entity_id": "PRIVATE_ENTRY_ID"}, EmailCount: intp(0), SMSCount: intp(0), Outcome: "completed", DurationSeconds: seconds(3600)}
	for _, matching := range []bool{true, false} {
		id, reason, operator, duration := "PRIVATE_ENTRY_ID", "converted", "equals", int64(600)
		if !matching {
			id, reason, operator, duration = "PRIVATE_OTHER_ID", "completed", "not_equals", 3600
		}
		c.Alternatives = append(c.Alternatives, workflow.BehaviorCase{Name: operator, EmailCount: intp(0), SMSCount: intp(0), Outcome: reason, DurationSeconds: seconds(duration), Events: []workflow.BehaviorEvent{{EventName: "process_finished", AfterSeconds: 600, Properties: map[string]any{"entity_id": id}, EntryBinding: &workflow.BehaviorBinding{EntryPath: "event.properties.entity_id", EventPath: "event.properties.entity_id", Operator: operator}}}})
	}
	w := workflow.Workflow{SchemaVersion: 1, BehaviorChecks: c, GlobalExit: &workflow.ExitCriteria{EventName: "process_finished", Reason: "converted", Correlation: &workflow.EventCorrelation{EntryPath: "event.properties.entity_id", EventPath: "event.properties.entity_id"}}, Nodes: []workflow.Node{{ID: "a", Type: "event_trigger", Label: "Entry", Config: workflow.Config{EventName: c.EntryEvent}}, {ID: "b", Type: "wait", Label: "Wait", Config: workflow.Config{DurationSeconds: 3600}}, {ID: "c", Type: "exit", Label: "Complete", Config: workflow.Config{Reason: "completed"}}}, Edges: []workflow.Edge{{ID: "1", Source: "a", Target: "b", Port: "started"}, {ID: "2", Source: "b", Target: "c", Port: "completed"}}}
	if issues := ValidateAcceptance(w); len(issues) > 0 {
		t.Fatal(issues)
	}
	w.GlobalExit.Correlation = nil
	if len(ValidateAcceptance(w)) == 0 {
		t.Fatal("unrelated event could satisfy scoped outcome")
	}
	c.Alternatives[1].Events[0].Properties = nil
	if workflow.ValidateBehaviorChecks(c) == nil {
		t.Fatal("missing identity treated as unrelated object")
	}
}
