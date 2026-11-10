package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Readiness func(context.Context) error

type HealthBody struct {
	Status  string `json:"status" enum:"ok,ready" doc:"Process or dependency status"`
	Service string `json:"service"`
}

type HealthResponse struct {
	Body HealthBody
}

func New(log *slog.Logger, ready Readiness, pools ...*pgxpool.Pool) http.Handler {
	mux := http.NewServeMux()
	cfg := huma.DefaultConfig("Customer Journey Composer API", "0.1.0")
	cfg.OpenAPIPath = "/api/openapi"
	cfg.DocsPath = "/api/docs"
	cfg.SchemasPath = "/api/schemas"
	api := humago.New(mux, cfg)
	huma.Get(api, "/healthz", func(_ context.Context, _ *struct{}) (*HealthResponse, error) {
		return &HealthResponse{Body: HealthBody{Status: "ok", Service: "journey-api"}}, nil
	}, operation("liveness", "Process liveness"))
	huma.Get(api, "/readyz", func(ctx context.Context, _ *struct{}) (*HealthResponse, error) {
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := ready(ctx); err != nil {
			return nil, huma.Error503ServiceUnavailable("Database or schema is not ready")
		}
		return &HealthResponse{Body: HealthBody{Status: "ready", Service: "journey-api"}}, nil
	}, operation("readiness", "Database and schema readiness"))
	if len(pools) > 0 {
		registerDomain(api, pools[0])
	}
	return middleware(log, mux)
}

func operation(id, summary string) func(*huma.Operation) {
	return func(op *huma.Operation) {
		op.OperationID = id
		op.Summary = summary
		op.Tags = []string{"System"}
	}
}

func middleware(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var id [16]byte
		_, _ = rand.Read(id[:])
		requestID := hex.EncodeToString(id[:])
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		start := time.Now()
		defer func() {
			if recover() != nil {
				log.ErrorContext(r.Context(), "request panic", "request_id", requestID)
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"title":"Internal Server Error","status":500}`))
			}
			// Never log query strings, credentials, bodies or customer identifiers.
			log.InfoContext(r.Context(), "request", "request_id", requestID, "method", r.Method, "duration_ms", time.Since(start).Milliseconds())
		}()
		next.ServeHTTP(w, r)
	})
}
