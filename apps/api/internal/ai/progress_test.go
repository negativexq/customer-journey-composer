package ai

import (
	"context"
	"reflect"
	"testing"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

func TestProgressReportsActualRepairBoundariesForBothProviders(t *testing.T) {
	for _, name := range []string{"claude", "openai"} {
		t.Run(name, func(t *testing.T) {
			provider := &sequence{outputs: []string{`{}`, validOutput}}
			engine, err := New(map[string]Provider{name: provider})
			if err != nil {
				t.Fatal(err)
			}
			var events []Progress
			ctx := WithProgress(context.Background(), func(p Progress) { events = append(events, p) })
			_, err = engine.Compose(ctx, Input{Provider: name, Action: "generate", Prompt: "Create", Workflow: workflow.Workflow{SchemaVersion: 1}})
			if err != nil {
				t.Fatal(err)
			}
			want := []Progress{{"generating", 1}, {"validating", 1}, {"repairing", 1}, {"generating", 2}, {"validating", 2}}
			if !reflect.DeepEqual(events, want) || provider.calls != 2 {
				t.Fatalf("events=%v calls=%d", events, provider.calls)
			}
		})
	}
}
