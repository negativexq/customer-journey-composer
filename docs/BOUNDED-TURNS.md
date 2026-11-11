# Bounded runtime turns — measured implementation

10 November 2026. Implements the first experiments from [runtime research](RUNTIME-RESEARCH.md): wait diagnostics, bounded advancement and fewer transactional state writes. Does not claim that all remaining contention is solved.

## Behavior and safety

- A claimed job may advance up to eight short nodes using the already-locked customer/journey/execution and its immutable workflow snapshot. A soft 10ms budget is checked between nodes; it is not a hard SQL timeout. The existing 15s processing context remains the outer limit.
- A new wait, event wait, message outbox intent, attribute mutation, terminal node or exhausted budget ends the turn. Timer resume always yields its dependent node as a durable continuation, so expensive downstream work does not run inside each due timer acknowledgment.
- Every node retains its own step history and analytics facts. The prior RUNNING insert followed by WAITING/SUCCEEDED update occurred inside one transaction and was not externally visible. A single final-state upsert now preserves the wait's original start time and avoids a redundant write/projection delta.
- Root claim/token checks, customer/journey/execution lock order, version isolation and atomic outbox remain. No external provider call occurs inside this transaction. A SQL failure rolls back the entire turn; a domain failure on an inline node materializes a failed job at that node so manual recovery does not replay preceding committed mutations.
- No migration, new production index, larger pool, Redis or workflow-definition cache was added. Production provider behavior and sandbox pure node decisions are unchanged.

Source commits: `a846d60` diagnostic tooling, `89e373f` initial bounded turns, `1aa63b2` timer fairness and one-write node states. `9ff4258` subsequently changes the Go module/import namespace to `github.com/negativexq/customer-journey-composer/apps/api`; that commit changes no runtime behavior.

## Matched comparison

```sh
./scripts/benchmark.sh --customers 10000 --ingest-rate 480 --profile --out ../../.tmp/steps.json
./scripts/benchmark.sh --customers 1000 --ingest-rate 480 --provider-delay-ms 100 --provider-delay-every 4 --profile --out ../../.tmp/steps-delayed.json
go tool pprof -top .tmp/steps.json.cpu.pprof
go tool pprof -top -alloc_space .tmp/steps.json.allocs.pprof
```

Same isolated PostgreSQL 17.11 (2 CPU, 2 GiB, tmpfs), 20-connection API/worker pools, eight runtime slots, four dispatchers, 480 offered events/s and five concurrent UI reads/s as the operational follow-up. New instrumentation is enabled in both baseline and final: CPU/allocation profiling and 200ms wait sampling via one dedicated diagnostic connection. SQL statements/statistics and metrics remain enabled. No external LLM/provider calls, production data or concurrent builds/test suites during timed runs. Monitoring services remained running.

All five runs started with clean source trees. This instrumented baseline is separate from earlier unprofiled comparisons. The source commits preceded the namespace-only change. Each JSON below has a matching `.json.prom` snapshot in the same directory; raw pprof files remain ignored under `.tmp` and can be regenerated with `--profile`.

| Run                                      | Source    | Artifact                                            |
| ---------------------------------------- | --------- | --------------------------------------------------- |
| Before                                   | `a846d60` | [steps-before.json](benchmarks/steps-before.json)   |
| Superseded: inline timer downstream work | `89e373f` | [steps-after.json](benchmarks/steps-after.json)     |
| Final repetition 1                       | `1aa63b2` | [steps-final-1.json](benchmarks/steps-final-1.json) |
| Final repetition 2                       | `1aa63b2` | [steps-final-2.json](benchmarks/steps-final-2.json) |
| Delayed provider, 1,000 customers        | `1aa63b2` | [steps-delayed.json](benchmarks/steps-delayed.json) |

## Results: 10,000 simultaneously active executions

Stage A begins before ingestion; the other elapsed times begin at the shared due deadline. Timer lateness uses the persisted timer-step completion timestamp, not transaction commit time. Legacy report keys named `stage_*_jobs_per_second` calculate logical node completions/sec, not actual durable-job counts; bounded turns make this distinction important.

| Measure                                 |   Before | Superseded |  Final 1 |  Final 2 |
| --------------------------------------- | -------: | ---------: | -------: | -------: |
| All executions WAITING (stage A)        |  40.415s |    26.002s |  22.958s |  23.495s |
| All executions completed (stage B)      |  68.453s |    73.604s |  61.433s |  64.453s |
| All mock messages provider-accepted     |  68.465s |    73.623s |  68.471s |  69.481s |
| Timer lateness p95                      |  11.412s |    36.503s |  10.352s |  10.526s |
| Timer lateness p99                      |  11.911s |    38.322s |  10.761s |  10.915s |
| Successful ingestion p95                | 69.438ms |   55.348ms | 52.029ms | 48.381ms |
| Successful ingestion p99                | 88.272ms |   72.731ms | 73.133ms | 61.856ms |
| Stage-B customer reads/locks            |   40,000 |     20,727 |   30,117 |   30,149 |
| Stage-B runtime claim calls             |   10,417 |      7,523 |    8,451 |    8,277 |
| Cumulative SQL time: customer read/lock |  43.594s |    42.507s |  68.131s |  78.214s |

Final entry-to-wait is 41.9–43.2% shorter; timer p95 is 7.8–9.3% lower; runtime completion is 5.8–10.3% shorter. Total mock provider drain is essentially unchanged, with a slightly slower second run. Customer-query count falls about 25%, but cumulative time on that query rises. This is a remaining bottleneck, not evidence of a solved locking problem.

The superseded implementation demonstrates why fewer jobs alone is insufficient: doing condition/message work in each timer turn pushed timer p95 to 36.5s. The final version explicitly restores the timer boundary. This intermediate run is preserved to avoid hiding the regression.

Ingestion Analytics p95 was 65.0ms (23 samples), then 15.0ms and 37.9ms (22 each). Samples are too few for a reliable UI tail/SLO claim; no general analytics speedup is asserted. No loaded-read failures or HTTP 429 retries occurred in these runs.

The 1,000-customer delayed-provider run accepts all messages in 10.049s, with timer p95 1.109s and no duplicate/failed logical actions. It is a fault-safety check, not a matched delayed-provider improvement claim: this turn did not rerun a profiled delayed baseline. The separate normal runs meet the 10,000-active threshold.

## Diagnosis: real waits, not guesses from total SQL duration

The sampler records only phase, normalized PostgreSQL query ID, wait category/event, active-backend counts, confirmed-blocker counts and maximum transaction age. It stores no raw SQL, arguments, customer IDs or backend PIDs. Collection is bounded to 512 aggregate groups, with a 100ms per-sample timeout. It adds no per-node production SQL or telemetry writes.

Before: stage B (`timer_burst`, including subsequent runtime/delivery work until executions complete) had 217 Lock-category backend samples, of which 212 confirmed blockers belong to `SELECT to_jsonb(c) ... FOR UPDATE`. This establishes actual row/transaction contention, not that all 43.6 cumulative SQL seconds are lock waits. Final repetitions have 241 and 418 Lock-category samples. Final 2 has two failed diagnostic samples; other runs have zero. These gaps are preserved in the report and must not be counted as “no wait.” Sampled occupancies are not independent latency observations or exact wait seconds. `CPU_or_not_waiting` does not prove CPU saturation.

The baseline allocation profile reports about 3,081 MiB cumulative allocated bytes versus 2,338 MiB in final 1. These are sampled allocation totals for the shared client/API/worker process, not peak/live heap or PostgreSQL memory. Leading allocation sites include pgx encoding/binding/row decoding and slice growth. CPU profiles contain substantial runtime/syscall activity; they do not measure PostgreSQL CPU and do not justify calling workflow JSON decoding the primary CPU bottleneck.

## Remaining work

At this report's source revision, the outbox scheduler locked up to 32 customer rows while processing its transaction. Runtime transitions need the same customer serialization. The sampler identifies waiting statements but not the exact blocker statement, so it does not prove attribution to that scheduler. Provider network calls already ran after commit and were not held under the customer lock.

The subsequent [scheduler transaction report](SCHEDULER-LOCKS.md) measures smaller batches and per-action commits, records regressions and describes the retained implementation. Queue fairness with realistic staggered deadlines and threshold-based refill remain separate experiments. Customer serialization, consent/caps, global exit and token fencing must remain intact.

## Verification

New real-PostgreSQL tests cover node history/action boundaries and projected node counts, durable budget yield/restart, inline failure/manual recovery without repeating mutations, SQL rollback/retry, and timer boundary/original wait duration. The locked-backend diagnostic test is included in the default integration command and CI. Final checks passed: offline Go build after module rename; formatting/lint/typecheck/race unit tests; complete isolated PostgreSQL suite including diagnostics; Docker API/worker rebuild; 32/32 browser tests; durable API-to-provider telemetry and alert-rule tests; actual worker restart recovery. Gitleaks found no leaks in the Git history before publication. Live LLM tests remain deferred.
