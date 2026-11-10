package observability

import (
	"context"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDurableTraceParentAndMinimalAttributes(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	old := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	defer otel.SetTracerProvider(old)
	defer provider.Shutdown(context.Background())
	ctx, parent := Start(context.Background(), "event.ingest")
	encoded := TraceParent(ctx)
	parent.End()
	if len(encoded) != 55 {
		t.Fatal("invalid W3C parent length")
	}
	childCtx, child := Start(Parent(context.Background(), encoded), "runtime.step", attribute.String("execution.id", "opaque-id"))
	child.End()
	if childCtx == nil || len(exporter.GetSpans()) != 2 || exporter.GetSpans()[1].SpanContext.TraceID() != exporter.GetSpans()[0].SpanContext.TraceID() {
		t.Fatal("trace context lost across durable job")
	}
	if len(exporter.GetSpans()[1].Attributes) != 1 {
		t.Fatal("unexpected customer data in trace")
	}
}
func TestInvalidExporterConfigurationRejectedWithoutNetwork(t *testing.T) {
	_, err := Init(context.Background(), "test", func(k string) string {
		if k == "OTEL_EXPORTER_OTLP_ENDPOINT" {
			return "https://user:private@example.invalid/path?secret=1"
		}
		return ""
	})
	if err == nil {
		t.Fatal("credential-bearing collector URL accepted")
	}
}

func TestConfiguredCollectorReceivesBoundedExportWithoutExternalNetwork(t *testing.T) {
	paths := make(chan string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		paths <- r.URL.Path
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(200)
	}))
	defer server.Close()
	old := otel.GetTracerProvider()
	defer otel.SetTracerProvider(old)
	shutdown, err := Init(context.Background(), "export-contract", func(k string) string {
		switch k {
		case "OTEL_EXPORTER_OTLP_ENDPOINT":
			return server.URL
		case "OTEL_TRACES_SAMPLER_ARG":
			return "1"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	_, span := Start(context.Background(), "export.test")
	span.End()
	if err = shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case path := <-paths:
		if path != "/v1/traces" {
			t.Fatal(path)
		}
	default:
		t.Fatal("configured collector received no spans")
	}
}
