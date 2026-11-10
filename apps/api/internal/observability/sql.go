package observability

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type PGXTracer struct{}
type suppressedKey struct{}
type queryKey struct{}
type acquireKey struct{}
type queryState struct {
	start     time.Time
	operation string
	span      trace.Span
}

func sqlOperation(sql string) string {
	sql = strings.TrimSpace(sql)
	if len(sql) == 0 {
		return "OTHER"
	}
	end := 0
	for end < len(sql) && end < 16 && sql[end] > ' ' {
		end++
	}
	op := strings.ToUpper(sql[:end])
	switch op {
	case "SELECT", "INSERT", "UPDATE", "DELETE", "WITH", "BEGIN", "COMMIT", "ROLLBACK", "COPY", "CREATE", "ALTER", "DROP", "ANALYZE":
		return op
	}
	return "OTHER"
}
func (PGXTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if ctx.Value(suppressedKey{}) == true {
		return ctx
	}
	state := queryState{start: time.Now(), operation: sqlOperation(data.SQL)}
	// No raw SQL, bind parameters, DSN, error messages or customer identifiers.
	if trace.SpanFromContext(ctx).IsRecording() {
		fingerprint := sha256.Sum256([]byte(data.SQL))
		ctx, state.span = Start(ctx, "postgresql "+state.operation, attribute.String("db.query.fingerprint", hex.EncodeToString(fingerprint[:8])), attribute.String("db.system.name", "postgresql"), attribute.String("db.operation.name", state.operation))
	}
	return context.WithValue(ctx, queryKey{}, state)
}
func (PGXTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	state, ok := ctx.Value(queryKey{}).(queryState)
	if !ok {
		return
	}
	result := "success"
	if data.Err != nil {
		result = "error"
	}
	sqlDuration.WithLabelValues(state.operation, result).Observe(time.Since(state.start).Seconds())
	if state.span != nil {
		if data.Err != nil {
			Fail(state.span, ErrorClass(data.Err))
		}
		state.span.End()
	}
}
func (PGXTracer) TraceAcquireStart(ctx context.Context, _ *pgxpool.Pool, _ pgxpool.TraceAcquireStartData) context.Context {
	if ctx.Value(suppressedKey{}) == true {
		return ctx
	}
	return context.WithValue(ctx, acquireKey{}, time.Now())
}
func (PGXTracer) TraceAcquireEnd(ctx context.Context, _ *pgxpool.Pool, data pgxpool.TraceAcquireEndData) {
	start, ok := ctx.Value(acquireKey{}).(time.Time)
	if !ok {
		return
	}
	result := "success"
	if data.Err != nil {
		result = "error"
	}
	acquireDuration.WithLabelValues(result).Observe(time.Since(start).Seconds())
}
