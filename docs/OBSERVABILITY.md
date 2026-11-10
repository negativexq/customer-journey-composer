# Operational telemetry

The application exposes Prometheus operational metrics and OpenTelemetry traces. Business analytics remain PostgreSQL projections. Metric labels never contain workspace/customer/execution IDs, email/phone, event names, workflow content, request IDs or raw URL paths. SQL instrumentation exports a bounded operation name and a query fingerprint on sampled spans, never SQL text, arguments, DSNs or raw error messages.

## Start locally

```sh
make up
make observe-up
make test-telemetry
```

The optional `compose.observability.yaml` overlay adds pinned OpenTelemetry Collector, Prometheus, Tempo and Grafana images. It directs API/worker traces to the collector; the main development stack can run without it. The overlay waits for Tempo readiness before starting the collector. Grafana is at http://localhost:3005/d/journey-operations and allows local anonymous **Viewer** access. An admin password is generated into ignored `.env` by `make setup`; no fixed credential is committed. Prometheus is at http://localhost:9095 and Tempo at http://localhost:3205. All published monitoring ports bind to loopback; these local settings are not a public monitoring deployment.

Metrics listen separately on port 9464 inside the API and worker containers, with no published host port or unauthenticated application `/metrics` route. Native services opt in separately:

```sh
METRICS_ADDR=127.0.0.1:9464 make dev-api
METRICS_ADDR=127.0.0.1:9465 make dev-worker
```

Prometheus retains seven days with a 512MB size limit; Tempo blocks have one-hour retention. Container memory is bounded. Use organization authentication, private networking, appropriate persistent/object storage and an explicit retention policy before external deployment. The local collector exports traces over the private Compose network; application execution does not depend on collector availability.

## Signals and cost boundaries

| Signal                                              | Meaning                                                                            |
| --------------------------------------------------- | ---------------------------------------------------------------------------------- |
| `journey_http_duration_seconds`                     | Full request duration, including auth/admission; normalized route/method only      |
| `journey_http_requests_total`                       | HTTP status counts; operational dashboards/alerts exclude health/readiness traffic |
| `journey_db_query_duration_seconds`                 | pgx query/exec time through result consumption; separate from pool acquisition     |
| `journey_db_acquire_duration_seconds`               | Time spent acquiring a pool connection, including failures                         |
| `journey_db_pool_*`                                 | Process-local pool max/acquired/total, read without SQL                            |
| `journey_worker_operations_total`                   | Runtime/delivery operation attempts by bounded outcome; not logical sends          |
| `journey_provider_attempts_total`                   | Provider calls by adapter/channel/result; retry attempts are included              |
| `journey_job_start_lag_seconds`                     | Runtime processing start minus persisted due_at; not claim-only latency            |
| `journey_queue_depth`, `journey_queue_depth_capped` | Due/pending row count capped at 10,000, with explicit cap indicator                |
| `journey_queue_oldest_age_seconds`                  | Oldest observed due/pending row age                                                |
| `journey_queue_sample_timestamp_seconds`            | Last successful worker queue probe; errors preserve prior values                   |
| `journey_lease_recoveries_total`                    | Expired lease recovery, including provider UNKNOWN transitions                     |
| `journey_runtime_retries_total`                     | Runtime transaction retries actually scheduled                                     |
| `go_*`, `process_*`                                 | Runtime/process metrics from the standard collector                                |

Queue sampling runs only in the worker, at most once every five seconds, with a one-second timeout and bounded index probes. It is suppressed from normal SQL telemetry to avoid measuring its own queries. Raw runtime/delivery queue probes include deliberately paused work, so a raw old-due value alone is not an alert that the runtime is broken. Counts/ages do not claim precise eligible backlog. Analytics age is directly actionable. Metrics collection adds no per-transition SQL writes.

Histogram percentiles in Grafana are bucket estimates over the selected window; they are not interchangeable with exact interpolated benchmark samples. p95 by route remains separate from DB duration/pool waits and job lag.

With bounded runtime turns, the operational `runtime_step` operation counts claimed job processing turns, which can include multiple short nodes. Node-level business metrics continue to come from individual `step_executions` records; job throughput must not be reported as node throughput. Timer resumes retain a separate scheduling boundary. Benchmark-only `--profile` and wait-event sampling are described in [bounded-turn measurements](BOUNDED-TURNS.md); they are not enabled on production request paths.

## Trace and log correlation

Event admission adds `journey_ingestion_admission_wait_seconds` and `journey_ingestion_admission_rejected_total`. These measure waiting outside the database pool and admission saturation; rate-limiter rejections are a separate cause of HTTP 429. Single and batch endpoints share the same transaction budget.

HTTP tracing starts before authentication. Huma supplies a route template; unmatched requests use one bounded `unmatched` label. Request logs include request ID, route template, method, status, duration, trace ID and span ID. A shared slog handler correlates logs whenever a trace context is present. Plain process/lifecycle logs have no fabricated trace ID. Logs remain structured stdout; no centralized log store is included in this overlay.

`events.ingest`, `runtime.step`, `delivery.dispatch`, provider calls and pgx query/exec spans carry the same trace across durable jobs. Enrollment persists W3C traceparent; workers restore it after restart. Runtime/SQL/provider failures use sanitized error categories. Once trace export is enabled, roots default to 10% head sampling and honor upstream sampling. Metrics count all instrumented attempts; head sampling does not guarantee a trace for every failure. Tail sampling and a durable centralized log pipeline are not implemented.

`OTEL_EXPORTER_OTLP_ENDPOINT` is a base HTTP URL and gets `/v1/traces` appended. `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` is the full trace endpoint. URLs cannot contain credentials/query strings; exporter auth headers come from environment secrets. `OTEL_TRACES_SAMPLER_ARG` controls root sampling and `OTEL_SDK_DISABLED=true` disables trace export. Prometheus metrics are controlled by their separate listener. Application exporter queues and timeouts are bounded; collector unavailability may drop telemetry but does not change deterministic execution state.

## Dashboard and alarms

The provisioned **Journey operations** dashboard covers request rate/p95, DB query/acquire p95, pool saturation, queue depth/age/freshness, worker attempts, start lag, provider attempts/p95 and Go memory/goroutines.

Seven Prometheus rules cover target-down, sustained API 5xx/p95, pool saturation, analytics backlog, stale queue sampling and new ambiguous provider outcomes (including lease recovery). Rules are evaluated locally and visible in Prometheus. No external notification destination or Alertmanager routing is configured. UNKNOWN outcomes require reconciliation, not an automatic resend.

`make test-telemetry` runs an isolated fictional workspace through test gate → publish → event → real runtime → mock SMS, checks the complete trace in Tempo and both scrape targets in Prometheus, and checks rule syntax and a firing-rule fixture with promtool. It does not call an LLM or external provider. Unit tests verify log/trace correlation and omission of raw SQL, arguments, URLs and errors. A real-PostgreSQL test verifies the queue count cap and age.

References: [Prometheus instrumentation](https://prometheus.io/docs/practices/instrumentation/), [OpenTelemetry Go exporters](https://opentelemetry.io/docs/languages/go/exporters/), [Tempo monolithic configuration](https://grafana.com/docs/tempo/latest/configuration/), [Grafana provisioning](https://grafana.com/docs/grafana/latest/administration/provisioning/).
