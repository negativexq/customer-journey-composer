package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"github.com/ofk/customer-journey-composer/apps/api/internal/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
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
	api.UseMiddleware(func(ctx huma.Context, next func(huma.Context)) {
		if info, ok := ctx.Context().Value(requestInfoKey{}).(*requestInfo); ok {
			info.route = ctx.Operation().Path
		}
		span := trace.SpanFromContext(ctx.Context())
		span.SetName(normalMethod(ctx.Method()) + " " + ctx.Operation().Path)
		span.SetAttributes(attribute.String("http.route", ctx.Operation().Path))
		next(ctx)
	})
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

type requestInfoKey struct{}
type requestInfo struct{ route string }
type responseStatus struct {
	http.ResponseWriter
	status int
}

func (w *responseStatus) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *responseStatus) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(p)
}
func (w *responseStatus) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *responseStatus) Flush() {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}
func normalMethod(method string) string {
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		return method
	}
	return "OTHER"
}
func middleware(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var id [16]byte
		_, _ = rand.Read(id[:])
		requestID := hex.EncodeToString(id[:])
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		budget := 20 * time.Second
		if strings.HasPrefix(r.URL.Path, "/api/v1/ai/") {
			budget = 90 * time.Second
		}
		bounded, cancel := context.WithTimeout(r.Context(), budget)
		defer cancel()
		r = r.WithContext(bounded)
		info := &requestInfo{route: "unmatched"}
		method := normalMethod(r.Method)
		ctx, span := observability.HTTP(observability.Parent(r.Context(), r.Header.Get("traceparent")), method, attribute.String("http.request.method", method))
		r = r.WithContext(context.WithValue(ctx, requestInfoKey{}, info))
		if span.SpanContext().IsValid() {
			w.Header().Set("X-Trace-ID", span.SpanContext().TraceID().String())
		}
		result := &responseStatus{ResponseWriter: w}
		start := time.Now()
		defer func() {
			if recover() != nil {
				log.ErrorContext(r.Context(), "request panic", "request_id", requestID)
				observability.Fail(span, "request_panic")
				if result.status == 0 {
					result.Header().Set("Content-Type", "application/problem+json")
					result.WriteHeader(500)
					_, _ = result.Write([]byte(`{"title":"Internal Server Error","status":500}`))
				}
			}
			status := result.status
			if status == 0 {
				status = 200
			}
			if status >= 500 {
				observability.Fail(span, "request_failed")
			}
			span.SetAttributes(attribute.Int("http.response.status_code", status))
			span.End()
			observability.ObserveHTTP(info.route, method, strconv.Itoa(status), time.Since(start))
			// Deliberately omit raw URLs, query strings, bodies and raw panic/error text.
			log.InfoContext(r.Context(), "request", "request_id", requestID, "route", info.route, "method", method, "status", status, "duration_ms", time.Since(start).Milliseconds())
		}()
		next.ServeHTTP(result, r)
	})
}
