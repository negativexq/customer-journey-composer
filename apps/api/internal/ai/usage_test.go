package ai

import (
	"context"
	"encoding/json"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUsageIncludesRejectedCandidateAndSuccessfulRepair(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		text := validOutput
		if calls == 1 {
			text = strings.Replace(validOutput, `"target":"done"`, `"target":"missing"`, 1)
		}
		w.Header().Set("x-request-id", "request-fixture")
		json.NewEncoder(w).Encode(map[string]any{"model": "reported-model", "status": "completed", "usage": map[string]any{"input_tokens": 100, "output_tokens": 20, "input_tokens_details": map[string]any{"cached_tokens": 10}, "output_tokens_details": map[string]any{"reasoning_tokens": 2}}, "output": []any{map[string]any{"content": []any{map[string]any{"type": "output_text", "text": text}}}}})
	}))
	defer server.Close()
	c, _ := New(map[string]Provider{"openai": &HTTPProvider{Kind: "openai", Key: "fixture-only", Model: "requested-model", BaseURL: server.URL, Client: server.Client()}})
	p, err := c.Compose(context.Background(), Input{Provider: "openai", Action: "generate", Prompt: "No customer data", Workflow: workflow.Workflow{SchemaVersion: 1, Nodes: []workflow.Node{}, Edges: []workflow.Edge{}}})
	if err != nil {
		t.Fatal(err)
	}
	g := p.Generation
	if g == nil || len(g.Attempts) != 2 || g.Usage == nil || g.Usage.InputTokens != 200 || g.Usage.OutputTokens != 40 || *g.Usage.ReasoningTokens != 4 || *g.Usage.CachedInputTokens != 20 {
		t.Fatalf("usage lost: %+v", g)
	}
	if g.Attempts[0].Accepted || !g.Attempts[1].Accepted || g.Attempts[1].Model != "reported-model" || g.Attempts[1].RequestedModel != "requested-model" {
		t.Fatalf("attempt/model metadata lost: %+v", g.Attempts)
	}
	raw, _ := json.Marshal(g)
	if strings.Contains(string(raw), "No customer data") || strings.Contains(string(raw), "schema_version") || strings.Contains(string(raw), "fixture-only") {
		t.Fatal("accounting retained prompt/output/secret")
	}
}
func TestUsageUnavailableIsNotZeroAndClaudeCacheInputsAreIncluded(t *testing.T) {
	report := &GenerationReport{Attempts: []Attempt{{ProviderResult: ProviderResult{Usage: &Usage{InputTokens: 10, OutputTokens: 5}}}, {ProviderResult: ProviderResult{Usage: nil}}}}
	report.finish()
	if report.Usage != nil {
		t.Fatal("partial usage reported as complete")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"model": "claude-reported", "stop_reason": "end_turn", "usage": map[string]any{"input_tokens": 100, "output_tokens": 20, "cache_read_input_tokens": 30, "cache_creation_input_tokens": 40}, "content": []any{map[string]any{"type": "text", "text": "{}"}}})
	}))
	defer server.Close()
	out, err := (&HTTPProvider{Kind: "claude", Key: "fixture-only", Model: "claude-requested", BaseURL: server.URL, Client: server.Client()}).Generate(context.Background(), Request{Kind: "journey"})
	if err != nil || out.Usage == nil || out.Usage.InputTokens != 170 || *out.Usage.CachedInputTokens != 30 || *out.Usage.CacheCreationTokens != 40 || out.Usage.ReasoningTokens != nil {
		t.Fatalf("Claude usage incorrect: %+v %v", out, err)
	}
}
func TestFailedCompositionStillReturnsAttemptUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"status": "completed", "usage": map[string]any{"input_tokens": 10, "output_tokens": 4}, "output": []any{map[string]any{"content": []any{map[string]any{"type": "output_text", "text": "{}"}}}}})
	}))
	defer server.Close()
	c, _ := New(map[string]Provider{"openai": &HTTPProvider{Kind: "openai", Key: "fixture-only", Model: "model", BaseURL: server.URL, Client: server.Client()}})
	p, err := c.Compose(context.Background(), Input{Provider: "openai", Action: "generate", Prompt: "Generate"})
	if err == nil || p.Generation == nil || p.Generation.Usage == nil || p.Generation.Usage.InputTokens != 20 || len(p.Generation.Attempts) != 2 {
		t.Fatal("failed attempts were not accounted for", err, p.Generation)
	}
}
