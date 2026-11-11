package ai

import (
	"context"
	"encoding/json"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/catalog"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
	"os"
	"testing"
)

func schemaFixture(t *testing.T, name string) string {
	t.Helper()
	raw, e := os.ReadFile("../../../../tests/fixtures/ai/" + name)
	if e != nil {
		t.Fatal(e)
	}
	var p map[string]any
	json.Unmarshal(raw, &p)
	w := p["workflow"].(map[string]any)
	delete(w, "acceptance_contract")
	if e, ok := w["global_exit"].(map[string]any); ok {
		e["filters"] = nil
	}
	for _, item := range w["nodes"].([]any) {
		n := item.(map[string]any)
		c := n["config"].(map[string]any)
		switch n["type"] {
		case "event_trigger", "wait_event":
			if _, ok := c["filters"]; !ok {
				c["filters"] = nil
			}
		case "email", "sms":
			fs := []any{}
			if m, ok := c["fallbacks"].(map[string]any); ok {
				for k, v := range m {
					fs = append(fs, map[string]any{"path": k, "value": v})
				}
			}
			c["fallbacks"] = fs
		}
	}
	p = map[string]any{"name": p["name"], "description": p["description"], "workflow": w, "explanation": "Reviewed cart recovery", "warnings": []string{}}
	raw, _ = json.Marshal(p)
	return string(raw)
}
func TestComposerRejectsStructurallyValidBusinessOmissions(t *testing.T) {
	// Fix the free-form reasons only: the missing profile/consent gates must still be rejected.
	raw := schemaFixture(t, "luna-recovery-2026-11-11.json")
	var candidate map[string]any
	json.Unmarshal([]byte(raw), &candidate)
	for _, v := range candidate["workflow"].(map[string]any)["nodes"].([]any) {
		n := v.(map[string]any)
		if n["id"] == "exit-no-sms" || n["id"] == "exit-non-converted" {
			n["config"].(map[string]any)["reason"] = "completed"
		}
	}
	bad, _ := json.Marshal(candidate)
	good := schemaFixture(t, "luna-recovery-corrected.json")
	provider := &sequence{outputs: []string{string(bad), good}}
	c, err := New(map[string]Provider{"openai": provider})
	if err != nil {
		t.Fatal(err)
	}
	in := Input{Provider: "openai", Action: "generate", Prompt: "Recover carts", AcceptanceContract: catalog.RecoveryEURContract, Workflow: workflow.Workflow{SchemaVersion: 1, Nodes: []workflow.Node{}, Edges: []workflow.Edge{}}}
	p, err := c.Compose(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if provider.calls != 2 || p.Workflow.AcceptanceContract != catalog.RecoveryEURContract {
		t.Fatal("contract was dropped or omissions not repaired")
	}
	provider = &sequence{outputs: []string{string(bad)}}
	c, _ = New(map[string]Provider{"openai": provider})
	if _, err = c.Compose(context.Background(), in); err == nil {
		t.Fatal("invalid semantic output accepted")
	}
}
func TestAISchemaRejectsProseExitCodesButManualWorkflowAllowsThem(t *testing.T) {
	raw, _ := os.ReadFile("../../../../tests/fixtures/ai/luna-recovery-2026-11-11.json")
	var manual struct {
		Workflow workflow.Workflow `json:"workflow"`
	}
	json.Unmarshal(raw, &manual)
	if issues := workflow.Validate(manual.Workflow); len(issues) > 0 {
		t.Fatalf("manual custom exits were reinterpreted: %+v", issues)
	}
	c, _ := New(map[string]Provider{"openai": &sequence{outputs: []string{schemaFixture(t, "luna-recovery-2026-11-11.json")}}})
	if _, err := c.Compose(context.Background(), Input{Provider: "openai", Action: "generate", Prompt: "Generate", Workflow: workflow.Workflow{SchemaVersion: 1}}); err == nil {
		t.Fatal("AI prose exit code accepted")
	}
}
