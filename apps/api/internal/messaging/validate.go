package messaging

import "github.com/ofk/customer-journey-composer/apps/api/internal/workflow"

// ValidateWorkflow checks graph rules and statically renderable message syntax.
// Missing customer values remain a preview/eligibility decision, not a reason
// to send private profile data to an AI provider during workflow generation.
func ValidateWorkflow(w workflow.Workflow) []workflow.Issue {
	issues := workflow.Validate(w)
	for _, node := range w.Nodes {
		if node.Type != "email" && node.Type != "sms" {
			continue
		}
		_, err := Render(node.Type, node.Config, map[string]any{"customer": map[string]any{}, "event": map[string]any{}, "context": map[string]any{}})
		if err != nil {
			issues = append(issues, workflow.Issue{NodeID: node.ID, Field: "content", Message: err.Error()})
		}
	}
	return issues
}
