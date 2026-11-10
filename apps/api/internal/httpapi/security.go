package httpapi

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"github.com/ofk/customer-journey-composer/apps/api/internal/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

type rateBucket struct {
	Tokens float64
	At     time.Time
	Seen   time.Time
}
type requestLimiter struct {
	mu      sync.Mutex
	buckets map[string]rateBucket
}

func (l *requestLimiter) allow(key string, rate, burst float64, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.buckets == nil {
		l.buckets = map[string]rateBucket{}
	}
	b, ok := l.buckets[key]
	if !ok {
		if len(l.buckets) >= 4096 {
			for k, v := range l.buckets {
				if now.Sub(v.Seen) > 5*time.Minute {
					delete(l.buckets, k)
				}
			}
			if len(l.buckets) >= 4096 {
				return false
			}
		}
		b = rateBucket{Tokens: burst, At: now}
	}
	b.Tokens = min(burst, b.Tokens+max(0, now.Sub(b.At).Seconds())*rate)
	b.At = now
	b.Seen = now
	accepted := b.Tokens >= 1
	if accepted {
		b.Tokens--
	}
	l.buckets[key] = b
	return accepted
}
func signupEnabled() bool {
	return os.Getenv("APP_ENV") == "development" || os.Getenv("PUBLIC_SIGNUP") == "true"
}
func registerSecurityMiddleware(api huma.API) {
	limiter := &requestLimiter{}
	api.UseMiddleware(func(ctx huma.Context, next func(huma.Context)) {
		parent := observability.Parent(ctx.Context(), ctx.Header("traceparent"))
		traced, span := observability.HTTP(parent, ctx.Method()+" "+ctx.Operation().Path, attribute.String("http.route", ctx.Operation().Path), attribute.String("http.request.method", ctx.Method()))
		defer func() {
			span.SetAttributes(attribute.Int("http.response.status_code", ctx.Status()))
			if ctx.Status() >= 500 {
				span.SetStatus(codes.Error, "request_failed")
			}
			span.End()
		}()
		if span.SpanContext().IsValid() {
			ctx.SetHeader("X-Trace-ID", span.SpanContext().TraceID().String())
		}
		ctx = huma.WithContext(ctx, traced)
		path := ctx.URL().Path
		timeout := 20 * time.Second
		if strings.HasPrefix(path, "/api/v1/ai/") {
			timeout = 90 * time.Second
		}
		if p, ok := ctx.Context().Value(principalKey{}).(Identity); ok {
			key := "ui:" + p.WorkspaceID
			rate, burst := 10.0, 200.0
			if strings.HasPrefix(path, "/api/v1/events") && ctx.Method() == "POST" {
				key = "ingest:" + p.WorkspaceID
				rate, burst = 500, 1000
			}
			if strings.HasPrefix(path, "/api/v1/ai/") {
				key = "ai:" + p.WorkspaceID
				rate, burst = 1.0/6, 2
			}
			if !limiter.allow(key, rate, burst, time.Now()) {
				ctx.SetHeader("Retry-After", "6")
				_ = huma.WriteErr(api, ctx, 429, "Workspace request limit reached; retry later")
				return
			}
		} else if strings.HasPrefix(path, "/api/v1/auth/") && ctx.Method() == "POST" {
			peer, _, err := net.SplitHostPort(ctx.RemoteAddr())
			if err != nil {
				peer = ctx.RemoteAddr()
			}
			rate, burst := 5.0, 60.0
			if os.Getenv("APP_ENV") == "development" {
				rate, burst = 50, 600
			}
			if !limiter.allow("auth:"+digest(peer), rate, burst, time.Now()) {
				ctx.SetHeader("Retry-After", "2")
				_ = huma.WriteErr(api, ctx, 429, "Authentication request limit reached")
				return
			}
		}
		request, cancel := context.WithTimeout(ctx.Context(), timeout)
		defer cancel()
		next(huma.WithContext(ctx, request))
	})
}
