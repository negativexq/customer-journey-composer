package observability

import (
	"context"
	"go.opentelemetry.io/otel/trace"
	"log/slog"
)

type correlationHandler struct{ next slog.Handler }

func Correlate(handler slog.Handler) slog.Handler { return correlationHandler{next: handler} }
func (h correlationHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}
func (h correlationHandler) Handle(ctx context.Context, record slog.Record) error {
	span := trace.SpanContextFromContext(ctx)
	if span.IsValid() {
		record.AddAttrs(slog.String("trace_id", span.TraceID().String()), slog.String("span_id", span.SpanID().String()))
	}
	return h.next.Handle(ctx, record)
}
func (h correlationHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return correlationHandler{next: h.next.WithAttrs(attrs)}
}
func (h correlationHandler) WithGroup(name string) slog.Handler {
	return correlationHandler{next: h.next.WithGroup(name)}
}
