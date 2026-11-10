package catalog

import (
	"github.com/negativexq/customer-journey-composer/apps/api/internal/messaging"
	"testing"
)

func TestBuiltinTemplatesPassAllGraphAndMessageRules(t *testing.T) {
	for _, template := range Builtins() {
		if issues := messaging.ValidateWorkflow(template.Workflow); len(issues) > 0 {
			t.Fatalf("%s: %v", template.ID, issues)
		}
	}
}
