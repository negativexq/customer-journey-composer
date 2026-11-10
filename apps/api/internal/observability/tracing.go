package observability

import (
	"context"
	"fmt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"math"
	"net/url"
	"strconv"
	"time"
)

func Init(ctx context.Context, service string, getenv func(string) string) (func(context.Context) error, error) {
	otel.SetTextMapPropagator(propagation.TraceContext{})
	options := []sdktrace.TracerProviderOption{sdktrace.WithResource(resource.NewSchemaless(attribute.String("service.name", service), attribute.String("service.version", "0.1.0"))), sdktrace.WithSampler(sdktrace.NeverSample())}
	endpoint := getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")
	if endpoint == "" {
		endpoint = getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	}
	if endpoint != "" && getenv("OTEL_SDK_DISABLED") != "true" {
		u, err := url.Parse(endpoint)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return nil, fmt.Errorf("OTLP endpoint must be an HTTP(S) URL without credentials or query")
		}
		ratio := 0.1
		if s := getenv("OTEL_TRACES_SAMPLER_ARG"); s != "" {
			ratio, err = strconv.ParseFloat(s, 64)
			if err != nil || ratio < 0 || ratio > 1 || math.IsNaN(ratio) {
				return nil, fmt.Errorf("trace sampling ratio must be 0–1")
			}
		}
		exporter, err := otlptracehttp.New(ctx, otlptracehttp.WithTimeout(5*time.Second))
		if err != nil {
			return nil, fmt.Errorf("trace exporter initialization failed")
		}
		options = append(options, sdktrace.WithBatcher(exporter, sdktrace.WithMaxQueueSize(2048)), sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))))
	}
	provider := sdktrace.NewTracerProvider(options...)
	otel.SetTracerProvider(provider)
	return provider.Shutdown, nil
}
func Parent(ctx context.Context, parent string) context.Context {
	return propagation.TraceContext{}.Extract(ctx, propagation.MapCarrier{"traceparent": parent})
}
func TraceParent(ctx context.Context) string {
	carrier := propagation.MapCarrier{}
	propagation.TraceContext{}.Inject(ctx, carrier)
	return carrier.Get("traceparent")
}
func Start(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return otel.Tracer("customer-journey-composer").Start(ctx, name, trace.WithAttributes(attrs...))
}

func HTTP(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return otel.Tracer("customer-journey-composer").Start(ctx, name, trace.WithSpanKind(trace.SpanKindServer), trace.WithAttributes(attrs...))
}
