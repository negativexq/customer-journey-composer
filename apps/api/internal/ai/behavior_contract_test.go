package ai

import (
	"encoding/json"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
	"strings"
	"testing"
)

func TestProviderContractPreservesScopeWithoutPrivateExamples(t *testing.T) {
	zero, one := 0, 1
	c := &workflow.BehaviorChecks{Version: 2, Confirmed: true, EntryEvent: "entry", CaseName: "Timeout", Conditions: []workflow.Rule{{Path: "customer.attributes.ready", Operator: "equals", Value: false}}, Inputs: map[string]any{"customer.attributes.ready": false, "customer.attributes.name": "PRIVATE_EXAMPLE"}, EmailCount: &zero, SMSCount: &zero, Outcome: "completed", Alternatives: []workflow.BehaviorCase{{Name: "Event matched", Events: []workflow.BehaviorEvent{{EventName: "ready", AfterSeconds: 30, EntryBinding: &workflow.BehaviorBinding{EntryPath: "event.properties.id", EventPath: "event.properties.ref", Operator: "not_equals"}, Properties: map[string]any{"token": "PRIVATE_PAYLOAD"}}}, ExpectedAttributes: map[string]any{"note": "PRIVATE_EXPECTED"}, EmailCount: &one, SMSCount: &zero, Outcome: "completed"}}}
	summary := behaviorContractSummary(c)
	if !strings.Contains(summary, "entry_binding") || !strings.Contains(summary, "not_equals") {
		t.Fatal("lost identity scope")
	}
	if strings.Contains(summary, "PRIVATE_") {
		t.Fatal("private data crossed provider boundary")
	}
	var public struct {
		Clauses []struct {
			Name   string `json:"name"`
			Events []struct {
				Name  string `json:"event_name"`
				After int64  `json:"after_seconds"`
			} `json:"subsequent_events"`
			Email int `json:"email_count"`
		}
	}
	if err := json.Unmarshal([]byte(summary), &public); err != nil || len(public.Clauses) != 2 || public.Clauses[0].Email != 0 || public.Clauses[1].Email != 1 || len(public.Clauses[1].Events) != 1 || public.Clauses[1].Events[0].After != 30 {
		t.Fatal("lost per-clause meaning", summary, err)
	}
	c.Version = 1
	if legacy := behaviorContractSummary(c); strings.Contains(legacy, "email_count") || strings.Contains(legacy, "PRIVATE_") {
		t.Fatal("unscoped legacy counts became global requirements")
	}
}
func TestValidationAccountingContainsCategoriesNotUntrustedValues(t *testing.T) {
	result := validationSummary([]workflow.Issue{{Field: "behavior_checks.alternatives.0.email_count", Message: "PRIVATE_VALUE"}, {Field: "schema", Message: "PRIVATE_OUTPUT"}, {Field: "rule", Message: "PRIVATE_NAME"}})
	raw, _ := json.Marshal(result)
	if result.Behavior != 1 || result.Schema != 1 || result.Workflow != 1 || strings.Contains(string(raw), "PRIVATE_") {
		t.Fatal("invalid or unsafe rejection diagnostics", string(raw))
	}
}
