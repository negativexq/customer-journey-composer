package observability

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var Registry = prometheus.NewRegistry()
var durationBuckets = []float64{.0001, .0005, .001, .0025, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 15, 90}
var httpDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "journey_http_duration_seconds", Help: "Full HTTP handler duration including authentication and admission.", Buckets: durationBuckets}, []string{"route", "method"})
var httpRequests = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "journey_http_requests_total", Help: "HTTP responses by normalized route/method/status."}, []string{"route", "method", "status"})
var sqlDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "journey_db_query_duration_seconds", Help: "pgx query/exec or whole BATCH duration including result consumption, excluding pool acquisition.", Buckets: durationBuckets}, []string{"operation", "result"})
var acquireDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "journey_db_acquire_duration_seconds", Help: "Pool acquisition duration, including unsuccessful acquisition.", Buckets: durationBuckets}, []string{"result"})
var operationDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "journey_worker_operation_duration_seconds", Help: "Runtime and delivery operation duration; result refers to this attempt.", Buckets: durationBuckets}, []string{"operation", "result"})
var operations = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "journey_worker_operations_total", Help: "Worker operation attempts, not unique logical sends."}, []string{"operation", "result"})
var providerDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "journey_provider_duration_seconds", Help: "Provider call duration by bounded adapter/channel/outcome.", Buckets: durationBuckets}, []string{"provider", "channel", "result"})
var providerCalls = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "journey_provider_attempts_total", Help: "Provider calls, including retries; ACCEPTED does not mean delivered."}, []string{"provider", "channel", "result"})
var jobLag = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "journey_job_start_lag_seconds", Help: "Runtime processing start minus persisted due_at; not claim-only latency.", Buckets: []float64{.01, .05, .1, .25, .5, 1, 2, 5, 10, 30, 60, 300, 1800, 7200, 86400}}, []string{"kind"})
var queueDepth = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "journey_queue_depth", Help: "Due/pending rows, capped at 10000; inspect queue_depth_capped."}, []string{"queue"})
var queueCapped = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "journey_queue_depth_capped", Help: "1 when actual depth exceeds the bounded 10000-row probe."}, []string{"queue"})
var queueAge = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "journey_queue_oldest_age_seconds", Help: "Age of oldest due/pending row, zero when empty."}, []string{"queue"})
var queueSample = prometheus.NewGauge(prometheus.GaugeOpts{Name: "journey_queue_sample_timestamp_seconds", Help: "Unix timestamp of last successful bounded queue probe."})
var queueErrors = prometheus.NewCounter(prometheus.CounterOpts{Name: "journey_queue_probe_errors_total", Help: "Queue sampler errors; previous values remain until next successful sample."})

var admissionDuration = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "journey_ingestion_admission_wait_seconds", Help: "Event admission wait outside the DB pool.", Buckets: durationBuckets})
var admissionRejected = prometheus.NewCounter(prometheus.CounterOpts{Name: "journey_ingestion_admission_rejected_total", Help: "Event requests rejected by timeout/cancellation before obtaining a transaction slot."})

func ObserveAdmission(d time.Duration, rejected bool) {
	admissionDuration.Observe(d.Seconds())
	if rejected {
		admissionRejected.Inc()
	}
}

var leaseRecoveries = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "journey_lease_recoveries_total", Help: "Recovered expired leases; UNKNOWN requires provider reconciliation."}, []string{"queue", "result"})
var runtimeRetries = prometheus.NewCounter(prometheus.CounterOpts{Name: "journey_runtime_retries_total", Help: "Runtime job retries scheduled after failed transactions."})

func ObserveLeaseRecovery(queue, result string, count int64) {
	if count > 0 {
		leaseRecoveries.WithLabelValues(queue, result).Add(float64(count))
	}
}
func ObserveRuntimeRetry() { runtimeRetries.Inc() }

var channelEvents = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "journey_channel_events_total", Help: "Provider channel gate changes: pauses, credential locks, breaker trips and alerts, without workspace labels."}, []string{"provider", "event"})

// ObserveChannel counts a change of a provider channel's gate or alert.
func ObserveChannel(provider, event string) { channelEvents.WithLabelValues(provider, event).Inc() }

func init() {
	Registry.MustRegister(channelEvents, admissionDuration, admissionRejected, leaseRecoveries, runtimeRetries, httpDuration, httpRequests, sqlDuration, acquireDuration, operationDuration, operations, providerDuration, providerCalls, jobLag, queueDepth, queueCapped, queueAge, queueSample, queueErrors, collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
}
func Handler() http.Handler {
	return promhttp.HandlerFor(Registry, promhttp.HandlerOpts{MaxRequestsInFlight: 2, Timeout: 5 * time.Second})
}
func ObserveHTTP(route, method, status string, d time.Duration) {
	httpDuration.WithLabelValues(route, method).Observe(d.Seconds())
	httpRequests.WithLabelValues(route, method, status).Inc()
}
func ObserveJobLag(kind string, d time.Duration) {
	if kind != "timer" {
		kind = "step"
	}
	jobLag.WithLabelValues(kind).Observe(max(0, d.Seconds()))
}
func ObserveProvider(provider, channel, result string, d time.Duration) {
	switch provider {
	case "smtp", "mock_sms", "mock_email":
	default:
		provider = "other"
	}
	if channel != "email" && channel != "sms" {
		channel = "other"
	}
	switch result {
	case "ACCEPTED", "RETRY", "FAILED", "UNKNOWN":
	default:
		result = "other"
	}
	providerDuration.WithLabelValues(provider, channel, result).Observe(d.Seconds())
	providerCalls.WithLabelValues(provider, channel, result).Inc()
}

type outcomeKey struct{}
type outcome struct{ result string }

func TrackOperation(ctx context.Context, operation string) (context.Context, func(error)) {
	start := time.Now()
	value := &outcome{result: "success"}
	ctx = context.WithValue(ctx, outcomeKey{}, value)
	return ctx, func(err error) {
		result := value.result
		if err != nil {
			result = "error"
			Fail(trace.SpanFromContext(ctx), ErrorClass(err))
		}
		operationDuration.WithLabelValues(operation, result).Observe(time.Since(start).Seconds())
		operations.WithLabelValues(operation, result).Inc()
	}
}
func SetOutcome(ctx context.Context, result string) {
	switch result {
	case "failed", "unknown", "retry", "skipped":
	default:
		return
	}
	if value, ok := ctx.Value(outcomeKey{}).(*outcome); ok {
		value.result = result
	}
	if result == "failed" || result == "unknown" {
		Fail(trace.SpanFromContext(ctx), result)
	}
}
func Fail(span trace.Span, category string) {
	span.SetStatus(codes.Error, category)
	span.AddEvent(category)
}
func ErrorClass(err error) string {
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "deadline_exceeded"
	}
	return "operation_failed"
}

func bindPool(pool *pgxpool.Pool) {
	for name, value := range map[string]func() float64{
		"journey_db_pool_connections": func() float64 { return float64(pool.Stat().TotalConns()) },
		"journey_db_pool_acquired":    func() float64 { return float64(pool.Stat().AcquiredConns()) },
		"journey_db_pool_max":         func() float64 { return float64(pool.Stat().MaxConns()) },
	} {
		Registry.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: name, Help: "Process-local PostgreSQL pool state."}, value))
	}
}

// Serve exposes a separate internal listener, never a public application route.
// Queue probes run only in the worker, with one bounded query every five seconds.
func Serve(ctx context.Context, addr string, pool *pgxpool.Pool, queues bool, log *slog.Logger) (func(), error) {
	if addr == "" {
		return func() {}, nil
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, errors.New("metrics listener could not start")
	}
	bindPool(pool)
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", Handler())
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second}
	sampleCtx, cancel := context.WithCancel(ctx)
	var group sync.WaitGroup
	group.Go(func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("metrics listener stopped")
		}
	})
	if queues {
		group.Go(func() {
			timer := time.NewTicker(5 * time.Second)
			defer timer.Stop()
			for {
				select {
				case <-sampleCtx.Done():
					return
				case <-timer.C:
					if err := SampleQueues(sampleCtx, pool); err != nil && sampleCtx.Err() == nil {
						queueErrors.Inc()
					}
				}
			}
		})
	}
	return func() {
		cancel()
		ending, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_ = server.Shutdown(ending)
		group.Wait()
	}, nil
}

// SampleQueues performs a bounded probe for the worker sampler and isolated diagnostics.
func SampleQueues(ctx context.Context, pool *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(context.WithValue(ctx, suppressedKey{}, true), time.Second)
	defer cancel()
	rows, err := pool.Query(ctx, `WITH runtime AS MATERIALIZED(SELECT due_at AS at FROM scheduled_jobs WHERE state='SCHEDULED' AND due_at<=now() ORDER BY due_at,id LIMIT 10001),delivery AS MATERIALIZED(SELECT due_at AS at FROM delivery_jobs WHERE status='SCHEDULED' AND due_at<=now() ORDER BY due_at,id LIMIT 10001),metrics AS MATERIALIZED(SELECT created_at AS at FROM metric_queue ORDER BY id LIMIT 10001) SELECT 'runtime',count(*),COALESCE(extract(epoch FROM now()-min(at)),0) FROM runtime UNION ALL SELECT 'delivery',count(*),COALESCE(extract(epoch FROM now()-min(at)),0) FROM delivery UNION ALL SELECT 'analytics',count(*),COALESCE(extract(epoch FROM now()-min(at)),0) FROM metrics`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type sample struct {
		queue string
		count int
		age   float64
	}
	values := []sample{}
	for rows.Next() {
		var v sample
		if err := rows.Scan(&v.queue, &v.count, &v.age); err != nil {
			return err
		}
		values = append(values, v)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	for _, v := range values {
		queueDepth.WithLabelValues(v.queue).Set(float64(min(v.count, 10000)))
		capped := 0.
		if v.count > 10000 {
			capped = 1
		}
		queueCapped.WithLabelValues(v.queue).Set(capped)
		queueAge.WithLabelValues(v.queue).Set(max(0, v.age))
	}
	queueSample.Set(float64(time.Now().Unix()))
	return nil
}
