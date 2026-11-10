# Analytics and PostgreSQL read paths

Metrics are asynchronously projected from compact transactional deltas, with a visible processing timestamp. Source-of-truth transitions append one queue record in the same transaction. No dashboard request calculates totals from historical execution/step/message JSON.

The projector claims at most 1,000 records with SKIP LOCKED. Queue deletion, distinct-customer insertion, lifetime/hourly counters and freshness watermark commit atomically. A crash or error rolls back the entire batch. An advisory lock serializes projector replicas only; ingestion/runtime workers do not take that lock. Batched JSON recordsets avoid per-record network calls and temporary-table catalog churn. Updates group deltas before writing counters, reducing hot-row writes. Duplicate events create no new execution and no new metric delta. Retried deliveries count as one attempted logical send.

`metric_totals` serves exact workspace/journey/version prefixes. The zero UUID denotes rolled-up workspace or all-version scope. Node counters are retained only per actual version, avoiding meaningless cross-journey node-ID aggregation. Node history is not copied into hourly tables. `metric_hourly` serves bounded UTC date ranges (maximum 90 calendar days). It includes workspace and journey summaries, avoiding sums across every journey on dashboard reads. Raw message content never enters the projection.

`metric_customers` inserts each workspace/journey/customer once. It also inserts one workspace-wide identity, so a customer in two journeys is not counted twice at workspace scope. Lifetime distinct counts are not computed by adding daily distinct counts. Time-ranged unique-customer metrics are deliberately not presented as lifetime counts.

Execution trends are entry-date cohorts: a later conversion updates the original entry cohort. Conversion rate is converted logical executions / logical entries in that cohort. Reentries count as entries; lifetime unique customers are separate. Channel cohorts use logical delivery creation time. `scheduled` currently represents logical delivery plans, including policy-blocked plans; ACCEPTED/BLOCKED/FAILED/UNKNOWN and attempted are separate measures. Provider acceptance does not prove delivery. Opens/clicks are not collected. Terminal duration is the sum of terminal execution durations divided by their count, including all terminal outcomes; averages are never averaged together.

Node success means the runtime step completed or scheduled its logical action. The execution inspector separately shows delivery eligibility, blocked/skipped reasons and provider attempts. Per-version node statistics prevent v1 executions from being plotted against v2 definitions. Customer tracking switches to the execution's actual immutable version.

Last-24-hour entries combine fully covered hourly summaries and two indexed raw boundary ranges of at most one hour each. This avoids rescanning 24 hours of execution history on every dashboard refresh. Summary data remains eventually consistent while the projector catches up. Full interval bounds are returned by the endpoint.

Query rules:

- Workspace is the leading key for all interactive reads. Recent execution, failure, customer/journey/state filters use matching ordered indexes and LIMITs.
- Live polling is every five seconds while visible, stops on unmount and does not overlap requests. The canvas reads bounded counters; tracking loads at most 50 executions and one inspector.
- Lists project metadata before joins and never fetch workflow or rendered HTML bodies. Large definitions/bodies are loaded by ID.
- Inbox pagination uses `(created_at,id)`; no OFFSET. Channel/status filters use indexed columns, not JSON expressions.
- Indexes are added for actual paths, not arbitrary custom attributes. The superseded inbox run index is removed. No broad JSON GIN indexes or Redis cache is required.
- The [measured performance report](PERFORMANCE.md) records a 10,000-execution workload, concurrent reporting reads, PostgreSQL statement costs and EXPLAIN (ANALYZE, BUFFERS). Queue access order was the major bottleneck; summary reads remained small, so no reporting cache or additional production index was introduced.

Retention must preserve aggregate counters independently of raw execution retention. Removing customer-linked records must also erase `metric_customers` identities and queued customer identifiers without decrementing anonymous historical aggregates. See the security/retention policy delivered in phase 11.
