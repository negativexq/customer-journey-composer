# Operational bottlenecks — 2026-11-10

Independent worker slots and bounded event admission improve total mock delivery drain, including delayed providers. They do **not** improve every stage: runtime completion and timer lateness regress under the mixed workload. This follow-up preserves those measurements and the intermediate implementation rather than replacing the earlier [SQL access-path report](PERFORMANCE.md).

## Reproduction and comparison boundary

```sh
./scripts/benchmark.sh --customers 10000 --ingest-rate 480 --out ../../.tmp/ops-normal.json
./scripts/benchmark.sh --customers 1000 --ingest-rate 480 --provider-delay-ms 100 --provider-delay-every 4 --out ../../.tmp/ops-delayed.json
```

Same runner and isolated PostgreSQL 17.11 container (2 CPU, 2 GiB, tmpfs), pools of 20, eight runtime slots, four delivery slots and 20 HTTP callers. Fixtures contain fictional customers and 1,000 inactive journeys. Single-event traffic is offered at 480 requests/s; actual throughput depends on service time. Five authenticated UI reads/s run during load. Metrics, pgx timing and five-second bounded queue sampling are enabled; no trace exporter starts. Every fourth delayed-provider attempt waits 100ms; two mock attempts return transient 429/500 before recovery. No real provider or LLM is invoked. Main application/customer data is untouched.

This method differs from the earlier unpaced report: do not splice its baseline with these new results. Each row is one measured run, not a statistical confidence interval or production SLO. Read probes have only 22 samples per endpoint during the 10,000-event ingestion phase; the 1,000-event phase is too short for credible API tail comparisons. Prometheus histogram estimates are distinct from these exact benchmark samples.

| Implementation                | Commit    | Normal report                                       | Delayed-provider report                                       |
| ----------------------------- | --------- | --------------------------------------------------- | ------------------------------------------------------------- |
| Instrumented baseline         | `5a65541` | [ops-before.json](benchmarks/ops-before.json)       | [ops-slow-before.json](benchmarks/ops-slow-before.json)       |
| Independent slots + admission | `0064f44` | [ops-after.json](benchmarks/ops-after.json)         | [ops-slow-after.json](benchmarks/ops-slow-after.json)         |
| Final: 2ms claim coalescing   | `1322154` | [ops-coalesced.json](benchmarks/ops-coalesced.json) | [ops-slow-coalesced.json](benchmarks/ops-slow-coalesced.json) |

All six runs started from clean Git trees. Each report has a matching `.json.prom` file in the same directory containing the process's final metric snapshot; full SQL statistics/plans and environment are in JSON. No build or test suite ran alongside the timed comparisons. The optional local monitoring stack remained running in every comparison.

## Reproduced defects and changes

1. **Runtime group barrier.** A locked customer prevented the next group of jobs from starting even with available processors. New regression test holds the first customer's lock and requires unrelated executions to complete before releasing it: baseline completed zero within two seconds; final passes. Workers claim only the number of available slots, keeping the 30-second lease from being spent in an internal preclaimed queue. A bounded 2ms coalescing interval groups nearby completions without waiting for the slow customer. Lease recovery runs once per second and retains token fences.
2. **Delivery group barrier.** A single blocked provider call prevented independent slots and outbox scheduling from progressing. Baseline accepted only three other messages within the regression window; final continues processing unrelated jobs. Scheduling/recovery and four fixed dispatchers now run independently, with bounded waits and cancellation. Provider UNKNOWN semantics and single logical delivery constraints remain intact.
3. **Interactive pool starvation.** Twenty valid event requests blocked on customer locks occupied the API pool; an authenticated overview request timed out at 750ms. Shared single/batch admission now allows at most `max(1, pool_max-4)` event transactions, waits outside the pool for at most 250ms and returns 429 with Retry-After on saturation. The same blocked-input regression now passes. This reserves transaction capacity, not a separate physical pool: authentication/other operations still share it and can momentarily reach the configured maximum. DB auth failures return 503 instead of misclassifying valid credentials as 401; the HTTP request budget now includes authentication.

Changes are in `runtime/worker.go`, `delivery/worker.go`, `httpapi/admission.go`, request/auth middleware and their PostgreSQL regression tests. No production indexes, migration, Redis, cache or larger pool was added.

## Results: normal 10,000 executions

Times begin at the common due deadline except stage A, which begins before ingestion. Timer lateness measures completion of the timer step, not claim time. Execution completion and provider acceptance are separate measurements.

| Measure                              | Baseline | Independent slots | Final coalesced |
| ------------------------------------ | -------: | ----------------: | --------------: |
| Confirmed simultaneously active      |   10,000 |            10,000 |          10,000 |
| All mock sends provider-accepted     |  88.519s |           70.507s |         66.464s |
| Stage A: all executions WAITING      |  34.107s |           38.483s |         35.729s |
| Stage B: all executions completed    |  45.358s |           70.495s |         66.453s |
| Timer completion lateness p95        |   8.984s |           10.280s |         10.761s |
| Timer completion lateness p99        |   9.419s |           10.722s |         11.163s |
| Successful ingestion HTTP p95        | 55.518ms |          40.964ms |        49.507ms |
| Successful ingestion HTTP p99        | 68.761ms |          63.300ms |        75.218ms |
| Actual event throughput              |  453.4/s |           466.7/s |         461.2/s |
| API cumulative pool acquire time     |  1,084ms |             712ms |           606ms |
| Ingestion Analytics p95 (22 samples) | 34.461ms |          17.691ms |        38.247ms |

Final provider drain is 24.9% shorter, but timer p95 is 19.8% higher and execution completion is slower. Concurrent scheduling and dispatch overlap runtime work instead of leaving delivery until later, increasing PostgreSQL contention. Claims are safer and smaller, so round trips rise versus the old preclaimed groups. API p99 also rises; Analytics has no consistent measured improvement across these runs. The admission regression establishes isolation under deliberately blocked writes, not a general promise that all UI reads get faster.

The first independent-slots implementation made 25,386 runtime claim statements in stage B, costing 10.064 cumulative SQL seconds. Coalescing reduces that to 10,068 statements / 4.785 seconds (60.3% fewer calls), but baseline was 1,733 / 1.905 seconds. Statement sums are not elapsed times and do not establish PostgreSQL CPU saturation. Claim query churn remains a real optimization target; increasing concurrency or adding another index is not justified by these results alone.

## Results: mixed delayed provider, 1,000 executions

| Measure                          | Baseline | Independent slots | Final coalesced |
| -------------------------------- | -------: | ----------------: | --------------: |
| All mock sends provider-accepted |  35.118s |           12.064s |         11.060s |
| All executions completed         |   4.040s |            7.048s |          6.048s |
| Timer completion lateness p95    |   0.859s |            1.056s |          1.110s |

Final provider drain is 68.5% shorter under this declared synthetic delay. This demonstrates removal of a barrier, not actual SMTP/SMS-provider throughput. With 1,000 customers this scenario does not itself satisfy the 10,000-active-execution requirement; the normal comparison does.

Every run has one event/execution/logical delivery per customer, exactly two expected retries, provider attempts = customer count + 2, zero failed executions, zero runtime retries and zero duplicate logical actions. Duplicate input requests did not enroll again. Loaded read failure maps are empty. No 429 admission/rate-limit retry occurred at this offered rate; the locked-input test separately exercises admission saturation. A single cumulative pool-acquisition value is not an individual-request latency percentile.

## Next measured work

The [runtime research](RUNTIME-RESEARCH.md) maps official Camunda, Temporal, River and PostgreSQL patterns to these opportunities, with explicit safety boundaries and an experiment order. It does not claim those changes are implemented.

- Profile runtime statement round trips/Go allocations and pool contention separately. Test bounded SQL batching or a bounded immutable-version cache only after identifying the dominant cost. Any queue microbatch change must keep the blocked-customer regression and lease safety.
- Test scheduler/runtime fairness to reduce timer lateness while keeping independent delivery slots. A provider-drain speedup alone is insufficient evidence to raise worker concurrency.
- Repeat sustained UI probes with many more samples, disk-backed data, remote latency, large paused prefixes and months of history. Current summary reads do not justify a new analytics cache or wide speculative indexes.
- Admission is request-based; separately bound batch event volume and evaluate noisy-workspace fairness before increasing limits. Verify throughput/429 tradeoffs at and above saturation.
- Operational metrics now distinguish HTTP time, DB time, pool waits, queue age, worker lag and provider time. [Observability](OBSERVABILITY.md) documents sampling, cardinality and queue caps. External alert routing, centralized logs and production monitoring deployment remain open.

## Verification

Formatting, lint, typecheck, race-enabled unit tests and the real-PostgreSQL runtime progress test passed before `1322154`. The final complete package-isolated PostgreSQL integration suite passed, as did the rebuilt Docker services, 32/32 desktop/mobile browser tests, actual worker restart recovery and the API-to-provider telemetry smoke with alert-rule checks. Final outcomes are also recorded in the implementation tracker. Live LLM tests remain deferred and hosted CI has not run.
