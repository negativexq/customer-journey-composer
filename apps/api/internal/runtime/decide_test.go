package runtime

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

func TestPureDecisionsUseProvidedClockAndPorts(t *testing.T) {
	now := time.Date(2026, 11, 10, 10, 0, 0, 0, time.UTC)
	data := map[string]any{"customer": map[string]any{"attributes": map[string]any{"orders": 2.0}}}
	wait := workflow.Node{Type: "wait", Config: workflow.Config{DurationSeconds: 7200}}
	d, err := Decide(wait, data, now, "")
	if err != nil || d.WaitUntil == nil || !d.WaitUntil.Equal(now.Add(2*time.Hour)) {
		t.Fatal(d, err)
	}
	d, err = Decide(wait, data, now, "completed")
	if err != nil || d.WaitUntil != nil || d.Port != "completed" {
		t.Fatal(d, err)
	}
	condition := workflow.Node{Type: "condition", Config: workflow.Config{Rule: &workflow.Rule{Path: "customer.attributes.orders", Operator: "greater_than", Value: 1.0}}}
	d, err = Decide(condition, data, now, "")
	if err != nil || d.Port != "true" || d.Condition == nil || !*d.Condition {
		t.Fatal(d, err)
	}
	waiter := workflow.Node{Type: "wait_event", Config: workflow.Config{EventName: "purchase_completed", TimeoutSeconds: 60}}
	d, err = Decide(waiter, data, now, "")
	if err != nil || d.WaitUntil == nil || d.EventName != "purchase_completed" {
		t.Fatal(d, err)
	}
	for _, port := range []string{"matched", "timeout"} {
		d, err = Decide(waiter, data, now, port)
		if err != nil || d.Port != port || d.WaitUntil != nil {
			t.Fatal(d, err)
		}
	}
	split := workflow.Node{ID: "experiment", Type: "split", Config: workflow.Config{Percent: 50}}
	counts := map[string]int{}
	for i := 0; i < 2000; i++ {
		who := map[string]any{"customer": map[string]any{"customer_id": fmt.Sprintf("customer-%d", i)}}
		first, err := Decide(split, who, now, "")
		again, _ := Decide(split, who, now.Add(time.Hour), "")
		if err != nil || first.Port != again.Port || first.Variant != strings.ToUpper(first.Port) {
			t.Fatal("a customer must keep one variant", first, again, err)
		}
		counts[first.Port]++
	}
	if counts["a"]+counts["b"] != 2000 || counts["a"] < 900 || counts["a"] > 1100 {
		t.Fatal("an even split must be roughly even", counts)
	}
	split.Config.Percent = 99
	mostly := 0
	for i := 0; i < 1000; i++ {
		d, _ := Decide(split, map[string]any{"customer": map[string]any{"customer_id": fmt.Sprintf("customer-%d", i)}}, now, "")
		if d.Port == "a" {
			mostly++
		}
	}
	if mostly < 960 {
		t.Fatal("the share must follow the configured percent", mostly)
	}
	tiers := workflow.Node{Type: "switch", Config: workflow.Config{Branches: []workflow.Branch{
		{Label: "Big spender", Rule: &workflow.Rule{Path: "customer.attributes.orders", Operator: "greater_than", Value: 10.0}},
		{Label: "Returning", Rule: &workflow.Rule{Path: "customer.attributes.orders", Operator: "greater_than", Value: 1.0}},
		{Label: "Also returning", Rule: &workflow.Rule{Path: "customer.attributes.orders", Operator: "greater_than", Value: 0.0}},
	}}}
	d, err = Decide(tiers, data, now, "")
	if err != nil || d.Port != "case_2" || d.Branch != "Returning" {
		t.Fatal("the first matching branch wins", d, err)
	}
	d, err = Decide(tiers, map[string]any{"customer": map[string]any{"attributes": map[string]any{}}}, now, "")
	if err != nil || d.Port != "else" || d.Branch != "" {
		t.Fatal("no match leaves through else", d, err)
	}
	if got := workflow.NodePorts(tiers); len(got) != 4 || got[0] != "case_1" || got[3] != "else" {
		t.Fatal(got)
	}
	hook := workflow.Node{ID: "crm", Label: "Tell the CRM", Type: "webhook", Config: workflow.Config{URL: "https://hooks.example.invalid/in", Fields: map[string]string{"orders": "customer.attributes.orders", "absent": "event.properties.none"}}}
	d, err = Decide(hook, data, now, "")
	if err != nil || d.Port != "next" || d.Webhook == nil || d.Webhook.URL != "https://hooks.example.invalid/in" {
		t.Fatal(d, err)
	}
	sent, _ := d.Webhook.Payload["data"].(map[string]any)
	if sent["orders"] != 2.0 || sent["absent"] != nil || d.Webhook.Payload["step"] != "crm" {
		t.Fatal("unexpected webhook payload", d.Webhook.Payload)
	}
	if output, _ := json.Marshal(d); strings.Contains(string(output), "hooks.example") {
		t.Fatal("the call must not be written into step history", string(output))
	}
	if _, err := Decide(workflow.Node{Type: "arbitrary_code"}, data, now, ""); err == nil {
		t.Fatal("unregistered node must be rejected")
	}
}
