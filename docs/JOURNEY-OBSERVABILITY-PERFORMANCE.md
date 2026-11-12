# Journey observability load test — 2026-11-12

The new journey Analytics and Live Monitor APIs stayed responsive with **10,000
simultaneous durable waits** and a synchronized timer burst. Final source and
projected counters reconciled. The main finding is **temporary projection lag**,
not a slow summary read: the metric queue reached 34,320 rows and a sampled
oldest-row age of 28.79 seconds during the burst.

## Evidence and reproduction

- Tested source: `81f3a9226e110737dde1fd1346d9e4d2ee2c22d7`, clean working tree.
- [Raw measurements and EXPLAIN plans](benchmarks/journey-observability-10000.json).
- [End-of-run Prometheus snapshot](benchmarks/journey-observability-10000.json.prom).
- `make check` and `git diff --check` passed before the source commit. The
  100-customer smoke run passed before this full run. Reconciliation unit tests
  reject lagging delivery counts, wrong distinct counts and incomplete backfill.

```sh
./scripts/benchmark.sh --customers 10000 --ingest-rate 300 \
  --observe-wait-seconds 60 \
  --out ../../docs/benchmarks/journey-observability-10000.json
```

The runner creates and removes a separate empty PostgreSQL container/database;
it refuses the normal application database. Actual loopback HTTP APIs, ingestion,
runtime workers, delivery workers and the asynchronous projector run together.
Only mock SMS is used. No SMTP, paid LLM, live customer data or external deployment.
Fixture publication and deadline changes belong only to this isolated workload.

## Environment and method

PostgreSQL 17.11, container capped at **2 CPU / 2 GiB**, 256 MiB shared buffers,
8 MiB work memory, tmpfs data. Go 1.26.11 on darwin/arm64, eight host logical CPUs.
The application/client/worker share one process, with separate API and worker
pools limited to 20 each. Eight runtime processors and four delivery processors
are unchanged. Other local containers were running; the host was not dedicated.
This is a warm local database test, not production disk/fsync/network capacity.

10,000 synthetic customers enter one six-node workflow/version on one UTC day.
The runner also inserts 1,000 inactive draft journeys. It offers 300 events/sec,
confirms all 10,000 WAITING executions, holds them for 60 seconds, then advances
only fixture timer deadlines to a common instant. Conditions take the false
branch, schedule one mock SMS and complete. Mock provider 429 and 500 outcomes
exercise two controlled retries. One hundred duplicate ingestion requests must
not create another execution.

Up to five **serial** authenticated UI reads/sec rotate through nine routes. The
four journey-specific routes are live insights without historical distinct,
30-day versioned insights with exact distinct, branch-filtered people and
accepted node/channel messages. Five other routes cover overview, global
analytics, journey list, customer tracking and recent entries. This is background
UI traffic beside runtime load, not a many-user API saturation test. Slow reads
can reduce the offered read rate. Reported route sample counts make that limit
visible. After projection drain, each route receives 30 paced warm reads.

Queue depth/age is sampled once/sec (depth capped at 100,001, never reached).
This diagnostic adds one bounded SQL query/sec. API/worker pool and Go heap samples
run every 250 ms; active PostgreSQL waits every 200 ms. Queue age is the oldest
FIFO row's age, not a percentile of every source event's end-to-end freshness.
The projection sampler remains active through warm probes, so its final
`projection_catchup` phase also includes idle post-drain samples.

## Correctness and throughput

| Check                                               | Actual result             |
| --------------------------------------------------- | ------------------------- |
| Simultaneous active executions                      | 10,000                    |
| Events / executions / logical deliveries            | 10,000 / 10,000 / 10,000  |
| Provider attempts / controlled retries              | 10,002 / 2                |
| Failed executions / duplicate logical actions       | 0 / 0                     |
| Ingestion limiter retries                           | 0                         |
| Ingestion elapsed / observed throughput             | 34.30 s / 291.54 events/s |
| Ingestion HTTP p50 / p95 / p99                      | 11.39 / 77.30 / 99.91 ms  |
| All executions reach WAITING, from ingestion start  | 35.31 s                   |
| All executions complete, from common timer deadline | 62.50 s                   |
| All provider accepts, from common timer deadline    | 88.61 s                   |
| Timer step completion lateness p50 / p95 / p99      | 5.65 / 10.75 / 11.16 s    |
| Projection catch-up after all provider accepts      | 1.01 s                    |

Final API counts for entries, completed, false-branch transitions, logical SMS
scheduled, channel accepted, node accepted and unique customers all equaled
10,000. Waiting and converted equaled zero as expected; baseline coverage was
complete. Loaded HTTP failures, projection sample errors and wait-sampler errors
were all zero. Queue-drain completion is polled every second; the 1.01 s catch-up
measurement is a polling upper bound. Timer lateness measures completed timer
steps, not claims or end-of-journey completion.

## Journey API latency

Milliseconds, real HTTP including auth, database work, serialization and local
round trip. These small route samples yield descriptive interpolated p99 values,
not stable production tail guarantees.

| Phase                         | Route                 | Samples | p50 ms | p95 ms | p99 ms | Max ms |
| ----------------------------- | --------------------- | ------: | -----: | -----: | -----: | -----: |
| 10,000 durable waits          | Live insights         |      33 |  34.05 |  39.46 |  41.03 |  41.21 |
| 10,000 durable waits          | Analytics insights    |      33 |  31.75 |  42.24 |  42.53 |  42.64 |
| 10,000 durable waits          | Branch people         |      33 |   2.94 |   6.32 |   7.12 |   7.45 |
| 10,000 durable waits          | Accepted SMS messages |      33 |   2.96 |   6.21 |   9.07 |  10.10 |
| Synchronized timer processing | Live insights         |      37 |  39.20 |  49.85 |  62.76 |  65.89 |
| Synchronized timer processing | Analytics insights    |      37 |  43.30 |  54.83 |  56.34 |  56.77 |
| Synchronized timer processing | Branch people         |      38 |  52.70 |  70.09 | 109.99 | 133.38 |
| Synchronized timer processing | Accepted SMS messages |      38 |  36.20 |  45.31 |  47.08 |  47.23 |
| Delivery drain                | Live insights         |      15 |   2.85 |   5.35 |   5.36 |   5.36 |
| Delivery drain                | Analytics insights    |      15 |   7.83 |  13.97 |  15.32 |  15.65 |
| Delivery drain                | Branch people         |      14 |  18.32 |  24.13 |  26.09 |  26.58 |
| Delivery drain                | Accepted SMS messages |      14 |   2.48 |   3.39 |   3.43 |   3.45 |

After drain, warm p95 was **9.98 ms** for live insights, **21.52 ms** for Analytics
with exact distinct, **7.33 ms** for branch people and **7.68 ms** for messages
(30 samples each). These phases have different competing work; do not treat
cross-phase variation as an optimization comparison.

## Where the cost remains

1. **Projection queue drain is the first target.** During the burst, peak depth
   was 34,320 and sampled oldest age reached 28.79 s. During stable waits the
   sampled depth/age were zero. The stage-B queue select/delete/JSON aggregation
   statement ran 127 times, **38.28 s cumulative / 301.44 ms mean**. Snapshot
   replacement/upsert itself ran 122 times, **4.50 s / 36.90 ms mean**. These are
   `pg_stat_statements` database statement times, not additive wall-clock stage
   duration. The queue statement includes selection, deletion, locking and JSON
   aggregation; this result alone does not isolate which part is responsible.
   Capture its loaded EXPLAIN/profile before changing the claim/delete or batch
   transport. Increasing worker concurrency or adding a cache would not directly
   remove this cost.
2. **Branch people can inspect many matching records to return a page.** Its
   post-drain plan used `steps_workspace_node_execution`, visited 10,000 matching
   steps, performed 10,000 execution primary-key lookups and a top-N sort for 25
   results: **21.90 ms** SQL in that sample. Timer-phase API p95 was 70.09 ms.
   Pagination bounds output, but this plan does not bound all underlying work.
   Compare an execution-ordered seek/EXISTS access path against this plan on
   selective and common branches before changing the query or adding an index.
3. **Exact distinct is deliberately outside live polling.** The 10,000-member
   distinct component took **13.30 ms**, versus **0.119 ms** for the 47-row cohort
   aggregate. Analytics exactness has a measurable cost; retain
   `include_unique=false` for routine live refresh. Larger history/membership
   needs its own test before promising all-time constant cost.
4. **Snapshot facts have storage/write cost.** 70,000 source facts consumed
   **71.01 MiB including indexes** at the sampled end state, with 46,200 updates.
   The 10,000 membership rows consumed 1.95 MiB; cohort summaries consumed 72 KiB.
   The queue processed 151,372 inserted/deleted records and retained 2.40 MiB of
   relation/index allocation after drain. These are relation statistics/sizes,
   not logical live payload size, WAL bytes or an exact incremental write
   amplification ratio against the previous implementation. Retention and
   vacuum behavior deserve a longer repeated-cycle test.

The accepted-message page used a nested-loop/index access path and took
**0.323 ms** in its post-drain SQL plan. No blanket index addition is justified
by that page. The API pool reached 20/20 acquired connections during ingestion;
its 302 empty-acquire events and 1.36 s aggregate acquire duration include pool
startup/normal acquisition. Worker peak was 14/20. These figures do not establish
sustained pool exhaustion. Sampled Go heap peak was **34.50 MiB**, including the
client, API and workers. Database wait samples were mostly CPU-or-not-waiting
and client waits; this is occupancy sampling, not proof of CPU saturation. The
Go heap and PostgreSQL memory limit are not process/database peak RSS measures.

## Scope and next action

This single synchronized-burst run validates correctness and responsiveness for
this fixture. It does not establish maximum sustained ingestion, concurrent
workspaces/users, long-lived multi-day/version history, historical migration
backfill, restart recovery under this exact load or production storage behavior.
Prior integration tests cover separate correctness scenarios. Older benchmark
runs use different code, pacing/read mixes and no 60-second hold; they are not a
controlled before/after comparison for this change.

Next priority: profile the queue select/delete/JSON statement under the measured
burst, then test one focused change against this same workload. Second priority
is the common-branch people query. Summary reads do not justify adding Redis or
rewriting services in another language from these measurements.
