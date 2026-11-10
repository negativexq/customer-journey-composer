package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/observability"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestRequestsCorrelateTracesAndLogsWithoutRawPaths(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	old := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	defer otel.SetTracerProvider(old)
	defer provider.Shutdown(context.Background())
	var logs bytes.Buffer
	handler := New(slog.New(observability.Correlate(slog.NewJSONHandler(&logs, nil))), func(context.Context) error { return errors.New("database password=private") })
	for _, path := range []string{"/readyz?token=private", "/unknown/private-customer?email=private@example.invalid"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if response.Header().Get("X-Trace-ID") == "" {
			t.Fatal("missing trace ID header")
		}
	}
	if strings.Contains(logs.String(), "private") {
		t.Fatal("URL/query/error appeared in logs")
	}
	var first map[string]any
	if err := json.Unmarshal(bytes.Split(logs.Bytes(), []byte("\n"))[0], &first); err != nil {
		t.Fatal(err)
	}
	if first["route"] != "/readyz" || first["status"] != float64(503) || first["trace_id"] == nil {
		t.Fatal(first)
	}
	spans := exporter.GetSpans()
	if len(spans) != 2 || spans[0].Name != "GET /readyz" {
		t.Fatal("duplicate or unnamed request spans", spans)
	}
	if normalMethod("CUSTOMER-ID-IS-A-METHOD") != "OTHER" {
		t.Fatal("unbounded method label")
	}
}
