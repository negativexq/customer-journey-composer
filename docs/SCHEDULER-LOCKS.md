# Scheduler transactions: measured tradeoffs

11 November 2026. Follow-up to [bounded runtime turns](BOUNDED-TURNS.md). The scheduler now commits each logical delivery before locking another customer. This bounds lock scope and reduces observed contention, but does **not** establish an end-to-end throughput improvement over the original baseline.

## What changed

The previous scheduler held up to 32 customer locks while rendering, checking eligibility and writing a batch. Runtime transitions serialize on those same customers. Simply lowering the batch size increased claim overhead and did not consistently improve the full pipeline.

The retained implementation uses an ordered pending-outbox access path and one customer per scheduling transaction. A public drain still processes up to 32 actions before yielding. Each action commits before the next customer lock; provider calls remain outside the transaction. The ordering subquery is deliberate: without that boundary, a lateral join can lock more customers than the final LIMIT suggests.

A fresh READ COMMITTED statement rechecks execution state, takes the journey share lock, locks the still-pending outbox row and reads bounded frequency reservations. It preserves customer → journey → outbox lock order and avoids using the earlier claim snapshot after a concurrent global exit. Delivery creation and outbox acknowledgement share one data-modifying statement. Existing uniqueness, consent, frequency caps, pause/exit checks and provider ambiguity handling remain enforced.

Source: `f571839` ordered access, `32c8251` combined write, `314a5f1` per-action commit, `5e7a9d1` combined fresh guards. No scheduling index, Redis, extra connection capacity or concurrency increase was added.

## Method and artifacts

Each run starts a fresh isolated PostgreSQL 17 container constrained to 2 CPU / 2 GiB with tmpfs storage. Actual loopback HTTP ingestion, eight runtime slots, four dispatchers, 20-connection pools, five authenticated reads/sec and CPU/allocation profiling are enabled. The offered rate is 450 events/sec. Ten thousand executions first reach durable WAITING; their timer deadlines then move to one common time five seconds ahead. Two mock-provider failures exercise retries. No SMTP, paid LLM, production data, concurrent builds or test suites are involved. Local monitoring services stay running.

```sh
./scripts/benchmark.sh --customers 10000 --ingest-rate 450 --profile --out /absolute/path/result.json
```

Stage A starts before ingestion. Completion and provider-drain times start at the common timer deadline. Timer lateness uses persisted step completion, not claim time. Completion polling is one second. Statement totals exclude network round trips; cumulative SQL duration is not lock-wait duration. PostgreSQL wait sampling runs every 200ms and reports occupancy, not exact wait seconds or proof of the blocking transaction's identity.

Each linked JSON has matching `.prom`, `.cpu.pprof` and `.allocs.pprof` artifacts. All source trees were clean except `locks-after-1`, whose dirty flag reflects an untracked baseline metrics artifact; it is retained as an exploratory result. Historical filenames containing “final” describe an earlier experiment, not the current implementation.

| Experiment / raw result                                       | Source    | All waiting (s) | All completed (s) | All accepted (s) | Timer p95 (s) | Blocked samples / sampler errors |
| ------------------------------------------------------------- | --------- | --------------: | ----------------: | ---------------: | ------------: | -------------------------------: |
| [Original baseline 1](benchmarks/locks-before.json)           | `c700148` |          32.480 |            52.360 |           59.397 |        10.649 |                           59 / 0 |
| [Original baseline 2](benchmarks/locks-before-2.json)         | `c700148` |          26.755 |            50.358 |           60.417 |        10.529 |                            4 / 0 |
| [Batch 8, original access](benchmarks/locks-after-1.json)     | `871edea` |          23.717 |            49.335 |           69.401 |        10.529 |                            0 / 0 |
| [Batch 8, ordered access 1](benchmarks/locks-after-2.json)    | `f571839` |          23.704 |            56.376 |           65.436 |        10.531 |                          141 / 0 |
| [Batch 8, ordered access 2](benchmarks/locks-after-3.json)    | `f571839` |          30.672 |            51.387 |           62.432 |        10.704 |                           43 / 0 |
| [Batch 16, combined write](benchmarks/locks-final-1.json)     | `32c8251` |          30.222 |            59.461 |           59.478 |        10.290 |                          242 / 0 |
| [Batch 32, ordered/write 1](benchmarks/locks-batch32-1.json)  | `032a218` |          28.539 |            61.526 |           67.562 |        11.537 |                          101 / 1 |
| [Batch 32, ordered/write 2](benchmarks/locks-batch32-2.json)  | `032a218` |          26.374 |            63.455 |           70.498 |        10.214 |                          218 / 0 |
| [Per-action, separate guards 1](benchmarks/per-action-1.json) | `314a5f1` |          25.717 |            48.813 |           77.896 |        11.792 |                            1 / 3 |
| [Per-action, separate guards 2](benchmarks/per-action-2.json) | `314a5f1` |          25.599 |            47.391 |           78.485 |        10.611 |                            2 / 1 |
| [Retained, combined guards 1](benchmarks/fresh-guard-1.json)  | `5e7a9d1` |          27.479 |            48.351 |           71.419 |        10.445 |                            0 / 0 |
| [Retained, combined guards 2](benchmarks/fresh-guard-2.json)  | `5e7a9d1` |          23.621 |            49.414 |           72.573 |        10.387 |                            0 / 0 |

## Interpretation

Retained repetitions complete executions in 48.351–49.414s and drain mock sends in 71.419–72.573s. Original baseline repetitions complete in 50.358–52.360s and drain in 59.397–60.417s: execution completion improves in this fixture, while final provider drain is about 18–22% slower. Timer p95 stays around 10.4s, so this change does not resolve the synchronized timer burst.

Both retained runs have zero confirmed blocked samples and zero sampler errors during the timer phase; this is sampled evidence, not proof that no wait ever occurred. Original baseline blocked samples are 59 and 4; intermediate ordered batch-32 runs have 101 and 218. Retained claim statements execute about 10,000 times and consume about 23 cumulative SQL seconds across stage B (timer processing plus delivery drain), compared with roughly 400 calls / 1.5s for ordered batch-32. This exposes the cost of restarting the ordered claim for each action.

Loaded timer-phase Analytics p95 varies from 33.4ms (53 samples) to 4.3ms (55); successful ingestion p95 varies from 68.7ms to 41.8ms. There are no loaded-read failures. API pool acquisition reaches its limit of 20 in both runs; cumulative acquisition duration is 1,338ms and 299ms. Worker acquisition peaks at 14/20. Peak Go allocated heap is about 29.6–30.3 MiB for the combined benchmark client/API/worker process, excluding PostgreSQL. Event-lookup EXPLAIN execution is 0.022ms and 0.012ms in warm local storage.

Smaller batches alone were rejected. Ordered claims reduce cumulative claim SQL at batch size 32, but those transactions still retain several customer locks. Per-action commits remove that batch-wide retention and improve runtime completion relative to that intermediate implementation; they also increase claim/commit frequency and leave a longer delivery tail.

The original baseline remains faster at draining all mock messages. Keep that comparison visible: fewer sampled lock waits and faster execution completion do not imply faster external delivery. The retained change bounds transaction scope and improves isolation between customers; scheduler claim/round-trip overhead remains the next throughput target. Staggered deadlines, sustained arrivals, slow providers and production storage require separate experiments before setting latency objectives.

API samples during these runs are modest and share the benchmark host with the client and workers. The raw files preserve count, p50/p95/p99, failures, pool acquisition pressure, sampled waits and peak Go heap. Do not treat a handful of UI tail samples as a service-level guarantee. Reporting reads continue to use indexed projections; no new reporting scan was placed in execution transactions.

## Event lookup index

The navigation feature adds one independently justified migration, `0018_execution_event_lookup.sql`: `(workspace_id, entry_event_id, started_at DESC, id)`. The bounded event-to-execution inspector query uses tenant/event equality and stable ordering; it does not fetch message bodies. Benchmark artifacts include EXPLAIN ANALYZE/BUFFERS for this query at 10,000 executions. Runs after `032a218` include the index's write cost. Earlier baselines do not, so ingestion differences cannot be attributed solely to scheduler changes. No payload or speculative analytics indexes were added.

## Correctness and verification

Every reported successful run persisted 10,000 events, executions and logical deliveries, 10,002 provider attempts, the two injected delivery retries, zero runtime retries, zero failed executions and zero duplicate logical actions.

Real PostgreSQL regression tests check that customers beyond the current transaction remain unlocked, the ordered claim reads only its row budget, the first action is visible while a second transaction is blocked, and cancellation/retry preserves partial progress without duplicates. The full integration suite also covers global exits, event/timeout races, frequency reservations, lifecycle/version isolation and lease recovery. The final runtime source passed quality/race checks, isolated integration tests, actual worker restart recovery and hosted CI at descendant `3d43888`. Live LLM evaluation and deployment remain deferred.
