package catalog

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

func TestIndependentChecksRejectRecordedSemanticFailures(t *testing.T) {
	raw, err := os.ReadFile("../../../../docs/validation/varied-journeys-2026-11-12/result.json")
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Journeys []struct {
			ID       string `json:"id"`
			Proposal struct {
				Workflow workflow.Workflow `json:"workflow"`
			} `json:"proposal"`
		} `json:"journeys"`
	}
	if err = json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	n := func(v int) *int { return &v }
	for _, j := range report.Journeys {
		if j.ID != "webinar" && j.ID != "loyalty" && j.ID != "lead" {
			continue
		}
		t.Run(j.ID, func(t *testing.T) {
			w := j.Proposal.Workflow
			c := &workflow.BehaviorChecks{Version: 1, Confirmed: true, EntryEvent: map[string]string{"webinar": "audit_webinar_registered", "loyalty": "audit_loyalty_updated", "lead": "audit_demo_requested"}[j.ID], EmailCount: n(1), SMSCount: n(1), Outcome: "completed", Inputs: map[string]any{}}
			switch j.ID {
			case "webinar":
				c.Inputs = map[string]any{"customer.attributes.webinar_attended": false, "customer.attributes.webinar_cancelled": false}
			case "loyalty":
				c.SMSCount = n(0)
				c.Inputs = map[string]any{"event.properties.points": 500, "customer.attributes.vip_member": false}
			case "lead":
				c.Inputs = map[string]any{"customer.attributes.company_size": 50, "customer.attributes.budget": 10000, "customer.attributes.qualified_lead": false, "customer.attributes.meeting_booked": false}
				c.Thresholds = []workflow.BehaviorThreshold{{Path: "customer.attributes.company_size", Operator: "greater_than_or_equal", Value: 50}, {Path: "customer.attributes.budget", Operator: "greater_than_or_equal", Value: 10000}}
			}
			w.BehaviorChecks = c
			if len(ValidateBehavior(w)) == 0 {
				t.Fatal("recorded failure was accepted")
			}
			for i := range w.Nodes {
				node := &w.Nodes[i]
				switch j.ID {
				case "webinar":
					if node.ID == "confirmation_email" {
						node.Type = "email"
						node.Config.Subject = "Webinar confirmation"
					}
				case "loyalty":
					if node.Type == "exit" && node.Config.Reason == "converted" {
						node.Config.Reason = "completed"
					}
				case "lead":
					if node.ID == "qualify" {
						node.Config.Rule = &workflow.Rule{Logic: "and", Rules: []workflow.Rule{{Path: "customer.attributes.company_size", Operator: "greater_than_or_equal", Value: 50}, {Path: "customer.attributes.budget", Operator: "greater_than_or_equal", Value: 10000}}}
					}
				}
			}
			if issues := ValidateBehavior(w); len(issues) != 0 {
				t.Fatal("reviewed correction rejected", issues)
			}
		})
	}
}
