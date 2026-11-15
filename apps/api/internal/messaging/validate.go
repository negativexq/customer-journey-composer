package messaging

import "github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"

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
		// An SMS that drops its code, or a message showing a code nobody set,
		// costs the campaign its point.
		uses := UsesVariable(node.Config, DiscountCode)
		switch code := node.Config.DiscountCode; {
		case code != "" && !ValidDiscountCode(code):
			issues = append(issues, workflow.Issue{NodeID: node.ID, Field: "discount_code", Message: "A discount code is 3 to 30 capital letters, digits and hyphens"})
		case code != "" && !uses && node.Type == "sms":
			// An email shows the code in its own box; an SMS has only its text.
			issues = append(issues, workflow.Issue{NodeID: node.ID, Field: "discount_code", Message: "Show the discount code in the message with {{ offer.code }}"})
		case code == "" && uses:
			issues = append(issues, workflow.Issue{NodeID: node.ID, Field: "discount_code", Message: "Set the discount code this message shows"})
		}
	}
	return issues
}
