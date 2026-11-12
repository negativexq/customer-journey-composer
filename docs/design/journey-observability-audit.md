# Journey analytics and live monitoring audit

Date: 2026-11-12. Scope: individual journey Analytics, Live Monitor, node/branch
breakdowns and customer inspection. This is a read-only audit and proposed product
contract, not an implemented redesign.

## Implementation update

Core journey scope, canvas/detail drilldowns, branch/message projections and
explicit exit outcomes are now implemented. The original findings below remain
an audit record. [Implemented behavior, screenshots, checks and remaining limits](journey-observability-implementation.md)
distinguishes delivered work from later audience/attribution features.

## Evidence

Reviewed `AnalyticsView.vue`, `LiveMonitor.vue`, `WorkflowNode.vue`, API analytics
and execution routes, the asynchronous projector and migration 0010 metric
triggers. Opened the existing demo **Welcome & Activation** journey through the
running UI: published v1, 18 listed executions, 14 active and four reported
conversions. These are existing fictional demo records, not a performance claim.
No journey, customer, event, execution, message or publishing mutation was made;
only the normal demo sign-in session was created. No AI call was made.

Local screenshots and read-only API evidence are under
`.tmp/journey-observability-audit/` (not committed). There was only one published
version in this inspected journey: the cross-version issue below is established
by the client/API source, not a reproduced multi-version UI test.

## What is already useful

- PostgreSQL source records and durable execution/message history.
- Asynchronous, atomic metric delta projection; routine analytics does not rebuild
  counts by scanning execution JSON. Logical sends are separate from attempts.
- Published-version node counters and selected-customer path highlighting.
- Polling every five seconds while the monitor is active and the document visible;
  no polling overlap, response selection guards and pause on deactivation.
- Execution transitions, pending waits, delivery decisions and attempts available
  in the inspector. Provider acceptance is explicitly distinct from delivery;
  unavailable open/click tracking is disclosed.

## Confirmed gaps

| Priority | Current behavior                                                                                                                                                           | Consequence                                                                                                                                                                                                   |
| -------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| P0       | Global exits count as converted even for refund/cancellation events, as reproduced in the paid audit.                                                                      | Conversion breakdowns can be wrong before any chart is drawn. Separate business goal outcomes from cancellation exits.                                                                                        |
| P0       | Monitor version changes the graph and node metrics, but neither the execution-list request nor its API accepts a version filter.                                           | A selected-version graph can be accompanied by executions from other versions. Tracking one execution switches to its version, but does not fix list scope.                                                   |
| P1       | Journey summary and channels are lifetime totals; the trend period changes only the trend; the node view selects the latest published version automatically.               | Neighboring sections have different scopes. Labels partially explain this, but users cannot make a coherent period/version comparison.                                                                        |
| P1       | Node metrics contain state counts/duration, not source-port/edge counts or node-specific delivery outcomes/reason groups.                                                  | Cannot answer which branch loses people, which message step is blocked, or why without individual record inspection. A successful message step can mean scheduling succeeded while the provider later failed. |
| P1       | Funnel iterates returned metric rows, not workflow topology, and includes only nodes with an entered metric.                                                               | Observed order starts with “Wait for activation”, then “New account”. Zero-entry nodes disappear, and alternatives do not form a sequential funnel.                                                           |
| P1       | Monitor has no node-click metric/customer drawer; execution inspector follows the entire list below the canvas.                                                            | Reading a path and diagnosing its people require scrolling away from the context.                                                                                                                             |
| P1       | Monitor uses fit-to-view with metric-heavy cards.                                                                                                                          | Observed six-node demo opens at 50%; labels and numbers are tiny. Builder readability improvements are not fully shared with this view.                                                                       |
| P1       | Analytics cards navigate to general journeys/customers rather than the filtered contributing records. Monitor list has no next-page control despite API keyset pagination. | A count is not an actionable explanation; only the latest 50 executions are listed by default.                                                                                                                |
| P1       | API returns projection watermarks but Analytics does not show them; Monitor's timestamp is browser refresh time.                                                           | A freshly fetched screen can still contain old projected counts. Mixed current record state and delayed counters need separate freshness indicators.                                                          |
| P2       | Header continues to emphasize “Editing draft” and builder actions while viewing live published metrics.                                                                    | Published data and editable draft context are visually ambiguous.                                                                                                                                             |
| P2       | No channel attribution policy/window or confirmed delivery/click integration.                                                                                              | Current data cannot support claims that a specific email/SMS caused a conversion.                                                                                                                             |

## Proposed journey workspace

Keep Builder, Preview & Test, Live Monitor, Analytics and Versions around one
journey. Live Monitor and Analytics should share graph rendering, node identities
and record drilldowns, with different defaults.

The journey header names the live published version and goal. Draft changes are
an explicit secondary link. Reading performance does not imply editing that draft.

### Live Monitor: what is happening now?

Use a compact operational strip: running, waiting, due-but-not-progressing, failed
and uncertain deliveries. Show “Live · refresh 5s”, freshness and a pause control.
A waiting customer is not an error; a configured wait past due needs to be checked
against lifecycle pause and scheduler tolerance before an alert is shown.

The canvas takes most of the page. Open at a readable trigger/current-step zoom,
with go-to-start, focus selection and fit-all controls. Do not automatically refit
on each poll. Use calm counters, not animated particles to imply live throughput.

Node summaries depend on their job:

| Node               | Compact summary                             | Click-through                                          |
| ------------------ | ------------------------------------------- | ------------------------------------------------------ |
| Trigger            | Entered; recent entry volume                | Entry records and their start event                    |
| Delay / wait-event | Waiting now; expected next release/deadline | People, wait start, due time, awaited event            |
| Condition          | Yes/No or matched/timeout counts and shares | Rule values, chosen branch and contributing executions |
| Email / SMS        | Provider accepted; blocked/failed           | Logical messages, decision reasons and attempts        |
| Exit               | Terminal reason/outcome                     | People and recorded goal/cancel evidence               |

Show global exits as separate annotated exits from the actual current step. A
customer can convert without visiting a drawn Exit node; zero visits to that node
must not make the conversion count look inconsistent.

Clicking a node opens a side drawer with **Summary / People / Messages**. The
canvas remains visible. Selecting a person shows the traversed path and a concise
timeline: event → decision and values → wait/deadline → send or skip reason →
outcome. Technical jobs, lease/retry/trace details remain under Advanced.

Search should find a person by permitted customer identifiers, not require a user
to know an internal execution ID. Show a person's separate enrollments/version
when re-entry is allowed. Page execution lists with the existing keyset cursor.

### Analytics: what worked, where and for whom?

Provide a shared version and date range at the top. Default reporting to the entry
cohort: “People/runs entering during this period”. Explain that later outcomes
continue to update that cohort. Live “waiting now” is explicitly a separate current
inventory, not a historical event count. An activity-by-occurrence-date view can
be added when the storage model supports it; don't relabel entry cohorts as
conversions that happened today.

Suggested hierarchy:

1. Entered, reached goal, completed without goal, in progress, blocked/failed and
   time-to-goal/terminal duration with their denominators.
2. Journey canvas in Analyze mode: node reach and branch shares. Tables remain an
   accessible alternative sorted by graph/path, including zero-activity nodes.
3. Channel and message-step breakdown with accepted/blocked/failed outcomes and
   reasons. Logical sends and retry attempts remain distinct.
4. Outcomes and customer drilldowns. Every count opens the records behind that
   exact scope and preserves journey/version/period/node/channel/reason filters.

The primary default is one logical enrollment as the conversion denominator.
Unique people are separate: repeated entries are not additional unique people.
Branch share divides by the executions evaluated at that branch, not all entries.
“Not reached” is not automatically drop-off: a person may be waiting, take an
alternative branch, exit globally or become ineligible. Young cohorts that have
not had the configured journey duration to finish need an “in progress” cue.

### Required breakdowns

| Breakdown                    | Business question                       | MVP data decision                                                                                                            |
| ---------------------------- | --------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| Step / path / branch         | Where do people go or stop?             | Version-scoped node/edge projections; distinct reach vs current occupancy.                                                   |
| Channel and message step     | Which action is failing or blocked?     | Delivery outcomes by source node; status/reason dimensions, not only channel totals.                                         |
| Decision/exit reason         | Why no message or no conversion?        | Separate consent, frequency cap, missing contact, provider failure, timeout, cancellation and actual goal.                   |
| Published version            | Did the new definition improve results? | Explicit filter on summary, graph and lists; no mixing historical node IDs. Compare cohorts with comparable maturity.        |
| Entry period                 | How do cohorts perform over time?       | Declared time basis and denominator on every section.                                                                        |
| Customer segment/locale/plan | Which audience differs?                 | Later allowlisted dimensions with clearly defined snapshot timing; never arbitrary JSON grouping on every dashboard request. |

Channel conversion attribution is a separate feature contract. Without a defined
model/window and appropriate delivery/engagement evidence, report journey goal
conversion and channel sending outcomes separately. Temporal order is not causal
lift. Provider accepted must continue to be labeled as accepted, not delivered.

## Data/query approach

Retain PostgreSQL and the existing asynchronous projector. Five-second polling is
sufficient for this MVP; a WebSocket or new service is not required to repair the
current experience.

- Fix metric outcome semantics and scope first. Add a version predicate to the
  execution-list API and use the same scope throughout the view.
- Append bounded node/edge/delivery-reason facts/deltas during existing atomic
  transitions. Retries must not add new entries or logical sends. Keep node/edge
  totals version-specific so reused node IDs cannot merge unrelated definitions.
- Date-filtered path analytics needs node/edge time/cohort projections: current
  `metric_hourly` only stores execution and channel dimensions. Current channel
  buckets use logical job creation date, whereas execution outcomes use entry
  date. A unified entry-cohort view must explicitly model both timestamps/bases;
  changing the frontend selector alone cannot make these totals comparable.
- Current node occupancy is a gauge from execution position/state. A terminal
  node-visit count or the sum of old step rows cannot substitute for it. Distinguish
  processing, a configured wait and a paused journey when diagnosing backlog.
- Read aggregate summaries in bounded workspace/journey/version/time scopes. Fetch
  one selected node/person's detail on demand; use keyset pagination for its lists.
  Do not continuously download all customers or execution histories for a canvas.
- Persist actual last-success/projector health separately from the last source
  event timestamp. No new events is not automatically stale or unhealthy. Display
  counter freshness separately from the selected execution's authoritative state.
- New branch projections need a coverage start/backfill policy. Do not imply that
  absent historical branch counters are zero. Rebuild historical data with an
  explicit bounded backfill, not a synchronous reporting request.
- Add indexes for implemented filter paths after EXPLAIN against realistic data;
  avoid speculative indexes or a new durable-state layer. Restrict segment
  breakdowns to bounded approved keys, with entry-time versus current-profile
  semantics documented. Do not claim median/p95 from sum/count averages alone.

## Proposed implementation order

1. **Correctness:** separate goal/cancel outcomes; version-safe lists and selected
   state; shared scope contract and explicit freshness. Keep sandbox/live separate.
2. **Actionable live view:** readable canvas, node drawer, current waiting/overdue
   inventory, customer timeline and paged contributing records.
3. **Breakdown data:** atomic edge and node-delivery/reason projections, coverage
   semantics, date/cohort queries and counter reconciliation tests.
4. **Journey analytics:** shared filters, graph/path analysis, exact count drilldowns,
   relevant channel detail and version comparisons. Audience dimensions follow.

Acceptance examples: a Yes/No split reconciles to evaluated executions; a blocked
SMS count opens precisely those messages and reasons; selecting v2 excludes v1
executions; a retry changes attempts without increasing logical sends; a refund
stops messages without adding a conversion; a paused journey doesn't report all
its due waits as worker failures; totals and drilldowns agree for the declared
cohort once projection has caught up. Existing execution, race, isolation and
published-version invariants remain required.

## Reference research

Customer.io exposes overall journey metrics, recent individual journeys and a
workflow Measure view with node metrics and branch-path totals. This supports
keeping outcome, path and person inspection in the same journey workspace.
[Automation and broadcast metrics](https://docs.customer.io/messaging/metrics/automation-metrics/).

Klaviyo exposes analytics on flow message cards and drills into recipient activity,
including waiting/skipped groups and explicit skip reasons. This is the relevant
pattern for a blocked count that opens actionable people/message records.
[Understanding flow analytics](https://help.klaviyo.com/hc/en-us/articles/115002779351).

Braze documents distinct Canvas-level and message-step conversion attribution and
an explicit conversion window; its definitions illustrate why step/channel totals
cannot simply be summed into journey conversion.
[Conversion events](https://www.braze.com/docs/user_guide/messaging/messaging_fundamentals/conversion_events).

The layout and implementation order above are recommendations for this repository,
not claims that all reference products implement the same runtime or metric model.
