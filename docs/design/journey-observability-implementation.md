# Journey-level observations

Implemented 2026-11-12, following the [journey observation audit](journey-observability-audit.md).

## Product behavior

Individual journeys share one read-only metrics canvas and record-inspection
surface between Live Monitor and Analytics. Versions, entry periods and step
selection use Reka UI Select; Summary / People / Messages use Reka Tabs. Existing
workspace colors, typography and workflow node identity remain consistent.

- Start at a readable 90% viewport rather than shrinking the entire workflow.
  Go to start, jump/focus a step and fit-all remain available. Polling and list
  refresh do not rebuild the graph or reset a chosen viewport.
- Compact per-type counters show waiting inventory, step reach or provider
  outcomes. Branches show counts and shares of recorded outgoing transitions.
  Global exits are annotated at their source node, not attributed to an unvisited
  Exit node.
- Selecting a node opens a side panel, keeping the canvas visible. Branch counts
  filter contributing executions; blocked/failed reason counts filter logical
  messages. The customer path and transition/wait/message history occupy that
  same detail pane rather than following a long page of executions.
- Execution and message lists use bounded keyset pagination. Channel table counts
  open matching messages. Selecting a message inspects its own execution and
  attempts; unrelated journey/execution IDs are rejected in the UI.
- Published version and entry-period scope apply to summaries, trends, paths,
  channel outcomes and contributing records. The selected version and per-view
  period persist for the session. New unconfigured views choose the latest
  published version; a traced execution selects its immutable version.
- Live defaults to all entry cohorts and polls every five seconds while visible.
  Analytics defaults to 30 days and refreshes explicitly. The live entry card does
  not recompute historical distinct membership on every poll; Analytics reports
  an exact distinct count for the chosen cohort.
- “Waiting now” means current occupancy among the selected entry cohort. A wait
  scheduled more than 30 seconds past due has an operational drilldown; a paused
  journey's held timers are excluded. Configured waiting is not itself a failure.
- Counter projection health and source timestamps are separate from browser
  refresh. An idle source does not become unhealthy merely because no new event
  arrived. Historical preparation is disclosed, as is ongoing counter catch-up.
- The node table uses graph order and includes zero-activity nodes. Alternative
  paths are not described as sequential drop-off. Provider acceptance is never
  labeled confirmed delivery or channel-attributed conversion.

The familiar patterns come from Customer.io's workflow Measure view and Klaviyo's
recipient activity/skip reasons. The implementation uses their interaction
patterns, not their proprietary components or metric definitions.
[Customer.io](https://docs.customer.io/messaging/metrics/automation-metrics/),
[Klaviyo](https://help.klaviyo.com/hc/en-us/articles/115002779351).
[Reka Select](https://reka-ui.com/docs/components/select) manages dropdown
positioning/focus/typeahead; [Reka Tabs](https://reka-ui.com/docs/components/tabs)
manages keyboard tab navigation.

[New application screenshots](../images/journey-observability/README.md) use the
existing fictional demo workspace; no journey was published for these captures.

## Correct outcome semantics

Global exits can specify `reason` as `converted`, `completed`, `condition_not_met`
or `manually_cancelled`. Both production and virtual-time runtime resolve this
through the shared workflow helper. New cancellation exits can therefore stop
without adding a conversion. The builder exposes the outcome and preserves exit
filters. Composer v3 requires the explicit reason in provider structured output.

Existing published snapshots with no reason retain their original `converted`
behavior. They and their historical counts are not silently rewritten. Correcting
an old cancellation definition requires a reviewed new version. This change does
not resolve every business-completeness issue from the separate paid AI audit.

## Projection and query contract

Migrations 0022–0023 extend the existing PostgreSQL metric queue with compact
snapshots; they do not add a second durable-state service. Existing lifetime
metrics remain compatible. New cohort tables contain version-scoped execution,
node, edge, current-position, exit-source and logical delivery/reason summaries.
Message cohort time is the execution's entry time, even if a send occurs later.
Unique membership is stored separately; daily distinct counts are never summed.

The projector selects the newest snapshot per source in a queue batch, subtracts
its prior contribution and adds the new snapshot atomically. Retried attempts
update one logical message's facts. A bounded 200-source cursor backfill runs in
the projector, not in a reporting request or the migration. Baseline snapshots
cannot supersede later real transitions, regardless of enqueue order. Backfill
and queue progress are separate from a guarantee of zero projection lag.

- `GET /journeys/{id}/insights?version=2&days=30` returns one entry-cohort scope,
  summaries, daily trends, distinct membership and health/coverage metadata.
- `include_unique=false` omits the distinct-membership calculation for live polls.
- Version zero aggregates high-level summaries only; it omits node/edge identities
  that could be reused with different meanings across versions.
- Execution lists accept version, time bounds, node/branch/current-position,
  outcome, active/state and overdue filters, retaining their existing cursor.
- Journey message lists filter by that execution cohort, version, source node,
  channel/status/reason and message cursor. Detail and attempts load on demand.
- Overdue lookup uses a stable statement timestamp and an indexed due-work access
  path. Runtime lease fences keep their existing clock and concurrency rules.

History is not rescanned to reconstruct dashboard counters. Per-person detail and
paged contributing record lists read authoritative source records. The exact
membership query and aggregate cohorts still grow with reporting history; this
change does not claim constant cost for arbitrarily large all-time reports.

## Verification and performance evidence

`make check` passed. The complete PostgreSQL integration suite passed, followed by
a focused API suite after the final polling-query change. All **72 desktop/mobile
Playwright cases passed**. The new browser case exercises two immutable versions,
blocked-recipient drilldown, 27-row execution/message pagination, keyboard tabs,
viewport focus, period/version restore and rejection of a mismatched execution.
PostgreSQL tests reconcile summary/branch/message counts, exact cohort distincts,
workspace isolation, retries and stale baseline priority. Runtime and simulation
tests cover explicit non-conversion global exits and legacy default semantics.

[Recorded EXPLAIN ANALYZE plans](../validation/journey-observability-2026-11-12/query-plans.txt)
were captured against the running local demo, with 9,075 cohort-summary rows across
the database and 18 executions in the inspected journey. A 37-row scoped summary
used the cohort primary-key index and executed in **0.202 ms**. The zero-result
overdue check executed in **0.142 ms**; for this small journey PostgreSQL chose the
active-execution and execution-job indexes, rather than the new workspace/due
index. These are individual warm local SQL samples, not API p95 values, a
10,000-execution capacity test or evidence that PostgreSQL always chooses the
same plan. Larger-load/write-amplification measurements remain future work.

No paid model call or external deployment was performed for this implementation.

## Deliberate limits

Audience/plan/locale snapshot breakdowns and message conversion attribution are
not implemented here. Open/click/bounce/webhook delivery facts are not invented.
Historical condition results show recorded booleans/ports and the immutable
workflow; arbitrary historical customer values are not reconstructed from a
current profile. Branch totals use recorded ports, not guessed historical paths.
Overdue counts currently cover scheduled waits, not every possible stalled claim.
Counter storage and projection add write work; the sampled read plans do not
establish its large-load cost. Retention/erasure procedures must include the
snapshot facts and cohort membership keys.
