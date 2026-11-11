package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type Request struct{ System, Prompt, Kind string }
type Provider interface {
	Generate(context.Context, Request) (ProviderResult, error)
}
type ProviderError struct {
	Code       string
	Status     int
	Temporary  bool
	RetryAfter time.Duration
}

func (e *ProviderError) Error() string {
	if e.Status != 0 {
		return fmt.Sprintf("AI provider returned HTTP %d", e.Status)
	}
	return e.Code
}

type HTTPProvider struct {
	Kind, Key, Model, BaseURL string
	Client                    *http.Client
}

func (p *HTTPProvider) Generate(ctx context.Context, in Request) (result ProviderResult, err error) {
	started := time.Now()
	result.RequestedModel = p.Model
	defer func() { result.DurationMS = time.Since(started).Milliseconds() }()
	if p.Key == "" || p.Model == "" {
		return result, &ProviderError{Code: "provider_not_configured"}
	}
	var body map[string]any
	path := "/v1/responses"
	switch p.Kind {
	case "openai":
		body = map[string]any{"model": p.Model, "store": false, "max_output_tokens": 6000, "input": []any{map[string]any{"role": "system", "content": in.System}, map[string]any{"role": "user", "content": in.Prompt}}, "text": map[string]any{"format": map[string]any{"type": "json_schema", "name": strings.ReplaceAll(SchemaVersion, ".", "_") + "_" + in.Kind, "strict": true, "schema": Schema(in.Kind)}}}
	case "claude":
		path = "/v1/messages"
		body = map[string]any{"model": p.Model, "max_tokens": 6000, "system": in.System, "messages": []any{map[string]any{"role": "user", "content": in.Prompt}}, "output_config": map[string]any{"format": map[string]any{"type": "json_schema", "schema": Schema(in.Kind)}}}
	default:
		return result, &ProviderError{Code: "unsupported_provider"}
	}
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(p.BaseURL, "/")+path, bytes.NewReader(data))
	if err != nil {
		return result, &ProviderError{Code: "invalid_provider_configuration"}
	}
	req.Header.Set("Content-Type", "application/json")
	if p.Kind == "openai" {
		req.Header.Set("Authorization", "Bearer "+p.Key)
	} else {
		req.Header.Set("x-api-key", p.Key)
		req.Header.Set("anthropic-version", "2023-06-01")
	}
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 45 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	response, err := client.Do(req)
	if err != nil {
		return result, &ProviderError{Code: "provider_request_failed", Temporary: false}
	}
	defer response.Body.Close()
	result.HTTPStatus = response.StatusCode
	result.RequestID = response.Header.Get("x-request-id")
	if result.RequestID == "" {
		result.RequestID = response.Header.Get("request-id")
	}
	if len(result.RequestID) > 200 || strings.Contains(result.RequestID, p.Key) {
		result.RequestID = ""
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, (2<<20)+1))
	if err != nil || len(raw) > 2<<20 {
		return result, &ProviderError{Code: "invalid_provider_response"}
	}
	var envelope struct {
		Model string `json:"model"`
		Usage *struct {
			Input        *int64 `json:"input_tokens"`
			Output       *int64 `json:"output_tokens"`
			Read         *int64 `json:"cache_read_input_tokens"`
			Create       *int64 `json:"cache_creation_input_tokens"`
			InputDetails struct {
				Cached *int64 `json:"cached_tokens"`
				Write  *int64 `json:"cache_write_tokens"`
			} `json:"input_tokens_details"`
			OutputDetails struct {
				Reasoning *int64 `json:"reasoning_tokens"`
			} `json:"output_tokens_details"`
		} `json:"usage"`
	}
	if json.Unmarshal(raw, &envelope) == nil {
		if len(envelope.Model) <= 200 && !strings.Contains(envelope.Model, p.Key) {
			result.Model = envelope.Model
		}
		u := envelope.Usage
		if u != nil && u.Input != nil && u.Output != nil && *u.Input >= 0 && *u.Output >= 0 {
			result.Usage = &Usage{InputTokens: *u.Input, OutputTokens: *u.Output, ReasoningTokens: u.OutputDetails.Reasoning, CachedInputTokens: u.InputDetails.Cached, CacheCreationTokens: u.InputDetails.Write}
			if p.Kind == "claude" {
				result.Usage.CachedInputTokens = u.Read
				result.Usage.CacheCreationTokens = u.Create
				if u.Read != nil {
					result.Usage.InputTokens += *u.Read
				}
				if u.Create != nil {
					result.Usage.InputTokens += *u.Create
				}
			}
		}
	}
	if response.StatusCode != 200 {
		seconds, _ := strconv.Atoi(response.Header.Get("Retry-After"))
		if seconds < 0 {
			seconds = 0
		}
		if seconds > 120 {
			seconds = 120
		}
		return result, &ProviderError{Code: "provider_http_error", Status: response.StatusCode, Temporary: response.StatusCode == 429 || response.StatusCode >= 500, RetryAfter: time.Duration(seconds) * time.Second}
	}
	if p.Kind == "openai" {
		var decoded struct {
			Status string `json:"status"`
			Output []struct {
				Type    string `json:"type"`
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"output"`
		}
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return result, &ProviderError{Code: "invalid_provider_response"}
		}
		if decoded.Status != "completed" {
			return result, &ProviderError{Code: "provider_output_incomplete"}
		}
		for _, item := range decoded.Output {
			for _, part := range item.Content {
				if part.Type == "refusal" {
					return result, &ProviderError{Code: "provider_refusal"}
				}
				if part.Type == "output_text" {
					result.Output = []byte(part.Text)
					return result, nil
				}
			}
		}
	} else {
		var decoded struct {
			StopReason string `json:"stop_reason"`
			Content    []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return result, &ProviderError{Code: "invalid_provider_response"}
		}
		if decoded.StopReason == "refusal" {
			return result, &ProviderError{Code: "provider_refusal"}
		}
		if decoded.StopReason != "end_turn" {
			return result, &ProviderError{Code: "provider_output_incomplete"}
		}
		for _, part := range decoded.Content {
			if part.Type == "text" {
				result.Output = []byte(part.Text)
				return result, nil
			}
		}
	}
	return result, &ProviderError{Code: "provider_returned_no_text"}
}
func ProvidersFromEnv() map[string]Provider {
	return map[string]Provider{"openai": &HTTPProvider{Kind: "openai", Key: os.Getenv("OPENAI_API_KEY"), Model: os.Getenv("OPENAI_MODEL"), BaseURL: "https://api.openai.com"}, "claude": &HTTPProvider{Kind: "claude", Key: os.Getenv("ANTHROPIC_API_KEY"), Model: os.Getenv("ANTHROPIC_MODEL"), BaseURL: "https://api.anthropic.com"}}
}
