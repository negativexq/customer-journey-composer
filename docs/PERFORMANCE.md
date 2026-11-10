# Measured performance and optimization — 2026-11-10

The main bottleneck was queue SQL access order, not the analytics summary model. The same 10,000-customer workload now drains all mock sends in **87.5–89.5 seconds**, down from **176.7 seconds** from the common timer deadline. Timer-step p95 lateness fell from **15.75 seconds** to **9.25–9.41 seconds**. No pool/concurrency increase, new production index, cache or Redis service was introduced.

Ingestion p95/p99 did **not** improve: the faster producer hits the unchanged request limiter and retries. Those end-to-end latencies are included below rather than hidden by reporting successful requests alone.

## Reproduce and inspect source data

```sh
./scripts/benchmark.sh --customers 10000 --out ../../.tmp/performance.json
```

The runner creates and removes its own PostgreSQL container. It refuses a database other than a fresh, empty `journey_benchmark`. It never touches the local application database. The extension/statistics resets are benchmark-only.

| Run                    | Source commit | Raw measured artifact                             |
| ---------------------- | ------------- | ------------------------------------------------- |
| Instrumented baseline  | `5c3b57d`     | [perf-before.json](benchmarks/perf-before.json)   |
| Queue SQL changes only | `9db724e`     | [perf-queues.json](benchmarks/perf-queues.json)   |
| Final, repetition 1    | `262ac48`     | [perf-after-1.json](benchmarks/perf-after-1.json) |
| Final, repetition 2    | `262ac48`     | [perf-after-2.json](benchmarks/perf-after-2.json) |

All four runs started with a clean Git tree. `262ac48` includes the runtime optimizations from `41d2152`; its last change only compacts sidebar typography/spacing. The earlier, uninstrumented [local-10000.json](benchmarks/local-10000.json) remains available but is not used as the before/after baseline because it did not include concurrent reporting reads or statement statistics.

## Environment and workload

- PostgreSQL 17.11, container limited to 2 CPU and 2 GiB RAM, 256 MiB shared buffers, 8 MiB work memory, tmpfs data directory. This is not a disk/fsync or remote deployment benchmark.
- Go 1.26.10 on darwin/arm64. API, client and worker share one Go process; PostgreSQL is in Docker. Go heap measurements include the synthetic client. PostgreSQL's RAM limit is not a measured peak.
- Separate API/worker pools each cap at 20 connections. Eight runtime processors, four delivery processors, 20 concurrent event callers; the same values in every run.
- 1,000 inactive draft journeys, one published six-node journey, 10,000 fictional customers and single-event HTTP requests. The execution/history tables start fresh, so this does not model months of production retention.
- Stage A confirms 10,000 simultaneously RUNNING/WAITING executions, all durably WAITING before proceeding. Stage B moves only isolated fixture timers to one wall-clock deadline five seconds ahead. Real workers then run timer → condition → SMS → exit; no virtual clock shortcut is used.
- Two transient mock provider outcomes (429 and 500), 100 duplicate event requests, and five authenticated UI reads/sec throughout ingestion/runtime/delivery. SMTP, real SMS and LLMs are never called.
- Completion polling is once/sec. Percentiles use interpolated samples. Two optimized repetitions show local variation; they are not confidence intervals or production SLAs.

## Runtime and ingestion results

The deadline-based durations start at the shared timer deadline. Execution completion and provider acceptance are separate checkpoints. Timer lateness is **timer step completion minus due_at**, not claim-only latency or HTTP latency.

| Metric                                                      | Baseline |  Final 1 |  Final 2 |
| ----------------------------------------------------------- | -------: | -------: | -------: |
| All 10,000 executions WAITING from ingestion start (s)      |    51.75 |    31.26 |    33.28 |
| All executions COMPLETED from common timer deadline (s)     |    74.42 |    46.34 |    45.34 |
| All 10,000 logical sends accepted by mock from deadline (s) |   176.73 |    89.50 |    87.46 |
| Timer step completion lateness p95 (s)                      |    15.75 |     9.41 |     9.25 |
| Timer step completion lateness p99 (s)                      |    16.41 |     9.79 |     9.71 |
| Committed single events / sec                               |   536.07 |   550.09 |   549.08 |
| Event-to-commit logical latency p50 (ms)                    |    19.67 |    14.08 |    15.45 |
| Event-to-commit logical latency p95 (ms)                    |    83.50 |   107.61 |    99.51 |
| Event-to-commit logical latency p99 (ms)                    |    95.98 |   193.35 |   176.56 |
| HTTP 429 retries by synthetic producer                      |     0.00 | 3,076.00 | 2,417.00 |

The per-workspace ingestion limiter remains 500 **requests/sec** with a 1,000-request burst. Finite-run throughput can exceed 500 because of that initial burst. The producer retries 429 every 50ms; the reported event-to-commit percentile includes those waits. After optimization it generates 2,417–3,076 rejected attempts, versus zero in the baseline. This demonstrates an admission/backoff bottleneck; the table is not an unconstrained API capacity measurement.

## APIs while writes and workers run

Values are p95 milliseconds, with actual sample count in parentheses. Ingestion lasts about 18 seconds, so those per-endpoint tail samples are exploratory. The two final runs are listed separately; percentiles were not averaged or merged.

| Phase / endpoint             | Baseline p95 (N) | Final 1 p95 (N) | Final 2 p95 (N) |
| ---------------------------- | ---------------: | --------------: | --------------: |
| ingestion / analytics        |       31.95 (19) |       7.93 (18) |      27.83 (18) |
| ingestion / overview         |       34.35 (19) |      12.93 (18) |      22.34 (19) |
| ingestion / journey_list     |       37.06 (19) |      35.71 (18) |      30.89 (18) |
| ingestion / customer_track   |       33.75 (18) |       8.66 (18) |      19.31 (18) |
| ingestion / entries_24h      |       36.04 (18) |      14.11 (18) |      32.51 (18) |
| timer_burst / analytics      |       20.54 (80) |       3.38 (51) |       9.93 (50) |
| timer_burst / overview       |       20.75 (79) |       2.75 (51) |       9.79 (50) |
| timer_burst / journey_list   |       21.57 (80) |       8.82 (51) |      11.20 (50) |
| timer_burst / customer_track |       20.13 (79) |       4.31 (52) |       9.83 (51) |
| timer_burst / entries_24h    |       24.58 (79) |      13.76 (52) |      14.38 (51) |

Quiet, post-drain p95 in the two final runs: Analytics **0.95–1.13ms**, Overview **1.11–1.14ms**, journey list **1.85–2.08ms**, customer tracking **0.74–0.75ms**, last-24-hour entries **0.99–1.39ms**. These are only 30 warm loopback requests per endpoint per run and must not be substituted for loaded response times. All concurrent read probes returned HTTP 200.

## What changed and why

1. **Runtime claim:** retain the due-job ordered index as the outer scan, then check one-to-one execution/journey eligibility before LIMIT. The original join order enumerated 10,000 executions to claim 32 jobs. At the same loaded baseline fixture, EXPLAIN measured **20.698ms / 54,829 buffer accesses**, versus **0.325ms / 435 accesses** for the ordered candidate. Those are buffer access counts, not unique physical pages. The latter used `jobs_due` and 32 execution/journey PK probes. No limit is applied before eligibility; paused/locked older rows do not starve eligible later jobs.
2. **Outbox and delivery claim:** remove selective Active-journey joins as the driving relation. Pending/due indexes now filter work before one-to-one lifecycle checks. Customer locks, per-item pending/lifecycle rechecks, consent/cap decisions and provider idempotency remain intact.
3. **Enrollment:** default/after-completion enrollment no longer performs three history probes. The customer lock serializes policy checks; the existing partial unique active-enrollment constraint rejects a second active execution during INSERT. Once/cooldown query only the history required by their actual policy. Tests include eight simultaneous distinct event IDs, completed-history rules and an active execution whose cooldown has elapsed.
4. **Fence validation:** materialize the single job identity/workspace lookup before checking state/token/lease. This prevents a cached plan from scanning the small claimed-lease partial index for every step. The token, lease, tenant predicate and lock order remain enforced. Forty thousand checks used about **43.7 buffer accesses/call** in the baseline and **5.5/call** in final run 1.

Top-level PostgreSQL execution totals below include trigger work but exclude client/network round trips. Totals across concurrently executing statements are not wall-clock durations. Counts differ because polling/batching changes with throughput.

| Stage-B statement group | Baseline seconds / calls | Final 1 seconds / calls | Final 2 seconds / calls |
| ----------------------- | -----------------------: | ----------------------: | ----------------------: |
| Runtime claim           |            32.21 / 2,297 |            1.47 / 1,732 |            1.53 / 1,715 |
| Outbox schedule         |            54.89 / 2,657 |            5.23 / 2,601 |            5.74 / 2,601 |
| Delivery claim          |           32.33 / 10,159 |           6.66 / 10,103 |           5.64 / 10,103 |
| Fence validation        |            8.53 / 40,000 |           1.17 / 40,000 |           1.14 / 40,000 |

API pool sampled peak stayed at 20 and worker peak at 11. Aggregate API acquisition duration fell from 2.839s to 0.694–1.244s; this is a cumulative metric, not an individual request percentile. Sampled Go heap stayed around 26–27 MiB. Pool growth was not the solution.

## Correctness and validation

Every run confirmed 10,000 events, 10,000 executions, 10,000 logical deliveries, 10,002 provider attempts, exactly two expected delivery retries, zero runtime retries, zero failed executions and zero duplicate logical actions. Duplicate inputs did not enroll again. Analytics projection drained before final read measurements.

Verified locally: formatting/lint/typecheck, race-enabled unit tests, all package-isolated PostgreSQL integration tests, 32 desktop/mobile browser tests, actual Docker API/worker/web rebuild/readiness and worker restart recovery. New queue tests put older paused jobs and locked rows/customers ahead of eligible work. Existing event/timeout, global-exit, cancellation, lease recovery, version and delivery-policy tests remain green. Live LLM tests remain deferred; hosted CI was not run.

## Remaining optimization opportunities, in priority order

1. **Ingestion admission/backoff:** rate-aware producer pacing, Retry-After/jitter and bounded batch use. Test a concurrency budget that leaves API connections for interactive reads before raising the current rate limit. Measure successful-attempt service time separately from logical retry latency; the current event percentile intentionally includes both. Batch ingestion's limit counts HTTP requests, so event volume also needs separate admission measurement.
2. **Provider head-of-line blocking:** the current delivery loop waits for a group of four dispatches before progressing. A slow real provider can leave otherwise free slots idle and delay outbox scheduling. Test delayed/mixed providers, then consider independent bounded scheduling/dispatch loops. Instant mock results here do not establish real-provider throughput.
3. **Paused/history-heavy queues:** measure large overdue paused prefixes, many workspaces and months of completed-job history. The new guards preserve semantics, but per-candidate eligibility still has a cost. Do not add indexes/denormalized scheduling state or retention machinery without these plans and measurements.
4. **Round trips and CPU:** after the large scans are removed, the runtime still executes several statements per step and reloads immutable workflow JSON. Use Go CPU/allocation profiles and statement timing to decide between bounded SQL batching and a bounded immutable-version cache. Neither was added speculatively.
5. **Analytics freshness:** current summary reads are cheap. Next measure projector lag alongside sustained ingestion and real retention rather than accelerating already-small quiet reads with another cache. The last-24h boundary counts deserve a larger history distribution before more indexing.

Reference: PostgreSQL documents [index ordering with LIMIT](https://www.postgresql.org/docs/17/indexes-ordering.html), [EXPLAIN actual rows/loops/buffer interpretation](https://www.postgresql.org/docs/17/using-explain.html), and [pg_stat_statements execution statistics](https://www.postgresql.org/docs/17/pgstatstatements.html). These explain the access-path and instrumentation choices; all numbers above come from this repository's measured artifacts.
