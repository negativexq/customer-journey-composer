package ai

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

// emailOutput is a valid one-email journey whose email says text.
func emailOutput(t *testing.T, text string) string {
	t.Helper()
	var root map[string]any
	_ = json.Unmarshal([]byte(validOutput), &root)
	w := root["workflow"].(map[string]any)
	nodes := w["nodes"].([]any)
	mail := map[string]any{"id": "mail", "type": "email", "label": "Cart email", "position": map[string]any{"x": 0, "y": 200}, "config": map[string]any{"subject": "Sepetin", "preheader": "", "html": "", "text": text, "fallbacks": []any{}, "discount_code": ""}}
	w["nodes"] = []any{nodes[0], mail, nodes[1]}
	w["edges"] = []any{map[string]any{"id": "a", "source": "start", "target": "mail", "port": "started"}, map[string]any{"id": "b", "source": "mail", "target": "done", "port": "next"}}
	out, _ := json.Marshal(root)
	return string(out)
}

func oneEmail() *workflow.BehaviorChecks {
	checks := reviewedEmptyChecks()
	one := 1
	checks.EmailCount = &one
	return checks
}

const cartBrief = "Sepeti terk edene https://example.com/sepet bağlantılı tek bir e-posta gönder ve %15 indirim sun."

func TestAnInventedLinkOrFigureGoesBackToTheModel(t *testing.T) {
	invented := emailOutput(t, "Son 3 gün! %15 indirim: https://example.com/kampanya")
	faithful := emailOutput(t, "%15 indirimin seni bekliyor: https://example.com/sepet")
	p := &sequence{outputs: []string{invented, faithful}}
	c, _ := New(map[string]Provider{"openai": p})
	result, err := c.Compose(context.Background(), Input{BehaviorChecks: oneEmail(), Provider: "openai", Action: "generate", Prompt: cartBrief})
	if err != nil || p.calls != 2 || result.Generation.Attempts[0].Validation == nil || result.Generation.Attempts[0].Validation.Facts != 2 {
		t.Fatal(err, p.calls, result.Generation)
	}
	if !strings.Contains(p.prompts[1], "Link https://example.com/kampanya is not in the brief") || !strings.Contains(p.prompts[1], "The number 3 is not in the brief") {
		t.Fatal("the model is told what to take out", p.prompts[1])
	}
	if len(result.NotDone) != 0 {
		t.Fatal(result.NotDone)
	}

	p = &sequence{outputs: []string{invented, invented}}
	c, _ = New(map[string]Provider{"openai": p})
	_, err = c.Compose(context.Background(), Input{BehaviorChecks: oneEmail(), Provider: "openai", Action: "generate", Prompt: cartBrief})
	var invalid *InvalidOutput
	if !errors.As(err, &invalid) || p.calls != 2 {
		t.Fatal("a draft stating what the brief does not is never offered", err)
	}
}

func TestWhatTheBriefAsksAndTheDraftLeavesOutIsShown(t *testing.T) {
	p := &sequence{outputs: []string{emailOutput(t, "Sepetin seni bekliyor.")}}
	c, _ := New(map[string]Provider{"openai": p})
	result, err := c.Compose(context.Background(), Input{BehaviorChecks: oneEmail(), Provider: "openai", Action: "generate", Prompt: cartBrief})
	want := []string{"The brief gives the link https://example.com/sepet, but no message uses it.", "The brief offers 15% off, but no message says so."}
	if err != nil || !slices.Equal(result.NotDone, want) {
		t.Fatalf("%v %q", err, result.NotDone)
	}
}

func TestOnlyTheMessagesTheModelChangedAreChecked(t *testing.T) {
	// A person wrote "Son 3 gün" earlier; a change elsewhere keeps it.
	var current struct {
		Workflow workflow.Workflow `json:"workflow"`
	}
	_ = json.Unmarshal([]byte(emailOutput(t, "Son 3 gün!")), &current)
	if issues := factIssues(Input{Prompt: "Rename the journey", Workflow: current.Workflow}, current.Workflow, ""); len(issues) != 0 {
		t.Fatal(issues)
	}
	changed := current.Workflow
	changed.Nodes = slices.Clone(current.Workflow.Nodes)
	changed.Nodes[1].Config.Text = "Son 5 gün!"
	if issues := factIssues(Input{Prompt: "Make it urgent", Workflow: current.Workflow}, changed, ""); len(issues) != 1 || !strings.Contains(issues[0].Message, "number 5") {
		t.Fatal("a rewritten message may keep the person's 3 but not add 5", issues)
	}
}
