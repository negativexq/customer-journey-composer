package ai

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

const validOutput = `{"name":"Cart recovery","description":"A deterministic cart journey","explanation":"Starts on a cart event and completes.","warnings":[],"workflow":{"schema_version":1,"nodes":[{"id":"start","type":"event_trigger","label":"Cart","position":{"x":0,"y":0},"config":{"event_name":"cart_updated","filters":null}},{"id":"done","type":"exit","label":"Complete","position":{"x":0,"y":200},"config":{"reason":"completed"}}],"edges":[{"id":"edge","source":"start","target":"done","port":"started"}],"global_exit":null,"reentry":null}}`

type sequence struct {
	outputs []string
	err     error
	calls   int
}

func (s *sequence) Generate(context.Context, Request) (ProviderResult, error) {
	s.calls++
	if s.err != nil {
		return ProviderResult{}, s.err
	}
	index := min(s.calls-1, len(s.outputs)-1)
	return ProviderResult{Output: []byte(s.outputs[index])}, nil
}
func TestSchemaArtifactAndCrossProviderSubset(t *testing.T) {
	want, _ := json.MarshalIndent(Schema("journey"), "", "  ")
	want = append(want, '\n')
	got, err := os.ReadFile("../../../../docs/schemas/composer.v2.json")
	if err != nil {
		t.Fatal(err)
	}
	var document any
	json.Unmarshal(got, &document)
	canonicalGot, _ := json.Marshal(document)
	canonicalWant, _ := json.Marshal(Schema("journey"))
	if !bytes.Equal(canonicalWant, canonicalGot) {
		t.Fatal("schema artifact drift; regenerate with go run ./cmd/schema")
	}
	if _, err := New(nil); err != nil {
		t.Fatal(err)
	}
	var visit func(any)
	visit = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			if v["type"] == "object" {
				if v["additionalProperties"] != false {
					t.Fatal("open object in strict schema")
				}
				if len(v["properties"].(map[string]any)) != len(v["required"].([]any)) {
					t.Fatal("all fields must be required")
				}
			}
			for _, child := range v {
				visit(child)
			}
		case []any:
			for _, child := range v {
				visit(child)
			}
		}
	}
	var decoded any
	json.Unmarshal(want, &decoded)
	visit(decoded)
}
func TestComposerRepairsAtMostOnceAndNeverMutatesInput(t *testing.T) {
	bad := strings.Replace(validOutput, `"target":"done"`, `"target":"missing"`, 1)
	provider := &sequence{outputs: []string{bad, validOutput}}
	composer, err := New(map[string]Provider{"claude": provider})
	if err != nil {
		t.Fatal(err)
	}
	in := Input{Provider: "claude", Action: "generate", Prompt: "Build cart recovery", Name: "Draft", Workflow: workflow.Workflow{SchemaVersion: 1, Nodes: []workflow.Node{}, Edges: []workflow.Edge{}}}
	before, _ := json.Marshal(in)
	proposal, err := composer.Compose(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(in)
	if !bytes.Equal(before, after) {
		t.Fatal("composer mutated the draft")
	}
	if provider.calls != 2 || len(proposal.Diff.AddedNodes) != 2 || len(workflow.Validate(proposal.Workflow)) != 0 {
		t.Fatal(proposal, provider.calls)
	}
	provider = &sequence{outputs: []string{bad}}
	composer, _ = New(map[string]Provider{"openai": provider})
	in.Provider = "openai"
	if _, err := composer.Compose(context.Background(), in); err == nil || provider.calls != 2 {
		t.Fatal("invalid output was accepted or retries were unbounded", provider.calls, err)
	}
}
func TestRejectsUnknownTypesInvalidJSONAndRefusals(t *testing.T) {
	for _, output := range []string{"not JSON", strings.Replace(validOutput, "event_trigger", "run_code", 1), strings.Replace(validOutput, `"schema_version":1`, `"schema_version":99`, 1)} {
		provider := &sequence{outputs: []string{output}}
		composer, err := New(map[string]Provider{"claude": provider})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := composer.Compose(context.Background(), Input{Provider: "claude", Action: "generate", Prompt: "Test"}); err == nil || provider.calls != 2 {
			t.Fatal("malformed model output accepted")
		}
	}
	provider := &sequence{err: &ProviderError{Code: "provider_refusal"}}
	composer, _ := New(map[string]Provider{"openai": provider})
	_, err := composer.Compose(context.Background(), Input{Provider: "openai", Action: "generate", Prompt: "Test"})
	if err == nil || provider.calls != 1 {
		t.Fatal("refusal must not be repaired", provider.calls)
	}
}
func TestMessageGenerationPreservesPartialDraftAndNormalizesFallbacks(t *testing.T) {
	provider := &sequence{outputs: []string{`{"subject":"Hello {{ customer.first_name }}","preheader":"Welcome","html":"<p>Welcome</p>","text":"Welcome","fallbacks":[{"path":"customer.first_name","value":"friend"}],"explanation":"A welcome draft.","warnings":[]}`}}
	composer, _ := New(map[string]Provider{"claude": provider})
	flow := workflow.Workflow{SchemaVersion: 1, Nodes: []workflow.Node{{ID: "message", Type: "email", Label: "Draft", Config: workflow.Config{Subject: "Old", Text: "Old"}}}, Edges: []workflow.Edge{}}
	proposal, err := composer.Compose(context.Background(), Input{Provider: "claude", Action: "improve_message", Prompt: "Improve welcome", Workflow: flow, NodeID: "message"})
	if err != nil {
		t.Fatal(err)
	}
	if flow.Nodes[0].Config.Subject != "Old" || proposal.Workflow.Nodes[0].Config.Fallbacks["customer.first_name"] != "friend" || len(proposal.Diff.ChangedMessages) != 1 {
		t.Fatal(proposal)
	}
}
func TestHTTPProviderWireContractsWithoutLiveLLMRequests(t *testing.T) {
	for _, kind := range []string{"openai", "claude"} {
		t.Run(kind, func(t *testing.T) {
			secret := make([]byte, 24)
			rand.Read(secret)
			key := hex.EncodeToString(secret)
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var request map[string]any
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
				}
				if request["model"] != "configured-model" {
					t.Error("model selection was substituted")
				}
				if kind == "openai" {
					if r.URL.Path != "/v1/responses" || r.Header.Get("Authorization") != "Bearer "+key || request["store"] != false {
						t.Error("invalid OpenAI request")
					}
					format := request["text"].(map[string]any)["format"].(map[string]any)
					if format["strict"] != true || format["type"] != "json_schema" {
						t.Error("structured output missing")
					}
					json.NewEncoder(w).Encode(map[string]any{"status": "completed", "output": []any{map[string]any{"type": "message", "content": []any{map[string]any{"type": "output_text", "text": validOutput}}}}})
				} else {
					if r.URL.Path != "/v1/messages" || r.Header.Get("x-api-key") != key || r.Header.Get("anthropic-version") == "" {
						t.Error("invalid Claude request")
					}
					if request["output_config"].(map[string]any)["format"].(map[string]any)["type"] != "json_schema" {
						t.Error("structured output missing")
					}
					json.NewEncoder(w).Encode(map[string]any{"stop_reason": "end_turn", "content": []any{map[string]any{"type": "text", "text": validOutput}}})
				}
			}))
			defer server.Close()
			provider := &HTTPProvider{Kind: kind, Key: key, Model: "configured-model", BaseURL: server.URL, Client: server.Client()}
			out, err := provider.Generate(context.Background(), Request{Kind: "journey", System: systemPrompt, Prompt: "No live customer profiles"})
			if err != nil || string(out.Output) != validOutput || calls != 1 {
				t.Fatal("provider contract failed", err, calls)
			}
		})
	}
}
func TestProviderErrorsNeverExposeRawBody(t *testing.T) {
	marker := "private-provider-detail"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401); io.WriteString(w, marker) }))
	defer server.Close()
	provider := &HTTPProvider{Kind: "openai", Key: "", Model: "configured-model", BaseURL: server.URL}
	random := make([]byte, 24)
	rand.Read(random)
	provider.Key = hex.EncodeToString(random)
	_, err := provider.Generate(context.Background(), Request{Kind: "journey"})
	if err == nil || strings.Contains(err.Error(), marker) || strings.Contains(err.Error(), provider.Key) {
		t.Fatal("provider secret/error body was exposed")
	}
}

func TestCancellationAndLongRetryAfterDoNotCreateExtraRequests(t *testing.T) {
	provider := &sequence{outputs: []string{validOutput}}
	composer, _ := New(map[string]Provider{"openai": provider})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := composer.Compose(ctx, Input{Provider: "openai", Action: "generate", Prompt: "Test"}); err == nil || provider.calls != 0 {
		t.Fatal("cancelled request reached provider")
	}
	limited := &sequence{err: &ProviderError{Code: "provider_http_error", Status: 429, Temporary: true, RetryAfter: 30 * time.Second}}
	composer, _ = New(map[string]Provider{"openai": limited})
	if _, err := composer.Compose(context.Background(), Input{Provider: "openai", Action: "generate", Prompt: "Test"}); err == nil || limited.calls != 1 {
		t.Fatal("long Retry-After was ignored")
	}
}
func TestIncompleteProviderResponseIsNeverAProposal(t *testing.T) {
	for _, kind := range []string{"openai", "claude"} {
		t.Run(kind, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if kind == "openai" {
					json.NewEncoder(w).Encode(map[string]any{"status": "incomplete", "output": []any{map[string]any{"content": []any{map[string]any{"type": "output_text", "text": validOutput}}}}})
				} else {
					json.NewEncoder(w).Encode(map[string]any{"stop_reason": "max_tokens", "content": []any{map[string]any{"type": "text", "text": validOutput}}})
				}
			}))
			defer server.Close()
			key := make([]byte, 24)
			rand.Read(key)
			provider := &HTTPProvider{Kind: kind, Key: hex.EncodeToString(key), Model: "configured-model", BaseURL: server.URL}
			_, err := provider.Generate(context.Background(), Request{Kind: "journey"})
			var external *ProviderError
			if !errors.As(err, &external) || external.Code != "provider_output_incomplete" {
				t.Fatal("incomplete response accepted", err)
			}
		})
	}
}
