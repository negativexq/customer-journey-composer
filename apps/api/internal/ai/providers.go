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
	Generate(context.Context, Request) ([]byte, error)
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

func (p *HTTPProvider) Generate(ctx context.Context, in Request) ([]byte, error) {
	if p.Key == "" || p.Model == "" {
		return nil, &ProviderError{Code: "provider_not_configured"}
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
		return nil, &ProviderError{Code: "unsupported_provider"}
	}
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(p.BaseURL, "/")+path, bytes.NewReader(data))
	if err != nil {
		return nil, &ProviderError{Code: "invalid_provider_configuration"}
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
		return nil, &ProviderError{Code: "provider_request_failed", Temporary: false}
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, (2<<20)+1))
	if err != nil || len(raw) > 2<<20 {
		return nil, &ProviderError{Code: "invalid_provider_response"}
	}
	if response.StatusCode != 200 {
		seconds, _ := strconv.Atoi(response.Header.Get("Retry-After"))
		if seconds < 0 {
			seconds = 0
		}
		if seconds > 120 {
			seconds = 120
		}
		return nil, &ProviderError{Code: "provider_http_error", Status: response.StatusCode, Temporary: response.StatusCode == 429 || response.StatusCode >= 500, RetryAfter: time.Duration(seconds) * time.Second}
	}
	if p.Kind == "openai" {
		var result struct {
			Status string `json:"status"`
			Output []struct {
				Type    string `json:"type"`
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"output"`
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil, &ProviderError{Code: "invalid_provider_response"}
		}
		if result.Status != "completed" {
			return nil, &ProviderError{Code: "provider_output_incomplete"}
		}
		for _, item := range result.Output {
			for _, part := range item.Content {
				if part.Type == "refusal" {
					return nil, &ProviderError{Code: "provider_refusal"}
				}
				if part.Type == "output_text" {
					return []byte(part.Text), nil
				}
			}
		}
	} else {
		var result struct {
			StopReason string `json:"stop_reason"`
			Content    []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil, &ProviderError{Code: "invalid_provider_response"}
		}
		if result.StopReason == "refusal" {
			return nil, &ProviderError{Code: "provider_refusal"}
		}
		if result.StopReason != "end_turn" {
			return nil, &ProviderError{Code: "provider_output_incomplete"}
		}
		for _, part := range result.Content {
			if part.Type == "text" {
				return []byte(part.Text), nil
			}
		}
	}
	return nil, &ProviderError{Code: "provider_returned_no_text"}
}
func ProvidersFromEnv() map[string]Provider {
	return map[string]Provider{"openai": &HTTPProvider{Kind: "openai", Key: os.Getenv("OPENAI_API_KEY"), Model: os.Getenv("OPENAI_MODEL"), BaseURL: "https://api.openai.com"}, "claude": &HTTPProvider{Kind: "claude", Key: os.Getenv("ANTHROPIC_API_KEY"), Model: os.Getenv("ANTHROPIC_MODEL"), BaseURL: "https://api.anthropic.com"}}
}
