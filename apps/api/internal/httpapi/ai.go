package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	composer "github.com/negativexq/customer-journey-composer/apps/api/internal/ai"
)

func registerAI(api huma.API, pool *pgxpool.Pool) {
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
		entries, catalogErr := loadDataCatalog(request, pool)
		if catalogErr != nil {
			return nil, catalogErr
		}
		if in.Body.Action != "explain_journey" && in.Body.Action != "explain_node" && in.Body.Action != "generate_message" && in.Body.Action != "improve_message" {
			hasEvent := false
			for _, entry := range entries {
				if entry.Enabled && entry.Kind == "event" {
					hasEvent = true
				}
			}
			if !hasEvent {
				return nil, huma.Error422UnprocessableEntity("Define or discover workspace events in Settings → Workspace data before generating a journey")
			}
		}
		request = composer.WithCatalog(request, entries)
		proposal, err := engine.Compose(request, in.Body)
		if proposal.Generation != nil {
			auditContext, auditCancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
			auditErr := recordGeneration(auditContext, pool, who(ctx), in.Body.Provider, in.Body.Action, proposal.Generation, err == nil)
			auditCancel()
			if auditErr != nil {
				return nil, huma.Error500InternalServerError("Generation completed but accounting could not be recorded; do not automatically retry")
			}
		}
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
	// Existing JSON routes remain compatible; streaming is explicitly opt-in.
	stream := func(ctx context.Context, in *struct{ Body composer.Input }) (*huma.StreamResponse, error) {
		if err := canEdit(ctx); err != nil {
			return nil, err
		}
		return &huma.StreamResponse{Body: func(output huma.Context) {
			output.SetHeader("Content-Type", "text/event-stream")
			output.SetHeader("Cache-Control", "no-store")
			output.SetHeader("X-Accel-Buffering", "no")
			output.SetStatus(200)
			streamGeneration(ctx, output.BodyWriter(), func(request context.Context) (composer.Proposal, error) {
				result, err := handle(request, in)
				if err != nil {
					return composer.Proposal{}, err
				}
				return result.Body, nil
			})
		}}, nil
	}
	huma.Post(api, "/api/v1/ai/journeys/generate/stream", stream)
	huma.Post(api, "/api/v1/ai/journeys/modify/stream", stream)
	huma.Post(api, "/api/v1/ai/messages/generate/stream", stream)
}

// No prompt, draft, customer content, provider body or private reasoning is persisted.
func recordGeneration(ctx context.Context, pool *pgxpool.Pool, identity Identity, provider, action string, report *composer.GenerationReport, accepted bool) error {
	raw, err := json.Marshal(map[string]any{"provider": provider, "action": action, "accepted": accepted, "generation": report})
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `INSERT INTO audit_logs(workspace_id,actor_id,action,details) VALUES($1,$2,'ai.generation',$3)`, identity.WorkspaceID, identity.ID, raw)
	return err
}
