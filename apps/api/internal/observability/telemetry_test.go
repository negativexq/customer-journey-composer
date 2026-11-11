package observability

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestSQLAndErrorTelemetryDoesNotExportSQLOrArguments(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	old := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	defer otel.SetTracerProvider(old)
	defer provider.Shutdown(context.Background())
	ctx, parent := Start(context.Background(), "test")
	tracer := PGXTracer{}
	ctx = tracer.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{SQL: "SELECT email FROM customers WHERE email=$1", Args: []any{"private@example.invalid"}})
	tracer.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{Err: errors.New("private@example.invalid password=secret")})
	parent.End()
	spans := exporter.GetSpans()
	if len(spans) != 2 {
		t.Fatal(len(spans))
	}
	if spans[0].Name != "postgresql SELECT" || spans[0].Status.Code != codes.Error {
		t.Fatal(spans[0])
	}
	for _, span := range spans {
		for _, a := range span.Attributes {
			s := a.Value.Emit()
			if strings.Contains(s, "private") || strings.Contains(s, "SELECT email") || strings.Contains(s, "secret") {
				t.Fatal("sensitive attribute", a)
			}
		}
		if strings.Contains(span.Status.Description, "private") {
			t.Fatal("raw error exported")
		}
	}
	var buffer bytes.Buffer
	logger := slog.New(Correlate(slog.NewJSONHandler(&buffer, nil))).With("service", "test")
	logger.InfoContext(ctx, "safe event")
	if !strings.Contains(buffer.String(), `"trace_id":`) || !strings.Contains(buffer.String(), `"span_id":`) {
		t.Fatal("missing log/trace correlation")
	}
}
func TestOperationClassifiersRemainBounded(t *testing.T) {
	for _, sql := range []string{"unknown-user-input-very-long", "-- customer secret\nSELECT * FROM users", ""} {
		if sqlOperation(sql) != "OTHER" {
			t.Fatal(sql)
		}
	}
	if sqlOperation("  select secret FROM customers") != "SELECT" {
		t.Fatal("lowercase operation")
	}
	ctx, end := TrackOperation(context.Background(), "test_operation")
	SetOutcome(ctx, "failed")
	end(nil)
	metrics, err := Registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, family := range metrics {
		if family.GetName() != "journey_worker_operations_total" {
			continue
		}
		for _, m := range family.Metric {
			operation, result := "", ""
			for _, l := range m.Label {
				if l.GetName() == "operation" {
					operation = l.GetValue()
				}
				if l.GetName() == "result" {
					result = l.GetValue()
				}
			}
			if operation == "test_operation" && result == "failed" && m.Counter.GetValue() > 0 {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("business failure was counted as success")
	}
}

func TestBatchTelemetryKeepsFailuresVisibleWithoutQueryContent(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	old := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	defer otel.SetTracerProvider(old)
	defer provider.Shutdown(context.Background())
	ctx, parent := Start(context.Background(), "batch-test")
	batch := &pgx.Batch{}
	batch.Queue("SELECT private_column FROM users WHERE email=$1", "private@example.invalid")
	tracer := PGXTracer{}
	ctx = tracer.TraceBatchStart(ctx, nil, pgx.TraceBatchStartData{Batch: batch})
	tracer.TraceBatchQuery(ctx, nil, pgx.TraceBatchQueryData{SQL: "SELECT private_column", Args: []any{"private@example.invalid"}})
	tracer.TraceBatchEnd(ctx, nil, pgx.TraceBatchEndData{Err: errors.New("private@example.invalid secret")})
	parent.End()
	spans := exporter.GetSpans()
	if len(spans) != 2 || spans[0].Name != "postgresql BATCH" || spans[0].Status.Code != codes.Error {
		t.Fatal("missing failed batch span", spans)
	}
	size := false
	for _, span := range spans {
		for _, a := range span.Attributes {
			value := a.Value.Emit()
			if strings.Contains(value, "private") || strings.Contains(value, "secret") || strings.Contains(value, "SELECT") {
				t.Fatal("sensitive batch attribute", a)
			}
			if a.Key == "db.batch.size" && a.Value.AsInt64() == 1 {
				size = true
			}
		}
		if strings.Contains(span.Status.Description, "private") {
			t.Fatal("raw batch error exported")
		}
		for _, event := range span.Events {
			if strings.Contains(event.Name, "private") || strings.Contains(event.Name, "secret") || len(event.Attributes) != 0 {
				t.Fatal("sensitive batch event", event)
			}
		}
	}
	if !size {
		t.Fatal("batch size not recorded")
	}
	suppressed := context.WithValue(context.Background(), suppressedKey{}, true)
	suppressed = tracer.TraceBatchStart(suppressed, nil, pgx.TraceBatchStartData{Batch: batch})
	tracer.TraceBatchEnd(suppressed, nil, pgx.TraceBatchEndData{})
	if len(exporter.GetSpans()) != 2 {
		t.Fatal("diagnostic batch was traced")
	}
}
