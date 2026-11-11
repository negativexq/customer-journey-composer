package catalog

import (
	"encoding/json"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
	"os"
	"strings"
	"testing"
)

func TestRecoveryAcceptanceRejectsOmissionsWithoutChangingWorkerSemantics(t *testing.T) {
	for _, name := range []string{"luna-recovery-2026-11-11.json", "luna-recovery-corrected.json"} {
		raw, err := os.ReadFile("../../../../tests/fixtures/ai/" + name)
		if err != nil {
			t.Fatal(err)
		}
		var p struct {
			Workflow workflow.Workflow `json:"workflow"`
		}
		if err = json.Unmarshal(raw, &p); err != nil {
			t.Fatal(err)
		}
		p.Workflow.AcceptanceContract = RecoveryEURContract
		issues := ValidateAcceptance(p.Workflow)
		if strings.Contains(name, "corrected") && len(issues) > 0 {
			t.Fatalf("corrected fixture failed: %+v", issues)
		}
		if !strings.Contains(name, "corrected") && len(issues) == 0 {
			t.Fatal("recorded missing decisions accepted")
		}
	}
	if issues := ValidateAcceptance(recoveryEUR().Workflow); len(issues) > 0 {
		t.Fatalf("reference failed: %+v", issues)
	}
	if issues := ValidateAcceptance(workflow.Workflow{AcceptanceContract: "unknown"}); len(issues) == 0 {
		t.Fatal("unknown contract accepted")
	}
}

func TestRecoveryAcceptanceRequiresExactTimingAndDynamicPersonalization(t *testing.T) {
	w := recoveryEUR().Workflow
	for i := range w.Nodes {
		if w.Nodes[i].Type == "wait" {
			w.Nodes[i].Config.DurationSeconds = 7200
		}
	}
	if issues := ValidateAcceptance(w); len(issues) == 0 {
		t.Fatal("wrong 1h timing accepted")
	}
	w = recoveryEUR().Workflow
	for i := range w.Nodes {
		if w.Nodes[i].Type == "sms" {
			w.Nodes[i].Config.Text = "Hello Alex, 89.90 COMEBACK10"
		}
	}
	if issues := ValidateAcceptance(w); len(issues) == 0 {
		t.Fatal("hardcoded personalization accepted")
	}
}
