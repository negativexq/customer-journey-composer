package ai

import (
	"context"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/semantic"
	"strings"
	"testing"
)

type catalogProvider struct{ t *testing.T }

func (p catalogProvider) Generate(_ context.Context, r Request) (ProviderResult, error) {
	if !strings.Contains(r.System, "Workspace vocabulary semantic.v1") || !strings.Contains(r.System, "Cart changed") || !strings.Contains(r.System, "Only used internally") {
		p.t.Fatal("catalog not sent to shared provider interface")
	}
	return ProviderResult{Output: []byte(validOutput)}, nil
}
func TestBothProvidersUseServerCatalogAndReportUnknownReferences(t *testing.T) {
	for _, provider := range []string{"openai", "claude"} {
		t.Run(provider, func(t *testing.T) {
			c, err := New(map[string]Provider{provider: catalogProvider{t}})
			if err != nil {
				t.Fatal(err)
			}
			entries := []semantic.Entry{{Key: semantic.Key("event", "other_event", ""), Kind: "event", EventName: "other_event", Label: "Cart changed", Description: "Only used internally", DataType: "event", Enabled: true, Aliases: []string{}}}
			proposal, err := c.Compose(WithCatalog(context.Background(), entries), Input{Provider: provider, Action: "generate", Prompt: "When the cart changes, finish"})
			if err != nil {
				t.Fatal(err)
			}
			if proposal.Grounding == nil || proposal.Grounding.Ready || proposal.Grounding.Mappings[0].Status != "missing" {
				t.Fatal("invented event not identified")
			}
		})
	}
}
