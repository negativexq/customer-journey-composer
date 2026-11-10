package httpapi

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/observability"
	"time"
)

// Share one admission budget across single and batch events. Requests waiting
// here hold no database connection; four pool slots remain for UI/auth/health.
func ingestionAdmission(api huma.API, poolSize int) func(*huma.Operation) {
	slots := make(chan struct{}, max(1, poolSize-4))
	return func(op *huma.Operation) {
		op.Middlewares = append(op.Middlewares, func(ctx huma.Context, next func(huma.Context)) {
			if err := canEdit(ctx.Context()); err != nil {
				_ = huma.WriteErr(api, ctx, 403, "Editor role required")
				return
			}
			start := time.Now()
			timer := time.NewTimer(250 * time.Millisecond)
			defer timer.Stop()
			select {
			case slots <- struct{}{}:
				observability.ObserveAdmission(time.Since(start), false)
				defer func() { <-slots }()
				next(ctx)
			case <-ctx.Context().Done():
				observability.ObserveAdmission(time.Since(start), true)
				_ = huma.WriteErr(api, ctx, 503, "Event ingestion request cancelled")
			case <-timer.C:
				observability.ObserveAdmission(time.Since(start), true)
				ctx.SetHeader("Retry-After", "1")
				_ = huma.WriteErr(api, ctx, 429, "Event ingestion is busy; retry later")
			}
		})
	}
}
