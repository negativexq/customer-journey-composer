package factcheck

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

// The ten journeys gpt-6-luna wrote on 2026-11-15 were reviewed by hand; they
// state nothing their brief does not, so the check must not refuse them.
func TestTheReviewedLiveJourneysPass(t *testing.T) {
	raw, err := os.ReadFile("../../../../docs/validation/live-engagement-2026-11-15/result.json")
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Journeys []struct {
			ID       string `json:"id"`
			Brief    string `json:"brief"`
			Proposal struct {
				Workflow workflow.Workflow `json:"workflow"`
			} `json:"proposal"`
		} `json:"journeys"`
	}
	if err := json.Unmarshal(raw, &result); err != nil || len(result.Journeys) != 10 {
		t.Fatal(err, len(result.Journeys))
	}
	for _, j := range result.Journeys {
		if issues := Check(Given(j.Brief), j.Proposal.Workflow, nil); len(issues) != 0 {
			t.Errorf("%s: %+v", j.ID, issues)
		}
		if notes := NotDone(j.Brief, j.Proposal.Workflow); len(notes) != 0 {
			t.Errorf("%s notes: %v", j.ID, notes)
		}
	}
}
