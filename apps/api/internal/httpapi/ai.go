package httpapi

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	composer "github.com/negativexq/customer-journey-composer/apps/api/internal/ai"
)

func registerAI(api huma.API) {
	engine, err := composer.New(composer.ProvidersFromEnv())
	if err != nil {
		panic("invalid checked-in AI schema")
	}
	var active sync.Map
	huma.Get(api, "/api/v1/settings/ai", func(ctx context.Context, _ *struct{}) (*Result[map[string]any], error) {
		enabled := os.Getenv("AI_PROVIDER") != "" && os.Getenv("AI_PROVIDER") != "disabled"
		return &Result[map[string]any]{Body: map[string]any{"enabled": enabled, "default_provider": os.Getenv("AI_PROVIDER"), "schema_version": composer.SchemaVersion, "providers": map[string]any{"claude": map[string]any{"configured": os.Getenv("ANTHROPIC_API_KEY") != "" && os.Getenv("ANTHROPIC_MODEL") != "", "model": os.Getenv("ANTHROPIC_MODEL")}, "openai": map[string]any{"configured": os.Getenv("OPENAI_API_KEY") != "" && os.Getenv("OPENAI_MODEL") != "", "model": os.Getenv("OPENAI_MODEL")}}}}, nil
	})
	handle := func(ctx context.Context, in *struct{ Body composer.Input }) (*Result[composer.Proposal], error) {
		if err := canEdit(ctx); err != nil {
			return nil, err
		}
		if os.Getenv("AI_PROVIDER") == "" || os.Getenv("AI_PROVIDER") == "disabled" {
			return nil, huma.Error503ServiceUnavailable("AI is disabled. Configure a server-side provider and model when ready for live generation")
		}
		key := who(ctx).WorkspaceID
		if _, busy := active.LoadOrStore(key, true); busy {
			return nil, huma.Error429TooManyRequests("An AI request is already running for this workspace")
		}
		defer active.Delete(key)
		request, cancel := context.WithTimeout(ctx, 85*time.Second)
		defer cancel()
		proposal, err := engine.Compose(request, in.Body)
		if err != nil {
			var invalid *composer.InvalidOutput
			var external *composer.ProviderError
			if errors.As(err, &invalid) {
				return nil, huma.Error422UnprocessableEntity("AI output did not pass schema and workflow validation after two attempts; refine the request or edit manually")
			}
			if errors.Is(err, context.DeadlineExceeded) || request.Err() != nil {
				return nil, huma.Error504GatewayTimeout("AI request timed out")
			}
			if errors.As(err, &external) {
				if external.Code == "provider_not_configured" {
					return nil, huma.Error503ServiceUnavailable("Configure the selected provider key and model on the server")
				}
				if external.Code == "provider_refusal" {
					return nil, huma.Error422UnprocessableEntity("The provider declined this request")
				}
				if external.Status == 429 {
					return nil, huma.Error429TooManyRequests("AI provider is rate limited; retry later")
				}
				return nil, huma.Error502BadGateway("AI provider failed or returned an incomplete response")
			}
			return nil, huma.Error422UnprocessableEntity(err.Error())
		}
		return &Result[composer.Proposal]{Body: proposal}, nil
	}
	huma.Post(api, "/api/v1/ai/journeys/generate", handle)
	huma.Post(api, "/api/v1/ai/journeys/modify", handle)
	huma.Post(api, "/api/v1/ai/messages/generate", handle)
}
