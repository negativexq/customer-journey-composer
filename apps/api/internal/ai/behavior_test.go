package ai

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

func reviewedEmptyChecks() *workflow.BehaviorChecks {
	zero := 0
	return &workflow.BehaviorChecks{Version: 1, Confirmed: true, EntryEvent: "cart_updated", EmailCount: &zero, SMSCount: &zero, Outcome: "completed"}
}
func TestMissingIndependentChecksStopsBeforePaidProvider(t *testing.T) {
	p := &sequence{outputs: []string{validOutput}}
	c, _ := New(map[string]Provider{"openai": p})
	if _, err := c.Compose(context.Background(), Input{Provider: "openai", Action: "generate", Prompt: "Send email"}); err == nil || p.calls != 0 {
		t.Fatal("missing expectations reached provider", err, p.calls)
	}
}
func TestSemanticMismatchTriggersBoundedRepair(t *testing.T) {
	var root map[string]any
	_ = json.Unmarshal([]byte(validOutput), &root)
	w := root["workflow"].(map[string]any)
	nodes := w["nodes"].([]any)
	mail := map[string]any{"id": "mail", "type": "sms", "label": "Confirmation email", "position": map[string]any{"x": 0, "y": 200}, "config": map[string]any{"text": "Hello", "fallbacks": []any{}, "discount_code": ""}}
	w["nodes"] = []any{nodes[0], mail, nodes[1]}
	w["edges"] = []any{map[string]any{"id": "a", "source": "start", "target": "mail", "port": "started"}, map[string]any{"id": "b", "source": "mail", "target": "done", "port": "next"}}
	bad, _ := json.Marshal(root)
	mail["type"] = "email"
	mail["config"] = map[string]any{"subject": "Hello", "preheader": "", "html": "", "text": "Hello", "fallbacks": []any{}, "discount_code": ""}
	fixed, _ := json.Marshal(root)
	checks := reviewedEmptyChecks()
	one := 1
	checks.EmailCount = &one
	p := &sequence{outputs: []string{string(bad), string(fixed)}}
	c, _ := New(map[string]Provider{"openai": p})
	result, err := c.Compose(context.Background(), Input{BehaviorChecks: checks, Provider: "openai", Action: "generate", Prompt: "Send one email and complete normally"})
	if err != nil || p.calls != 2 || result.Workflow.Nodes[1].Type != "email" || result.Workflow.BehaviorChecks == nil || result.Generation.Attempts[0].Validation == nil || result.Generation.Attempts[0].Validation.Behavior == 0 {
		t.Fatal(result, err, p.calls)
	}
	p = &sequence{outputs: []string{string(bad)}}
	c, _ = New(map[string]Provider{"openai": p})
	refused, err := c.Compose(context.Background(), Input{BehaviorChecks: checks, Provider: "openai", Action: "generate", Prompt: "Send one email"})
	if err == nil || p.calls != 2 {
		t.Fatal("semantic mismatch accepted", err, p.calls)
	}
	// The person sees why, and the attempts keep the reasons for review.
	var invalid *InvalidOutput
	if !errors.As(err, &invalid) || invalid.Reasons() == "" || len(refused.Generation.Attempts[1].Issues) == 0 {
		t.Fatal("refusal reasons missing", err, refused.Generation)
	}
}

type privacyProvider struct{ t *testing.T }

func (p privacyProvider) Generate(_ context.Context, r Request) (ProviderResult, error) {
	if strings.Contains(r.Prompt, "PRIVATE_SYNTHETIC") || strings.Contains(r.System, "PRIVATE_SYNTHETIC") {
		p.t.Fatal("synthetic profile values sent to provider")
	}
	return ProviderResult{Output: []byte(validOutput)}, nil
}
func TestBehaviorExampleValuesStayOutOfProviderRequest(t *testing.T) {
	checks := reviewedEmptyChecks()
	checks.Inputs = map[string]any{"customer.attributes.note": "PRIVATE_SYNTHETIC"}
	c, _ := New(map[string]Provider{"openai": privacyProvider{t}})
	if _, err := c.Compose(context.Background(), Input{BehaviorChecks: checks, Provider: "openai", Action: "generate", Prompt: "Complete normally", Workflow: workflow.Workflow{SchemaVersion: 1, BehaviorChecks: checks}}); err != nil {
		t.Fatal(err)
	}
}

type fixedOutput struct{ out string }

func (f fixedOutput) Generate(context.Context, Request) (ProviderResult, error) {
	return ProviderResult{Output: []byte(f.out)}, nil
}

// A change request keeps the journey's own name and description.
func TestChangeKeepsJourneyNameAndDescription(t *testing.T) {
	var root map[string]any
	_ = json.Unmarshal([]byte(validOutput), &root)
	root["name"] = "Changed name"
	root["description"] = "Adds one step"
	out, _ := json.Marshal(root)
	c, _ := New(map[string]Provider{"openai": fixedOutput{string(out)}})
	var base struct {
		Workflow workflow.Workflow `json:"workflow"`
	}
	_ = json.Unmarshal([]byte(validOutput), &base)
	base.Workflow.BehaviorChecks = reviewedEmptyChecks()
	result, err := c.Compose(context.Background(), Input{Provider: "openai", Action: "modify", Prompt: "Add a step", Name: "Cart recovery", Description: "Recovers abandoned carts", Workflow: base.Workflow})
	if err != nil || result.Name != "Cart recovery" || result.Description != "Recovers abandoned carts" || result.Diff.MetadataChanged {
		t.Fatal(err, result.Name, result.Description)
	}
	result, err = c.Compose(context.Background(), Input{Provider: "openai", Action: "modify", Prompt: "Add a step", Workflow: base.Workflow})
	if err != nil || result.Name != "Changed name" {
		t.Fatal("an unnamed journey should take the proposed name", err, result.Name)
	}
}
