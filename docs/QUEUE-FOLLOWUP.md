# Queue follow-up: bounded scans and timer transport

11 November 2026. Continues [scheduler transaction measurements](SCHEDULER-LOCKS.md). The first change reduces repeated pending-queue work while retaining one customer per transaction. The second batches the transport of runtime lock reads; it does not merge their SQL statements or relax state checks.

## Scheduler continuation

Within a public drain of at most 32 actions, the scheduler seeks after the last committed `(created_at, id)` using the existing `outbox_pending` partial index. Each action still commits before another customer is locked. The cursor advances only after a successful commit and is discarded at the end of the drain. Every next drain starts at the beginning, so an earlier locked customer or an action inserted with an earlier timestamp is reconsidered. No persistent cursor, migration or new index is needed.

Real PostgreSQL tests cover equal timestamps, the UUID tie breaker, earlier locked customers, backdated inserts, cancellation/partial progress and the existing lock budget. EXPLAIN verifies both a bounded row visit count and an index condition on the continuation tuple. This is a bounded scan optimization, not a delivery guarantee change.

## Runtime lock-read batching

Customer, journey and execution reads remain separate READ COMMITTED statements in that order. pgx sends them together instead of waiting for each result before sending the next query. The batch closes before the job fence runs. The lease clock is deliberately read **after** the lock reads complete; capturing it before a lock wait would allow expired work to advance.

Regression tests hold the customer lock, pause the journey or advance the virtual clock past the lease, then release the lock. No node may execute using a stale state or stale lease; a fresh claim subsequently resumes normally. Existing event/timeout/global-exit races, transaction rollback and immutable-version tests remain required.

SQL telemetry records one sanitized `postgresql BATCH` span and a whole-batch duration. It does not interpret time spent consuming buffered results as separate server execution durations. SQL text, bind parameters and raw errors remain excluded. PostgreSQL statement statistics retain the per-statement breakdown.

## Measurements

Same profiled workload as the preceding report: fresh PostgreSQL 17, 2 CPU / 2 GiB, tmpfs, 10,000 simultaneous waiting executions, 450 offered events/sec, eight runtime slots, four dispatchers, five UI reads/sec and a common timer deadline five seconds ahead. No heavy builds/tests run concurrently. Every result links its source commit and dirty flag; matching Prometheus/CPU/allocation artifacts are alongside the JSON.

These matched runs use Go 1.26.10, as recorded in each artifact. The subsequent hosted audit identified GO-2026-6859, prompting a separate Go 1.26.11 security update. Historical benchmark files are unchanged; their numbers must not be relabeled as measurements of the patched toolchain.

| Variant                                                   | Source    | Waiting (s) | Completed (s) | Mock accepted (s) | Timer p95 (s) |
| --------------------------------------------------------- | --------- | ----------: | ------------: | ----------------: | ------------: |
| [Previous 1](benchmarks/fresh-guard-1.json)               | `5e7a9d1` |      27.479 |        48.351 |            71.419 |        10.445 |
| [Previous 2](benchmarks/fresh-guard-2.json)               | `5e7a9d1` |      23.621 |        49.414 |            72.573 |        10.387 |
| [Cursor 1](benchmarks/cursor-1.json)                      | `80329e6` |      29.545 |        48.337 |            67.412 |        10.375 |
| [Cursor 2](benchmarks/cursor-2.json)                      | `80329e6` |      30.887 |        48.427 |            67.498 |        10.463 |
| [Cursor + lock-read batch 1](benchmarks/readbatch-1.json) | `5c1b4d6` |      23.589 |        46.376 |            67.474 |         9.508 |
| [Cursor + lock-read batch 2](benchmarks/readbatch-2.json) | `5c1b4d6` |      26.749 |        45.379 |            65.448 |         9.411 |

The cursor reduces cumulative scheduling claim SQL from 23.0–25.0s to 16.4–18.7s across stage B. Mock acceptance drops from 71.4–72.6s to 67.4–67.5s. Adding lock-read batching lowers timer p95 in both repetitions to 9.41–9.51s and execution completion to 45.4–46.4s; total mock acceptance is 65.4–67.5s. The earlier original 59–60s drain is still faster. All four new runs retain 10,000 logical deliveries, the two injected retries, zero duplicate actions, zero failed executions and zero loaded-read failures. Timer-phase confirmed-blocker samples are 0/0 for cursor repetitions and 0/1 for batched repetitions, with zero diagnostic errors. These occupancies are not exact wait durations.

The synchronized burst remains a stress case, not an ordinary single-customer latency. Provider acceptance is a mock-adapter outcome, not proof of external delivery. Do not attribute ingestion variation to a scheduler-only change: there are only two repetitions per variant on a shared local host.

## Deadline distribution

The benchmark additionally supports `--timer-spread-seconds` from 0 to 600. Zero preserves the original burst. A nonzero value assigns only the isolated fixture's pending timers evenly across the declared window; one timer stays at the first deadline and unrelated workspace/job state is untouched. This is **workload generation**, not production jitter or a change to customer-configured wait times.

```sh
./scripts/benchmark.sh --customers 10000 --ingest-rate 450 --timer-spread-seconds 60 --profile --out /absolute/path/spread.json
```

The report declares first/last due times. Timer lateness is measured against each job's own due time. Total completion/drain is measured from the first deadline and includes the intentional spread, so it cannot be compared directly with burst elapsed time as a throughput improvement. Legacy `stage_*_jobs_per_second` keys count logical node completions over the whole window and are not worker service capacity.

One clean [60-second spread run](benchmarks/spread-60.json), source `98cb892`, retains 10,000 simultaneous WAITING executions before release. Timer p50/p95/p99 are 0.283s / 0.554s / 0.613s. Executions complete 60.476s and mock sends are accepted 68.524s after the first deadline, including the 60-second spread. All 10,000 logical sends finish with zero duplicates, zero failed executions and zero loaded-read failures. This single diagnostic run shows the effect of this workload's deadline distribution; it is not a matched production optimization claim or a universal timer SLO. Production customer deadlines are unchanged.

## Remaining work

Bounded transaction scope is retained, but the original pre-isolation baseline's 59–60s mock-send drain remains an explicit comparison. Repeated candidate selection, dispatch, projection and round trips still need profiling as a full pipeline; reducing one SQL total does not prove an equivalent end-to-end gain. Sustained traffic, hot customers, slower storage and delayed providers require separate workloads before setting SLOs.

The UI/UX, workspace-aware AI catalog, richer analytics and operational integrations in the [design direction](design/modern-saas-direction.md) remain separate work. Live LLM evaluation and deployment remain deferred.
