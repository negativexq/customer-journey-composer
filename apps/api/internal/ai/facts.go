package ai

import (
	"reflect"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/factcheck"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

// generatedMessages are the email and SMS steps the model wrote or rewrote in
// this answer. A step a person already had, unchanged, is theirs and is not
// checked.
func generatedMessages(before, after workflow.Workflow, only string) map[string]bool {
	old := map[string]workflow.Config{}
	for _, n := range before.Nodes {
		old[n.ID] = n.Config
	}
	steps := map[string]bool{}
	for _, n := range after.Nodes {
		if n.Type != "email" && n.Type != "sms" || only != "" && n.ID != only {
			continue
		}
		previous, existed := old[n.ID]
		if !existed || !reflect.DeepEqual(messageFields(previous), messageFields(n.Config)) {
			steps[n.ID] = true
		}
	}
	return steps
}

func messageFields(c workflow.Config) [5]string {
	return [5]string{c.Subject, c.Preheader, c.HTML, c.Text, c.DiscountCode}
}

// givenFacts are what generated text may state: the request, the journey's
// own name and description, and every message a person already has.
func givenFacts(in Input) factcheck.Facts {
	texts := []string{in.Prompt, in.Name, in.Description}
	for _, n := range in.Workflow.Nodes {
		if n.Type == "email" || n.Type == "sms" {
			c := n.Config
			texts = append(texts, c.Subject, c.Preheader, c.HTML, c.Text, c.DiscountCode)
		}
	}
	return factcheck.Given(texts...)
}

// factIssues refuses generated text that states a link, code or number the
// request does not, the same way a schema failure is refused.
func factIssues(in Input, proposed workflow.Workflow, only string) []workflow.Issue {
	steps := generatedMessages(in.Workflow, proposed, only)
	if len(steps) == 0 {
		return nil
	}
	return factcheck.Check(givenFacts(in), proposed, steps)
}
