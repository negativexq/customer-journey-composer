# Implementation tracker

Source of truth for product scope: [PRD.md](PRD.md).

| Phase                                  | Status                                    | Evidence / remaining work                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| -------------------------------------- | ----------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1. Repository & Foundation             | Complete                                  | Vue/Go/Docker stack; PostgreSQL migrations; quality checks; integration and browser tests passed locally.                                                                                                                                                                                                                                                                                                                                                                                            |
| 2. Authentication & Journey Management | Complete                                  | Sessions, CSRF, RBAC, workspace isolation, revision-guarded drafts and management UI; database and 6 browser tests passed.                                                                                                                                                                                                                                                                                                                                                                           |
| 3. Visual Canvas & Validation          | Complete                                  | Typed schema, graph/rule validator, immutable snapshots and Vue Flow editor; database and 8 browser tests passed.                                                                                                                                                                                                                                                                                                                                                                                    |
| 4. Customer & Event Infrastructure     | Complete                                  | Profiles, scoped API keys, idempotent single/batch ingestion and atomic enrollment; database and 10 browser tests passed.                                                                                                                                                                                                                                                                                                                                                                            |
| 5. Persistent Runtime                  | Complete                                  | Fenced worker, durable timers/waiters, global exit, shared pure decisions, outbox; race/recovery/version tests and actual Docker restart passed.                                                                                                                                                                                                                                                                                                                                                     |
| 6. Email & SMS                         | Complete                                  | Safe rendering, channel studio, SMTP/Mailpit and mock SMS, durable deliveries/attempts, policy and retry tests; 14 browser tests passed.                                                                                                                                                                                                                                                                                                                                                             |
| 7. AI Journey Composer                 | Implemented; deterministic fixes verified | Shared schema, Claude/OpenAI adapters, bounded repair and diff/approval; deterministic contracts pass. Audit fixes: required-case publication gate, composer.v2 exits, reviewed behavior contract and per-attempt provider usage. Reviewed correction passes 10/10; original 4/10 is blocked. Authorized fresh Luna follow-up passes 10/10 after one automatic repair; actual total usage is 13,751 tokens. Broader live evaluation remains deferred; see validation/luna-live-2026-11-12/README.md. |
| 8. Preview & Test Lab                  | Complete                                  | Virtual clock, snapshots, replay, event/attribute sequences, assertions and fault injection; 20 browser tests passed.                                                                                                                                                                                                                                                                                                                                                                                |
| 9. Test Inbox                          | Complete                                  | Three-panel inbox, scoped filters, payload/trace/attempts, replay comparison and actual test-send distinction; 20 browser tests.                                                                                                                                                                                                                                                                                                                                                                     |
| 10. Live Monitor & Analytics           | Complete                                  | Transactional delta projections, indexed summaries, version-aware monitor and inspector; 22 browser tests passed.                                                                                                                                                                                                                                                                                                                                                                                    |
| 11. Lifecycle & Reliability            | Complete                                  | Audited lifecycle/publish gates, recovery, roles, bounded requests, OTel propagation and dependency/image checks; 24 browser tests.                                                                                                                                                                                                                                                                                                                                                                  |
| 12. Final Product & Demo               | In progress                               | Templates, populated opt-in demo, product README, ten 3200×2100 captures, measured 10,000-execution workloads and 46 browser tests; final recorded demo/deployment validation remain; deployment explicitly deferred by user.                                                                                                                                                                                                                                                                        |

## Accepted decisions

- Develop in the phase order above; make small, verified Conventional Commits.
- PostgreSQL is authoritative; no Redis dependency for durable state.
- AI output is a proposal, never an automatic save or publish.
- Both Anthropic Claude and OpenAI APIs must be supported behind the same provider boundary.
- Live LLM/provider evaluation is deferred except for the explicitly authorized Luna smoke on 2026-11-11 and follow-up on 2026-11-12. Deterministic unit/integration tests must not call paid providers. The smoke result and remaining gaps are recorded in [LUNA-SMOKE.md](LUNA-SMOKE.md).
- Report test execution and benchmark measurements honestly; pending work is not complete.

## Next slice

Performance experiments are stopped at the user's request. The rejected experiment
branches were deleted locally and remotely; their changes were not merged into
main. Revisit performance work only for an observed usage problem or an agreed
latency target, rather than continuing synthetic tuning.

Next product work remains the staged [design direction](design/modern-saas-direction.md):
Content/Audience surfaces, AI brief/plan review, analytics drill-down and the
recorded walkthrough. No new feature or optimization was introduced by the
acceptance verification below. Deployment and broader paid LLM testing remain deferred; the user-authorized audit corrections and local evidence are recorded in [AI-REMEDIATION.md](AI-REMEDIATION.md). No further paid model generation was performed.

## Interface review — 2026-11-14

A walkthrough of the running local build at 1440px and 375px found presentation gaps rather than missing modules. Delivered in one web slice:

- Versions tab: published versions are listed with publish time and which one receives new entries; a version expands to the readable journey diagram, with the JSON definition behind an explicit toggle. Draft-versus-latest and version-to-version comparisons list added, removed and reconfigured steps plus routing, global-exit and re-entry changes. Canvas position changes are not reported as changes.
- Sunset, Complete and Archive (editor menu and journey list) now require confirmation in a modal that focuses Cancel; Escape and the backdrop cancel. Pause/Resume stay one click because they are reversible. The overflow menu closes on outside press and Escape.
- Errors show the server message without the `Error:` prefix; timeouts and unreachable-server failures have their own copy. List timestamps use one `Nov 13, 2026, 01:18` format instead of the browser locale default. Singular counts are no longer pluralized. The search shortcut hint matches the platform and no longer wraps.

Verified locally: `make check` (including the new `scripts/presentation.test.mjs`), Docker web rebuild and `npx playwright test --workers=2` with 80/80 desktop/mobile tests, including the new `versions.spec.ts` and the confirmation steps in `lifecycle.spec.ts`. Not covered by a browser test: the journey-list Archive confirmation. No integration tests, benchmark, paid LLM call or hosted CI run was performed for this slice. Remaining staged design work (Content/Audience modules, analytics drill-down, collapsible sidebar, dark theme) is unchanged.

## Analytics drill-down — 2026-11-14

Most count-to-record navigation already existed in the journey Monitor/Analytics view (summary cards, outcomes, branch shares and channel counts). This slice closes the remaining gaps with no backend change; the existing `node_id`/`current` execution filters are reused.

- Step reach table: non-zero Reached and Waiting now counts are links that list the executions that reached, or are currently waiting at, that step.
- Any drill-down that changes the execution list scrolls the list into view when it is off screen. Its header states the active breakdown (reached/currently at a step, branch, outcome, past-due waits) and offers one Clear breakdown action.
- A row in the workspace Journey comparison table opens that journey's Analytics tab instead of its Builder.

Verified locally: `make check`, Docker web rebuild and `npx playwright test --workers=2` (80/80 desktop/mobile), with new assertions in `journey-insights.spec.ts` and `analytics.spec.ts`. Integration tests were not rerun because no API or SQL changed. Not built: drill-down from the workspace-level Node funnel bars, saved breakdown URLs and exporting the matching customers.

## Interface polish and customer route — 2026-11-14

Frontend-only slices; no API, SQL or runtime behavior changed.

- Color tokens and dark theme: every interface color resolves through `apps/web/src/tokens.css`, where each token carries a light and a dark value via `light-dark()`. The theme follows the system; a topbar control pins light or dark per browser (`lib/theme.ts`) and an inline script applies a pinned choice before first paint. The canvas now follows the theme; the earlier "canvas stays light in dark OS mode" rule is retired and `canvas-controls.spec.ts` asserts the new behavior. `scripts/tokens.test.mjs` fails on opaque literal colors in component styles and on undefined tokens.
- Loading, feedback and motion: `SkeletonBlock` replaces plain loading sentences; `ToastHost` confirms completed actions without announcing twice what a status region already reports; `motion.css` adds short transitions that are disabled for reduced motion; `EmptyState` replaces six hand-built empty states.
- Customers: a searchable people list beside a profile with Journey, Activity and Profile tabs. Journey draws the person's route on the published journey map and lists every stop with its outcome, branch, time spent and message (`lib/customer-journey.ts`, `CustomerJourney.vue`, `JourneyRoute.vue`). Activity merges events, messages and journey entries. Profile is read-only until Edit profile is chosen. The execution drawer uses the same route with the journey name, version and outcome.
- Canvas: a keyboard shortcut reference (toolbar button or `?`), and connection width on the metrics canvas follows each edge's share of traffic.

Verified locally: `make fmt-check`, `make lint`, `make typecheck`, `make test` (Node tests now 20, including route/activity and token guards), Docker web rebuild and `npx playwright test --workers=2` (82/82 desktop/mobile), with new or updated assertions in `theme.spec.ts`, `canvas-controls.spec.ts`, `activity-navigation.spec.ts` and `events.spec.ts`. Light, dark and 375px layouts of the main screens were checked in a browser, and a scripted contrast pass over 14 routes in the dark theme found one text below 3:1, which was fixed. Integration tests were not rerun because no API or SQL changed.

Not covered: the dark values are derived from the light palette by a lightness mapping and were reviewed on the main screens, not on every state of every screen; dialogs, the AI composer, Message Studio and publish review were not inspected in dark. Toasts have one browser assertion (customer creation). There is no undo for archive because the API has no unarchive transition. The customer list endpoint does not return consents, so consent is shown on the profile only.

## Settings sections and branch layout — 2026-11-14

Frontend-only; no API, SQL or runtime behavior changed.

- Settings: the single long page is now seven sections behind a section menu (`/settings?section=…`): Messaging, Workspace data, Team, API keys, AI providers, Audit log and System. Team lists members with a role selector and opens the new-member form on demand; revoking an API key asks for confirmation; the audit log shows a readable action, the person and a formatted time, with raw details behind a disclosure; System shows failed jobs, undelivered messages, analytics freshness, database connections and API memory as figures, with the raw payload behind "Technical details". The data catalog gains a kind filter and shows its definition form only while adding or editing.
- Canvas layout: Auto layout, generated journeys and the metrics canvas share `lib/flow-layout.ts`, which keeps branch targets under their own handle (Matched/Yes left, Timeout/No right) so the two connections no longer cross. Positions already saved in a draft are not rewritten; Auto layout applies the new arrangement.

Verified locally: `make fmt-check`, `make lint`, `make typecheck`, `make test` (Node tests now 22, including `flow-layout.test.mjs`), Docker web rebuild and `npx playwright test --workers=2` (84/84 desktop/mobile) with the new `settings.spec.ts`. Settings was checked in a browser in light, dark and at 375px. Integration tests were not rerun because no API or SQL changed.

Not covered: the layout swap exchanges two branch columns and does not re-pack columns of different widths, so very uneven branches can leave extra space; it was verified on the demo recovery journey and in unit tests, not on every template.

## Builder canvas refresh — 2026-11-15

Frontend-only; the workflow document, API and runtime are unchanged.

- Chrome: the toolbar row above the canvas is gone. One floating bar holds Steps, AI composer, undo/redo, zoom, fit and step settings; Auto layout, clipboard actions, navigation, Short exit links, lock and the shortcut list sit behind its More menu. The separate AI composer button on the tab row was folded into the bar. A small chip shows the step count. The step library starts closed.
- Step cards: a first line says when a customer arrives ("Starts when", "If purchase_completed arrives", "Otherwise, after 1 day"), from `lib/step-captions.ts`; the detail moves to a tinted footer; the in-card branch row is removed because the branch is now stated once on the arriving step. The same captions show on the Live Monitor and customer route maps.
- Adding steps: every unconnected output has an add button that opens a step picker and places and connects the new step under its handle; an empty canvas offers "Add a trigger". Clicking empty canvas clears the selection.
- Step settings open with one sentence on what the step does.
- Phones: the bar docks to the screen above the panels and drops zoom buttons in favor of pinch; with the AI panel open it stays inside the canvas so it does not cover the panel's actions.

Verified locally: `make fmt-check`, `make lint`, `make typecheck`, `make test` (Node tests now 24, including `step-captions.test.mjs`), Docker web rebuild and `npx playwright test --workers=2` (86/86 desktop/mobile) with the new `canvas-add.spec.ts` and updated `canvas-controls`, `canvas-readability` and `templates` specs. The builder was checked in a browser in light and dark at desktop width.

Not covered: the phone layout was verified through the mobile test project, not by eye beyond one screenshot taken before the bar was docked. A step reached by several paths shows no caption. New steps are placed at a fixed offset under their handle and can overlap an existing step in a dense graph until Auto layout is used.

## Step settings and A/B split — 2026-11-15

- Step settings (frontend only): the panel opens with the step's icon and purpose, lists publishing issues first, groups fields under a named section and shows where the step comes from and where each output leads, with an add action for unconnected outputs.
- A/B split: a new `split` step with a `percent` share for variant A and outputs `a` and `b`. Assignment is `workflow.SplitPort`, a pure hash of customer ID and step ID, so `runtime.Decide` stays free of random choice, the simulator and the worker agree, and a customer keeps the same variant on every pass. Migration `0024_split_variants.sql` counts edge traffic for any reported output instead of a fixed port list. `GET /api/v1/journeys/{id}/experiments?version=N` returns, per split and variant, customers entered, converted and still in progress. The builder has the step in the picker and library with a share slider; arriving steps are captioned "Variant A · 50% of customers"; journey Analytics shows an Experiments panel whose counts open the matching executions.

Verified locally: `make fmt-check`, `make lint`, `make typecheck`, `make test` (including new decision and validation cases for the split), `make test-integration` (all packages pass on an isolated PostgreSQL), rebuilt `api`, `worker` and `web` containers, and `npx playwright test --workers=2` (88/88 desktop/mobile) with the new `experiments.spec.ts`, which publishes a split journey, enrolls 24 customers through the real worker and checks both the endpoint and the Analytics panel.

Not covered: the AI composer's structured-output schema does not include `split`, so AI proposals never contain one and an AI modification of a journey that has one is untested. The Experiments panel reports a raw difference and says so; there is no significance test, and it withholds a leader under 30 customers per variant. Splits have exactly two variants. There is no dedicated Go integration test for the split beyond the unit tests and the browser test against the running stack.

## Publish changes, version restore and wait types — 2026-11-15

- Publish review (frontend only) opens with what the publish changes compared with the latest version, or states that it is the first version or that nothing behaves differently. A published version can be restored to the draft from Versions after a confirmation. Confirm dialogs now use unique label ids.
- Wait types: a wait is still a fixed delay by default. `wait_mode: "time"` waits for the next `at_time` (HH:MM in the step's timezone), optionally only on chosen weekdays; `wait_mode: "date"` waits until `date_path` on the customer or event plus `offset_seconds`. `workflow.WaitDeadline` is pure and shared by the worker and the simulator; a missing, unreadable or past date continues immediately and every wait is capped at 365 days. The settings panel offers the three types and clears the fields of the type left behind.

Verified locally: `make fmt-check`, `make lint`, `make typecheck`, `make test` (new `waits_test.go` deadline and validation cases; Node tests now 25), `make test-integration` (exit 0, 15 packages), rebuilt `api`, `worker` and `web`, and `npx playwright test --workers=2` (90/90 desktop/mobile) with the new `wait-types.spec.ts` and extended `versions.spec.ts`. The wait test checks the saved configuration and that the sandbox clock stops exactly three days before the customer's date.

Not covered: time-of-day and date waits were exercised through unit tests and the sandbox, not by a live worker waiting on a real timer. Daylight-saving transitions rely on Go's local-time normalization and have no dedicated case. The AI composer schema does not include the new wait fields.

## Multi-way branch — 2026-11-15

A new `switch` step holds 2 to 5 named branches, each with a condition. Branches are checked in order and the first match wins; everyone else leaves through `else`. Outputs are `case_1`…`case_n` and `else`, so outputs now depend on the step: `workflow.NodePorts` on the server and `ports(kind, config)` in the web app. The builder shows one handle per branch, captions the arriving steps ("If Gold", "Otherwise"), and the settings panel names, adds and removes branches, renumbering the connections when one is removed.

Verified locally: `make fmt-check`, `make lint`, `make typecheck`, `make test` (new decision and validation cases), `make test-integration` (exit 0, 15 packages), rebuilt `api`, `worker` and `web`, and `npx playwright test --workers=2` (92/92 desktop/mobile) with the new `multi-branch.spec.ts`, which checks four sandbox outcomes, the canvas, and that removing a branch keeps the others connected.

Not covered: the step was exercised in the sandbox and the builder, not through a published journey on the live worker (the worker shares the same decision function). Auto layout orders multi-way targets by branch through the layout seed only; the column swap that guarantees order applies to two-way steps. Branch counts and conversion per branch are not broken out in Analytics beyond the existing edge traffic. The AI composer schema does not include the step.

## Customer paths diagram — 2026-11-15

Frontend only. Journey Analytics gains a left-to-right flow diagram for the selected version: a bar per step sized by customers who reached it and a band per connection sized by customers who took it, from the existing projected `node`/`edge` metrics (`lib/path-flow.ts`, `PathFlow.vue`). One hue carries magnitude, labels use text colors, each bar and band has a tooltip and opens the matching executions, and the Step reach table below holds the same numbers as text.

Verified locally: `make fmt-check`, `make lint`, `make typecheck`, `make test` (Node tests now 28 with `path-flow.test.mjs`), Docker web rebuild and `npx playwright test --workers=2` (92/92 desktop/mobile); `experiments.spec.ts` asserts the bars, bands and totals after waiting for the projection. Checked by eye on the demo journey in the dark theme at desktop width.

Not covered: customers who leave at a step through the journey's global exit simply shrink the next bar; there is no separate "left here" band. Light theme and phone width were not checked by eye. Column order uses a simple weighted ordering, not full crossing minimization, so a dense graph can still cross bands.

## Webhook step — 2026-11-15

A new `webhook` step posts chosen fields to another system as JSON: `url` (http or https, no credentials) and `fields`, up to 20 names mapped to `customer.`, `event.` or `context.` paths; a missing value is sent as null. `runtime.Decide` builds the call, and the worker writes it to `webhook_calls` (migration `0025_webhook_calls.sql`) in the same transaction as the step, unique per execution and step, so a retried step never calls twice. A `WebhookDispatcher` in the worker claims pending calls with `FOR UPDATE SKIP LOCKED`, sends them with a 5 s timeout, no redirects and an `Idempotency-Key` header equal to the call id, and retries up to three times before `FAILED`. Only hosts listed in `WEBHOOK_ALLOWED_HOSTS` are contacted; the default is empty, so the demo records calls as `RECORDED` and never sends data out. The sandbox only traces "Would call …". Execution detail returns the calls. The builder offers the step under Customer & outcomes with an address field and a field editor; the card reads "POST to host · n fields" and the customer route states whether the call was sent, failed, queued or only recorded.

Verified locally: `make fmt-check`, `make lint`, `make typecheck`, `make test` (new decision and validation cases; Node tests now 29), `make test-integration` (exit 0, 15 packages) including `webhooks_integration_test.go`, which uses a local receiver to check retries to `FAILED`, an unlisted host left `RECORDED`, the same idempotency key across attempts, null for a missing field, a single `SENT` on 204 and uniqueness per execution and step. Rebuilt `api`, `worker` and `web`, and `npx playwright test --workers=2` (94/94 desktop/mobile) with the new `webhook.spec.ts`, which edits the step on the canvas, catches an invalid address, checks the sandbox trace, publishes, enrolls a customer through the live worker and reads `RECORDED` from the execution and the customer route.

Not covered: no browser or live-worker test sends to an allowed host; sending is covered only by the Go integration test. Calls are not signed (no HMAC secret), headers and method are fixed, and the response body is not used by later steps. Field order is not preserved because PostgreSQL `jsonb` reorders keys. The AI composer schema does not include the step.

## Scheduled audience entry — 2026-11-15

The trigger can now enroll an audience instead of waiting for an event: `entry_mode: "audience"` with the audience rule in the trigger's `filters` (customer fields only), `at_time`, optional `weekdays` and `timezone`. Such a trigger uses the reserved internal event `audience_entry`; an event-based trigger may not use that name, and customer events with it never start an audience journey. Because the rule lives in the entry filter, the sandbox and the publish gate treat it like any entry check. An `AudienceScheduler` in the worker runs each active audience version at the latest due slot after it was published (a stopped worker runs a missed slot once, not once per slot); `audience_runs` (migration `0026_audience_runs.sql`) is unique per version and slot, so only one worker claims a slot. Each matching customer is handled in its own transaction that locks the customer row, records an `audience_entry` event whose ID is derived from the run and customer, and enrolls through the same `enroll` function as an incoming event, so re-entry rules and the one-active-execution index apply. `GET /api/v1/journeys/{id}/audience` returns the next run and the latest runs; `POST /api/v1/journeys/{id}/audience/runs` (Admin) enrolls the audience now and is audited. The trigger settings offer the entry type, the audience rule, time, days and timezone; the card reads "Audience at 09:00 on Mon"; Live Monitor shows an Audience entry panel with the next run, recent runs and "Enter matching customers now"; the customer route says the person entered with the scheduled audience.

Verified locally: `make fmt-check`, `make lint`, `make typecheck`, `make test` (new schedule and validation cases; Node tests now 30), `make test-integration` (exit 0, 15 packages) including `audience_integration_test.go`, which checks that a sent `audience_entry` event enrolls no one, nothing runs before the first slot, only matching customers enter, a slot runs once however often the scheduler ticks, active customers are not enrolled twice, finished customers re-enter on a later run, and missed slots collapse into one run. Rebuilt `api`, `worker` and `web`; `npx playwright test --workers=2` ran 95 of 96 passing with the new `audience-entry.spec.ts`; the one failure, `management.spec.ts` on mobile, was a 30 s timeout unrelated to this change and passed 2/2 when rerun alone. Settings panel and Live Monitor panel checked by eye at desktop width in the light theme.

Not covered: audiences are evaluated against stored profile fields only, not event history ("bought twice in 30 days"); there are no saved, reusable segments; a run reads at most 50,000 customers and runs synchronously when started by hand; the scheduled path was exercised with a test clock, not by waiting for a real slot. The AI composer schema does not include audience entry.

## Live Monitor sections — 2026-11-15

Frontend only. Live Monitor is split into three sections under the KPI row, like Settings: Journey map, Executions and Messages. The map is the default; a link to an execution or message opens Executions. Counts that filter people (a KPI, a step count, an overdue warning) switch to Executions instead of scrolling; choosing a step from the Step filter switches to the map, and the map now opens centred on an already selected step. Messages shows the journey's message list full width without the step chrome. A selected customer stays selected across sections. Journey Analytics keeps its single page. The Executions list is taller in its own section; on phones the section labels drop their icons to fit.

Verified locally: `make fmt-check`, `make lint`, `make typecheck`, `make test`, Docker web rebuild and `npx playwright test --workers=2`: 94 of 96 passed; the two failures were `builder.spec.ts` (desktop and mobile), which tracked a customer without opening Executions. The test was updated and then passed 2/2. `journey-insights.spec.ts` now also checks the default section and the Messages section; `monitor-context.spec.ts` and `builder.spec.ts` open Executions and return to the map. Checked by eye in the dark theme at 1440 px and 375 px.

Not covered: the chosen section is not in the URL, so a reload returns to the map unless an execution or message is linked. The light theme was not checked by eye.

## Compact journey header — 2026-11-15

Frontend only. The journey header drops from four rows to two: an icon back button ("Back to journeys"), the name and status share a row with the save note and actions, and the draft/published context, latest version and a one-line description sit on the row below. The placeholder description is no longer shown when a journey has none. Under 1050 px the actions wrap under the title, starting from the left; under 900 px the description gets its own line.

Verified locally: `make fmt-check`, `make lint`, `make typecheck`, `make test`, Docker web rebuild and `npx playwright test --workers=2`: 95 of 96 passed. The failure was `audience-entry.spec.ts` on desktop reading the customer route before the worker had finished the execution; it passed 3/3 alone, and the test now waits for the execution to complete, then passed 2/2. Checked by eye in the dark theme at 1440, 1024 and 375 px on Builder and Live Monitor.

Not covered: the light theme was not checked by eye.

## Test Lab and Test Inbox review — 2026-11-15

- Test Lab's Recent test runs named each run by the first characters of its ID. `GET /api/v1/test-runs` now also returns the journey name, scenario name and published version, and the list reads "Email and SMS · no purchase" over "Abandoned Cart Recovery · Draft rev 6 · date" (`lib/test-runs.ts`). The Test Inbox run filter uses the same names.
- Test Inbox message rows were centred by an inherited `justify-content` and had no hierarchy. Each row now shows a channel icon, the message step as its title, channel and journey, recipient and time, and a status badge. The message header no longer repeats the status as plain text. On phones the message list comes first, then the preview, then the filters.

Verified locally: `make fmt-check`, `make lint`, `make typecheck`, `make test` (Node tests now 31 with `test-runs.test.mjs`), rebuilt `api` and `web`, and `npx playwright test --workers=2`: 96/96 desktop/mobile after updating one `recovery.spec.ts` assertion from the old upper-case "EMAIL" row text to "Email ·". Test Lab and Test Inbox checked by eye in the dark theme at 1440 px and 375 px.

Not covered: Test Lab's scenario form and timeline were looked at but not changed; the light theme was not checked by eye.

## Message engagement: tracking, unsubscribe and provider reports — 2026-11-15

Adapted from the sister project Smart Campaign Composer, whose short links, signed unsubscribe and provider reports had no counterpart here.

- **Reports after acceptance.** New `delivery_events` table keeps the first `delivered`, `bounced`, `opened`, `clicked` and `unsubscribed` report of each delivery (primary key per delivery and kind). A delivery's status still comes only from our own provider calls.
- **Tracked links.** When a delivery is created, each http(s) link in its email or SMS is rewritten to `PUBLIC_BASE_URL/api/t/<12-character code>`, stored in `tracking_codes` with the delivery in the same statement. A retry resends the same codes. `GET` counts and redirects (302, `no-store`) only to the stored URL, so it is not an open redirect. `HEAD` redirects without counting. SMS segments are recounted after shortening.
- **Opens and unsubscribe.** Tracked emails get a 1×1 open pixel, an unsubscribe footer link, and RFC 8058 `List-Unsubscribe`/`List-Unsubscribe-Post` headers on SMTP. Opening the unsubscribe link only shows a confirmation page (CSP `default-src 'none'`). The `POST` withdraws email consent, and dispatch already re-reads consent. A test send changes no customer. The Test Inbox preview blocks remote images, so viewing a message does not count as an open.
- **Provider reports.** `POST /api/provider/events` accepts `delivered`/`bounced` reports signed with `PROVIDER_WEBHOOK_SECRET` (`X-Provider-Signature: sha256=<hex HMAC>`), naming the message by the idempotency key we sent. It is off without a secret. The in-process mock providers record a `simulated` report on acceptance. Recipients `bounce@…`, `…+bounce@…` and phones ending `0000` bounce. SMTP/Mailpit sends no delivery report.
- **Journeys react.** Each report of a live journey message becomes a customer event (`message_delivered`, `message_bounced`, `message_opened`, `message_clicked`, `message_unsubscribed`) with a derived event ID, ingested once (`ingested_at`).
  - A `wait_event` step can set `message_node`. It then matches only engagement with that earlier email/SMS step in the same run.
  - Validation requires the message step to come before the wait and the event to exist for that channel (an SMS has no open or unsubscribe).
  - The sandbox matches by step.
  - The events are built into the data catalog after the workspace's own definitions.
- **Analytics.** Engagement joins the delivery metric snapshot, so insights count opened/clicked/bounced/unsubscribed per message step and version once per delivery.
- **UI.**
  - Builder: "Wait for event" offers "Listen for: A customer event / Message engagement", with a message picker (earlier steps only) and an engagement picker filtered by channel. Captions read "If they click a link in Reminder".
  - Metrics canvas and Live Monitor show Opened/Clicked (Bounced when non-zero).
  - Delivery details show an "After sending" list.

Verified locally:

- `make fmt-check`, `make lint`, `make typecheck`, `make test` (Go unit and Node 31/31) and `git diff --check`.
- `./scripts/test-integration.sh`: all packages, including new tests. These cover tracked click → waiting step → `converted`, a repeated click reaching the journey once, another message's click not matching, unsubscribe withdrawing only email consent, insights counting one click, and the HTTP endpoints (HEAD not counted, pixel, unsubscribe GET/POST, unknown codes 404, unsigned/wrong/unknown reports rejected, duplicate signed reports kept once).
- Playwright 98/98 desktop/mobile, including the new `engagement.spec.ts`. That test publishes a journey and sends the real email through Mailpit, then follows the tracked link from Mailpit's copy and sees the run exit as converted.
- Checked by eye: the Builder settings and caption (light, 1440 px) and the unsubscribe page (375 px).
- `activity-navigation.spec.ts` now picks the entry event row, because the mock SMS delivery report is an event of that customer too.

Not covered:

- The AI composer schema does not yet offer `message_node`; AI drafts can use the events but not scope them to one message.
- Email open counts include clients that load images on their own.
- There is no SMS opt-out keyword, no per-link click breakdown and no Mailpit delivery webhook.
- No real provider was called.

## Provider resilience, discount codes and the notification bell — 2026-11-15

Adapted from Smart Campaign Composer's delivery worker, `docs/errors-and-retries.md` and `DECISIONS.md`. Its shared state lives in PostgreSQL here, not Redis. Full rules: [DELIVERY-RESILIENCE.md](DELIVERY-RESILIENCE.md).

- **Channel gate.** New `provider_channels` row per workspace and provider. Claims skip held channels and re-check the gate under the row lock (`ErrGated`).
  - A 429 pauses the channel for `Retry-After` (1–30 s).
  - A 401 or SMTP 530/534/535 locks it until one probe per 30 s is accepted. Only the current lock's probe opens it.
  - A breaker opens at ≥5 calls and ≥50% failures in 30 s, then probes 3 times with a doubling wait up to 30 s.
  - A per-second rate window counts in-flight calls, answers for 1.1 s and unanswered calls for 13.1 s.
- **Attempts.** 429s, 401s and failed probes cost no attempt (`free_attempts`, bounded at 50). Backoff gets up to 50% jitter. Attempts record `provider`, `gate` and `signal`.
- **Alerts.** A breaker trip opens one critical notification. It closes only after a minute with most calls going through, so a flapping provider does not flap the bell. Credential locks notify on lock and unlock. New `journey_channel_events_total` metric.
- **Notification bell.** New `notifications` table, per-user `notifications_seen_at`, and an `audit_logs` trigger for journey publish, lifecycle changes and audience runs. The topbar bell shows an unread count; items open their journey or the channel settings.
- **Channel card.** Settings → Messaging → Channels shows live state, rate, 10-minute calls and the last problem. Admins edit the rate and can simulate a 429, outage or refused key for 1–30 minutes.
- **Discount codes.** Email/SMS steps take `discount_code` (3–30 capitals, digits, hyphens), filled in at `{{ offer.code }}`. Emails show it in a code box unless the body writes it. Validation refuses an SMS that hides its code and a message showing an unset one.
- **Longest-case SMS length.** The preview measures each SMS with the costliest attribute value among SMS-consenting customers (or the fallback) and tracked links, and warns when that needs more segments.

Verified locally:

- `make fmt-check`, `make lint`, `make typecheck`, `make test` and `git diff --check`.
- `./scripts/test-integration.sh`: all packages, including the new channel, notification and preview tests.
- `npx playwright test`: all desktop and mobile tests, including the new `channel-resilience.spec.ts`.
- Checked by eye at 1440 px (light): the channel card in a held state, the open bell, the discount field and the SMS longest-case note.

Not covered:

- An unreachable provider still uses an attempt.
- Journeys do not pause after a long outage.
- The rate window was not load-measured.
- The AI composer did not yet set `discount_code` (added in the next entry).
- No real provider was called.

## Live engagement and discount-code generation — 2026-11-15

- `composer.v6` lets the AI set a wait step's `message_node` and a message's `discount_code`. The prompt says to copy only a code the brief names and to show it in SMS as `{{ offer.code }}`.
- Test Lab assigns an open or click that names no step to the latest accepted message, so contract cases can describe "clicks the email".
- `scripts/live-engagement-scenarios.mjs` runs ten briefs against the local stack in fresh workspaces. For each, it checks the result in Test Lab and the test inbox, and reads the database back. It never publishes or sends.

Verified locally:

- `make fmt-check`, `make lint`, `make typecheck`, `make test`, `git diff --check`.
- An authorized live run with OpenAI `gpt-6-luna` passed 10/10 journeys and 47/47 cases. It used 13 provider calls and 95,151 tokens. Every journey stayed a Draft with no versions, executions, customers or deliveries. [Report](reports/live-engagement-2026-11-15.md).

Not covered:

- In a coded email, the "Your code" text line and the code box label were English whatever the copy's language (fixed in the next entry).
- One run per scenario. Claude was not run live.

## Text around a message in the customer's language — 2026-11-15

The live run showed "Your code: SEPET15" under Turkish copy. This follows the sister project Smart Campaign Composer, where a person's stored language decides what surrounds a message.

- A new `internal/language` package lists Turkish, English, German, French and Spanish. Each has a discount-code label and the unsubscribe footer's words.
- `messaging.Render` reads the customer's existing `locale` (`tr-TR`, `de_AT` and similar are accepted) and records the language on the message. Any other locale gets English.
- The code box, the code's text line and the tracked email's unsubscribe footer use that language. Messages rendered before this change have no language and stay English.
- The message copy is still the author's, one version per step. Writing a separate version for each language, as the sister project does, is not built.
- Customers → Locale shows a `tr-TR` placeholder and lists the supported languages in its tooltip.

Verified locally:

- `make fmt-check`, `make lint`, `make typecheck`, `make test` and `git diff --check`.
- `./scripts/test-integration.sh`: all packages.
- On the rebuilt stack, `POST /messages/preview` labelled a coded email "İndirim kodun" for `tr-TR`, "Ihr Rabattcode" for `de` and "Your discount code" for `ru`.

## Generated messages compared with the brief — 2026-11-15

Taken from Smart Campaign Composer's "The rule and the messages are compared with the brief" and "What was asked and not done is shown".

- New `internal/factcheck` reads links (with or without a scheme, Turkish suffixes trimmed), discount codes, numbers, lengths of time and discount percentages, ignoring variables, markup and entities.
- The composer refuses generated text stating a link, code or number that the request and the person's existing messages do not contain. The model gets the reasons for its one repair; a second failure returns no proposal. Attempts count these issues as `validation.facts`.
- A typed-out code must become `discount_code` and `{{ offer.code }}`.
- `Proposal.not_done` lists the brief's links, codes and discounts that no message carries. The creation review and the AI panel show them, and the AI panel asks for acknowledgement.
- The prompt also says not to state how a customer was selected; the backend does not check this.
- The recovery acceptance test now uses its recorded brief. Its 2026-11-11 fixture is upgraded to set COMEBACK10 as the SMS step's code.

Verified locally:

- `make fmt-check`, `make lint`, `make typecheck`, `make test` and `git diff --check`.
- `go test ./internal/factcheck ./internal/ai`: paired refusals and passes, repair feedback, the not-done notes, and the ten reviewed live journeys passing unchanged.
- `npx playwright test --workers=2`: 102/102. `behavior-checks.spec.ts` shows the note on desktop and mobile. A first run failed 8 tests: `not_done` without `omitempty` became a required field, so saving an older proposal was refused. Making it optional fixed that.
- The review panel was checked by eye.

Not covered:

- The check's live cost was measured afterwards on the ten reviewed briefs: [live-factcheck-2026-11-15](reports/live-factcheck-2026-11-15.md). It refused nothing and caused no repair. How often it catches an invented fact live is not measured.

## Customer paths and section heading spacing — 2026-11-17

- Customer paths was hard to read. Where customers stop at a step without continuing, the chart now says "N left the journey here" under the bar. Before, 12 → 9 at "Wait for purchase" showed nothing for the 3. Bands leaving a branching step carry their branch name and count in the middle of the band ("Timeout · 9"). Labels have a background halo so they read over bands, the chart is 220px tall instead of 300, and the caption is plainer.
- Section headings had 20px above and nothing below, so tables and lists started right under the title (Step reach, Journey outcomes, Channel performance and other sections). `.surface-heading` now has 16px below; places that set their own padding keep it.
- "Step reach · workflow order" is a stacked heading, "Step reach", with the note under it. Channel cells read "Email" and "SMS" instead of all caps.
- `journey-insights.spec.ts` follows the new heading.

Verified locally:

- Journey analytics, the overview and workspace analytics were checked by eye at 1280×900.
- `make fmt-check`, `make lint`, `make typecheck`, `make test`, `git diff --check`, and the full `npx playwright test` (104 passed).

## Test Inbox rows and canvas keyboard focus — 2026-11-17

- In the Test Inbox list, the status badge wrapped into the icon column, so the icon column grew to the badge's width and pushed the text away from the icon. The badge now sits under the text. The channel name is bold and colour-coded: Email violet, SMS blue.
- Tabbing to a step below the visible canvas made the browser scroll Vue Flow's own box (415px at 1280×800). The view then fell out of line with the canvas and its controls. The flow box now clips its overflow, and focus on an off-screen step pans the view to it.

Verified locally:

- At 1280×800, tabbing from Send Email to Recovery complete panned the view, and no element had a scroll offset.
- The inbox rows were checked by eye in light and dark themes.
- `make fmt-check`, `make lint`, `make typecheck`, `make test`, `git diff --check`, and the full `npx playwright test` (104 passed).

## Screenshot refresh — 2026-11-17

The eighth group from the design review.

- The 13 product screenshots were recaptured with `npm run screenshots:product` on the local Docker stack. They now show the redesigned Test Inbox, the new journey header line and the plain labels.
- The builder shot dragged the canvas from a fixed point. Selecting a step had scrolled the canvas box, so the drag started on the tabs, selected header text and cut off the left branch. The script now resets that scroll, pans from empty canvas beside the selected step so both branches fit, and stops if the start point is not on the canvas.

Verified locally:

- Every screenshot was checked by eye. `docs/images/manifest.json` records the source commit.
- `make fmt-check`, `make lint`, `make typecheck`, `make test`, `git diff --check`.

## Plain labels and AI setup guidance — 2026-11-17

The seventh group from the design review.

- Internal terms in visible copy were replaced. "logical message/enrollment/sends" now read as messages and entries; "rev N" and "Draft rev N" read "edit N" and "Draft edit N"; "Projection healthy" reads "Counters up to date"; "goroutines" reads "background tasks"; "Held (429/401)" reads "Held by provider", and the status codes moved to a tooltip.
- Settings › AI providers showed "Not configured" with no next step. When live generation is off or no provider has a key, a "How to turn it on" box lists `AI_PROVIDER`, `ANTHROPIC_API_KEY`/`ANTHROPIC_MODEL` and `OPENAI_API_KEY`/`OPENAI_MODEL`. These are set in the API server's environment and the API is restarted.
- The AI panel opened with a yellow warning when AI is off. It now shows a neutral note: editing, testing and publishing still work, and Settings explains how to enable AI. The journey creation notice points to the same place.
- `scripts/test-runs.test.mjs` and `tests/e2e/channel-resilience.spec.ts` follow the new labels.

Verified locally:

- Settings › AI providers on the Docker stack with AI disabled shows the setup box (checked by eye, dark theme).
- `make fmt-check`, `make lint`, `make typecheck`, `make test`, `git diff --check`, and the full `npx playwright test` (104 passed).

## Test Lab list and disabled reasons — 2026-11-17

The sixth group from the design review.

- Recent test runs listed every run; the review saw 42 in one column. The list now shows the newest 8 and a "Show N more · 8 of 42" row that adds 8 at a time.
- Create test run was disabled with no reason. A line under the actions now says why: the role can only view, no journey or saved scenario is chosen, the journey could not be loaded, or the entry event does not start the journey.
- "VIRTUAL CLOCK" reads "Virtual clock".

Verified locally:

- Test Lab on the Northstar demo shows 8 of 10 runs with "Show 2 more", and "Choose a journey or a saved scenario first." when none is chosen.
- `make fmt-check`, `make lint`, `make typecheck`, `make test`, `git diff --check`, and the full `npx playwright test` (104 passed).

## Consistent labels and counts — 2026-11-17

The fifth group from the design review.

- Small uppercase labels above headings were removed from Overview, Customers, Events, Settings, Test Inbox, the sign-in page, the publish review, the activity drawer and step settings. They repeated the heading below them in mixed casing. The AI panel's "START WITH A BRIEF" reads "Example brief". The unused `.eyebrow` and `.inspector-eyebrow` rules are gone.
- On the Northstar journey the start step said "Reached 12" while the journey had 14 entries. Two purchases arrived before the start step ran, so those executions ended on the trigger and showed as "Exited here 2" without being counted as reaching it. A step's Reached count is now at least its completed plus exited count, so the start step reads 14.
- Overview's recent entries showed the raw state ("completed") where Analytics says "Completed without conversion". The overview activity query now returns `exit_reason`, and the row uses the same outcome labels as Analytics.
- Overview's channel cards showed a bare "11" next to "12 logical message plans". They now read "12 planned messages" and "11 accepted".

Verified locally:

- The rebuilt Overview, workspace Analytics and journey Analytics on the Northstar demo: entries 14, start step reached 14 (12 completed, 2 exited), recent rows "Completed without conversion", channel cards "12 planned messages · 11 accepted" and "9 · 8".
- `make fmt-check`, `make lint`, `make typecheck`, `make test`, `make test-integration`, `git diff --check`, and the full `npx playwright test` (104 passed).

## Journey editor header — 2026-11-17

The fourth group from the design review: the header showed Active, Editing draft and Latest published v1 side by side, gave six actions the same weight, and kept Publish as the primary button when publishing would change nothing.

- The version line now reads "v1 live · 2 unpublished changes" or "v1 live · no unpublished changes". The count comes from the same draft-to-version comparison as the Versions tab, loaded when the editor opens and again half a second after a draft edit. Live Monitor and Analytics show only "v1 live".
- Publish is the primary button only while the draft has unpublished changes or the journey has never been published. Otherwise it is a secondary button whose tooltip says publishing would not change behavior.
- Save is a quiet button beside the save state. On a phone, Validate, Test, Pause and Resume show only their icons, with the label kept for assistive technology and as a tooltip, so the actions fit on one row. The header is 130 px tall at 390 px.
- The reseeded Northstar journey shows "2 unpublished changes": its v1 uses the accelerated sandbox waits and the draft holds the real ones, which Versions already listed.

Verified locally:

- The versions e2e test now also checks the header line and Publish's emphasis with no changes and with one change.
- The header captured at 1280 and 390 px on the Northstar journey.
- `make fmt-check`, `make lint`, `make typecheck`, `make test`, `git diff --check`, and the full `npx playwright test` (104 passed before the new assertions; the versions spec passes with them).

## Phone navigation — 2026-11-17

The third group from the design review.

- The phone navigation is one scrolling row of ten items, with nothing to show that more items exist. It now fades on the side that has more items, scrolls the current page to the middle when the page changes, and marks it with `aria-current="page"`, Settings included.
- The review asked for the workspace switcher on phones. The sidebar's workspace box only shows the one workspace a user belongs to, and the chevron beside it opened nothing. The chevron is removed rather than copied to the phone layout.

Verified locally:

- A Playwright check at 390 px on Overview, Test Inbox and Settings > Team: the current item is in view with `aria-current`, and the fade classes follow the scroll position.
- `make fmt-check`, `make lint`, `make typecheck`, `make test`, `git diff --check`, and the full `npx playwright test` (104 passed).

## Canvas overlap fixes — 2026-11-17

The second group from the design review.

- With the AI composer or step library open at 1280 px the canvas is about 710 px wide, and the mini map sat under the centered canvas bar. A container query on the canvas now hides the mini map when the canvas is narrower than 840 px.
- On a phone the first step filled the width and opened under the "7 steps" chip. The initial view now starts the first step below the chip when the centered step would reach under it; wide canvases keep the old 32 px top.
- The review's other two canvas overlaps were false positives. "Keyboard shortcuts" under the zoom level at 1280 px and "Auto layout" at 390 px are items in the closed ⋯ menu. Chrome reports a layout box for closed `<details>` content but does not paint it, and with the menu open at 390 px every item is visible.

Verified locally:

- A Playwright check of the reseeded Northstar journey: the mini map is hidden with the AI composer open and shown when it closes at 1280 px. At 390 px the first step clears the chip and the canvas bar and open menu are fully visible.
- `make fmt-check`, `make lint`, `make typecheck`, `make test`, `git diff --check`, and the full `npx playwright test` (104 passed).

## Mobile and accessibility fixes — 2026-11-17

The first group from the full design review (ui-ux-pro-max and impeccable), in the order the user approved.

- The mobile navigation's selected item used `--violet-69-text` on the soft accent, about 2.5:1. It now uses `--accent-text-strong` at weight 600. The Settings item shows as selected on Settings.
- Phone touch targets are 44 px: the navigation items and the top bar's search, theme and notification buttons.
- Settings > Team on a phone put one letter per line next to a fixed role picker. The role picker now moves under the name below 560 px. A rule for a native `select` that the custom picker had replaced was removed.
- Overview at 320 px: the channel cards use as many columns as fit, and the journey status filters and section headings wrap.
- The journey list scrolled sideways on phones. The screen-reader-only "Actions" header was positioned against the page rather than the table's scroll box, so the table's scroll box is now positioned.
- Builder: the step library and step settings headings are h2 after the journey's h1, not h3. Connection port labels are 11 px in the muted text color instead of 10 px gray. "Explore journeys" uses the text color.

Verified locally:

- A Playwright audit with the demo account at 320 and 390 px over Overview, Settings > Team, Journeys, Test Lab, Analytics and Customers: no page scrolls sideways and no top bar or navigation control is under 44 px. Settings > Team at 320 px checked by eye.
- `make fmt-check`, `make lint`, `make typecheck`, `make test`, `git diff --check`, and the full `npx playwright test` (104 passed).

## Designed email bodies — 2026-11-16

The user found the Northstar cart email plain: every line was bare text. The renderer sanitized email HTML with bluemonday's UGC policy, which removes every `style` attribute, so no template could carry a design.

- Email HTML now keeps a fixed list of layout and typography properties (color, background color, font, size, weight, line height, letter spacing, alignment, margin, padding, borders, radius, width, max-width, height, display). bluemonday's handler for each property checks the value. Background images, positioning, z-index and expressions are still removed, and a personalized value cannot add a second property to a style.
- Tables may carry `role="presentation"`, `cellpadding`, `cellspacing` and `border`.
- Links no longer get `rel="nofollow"`, which means nothing in an email. Adding it made every email with a link report "HTML was sanitized" even when nothing was removed. Reformatted styles no longer raise that warning either.
- The Northstar cart email in `scripts/showcase-recipe.mjs` is a 560 px card on a warm background: a letter-spaced NORTHSTAR header, the heading and greeting, a product card with a swatch, name, variant and price, a dark button back to the cart, and the existing sign-off. The copy is unchanged.

Verified locally:

- New Go tests keep the safe styles without a warning and remove the unsafe ones; the new template renders through `messaging.Render` with no warnings.
- `make fmt-check`, `make lint`, `make typecheck`, `make test`, `make test-integration` and `git diff --check` pass.
- The rendered email was captured at 680 and 390 px and checked by eye.
- With the user's approval the API was rebuilt and the Northstar demo reseeded with `npm run demo:reset`; the previous workspace is kept archived, as the reset does.
- The phone preview now renders an email at 390 px, a real phone's width, scaled to the drawn screen. The Test Inbox and Message Studio preview documents no longer pad the body by 24 px, which pushed full-width emails past the right edge; plain content keeps a side margin.
- The new email was captured from the reseeded Test Inbox in the phone and desktop views; it shows no sanitize warning. `make fmt-check`, `make lint`, `make typecheck`, `make test` and the messaging, recovery, templates, test-lab, engagement, demo and acceptance Playwright specs (16 passed) pass.

## Test Inbox design — 2026-11-16

The user reviewed the Test Inbox with the ui-ux-pro-max and impeccable skills and asked for a clearer filter column, a preview that looks like a real phone, a tab menu that fits on one line, readable payload details and a desktop preview that reads as an opened email.

- Segmented controls (channel, device and every other use) have one accent thumb that slides to the chosen option over 0.45 s. The chosen label turns white; reduced motion turns the slide off.
- The customer filter has a visible "Customer ID" label, a search box with a clear button and a labelled Filter button. Recipients appear as chips with initials that toggle the filter.
- The list scrolls inside its column and the detail column stays in view on desktop, so the page is no longer thousands of pixels tall.
- SMS and mobile email previews are drawn inside a phone: status bar, a thread header and one message bubble with links, then the character count and segments below the phone. The technical facts moved to Payload. The phone colors are theme tokens (`--phone-screen`, `--phone-bubble`, `--phone-link`).
- The desktop email preview is a mail window with the subject, recipient, time and preheader. The email renders at 640 px and is scaled to the column; Open full size shows it unscaled in a dialog.
- Detail tabs are a single-line tab list (`role="tab"`, arrow keys) with a sliding underline; "Execution trace" is now "Trace".
- Payload shows template and source step as tiles, trace and workflow snapshot as short IDs with copy buttons, personalization as token → value rows, and the raw JSON in a disclosure.

Verified locally:

- `make fmt-check`, `make lint`, `make typecheck`, `make test` and `git diff --check` pass. The local `.agents/`, `.claude/` and `.impeccable/` folders are now ignored, which unblocked `make fmt-check` and `make lint`.
- The 11 Playwright specs that reach the inbox, contract editor or test lab (behavior-checks, data-catalog, ai, engagement, scoped-contract, test-lab, messaging, navigation, recovery, templates, versions): 48 passed. The two tab clicks in recovery and test-lab now use the tab role.
- Checked in the built-in browser at 1280 px in light and dark: thumb and underline positions match the chosen option, payload keys no longer overlap values, and the full-size dialog renders the email.

## Behavior contract editor design — 2026-11-16

The user found the contract editor colorless and uneven: consent checkboxes sat above their labels, condition rows mixed labels inside and above their boxes, and every control looked alike. The design follows the impeccable and ui-ux-pro-max skills.

- Each part of the contract (situation, subsequent events, expected result, eligibility boundaries) has a heading with an icon, divided by rules instead of boxed in nested cards. Expected result is the one tinted panel.
- Email and SMS consent are switches with the channel's icon and say whether the customer opted in. Each input keeps an accessible name, such as "Email consent in this situation".
- A condition reads as a sentence: an IF or AND tag, then the field, comparison and value, all with the caption inside the box, and a remove icon. Number fields in expected result and events use the same box. On a phone the parts stack and the remove target is 44 px.
- Add buttons are dashed with a plus; remove links are red. Empty conditions say what that means. An alternative situation no longer repeats the long introduction.
- The creation page's 18 px label margin no longer pads rows inside the editor.
- The brief page's example cards carry an icon.

Verified locally:

- Impeccable's detector found nothing in the changed components.
- `make typecheck`, `make test`, `git diff --check`, and ESLint and Prettier on the repository's own files. `make fmt-check` and `make lint` also read the untracked `.agents/` and `.claude/` skill folders the user installed, and fail on their files.
- `npx playwright test` against the rebuilt web: 104 passed.
- The editor checked by eye at 1280 and 390 px; the creation screenshots were captured again.

## Screenshot refresh and README — 2026-11-16

The 21 screenshots were from November 13 and showed an older interface, at a 1600 px viewport that made the text small on a page. All were captured again from the running app, and the README was rewritten in the plainer layout of Smart Campaign Composer.

- Captures now use a 1280 × 800 CSS viewport at 2× (2560 × 1600 PNGs). The journey list is 1280 × 560; the email preview (1280 × 1180) and publish review (1280 × 1080) are taller so they show whole. Live Monitor drilldowns scroll to the map and executions instead of the page header.
- The capture scripts follow the current UI: Focus selected step is in the More canvas actions menu, the scenario and test run pickers are custom selects, and the customer profile is read-only. The creation capture serves its own AI settings, so it runs with AI disabled. Its manifest now records the real image size instead of a fixed 3200 × 2100.
- The creation gallery is now a local replay of the showcase recipe, not the live OpenAI capture of November 13; the gallery pages say so and link that capture's report.
- The capture found five interface problems, now fixed: a builder card's clamped third line showed through its bottom padding; Fit view put steps under the floating toolbar; the Live Monitor execution summary ran together as "Version 1maya-patelExit: completed"; the metrics map's start and Fit all put the first step under its buttons; the Test Inbox and Message Studio said "1 segments", and the inbox's To line ended in "·" without a preheader.
- With the user's approval, the leftover archived journey "Engagement check (temporary)" was deleted from the Northstar demo, and its journey counters in `metric_totals` were lowered to match, so the overview counts one journey.
- README: what the project does, an Under the hood table linking each piece of engineering to its docs and tests, a nine-step product tour, how execution behaves, install from scratch with troubleshooting, and the provider, telemetry, test and layout sections. Stale claims about November 13 runs and 3200 px captures were removed from the gallery pages and SHOWCASE.md.
- The GitHub description and topics were updated to describe the project.

Verified locally:

- `node scripts/capture-product.mjs` and `node scripts/capture-creation.mjs` against the rebuilt stack; every picture was checked by eye.
- `make fmt-check`, `make lint`, `make typecheck`, `make test`, `git diff --check`.
- `npx playwright test` against the rebuilt web: 104 passed.

## Empty message labels — 2026-11-16

In the live check a birthday email ended with "Mağazaya dönmek için:" and "Hediye kodun:" and nothing after them: the brief gave no link or code, and the model rightly invented none, but left the labels.

- A message line that ends with a colon and is followed by nothing, or only by another such line, is now a message issue: `"Hediye kodun:" introduces nothing; add what it announces or remove the line`. Text and HTML are both checked. A list, link or `{{ offer.code }}` after the label counts as content, and so does the email's discount code box.
- The check is part of `messaging.ValidateWorkflow`, so AI journey proposals are sent back for repair, and the editor's validation and the publish check report it. "Write a message" and "Improve a message" check it too.

Verified locally:

- `TestDanglingLabel`, `TestEmptyLabelIsAMessageIssue` and `TestMessageWithAnEmptyLabelIsRepaired`: the first attempt is refused with the label named, and the repair prompt carries it.
- `make fmt-check`, `make lint`, `make typecheck`, `make test`, `git diff --check`; existing templates, the showcase and the EUR recovery catalog still validate.
- `npx playwright test --workers=2` against the rebuilt API: the first run, started as the API container restarted, had 12 failures; a second full run passed 104 of 104, and the before/after test passed 10 of 10 repeats.

Not covered: not tried with a live model.

## Live verification of the latest AI changes — 2026-11-16

The user asked to verify the open items except Claude. The API ran with `AI_PROVIDER=openai` and `gpt-6-luna`. The key was passed only to the `docker compose` process, and the API went back to `disabled` with no key afterwards. No proposal was applied; the test journeys are unchanged.

- **Remote CI.** GitHub Actions passed `quality` and `integration` on `e439016`, the head that holds every commit of this session. `f896fb4` was cancelled because the next push replaced it. The Playwright suite does not run in CI; it ran locally only.
- **Before and after.** "Remove the email consent check. Change nothing else." on the cart journey: the model again shortened the order wait, and also cut "stoklar sınırlı" from the email subject. The panel listed "order_completed · up to 2 hours → order_completed · up to 1 hour" and the subject change, warned about the wait, and kept Apply disabled until the box was ticked.
- **Explain the journey.** The explanation matched the graph, and there was no Apply button.
- **Facts the brief does not state.** `scripts/live-factcheck-invention.mjs` sent four briefs, each in a fresh workspace; three asked for a discount, gift code, link, price cut or end date without giving one, and one stated every fact. Raw evidence: [result.json](validation/live-invention-2026-11-16/result.json). All four were accepted on the first attempt. The model invented nothing: it left the values out and named each missing one in a warning, and the control email used `WELCOME20`, 20% and the link. So the fact check refused nothing, and how often it catches an invented fact is still not measured live.
- **Timeout.** Nine calls took 7.5–30.6 s. The request that hit 45 s before took 25 s and 14 s this time, so no call has yet needed the longer limit.

New findings, not fixed:

- The birthday email kept the labels "Mağazaya dönmek için:" and "Hediye kodun:" with nothing after them, because the brief gave no link or code. Nothing flags an empty label like this. Fixed in "Empty message labels".
- "Explain the journey" returned a warning about the provider schema ("fallbacks … {path,value} dizisi"), which a marketer cannot act on.
- The cart email says "İndirim kodunuz kısa süre geçerli; stoklar sınırlı". The brief stated neither; the model added them when asked to make the email more urgent. The fact check covers links, codes and numbers, not claims like scarcity or a deadline, so this passed with only a model warning. The user chose to leave this open for now.

## AI changes show before and after — 2026-11-16

In the live check the model was told to "change nothing else" and still moved the order wait from 2 to 1 hour and another wait from 24 to 25 hours. The panel listed the steps as "Updated nodes" only, and the step label changed with the value, so the change was easy to miss.

- A proposal for an existing journey lists each changed step with its old and new setting, for example "Wait 2 hours → Wait 1 hour", and "Renamed from …" when the label changed. It reuses the version comparison.
- When timing, conditions or other non-message settings change, a warning names the steps and "I reviewed the warnings and assumptions" must be ticked before Apply. A rewritten email or SMS, or a rename alone, does not ask for this.
- Durations read in the largest whole unit: 90,000 s is "25 hours", not "1.0416666666666667 days". This also changes the canvas and Test Lab captions.

The panel does not tell which changes were asked for; it shows every change and asks a person to confirm the ones that change behavior.

Verified locally:

- `scripts/proposal-review.test.mjs`: three tests for the change lines, which changes need confirmation, and duration labels.
- New Playwright test in `tests/e2e/ai.spec.ts`: a change that shortens a wait shows "Wait 2 hours → Wait 1 hour" and the warning, and Apply stays disabled until the box is ticked. A screenshot of the panel at mobile width was checked by eye.
- `make fmt-check`, `make lint`, `make typecheck`, `make test`, `git diff --check`. `npx playwright test --workers=2` against the rebuilt web: 104 passed.

Not covered at the time: not tried with a live model. Verified live in the entry above.

## Longer AI provider timeout — 2026-11-16

A live "Change this journey" request failed after 45 s with "AI provider failed or returned an incomplete response". Each provider call had 45 s, so that two attempts fit the 90 s request budget and the web client's 90 s wait.

- One provider call now has 90 s (`ai.ProviderTimeout`).
- An AI request has 200 s, two calls plus 20 s for validation (`ai.RequestBudget`). The API extends the write deadline of AI routes past its 120 s server limit.
- The proxy waits 220 s on AI routes and the web client 215 s, so neither cuts a request the API is still working on. The live harness waits 215 s.

Verified locally:

- New `TestAIRequestOutlastsTwoProviderCalls` checks the request deadline and reads the timeouts in `nginx.conf` and `api.ts`.
- `make fmt-check`, `make lint`, `make typecheck`, `make test`, `git diff --check`.
- Rebuilt API and web with AI disabled; the web container serves the 220 s proxy timeout. `npx playwright test --workers=2`: 102 passed.

Not covered: no live call has taken more than 45 s since the change, so a slow call finishing is not shown.

## Fixes from the browser LLM test — 2026-11-16

The browser test of journeys made with gpt-6-luna found six more problems. All are fixed without a provider call.

- **Test Lab sample.** `offer.code` is no longer asked for as test data. A rule like `amount > 49` gets a passing value instead of 0. The reviewed contract's example inputs and consents fill the sample, so a test run with the defaults follows the main path.
- **Bound follow-up events.** When a follow-up event is scoped to the entry (for example the order's cart number equals the entry's cart number), the contract form copies the entry example into the private event example, or a different value for "different object". The error now names the situation and the field instead of "Clause 2: Private event example contradicts…".
- **Refusal reasons.** When both attempts fail, the 422 message ends with the last three problems as "step: message". Each refused attempt in the `ai.generation` audit record keeps up to five issues; they name steps and rules, not customer data.
- **Change keeps name and description.** "Change this journey" and other edits keep the journey's existing name and description; only "Draft a new journey" takes the model's.
- **Message actions list message steps only.** "Write a message" and "Improve a message" offer only email and SMS steps; "Explain a step" still offers all.

Verified locally:

- New tests: two in `scripts/test-context.test.mjs`, `TestBoundEventExampleErrorNamesTheSituationAndField`, `TestChangeKeepsJourneyNameAndDescription`, and refusal reasons in `TestSemanticMismatch`.
- `make fmt-check`, `make lint`, `make typecheck`, `make test` and `git diff --check`.
- Browser, with AI disabled, on the cart journey of the test workspace: Test Lab shows no `offer.code` hint and fills amount 50 and the name; a run with the defaults passed with two sandbox messages. Changing the contract's cart number updated the bound order example. "Improve a message" listed only the email and SMS steps.
- `npx playwright test --workers=2` against the rebuilt local stack: 102 passed.

Live check, same day, with the user's approval: the API ran with `AI_PROVIDER=openai` and `gpt-6-luna`. The key was passed only to the `docker compose` process, and the API went back to `disabled` with no key afterwards. No proposal was applied; both drafts are unchanged.

- "Change this journey" on the birthday journey (subject with the first name) and on the cart journey (remove the email consent check) both kept the journey's name and description.
- "Send the email at once, without waiting for the order" broke the reviewed contract. Both attempts were refused, and the panel showed "Last problems: Clause 1: virtual completion time differs from the reviewed expectation; …". The audit record holds 1 and 4 issues for the two attempts.

New findings, not fixed:

- The first try of that request failed with "AI provider failed or returned an incomplete response". The audit record shows `provider_request_failed` after 45,002 ms: the provider client has a fixed 45 s timeout (`internal/ai/providers.go`). The retry took 31 s and 24 s. Fixed in the next entry.
- Asked to "change nothing else", the model still moved the order wait from 2 to 1 hour and the later wait from 24 to 25 hours. The contract passed, and the diff listed the nodes, so a reviewer can see it. Addressed in "AI changes show before and after".

## Coded journeys can be approved — 2026-11-16

Found while creating journeys through the browser with gpt-6-luna. A journey whose message showed `{{ offer.code }}` could not be approved: the data-mapping check looked for `offer.code` in workspace data, marked it "Not connected" and kept "Approve & open builder" disabled. The earlier live runs saved drafts through the API and never reached this screen.

- `offer.code` is the message step's own discount code, so grounding now skips it like `event.event_name`.

Verified locally:

- New `TestOfferCodeIsNotWorkspaceData` failed before the change and passes after.
- `make fmt-check`, `make lint`, `make typecheck`, `make test` and `git diff --check`.
- In the browser, "Refresh data" on the cart proposal removed the "Not connected" row and enabled approval.

Not covered at the time: Test Lab still asked to "Review offer.code in Advanced options". Fixed in the next entry.

## Text contrast — 2026-11-15

Frontend-only. Secondary and colored text was too pale: the sidebar menu measured 3.1:1 on white, below the 4.5:1 that WCAG AA and the UI/UX Pro Max guidance require for normal text.

- `--muted` is darker in light mode (`#556075`). Text that used the pale gray tokens now uses it, and the sidebar menu uses a darker gray.
- Selected tabs, filters, counts and avatars use the accent text tokens. Status badges, alerts, errors, notices, the virtual clock and Test Lab assertions use the darker green, red, amber and blue text steps; a new `--blue-50-text` was added for blue.
- No text is smaller than 12px; the 11px labels in the stylesheet and five components were raised.
- Disabled buttons keep their reduced opacity, which the guidance allows.

Verified locally:

- A Playwright script measured the rendered contrast of every text element on 20 screens, in light and dark, at 1440×900. Before: 499 elements below the threshold (26 distinct). After: 20, all disabled buttons (4 distinct).
- Overview and the journey builder were checked by eye in light and dark.
- `make fmt-check`, `make lint`, `make typecheck`, `make test`, `git diff --check`, Docker web rebuild and `npx playwright test --workers=2`.

Not covered: text over images or gradients was skipped by the measurement, and only the first page of each screen was measured; dialogs, menus and hover states were not opened.

## Spacing and sub-navigation sweep — 2026-11-15

- Preview & Test's "Message preview" / "Journey path & Test Lab" buttons sat directly under the journey tabs and looked like a second tab row with no selected state. They are now a segmented control with space above and below and `aria-pressed` on the chosen view.
- The same pattern was checked across the app. Test Inbox's detail tabs (Rendered, Raw, Variables, Delivery) looked identical to its action buttons; the actions now sit in their own row with icons and the tabs use the segmented control. Live Monitor's Executions filters touched the panel edge and now have padding. Template cards align their buttons to the card bottom. The Events "New event ID" button sits beside its field. The Overview greeting lost a stray space before its full stop.

Verified locally: `make fmt-check`, `make lint`, `make typecheck`, `make test`, `git diff --check`, rebuilt `web`, and `npx playwright test --workers=2`: 96/96 desktop/mobile. Overview, Journeys, Templates, Analytics, Customers, Events, Settings, Versions, journey Analytics, Live Monitor, Preview & Test and Test Inbox checked by eye in the dark theme at 1440 px.

Not covered: the light theme and phone widths were not checked by eye for these changes.

Follow-up: Preview & Test's Message node field was a native `<select>`. Safari drew its menu in the system colours (dark over the light theme) and as wide as the full-width field, and it listed "First message" on top of the first message's own name. It is now the app's themed dropdown (`FilterSelect`), labelled "Message step", listing each email/SMS step once with its channel and position. Checked open in the dark and light themes at 1440 px; `make fmt-check`, `make lint`, `make typecheck`, `make test`, `git diff --check` and Playwright 96/96 passed. The app's other native selects are unchanged.

Follow-up: every remaining native `<select>` (45 across 13 components) now uses `ui/AppSelect.vue`, a themed listbox built on reka-ui. It accepts any value type, including `''` and numbers, opens inside a modal `<dialog>` when it sits in one (the edge-insertion dialog made an outside menu unclickable), and keeps `<label>` association. The Rule editor's operator list now stores the operator code rather than its display text. E2E tests pick options through `tests/e2e/helpers/select.ts` and read the chosen label instead of a value. Verified locally: `make fmt-check`, `make lint`, `make typecheck`, `make test`, `git diff --check`, rebuilt `web`, Playwright 96/96 desktop/mobile. Checked by eye: Test Inbox Status menu in the light theme at 1440 px.

## Dropdown content and choice controls — 2026-11-15

- Choices of two or three short options are now a `SegmentedControl` (radio group with arrow keys) instead of a menu: AI provider, rule match (all/any), data definition kind, Overview activity period, journey re-entry, entry type, wait type, date direction, attribute operation, Test Inbox channel and device, Test Lab mode and fault channel.
- Menu options use plain, sentence-case labels instead of raw codes, Title Case and dash-joined phrases (`SIMULATED` → Simulated, "Goal reached — converted" → Converted). Where a choice needs explaining, a second line says so. The Builder's exit reason and the journey's global exit outcome now share the same labels.
- `AppSelect` options can carry the product's own markers: step icons and Timing/Branching/Actions groups in the insert-step menu, status-coloured dots for message statuses, exit reasons, expected outcomes and provider faults, and channel icons. Hover is neutral grey; the selected item is bold with a check. The menu is at least 220 px wide.
- Test Inbox filters show their caption inside the field ("Status · All statuses") rather than as a separate form label.

Verified locally: `make fmt-check`, `make lint`, `make typecheck`, `make test`, `git diff --check`, rebuilt `web`, Playwright 94/96 then the 2 failing `data-catalog.spec.ts` cases (desktop/mobile) passed 4/4 after picking same-named definitions by their path line. Checked by eye: Test Inbox (light 1440 px, dark 375 px), Overview period control, Builder wait-step settings (light 1440 px).

Not covered: message status badges in Test Inbox rows still show upper-case codes; the insert-step menu and Test Lab advanced options were not looked at by eye.

Follow-up: `StatusBadge` now shows status codes as words everywhere it is used (Test Inbox, Test Lab, Live Monitor, Customers, journey lists): `SIMULATED` → Simulated, `TEST_ACCEPTED` → Accepted, `PASS` → Passed, `CONDITION_NOT_MET` → Not eligible. The code stays on the badge as `data-status`, and its colour is unchanged. One `test-lab.spec.ts` assertion now expects "Accepted". Verified locally: `make fmt-check`, `make lint`, `make typecheck`, `make test`, `git diff --check`, rebuilt `web`, Playwright 96/96 desktop/mobile; Test Inbox rows checked by eye in the light theme at 1440 px.

## Product acceptance verification — 2026-11-11

Verified locally against source `b1b7ab4`, before this documentation update:

- `make check`: formatting, lint, typecheck/build, race-enabled Go unit tests and Node tests pass.
- `make test-integration`: all isolated PostgreSQL package tests pass, including runtime races, event deduplication, delivery policy, analytics and sandbox isolation.
- `npm run test:e2e -- --workers=2`: 46/46 desktop/mobile browser tests pass. Coverage includes canvas persistence and reload/deep links; canonical EUR recovery scenarios A–E and the seven-day enrollment cooldown; personalized email/SMS and run-scoped Inbox decisions; explicit publishing; actual Mailpit SMTP/mock SMS execution; customer monitoring and analytics navigation.
- A one-off extension of `scripts/test-recovery.mjs` used real API-key ingestion, then stopped/started the actual local worker and submitted the duplicate event. It verified one execution, three unique steps and matching projected entries/completed/unique-customer totals of 1/1/1. The verification API key was revoked. The extension and execution logs remain in ignored `.tmp` files; no credentials were committed.
- No application fix was needed. No benchmark, paid LLM call, external messaging provider or deployment was performed. These are local results, not a claim about remote CI.

The original 1h/24h/48h recovery windows were checked with the sandbox virtual
clock. Actual process-restart verification used the existing short durable-wait
workflow; it did not wait 73 real hours or alter a live journey's configured times.

## Bounded runtime turn delivery — 2026-11-10

- `a846d60` adds bounded wait-event/blocker occupancy sampling on a separate diagnostic connection and optional CPU/allocation profiles. The default integration command now includes the diagnostic lock test.
- `89e373f` introduces bounded short-step turns with per-node history and recoverable inline failures. The first timer-inline benchmark regressed p95 to 36.503s; this behavior is superseded, with the artifact retained.
- `1aa63b2` restores a timer-resume scheduling boundary and writes each node result once per transaction. Two final profiled 10,000-execution runs improve entry-to-wait from 40.415s to 22.958–23.495s and timer p95 from 11.412s to 10.352–10.526s. Total delivery drain is essentially unchanged; customer-lock contention remains. See [measured bounded turns](BOUNDED-TURNS.md), including the slower intermediate run and sampling limitations.
- `9ff4258` aligns Go module/imports with `github.com/negativexq/customer-journey-composer/apps/api`. A private [GitHub repository](https://github.com/negativexq/customer-journey-composer) was created at the user's request. Historical benchmark source identities remain unchanged.
- Verified locally: offline Go build, `make check`, complete PostgreSQL integration suite including diagnostic sampling, 32/32 desktop/mobile browser tests, Docker rebuild/readiness, telemetry/alert-rule checks and actual worker restart recovery. All five new benchmark runs preserve zero duplicate/failed logical actions and two expected provider retries. Gitleaks scans 51 pre-publication commits with no findings. No live LLM tests were run.

## Operational review delivery — 2026-11-10

- `2c034f6` fixes Nginx upstream DNS after API container replacement; the isolated IP-replacement test passed without restarting the proxy.
- `d0d9ab6` adds private Prometheus metrics, bounded queue sampling, sanitized pgx spans, correlated request/worker logs, collector/Tempo/Grafana provisioning and seven tested alert rules. Local external notification routing and centralized log storage remain unimplemented.
- `5a65541` adds paced ingestion, separate successful-attempt latency and delayed-provider benchmark scenarios.
- `709b2d0` removes runtime/delivery group barriers with bounded independent processors; `0064f44` adds shared event admission outside the API connection pool and correct transient authentication failure status; `1322154` coalesces nearby free slots for 2ms to reduce claim-query churn without waiting for slow customers.
- Six measured artifacts preserve baseline, intermediate and final normal/delayed runs. Final 10,000-message mock provider drain is 66.464s versus matched baseline 88.519s; delayed 1,000-message drain is 11.060s versus 35.118s. Zero duplicate logical actions or failed executions in all six runs. Timer p95 regresses from 8.984s to 10.761s; Analytics tail samples are inconsistent. These are documented open performance issues, not hidden by the end-to-end gain.
- Verified final code: `make check`, complete package-isolated PostgreSQL integration suite, 32/32 desktop/mobile browser tests, Docker rebuild/readiness, actual worker restart recovery and `make test-telemetry` (durable trace, both Prometheus targets, alert syntax/firing fixture). Dependency audit at the telemetry slice found zero reachable Go vulnerabilities and zero npm findings. Hosted CI and live LLM tests were not run.
- [Competitor research](UX-RESEARCH.md) covers official Customer.io, Braze, Iterable, Knock and Loops docs/screens. New publish review, version clarity, readable timeline/decisions and transient-auth UI are a prioritized backlog; those components are not claimed as implemented.

## Phase 1 delivery — 2026-11-10

- `3fa3de5` — requirements and phase order recorded.
- `370e7c4` — runnable Vue/TypeScript and Go monorepo, typed health/readiness/OpenAPI, transactional PostgreSQL migrations, Docker Compose with Mailpit, Makefile and deterministic tests.
- `d33ef52` — GitHub Actions quality, commit-subject, real database and browser checks. Action definitions are pinned by commit SHA.
- Verified locally: `make check`, `make build`, `make test-integration`, `make up`, and `make test-e2e` (4/4 desktop/mobile checks). Workflow validated by actionlint v1.7.12. `npm audit` reported zero vulnerabilities for the installed lockfile.
- Exposed locally: web `http://localhost:5173`, API `http://localhost:8090`, Mailpit `http://localhost:8025`.
- No live LLM calls were made. Provider adapters remain phase 7 work; both Claude and OpenAI are included in the scope.
- No remote repository is configured, so GitHub-hosted CI has not run. No journey execution or load benchmark exists yet.

See [verification notes](VERIFICATION.md) for reproducible checks and [architecture](ARCHITECTURE.md) for implemented versus planned boundaries.

## Phase 2 delivery

- `d6aea82` — bcrypt passwords, hashed expiring sessions, CSRF checks, workspace scoping, Viewer write rejection, persistent journeys and optimistic draft revisions.
- Management UI provides workspace registration/login/logout, dashboard, searchable/filterable/sortable journeys, creation, duplication, archive and reopenable drafts.
- Verified: `make check`, isolated real-PostgreSQL tests (including concurrent revision updates and cross-workspace access), Docker rebuild and 6/6 desktop/mobile browser tests.
- User steering: optimize PostgreSQL schema, indexes and analytics queries for low read latency and bounded database load. Validate plans with EXPLAIN; avoid speculative indexes and unbounded scans on dashboard requests.

## Phase 3 delivery

- Typed workflow vocabulary for all eight node types; required ports, cycles, reachability, configuration, bounded conditions, reentry and global exit validation.
- Row-locked publishing produces immutable numbered snapshots; database triggers reject mutation. Draft revision and publish revision guard concurrent edits.
- Vue Flow canvas: drag/drop, connection editing, node panels, AND/OR condition groups, global exit/reentry settings, zoom/pan/fit/minimap, multi-selection, undo/redo, copy/paste, duplicate, auto layout, autosave and validation markers.
- List queries fetch metadata instead of large workflow JSON; versions fetch definitions individually. This avoids TOAST reads and large dashboard/list responses.
- Verified: `make check`, real PostgreSQL graph/version tests, and 8/8 desktop/mobile browser tests including connecting nodes, validation, publication and reopening the canvas.

## Phase 4 delivery

- `1b3ea84` — profiles, hashed/revocable ingestion-only API keys, event persistence/deduplication and atomic execution/job creation. All queries derive workspace scope from identity.
- `3027a7b` — customer email-prefix index, bounded keyset search path, metadata-only event lists and a single database query per readiness check.
- Active entry matching uses a partial workspace/event index; reentry probes use indexed existence/latest lookup rather than counting history; active enrollment is database-constrained.
- Customer/event/settings screens support profile attributes/consents, event timeline/payload inspection, manual ingestion and API key creation/revocation.
- API policies document late/future events and best-effort per-item batch semantics.
- Verified: `make check`, isolated PostgreSQL tests (8 concurrent identical events => exactly one execution/job, conflicting IDs, active enrollment, late/future events, scoped/revoked API keys), Docker stack and 10/10 browser tests. The builder test now publishes then starts an execution through the real event HTTP API.

## Phase 5 delivery

- PostgreSQL worker claims due jobs with SKIP LOCKED, bounded batches, expiring leases and regenerated fencing tokens. Old claims cannot commit effects; transaction failures retry with bounded backoff.
- Customer locks serialize event, timer and execution transitions. Durable waiters resolve exactly once, global exit cancels jobs/waiters/pending outbox actions, and published-version references remain fixed.
- Shared pure `Decide` function and Clock boundary cover all node types; message nodes produce a unique transactional outbox entry for phase 6 delivery. No provider calls occur in step transactions.
- Event context references the matched event ID rather than copying payload JSON into each execution. Execution list/trace APIs are bounded and workspace-scoped.
- Verified: quality/unit checks, real PostgreSQL claim recovery and stale-token tests, durable wait timing, 12 event/timeout races, global exit, attribute update, pause/sunset and version isolation; 10 browser tests; actual Docker worker stop/start preserving a wait and event deduplication.

## Phase 6 delivery

- `c33d2e7` — bounded, code-free personalization; HTML sanitization/escaping; link checks; GSM-7/UTF-16 SMS segment accounting; SMTP and fault-injectable mock adapters.
- `ee358d9` — transactional outbox scheduling, durable logical delivery jobs/attempts, 5-attempt backoff, fenced dispatch and explicit UNKNOWN outcomes. Consent is rechecked before dispatch; global exit cancels pending jobs.
- Workspace frequency caps count reservations and accepted logical sends, not retry attempts. Indexed acceptance-time lookups and bounded counts avoid history scans; old pending/unknown jobs retain reservations.
- Channel Studio provides email/SMS composition, templates, customer-isolated preview overrides, missing values, mobile/desktop preview, rendered content, links and allowlisted/idempotent test sends. Customer profiles expose delivery decisions/attempts.
- SMTP defaults to local Mailpit; SMS remains a mock. No external SMS provider or paid LLM request was used.
- Verified: all quality/unit/integration checks; local SMTP protocol and lost-acceptance response; 429→500→success, consent revocation, caps, global exit, stale delivery leases, preview isolation and test-send idempotency; 14/14 browser tests including actual Mailpit receipt and live SMTP/mock SMS; actual worker restart recovery passed again.

## Phase 7 delivery

- `b053176` — versioned closed JSON Schema, typed Claude/OpenAI HTTP adapters, local schema/graph/message validation and a maximum of two provider attempts. Models are configured explicitly; there is no automatic provider/model substitution.
- API endpoints only return proposals/explanations. They have no save/publish path and never load live customer/event data. Per-node actions minimize input; cancellation and long provider Retry-After are respected.
- AI panel implements all requested actions, diffs, warning acknowledgement, Apply/Discard and stale-draft rejection. Apply uses ordinary revision-guarded draft saving; publication remains separate.
- Tests caught and fixed shared-slice mutation in message proposals. Incomplete outputs, unknown node types, refusals, schema drift and unbounded repair are rejected.
- Verified: all quality/unit checks, local HTTP provider contracts and real PostgreSQL integration tests; 18/18 browser regressions including AI review/stale behavior. No paid/live LLM request was made. Actual provider schema acceptance, model quality/latency and live prompt demo remain deferred by user instruction; see [AI.md](AI.md).

## Phase 8 delivery

- `3a67d8c` — isolated deterministic scenario runtime with shared production decisions/eligibility, snapshot deduplication, revision-guarded persistence and separate rendered message storage.
- Test Lab and journey path preview expose virtual time, run/pause/step/advance, scenarios, fault injection, assertions, replay and customer overrides. Draft saving is flushed before snapshot capture. Named timezones are embedded for minimal production containers.
- Verified: `make check`, real PostgreSQL integration tests, and 20/20 desktop/mobile browser tests. Scenarios cover 26-hour flow, exact-deadline purchase, duplicates, retry/caps, deterministic replay and 1,000 pure sandbox customers; live records remain unchanged. No provider or live LLM calls occur in simulations.

## Phase 9 delivery

- `05e0043` — bounded inbox metadata API, individually loaded rendered outputs, provider attempts and execution traces; replay-parent comparison. Database trigger mirrors only explicit TEST sends and preserves immutable bodies.
- Three-panel inbox provides run/customer/journey/channel/status filters, isolated email/SMS rendering, copy, replay, payload, policy and attempt inspection. Pending real tests use TEST_PENDING; acceptance uses TEST_ACCEPTED.
- Verified: `make check`, all isolated PostgreSQL integration tests, 20/20 browser tests including inbox filtering/parent comparison, actual Mailpit TEST_ACCEPTED and exclusion of live deliveries.

## Phase 10 delivery

- `42c86be` — atomic asynchronous analytics projection, workspace/journey/version totals and hourly cohorts, lifetime distinct identities, indexed activity and version-scoped node stats. Batched JSON processing avoids temporary-table catalog churn and per-event hot summary writes.
- Live Monitor tracks customers on their actual published version, highlights visited/current paths, polls only while visible and exposes steps, deadlines, jobs, delivery decisions and attempts. Analytics shows denominators, cohort trends, channel outcomes, node funnel and comparisons. Dashboard totals no longer depend on the first 200 journeys.
- Last-24-hour entries use hourly summaries plus two indexed one-hour boundary ranges; raw history is not scanned on refresh. Rendered content does not enter projections. See [ANALYTICS.md](ANALYTICS.md).
- Verified: `make check`, package-isolated real PostgreSQL integration tests, concurrent projector/no-op/recovery rollback tests, distinct counts and retry-vs-logical-send counts; 22/22 browser tests including monitor/analytics and local demo login. Benchmark and EXPLAIN measurements remain phase 12 work.
- Additional user request: `e887409` adds generated local demo credentials prefilled at login, gated to development; desktop/mobile login and production gating tests passed.

## Phase 11 delivery

- `d141b4d` — row-locked lifecycle, matching full-test publish gate, atomic audit, cancellation/fenced recovery, Admin/Editor/Viewer management, production signup guard/bootstrap, bounded request/work budgets and durable OpenTelemetry parent propagation.
- `5f0df44` — lifecycle/publish UI, execution cancel/recovery, user roles, audit and system status. Pause/resume preserves deadlines; sunset drains without enrolling; completion/archive rejects active or pending work.
- Go 1.26.6 produced 12 reachable standard-library findings. Updated to 1.26.10 and x/net v0.62.0, then reran scans: govulncheck reports 0 reachable vulnerabilities; npm audit reports 0. Initial web container scan found HIGH/CRITICAL OS issues; updated API Alpine and stable NGINX base images. Both final application images report 0 HIGH/CRITICAL findings with Trivy v0.76.2.
- CI now runs dependency and image scans. Local actionlint passed; no remote is configured, so hosted CI has not run. Retention/erasure operating policy and trace/secret boundaries are documented in [SECURITY.md](SECURITY.md).
- Verified: all quality/unit and package-isolated PostgreSQL integration tests; cancellation against a stale claim, recoverable attribute failure without duplicate increment, last-Admin protection, publish gates and atomic audit; 24/24 desktop/mobile browser tests; actual Docker worker restart recovery passed.

## Performance and UX follow-up

- `5c3b57d` records loaded API probes and PostgreSQL statement statistics; `9db724e` fixes due/outbox claim access order; `41d2152` removes redundant enrollment history probes and narrows lease-fence lookup to job identity.
- The [performance report](PERFORMANCE.md) compares a clean-tree instrumented baseline, an intermediate queue-only run and two optimized 10,000-customer repetitions, with raw artifacts and explicit sampling/environment limits. Mock-send drain from the common timer deadline fell from 176.7s to 87.5–89.5s; timer-step p95 lateness from 15.75s to 9.25–9.41s. Rate-limited ingestion p95 did not improve and is reported separately.
- `a2843c9` redesigns the builder/dashboard/reporting controls; `a27503b` preserves URL pages and journey tabs on refresh/back/forward; `e883610` simplifies typography; `262ac48` compacts sidebar navigation to 16px icons, 13px text and 34px rows.
- Verified after runtime changes: `make check`, all package-isolated PostgreSQL integration tests (including active/once/cooldown reentry, paused/locked queue selection, races and lease fencing), 32/32 desktop/mobile browser tests, Docker API/worker/web rebuild/readiness and actual worker restart recovery. Every benchmark run retained 10,000 logical sends, zero duplicate actions and zero failed executions.
- No production indexes, extra connection capacity, Redis dependency or paid/live LLM calls were added. The final demo/deployment/documentation phase remains in progress; benchmark completion alone does not close it.

## UX handoff and EUR recovery example

- `bce743a` adapts the provided handoff to the existing Vue Flow editor: grouped/searchable node library, collapsible panels, compact draft/version header and duration units backed by the existing seconds schema. Source handoff and overview are preserved in `docs/design`.
- `a347f99` adds the €50+ EUR recovery template, editable AI brief (no automatic model call), five synthetic A–E sandbox scenarios, personalized email/SMS and run-scoped inline Test Inbox. See [the example contract](design/abandoned-cart-eur.md) for seven-day reentry semantics, formatted amounts, current-cart purchase state and sample discount code. Existing demo workspaces are not reset.
- Publish review uses existing server gates and revision protection. Monitor and inbox replace primary raw payload views with transition, eligibility and attempt summaries; advanced payloads remain available. Builder, Test Lab, publish and monitor keep explicit draft/live context. Analytics labels branched node reach without suggesting a linear funnel.
- Verified: `make check`, all package-isolated PostgreSQL integration tests, and 38/38 desktop/mobile browser tests, including A–E simulation, consent paths, sandbox isolation, publish acknowledgement, tab state retention and deliberately delayed monitor responses. Live Claude/OpenAI calls remain deferred. No runtime scheduling or analytics projection changes, new indexes or new polling endpoints were introduced for this UI work.

## Reliability and navigation follow-up

- Session restoration distinguishes an anonymous 401 from temporary server/network errors. Temporary failure preserves the URL and offers an explicit retry without clearing cookies or showing sign-in. Verified with `make check` and six desktop/mobile navigation checks, including consecutive 503/network failures followed by recovery into the original version tab.
- Production deployment and live LLM calls remain outside this user-authorized slice.

- `c700148` preserves session deep links across temporary access failures. Journey detail retries also preserve the route rather than treating 503/network failures as missing journeys.
- `e237998`, `68bd2da` and `a26b283` add edge insertion with explicit branch selection, one-step undo, a shared lazy activity drawer, customer/event/execution/message links and persisted native monitor selections. New bounded metadata endpoints use existing customer history and new workspace/event indexes; message bodies are not loaded by list queries.
- `446d392` updates libxslt in the existing non-root web runtime. Local scans of API/web report zero HIGH/CRITICAL findings; this corrects the previous hosted CI's image-scan failure, whose quality, integration, browser and recovery checks passed.
- Verified current functional code with `make check`, all isolated PostgreSQL integration packages and 44/44 desktop/mobile E2E tests, including explicit branch selection, Escape, reload/back/forward, stale selections and cross-workspace activity isolation. Remote CI and final performance comparisons are recorded separately after execution. User explicitly requested no deployment.

- Explicit user follow-up: migrated builder/monitor/custom edges/nodes to pinned `@xyflow/vue` 2.0.0-next.21, removed old add-ons, adopted provider-scoped state and immutable updates, and prevented native HTML dragging from intercepting node connections. Widened Elements, made it a mobile overlay, hid the unselected inspector, fixed blank zoom icons and kept canvas light under dark OS preferences. `make check` and 46/46 current desktop/mobile tests passed. See [the latest design direction](design/modern-saas-direction.md) for the full staged plan and remaining modules.

## Product showcase and CI evidence

- Research after `f55c79a`: [next queue experiments](QUEUE-NEXT-RESEARCH.md) compare official PostgreSQL/Camunda/River guidance with current SQL costs. An isolated PostgreSQL 100,000-row projector-delete probe, six alternating repetitions per variant, shows a full-table scan versus a targeted 1,000-row PK lookup (median 18.722ms versus 6.376ms). Raw SQL/plans are preserved; all probe deletes roll back. This is not a production/runtime change or an end-to-end speedup claim. Next experiments are targeted projector deletion, bounded scheduler candidate pages and capacity-threshold refill, each against a fresh Go 1.26.11 baseline.

- Hosted audit at `c57cd43` found GO-2026-6859 in the pinned Go 1.26.10 standard library (`net/http` HTTP/1 crash). Toolchain minimum and Docker builder are updated to Go 1.26.11, whose official release notes include the fix. Local `make audit` then reports zero reachable Go vulnerabilities and zero npm findings; Trivy API/web scans report zero HIGH/CRITICAL findings. The scanner separately reports an unimported `x/crypto/openpgp` module advisory; no imported package/reachable finding remains. Historical performance artifacts retain their actual Go 1.26.10 source identity.

- Ordered performance follow-up: `80329e6` seeks within bounded scheduler drains; `5c1b4d6` batches ordered runtime lock reads while keeping lease time evaluation after locks, with sanitized batch telemetry; `98cb892` adds isolated spread-deadline workloads. See [actual measurements](QUEUE-FOLLOWUP.md). New correctness cases cover locked/backdated/tied outbox rows, index seeking, pause during lock waits and lease expiry during lock waits. `make check`, all package-isolated PostgreSQL integration tests, 46/46 browser tests, actual worker restart and durable telemetry/alert checks passed locally. Live LLM calls and deployment remain deferred.

- The [scheduler follow-up](SCHEDULER-LOCKS.md) records all smaller-batch/per-action experiments and two clean `5e7a9d1` repetitions with 10,000 active executions. Current completion is 48.351–49.414s, mock-provider drain 71.419–72.573s, zero duplicate/failed actions and zero sampled blocked timer-phase backends. Drain remains slower than the original 59.397–60.417s baseline; claim/commit overhead and synchronized timer lateness remain explicit targets, not completed optimization claims.

- `bb457b5` adds an explicit, loopback-only `make demo`: 54 fictional profiles, 72 actual local enrollments, lifecycle examples and five isolated EUR recovery scenarios. Two repeated completed seeds retained fixture identities and counts. Live actions use mock SMS; email previews use sandbox records. No paid LLM or external email/SMS request is made.
- `6617f9e` gives Inbox filters stable accessible names. Actual screenshot capture produced ten 3200×2100 PNGs, with production entries unchanged at 72 before/after the additional isolated test run. See [the gallery](images/README.md) and [reproduction instructions](SHOWCASE.md). README now presents the product with real application screenshots; native setup details moved to [local development](LOCAL-DEVELOPMENT.md).
- Hosted [CI run 42938864757](https://github.com/negativexq/customer-journey-composer/actions/runs/42938864757), source `3d43888`, succeeded: quality, isolated PostgreSQL integration, 46 browser tests, actual worker recovery and dependency/image security. This is revision-specific evidence, not a claim about later commits. Local durable telemetry/Prometheus/alert checks also passed after restoring the observability Compose overrides.
- `9d69ca3` keeps monitor filters in a wrapping toolbar so the canvas stays visible and aligns Inbox filters to their content instead of stretching them with the message preview. The final local `make check`, 46/46 desktop/mobile browser tests, Git diff whitespace check and history secret scan passed. The capture helper waits for application loading state and fonts instead of network idleness, which embedded message previews may never reach.

## AI workflow acceptance audit — 2026-11-11

[The focused audit](AI-WORKFLOW-AUDIT.md) identifies an insufficient publication
test gate, missing semantic acceptance checks, ambiguous AI exit codes, delayed
no-channel exit and discarded provider usage. A GET-only preflight probe returned
ready=true for the exact generated workflow after 4 passing and 6 failing cases;
no publication was attempted. Runtime/analytics/sandbox behavior is unchanged.
Required-suite gating and typed AI outcomes are the first recommended corrections;
no new generation, optimization or corrective code was introduced by the audit.

## AI audit corrections — 2026-11-12

Implemented required acceptance gating, canonical AI exit codes, a reviewed EUR
behavior contract with profile/consent/timing checks, and sanitized provider usage
accounting. The reviewed correction passes 10/10; the unchanged original remains
4/10 and is now blocked from publishing. [Evidence and remaining live-test limits](AI-REMEDIATION.md).

## Fresh paid Luna follow-up — 2026-11-12

The explicitly requested live Composer action made two provider attempts: the
first failed application validation; bounded repair produced the accepted second
candidate. Actual provider usage totals 7,094 input + 6,657 output = 13,751 tokens
(reasoning is included in output). Compose HTTP duration is 47.169s. The fresh
17-node/18-edge proposal was applied unchanged and passes all ten customer/event
scenarios. It remains Draft, with no real sends or live executions. Exact timing
boundaries, initial unpaid UI setup retry, raw outputs, screenshots and the
first-rejection telemetry limitation are recorded in
[the chronological report](validation/luna-live-2026-11-12/README.md).

## New live Luna onboarding brief — 2026-11-12

One user-authorized Composer action produced a fresh 19-node/19-edge onboarding
workflow on its first provider attempt, with no manual edits or repair request.
The exact proposal passes ten sandbox scenarios, including activation events,
profile-only changes, consent branches and once-only enrollment. Actual usage:
1,954 input + 4,141 output = 6,095 tokens; Composer round trip 38.219s; full
browser harness including review 53.763s. General workflow mode was selected,
so this is explicit scenario evidence rather than an onboarding behavior contract.
It remains Draft with no real sends, live customers/executions or versions.
[Brief, output, scenarios, screenshots and timing boundaries](validation/luna-onboarding-2026-11-12/README.md).

## Generation feedback — 2026-11-12

Implemented visible canvas/panel processing feedback, elapsed time, cancellation,
brief retention and real server lifecycle streaming while preserving JSON API
compatibility and human Apply/Publish boundaries. Both providers use shared
progress hooks; no extra model request is introduced. Local `make check` passes;
8/8 desktop/mobile AI browser cases pass with mocked streams, including a virtual
45-second wait, cancellation and failure. Go race tests cover real early HTTP
flush, disconnect cancellation and repair boundaries for both provider names.
A full disk initially prevented verification; clearing the reproducible Go build
cache allowed the checks to pass. No paid provider call was made for this slice.

## Resumable creation workspace — 2026-11-12

Replaced the creation popup with Details → Method → Brief → Review → Builder.
Migration 0020 stores brief/proposal state separately from list/analytics paths;
revisions fence autosave and atomic approval. Brief/Review reopen at the saved
step; interrupted generation does not automatically resubmit. Four examples,
single submitted-brief display, real generation feedback, box-and-arrow review,
correction briefs and approval preparation are implemented. Approval opens an
editable Draft and does not publish. `make check`, complete PostgreSQL integration
suites and all 60 desktop/mobile E2E cases pass locally. Screenshots were inspected;
no new paid call was made. [Behavior, persistence and evidence](design/creation-workspace.md).

## Canvas readability and final creation verification — 2026-11-12

Readable trigger-centered opening, source branch labels, directional connections,
selection focus, connected-step jumps and optional short shared-exit references
are implemented without modifying workflow semantics. Grouped rules and wait
timeouts are visible; template variables use readable summary labels. Auto layout
measures rendered cards and is undoable. Final `make check` and all 64 desktop/mobile
E2E cases pass locally, including graph preservation and restore-failure safety.
The full PostgreSQL integration suite passed for migration 0020 and the creation
API before this frontend-only follow-up. [Screenshots and limits](design/creation-workspace.md).

## Journey-aware test inputs and business briefs — 2026-11-12

Test Lab prepares a synthetic profile and entry event from the selected draft or
immutable version, including referenced condition and message fields. It offers
structural progression, global-exit and consent checks for human review. Suggested
checks start exploratory; saving a suggested acceptance case requires explicit
review. A mismatched entry event is flagged and requires intentional negative-test
selection. Existing authored inputs and saved scenario snapshots are preserved;
canonical EUR recovery cases remain available. No production profile is changed.

Brief examples now use business language rather than event/attribute identifiers.
This does not implement a workspace data catalog: grounded mapping to observed
workspace events and attributes, ambiguity resolution and friendly field labels
remain follow-up work. Structural test suggestions do not prove brief compliance.

Validation: `make check` (including Go race tests and six script tests),
`git diff --check` and all 66 desktop/mobile Playwright cases passed locally.
Browser coverage includes onboarding-specific inputs, mismatched-event feedback,
explicit suggested-case review and a passing isolated global-exit run. No paid
provider call, external deployment, schema change or production query was added.

## Workspace semantic layer — 2026-11-12

Migration 0021 adds a workspace-scoped design-time data catalog. Settings supports
business labels, aliases, meanings, scalar types, disabling definitions and explicit
bounded discovery of recent event/profile schema. Discovery persists names/types,
not customer values, and preserves curated metadata. Both Claude and OpenAI use
server-loaded enabled definitions through the shared composer; an empty event
catalog is rejected before provider calls. Derived `semantic.v1` reports identify
missing/disabled references, ambiguous labels/aliases and type conflicts.

Creation Review and the builder AI panel expose readable data mappings with
collapsible technical details. Explicit choices rewrite rules, event names,
variables and fallbacks together; conflicting merged fallbacks are rejected.
Creation stores choices and resumes them after refresh; server approval recomputes
grounding and blocks unresolved references. Scratch/manual authoring and immutable
published workflows retain their existing behavior. No catalog query or AI call
was added to ingestion, runtime, delivery or analytics. Workspace catalog writes
use `FOR NO KEY UPDATE` to preserve runtime foreign-key lock compatibility.

Local validation: `make check`, all 70 desktop/mobile browser cases and the full
isolated PostgreSQL integration suite passed. The first full integration run hit
the existing worker test's one-turn completion assertion under concurrent load;
the entire suite passed on a repeat without that load. The dedicated catalog/API
suite also passed with workspace/Viewer guards, discovery value exclusion,
curation preservation, approval blocking and foreign-key lock compatibility.
Screenshots were inspected. Provider tests use deterministic fixtures for both
provider names; no paid provider call or external deployment was made.
[Setup, API, performance bounds and deliberate limits](SEMANTIC-LAYER.md).

## Authorized five-brief paid audit — 2026-11-12

After explicit user authorization, five fresh OpenAI gpt-6-luna generations used
Turkish business briefs and workspace catalogs. All produced valid graphs; four
passed grounding and opened isolated Drafts, while the ambiguous fifth stayed
blocked. Seven provider attempts consumed 21,251 input + 19,900 output = 41,151
reported tokens. The generation pipeline took 182.647 seconds. No live customer,
execution, delivery or published version was created in these test workspaces.

Unmodified workflow replays passed 22/29 independently specified sandbox checks.
Observed failures: refund global exit incorrectly counts conversion; cart profile
goal checks are absent before sends; a trial message lacks a fallback; a numeric
threshold is approximate; and email opt-out cuts off an allowed SMS path. A
separate no-LLM probe showed that confirming the fifth proposal's ambiguity allows
approval even though a missing loyalty requirement was omitted as warning-only.
These defects remain open; grounding is not proof of business completeness.
[Full report and recorded fixtures](reports/semantic-live-five-2026-11-12.md).

## Journey outcome correctness — 2026-11-12

Global exits now accept an explicit outcome. Production and virtual-time runtime
share the same resolution: missing outcome retains legacy `converted`; an explicit
refund/cancellation can stop without conversion. Published definitions are not
rewritten. Builder preserves existing exit filters and exposes the outcome.
Composer v3 adds this outcome to both providers' structured schema; the v2 artifact
remains available. `make check` and the isolated PostgreSQL runtime suite passed,
including cancellation during a wait and cancellation of pending jobs. No paid
AI call or external deployment was made.

## Journey breakdown projections and scoped APIs — 2026-11-12

Migration 0022 adds asynchronous entry-cohort snapshots for executions, node visits,
branch transitions and logical message outcomes/reasons. The projector coalesces
per-source snapshots and atomically replaces prior contributions; retries do not
inflate entries or logical sends. A bounded cursor backfill gives historical data
an explicit incomplete state. Baselines cannot overwrite newer transitions and
never lock runtime source rows. Exact period unique-customer counts use a narrow
membership table rather than adding daily distinct counts. Existing lifetime APIs
remain compatible. Projection heartbeat is separate from source activity.

New journey insights and message drilldowns share version and entry-cohort scope;
execution lists accept version/node/branch/current-position/outcome/time filters
and retain keyset pagination. Paused journeys are excluded from overdue-wait
counts. PostgreSQL API/analytics integration tests passed, including reconciliation,
version/workspace isolation, cohort boundaries, reason removal after retry and
late-baseline idempotency. `make check` passed. No throughput benchmark is claimed
for this new projection path; no paid AI call was made.

## Live polling read cost and scope guards — 2026-11-12

Live polling can omit the exact historical distinct-customer calculation; Analytics
still requests it. Overdue reads start from due scheduled work with a stable query
timestamp, backed by migration 0023's workspace/due index. All-version summaries
exclude node/edge identities that would merge reused IDs across versions. API
integration tests cover these read modes and verify that a stale baseline cannot
restore an old blocked state over an accepted message. `make check`, the full
PostgreSQL suite and the follow-up API suite passed locally. No larger-load latency
or throughput result is claimed for this change.

## Journey-level Live Monitor and Analytics — 2026-11-12

A shared readable metrics canvas now powers both journey detail views. Reka UI
Select/Tabs provide scope and step navigation with keyboard behavior. Node panels
show Summary/People/Messages; branch and decision-reason counts lead to filtered,
paged records and the customer's path/timeline in the adjacent pane. Version and
entry period are coherent across sections and persist for the session. Polling
preserves the viewport, stops when inactive/hidden and omits historical distinct
membership; Analytics refreshes explicitly. Graph-order tables include zero-entry
nodes. Global exits are annotated at the node they left. Wrong-journey execution
and wrong-execution message inspection are guarded. Existing version snapshots
retain legacy exit defaults; new cancellation outcomes are explicit.

`make check`, the full PostgreSQL integration suite, the follow-up scoped API suite
and all 72 desktop/mobile browser cases passed. Screenshots were inspected; SQL
plans were sampled on the existing demo and documented as small local samples,
not capacity/p95 results. Audience dimensions and channel conversion attribution
remain outside this slice. No paid AI call or external deployment was made.
[Behavior, screenshots and limitations](design/journey-observability-implementation.md).

### Journey observation load validation — 2026-11-12

The clean `81f3a92` benchmark runner passed an isolated 10,000-active-execution
run after its 100-customer smoke check and `make check`. At 300 offered events/sec,
all 10,000 entries, completions, branch transitions, distinct customers and
logical/provider-accepted SMS counts reconciled. There were zero failed
executions, duplicate logical actions or loaded HTTP failures. During the common
timer burst live/Analytics p95 were 49.85/54.83 ms. The projection queue peaked at
34,320 rows and 28.79 seconds oldest age, then drained within the one-second
polling interval after delivery completion. This identifies queue drain cost and
common-branch people lookup as follow-ups; it does not claim saturation capacity
or a controlled improvement against older runs.
[Method, raw data, query plans and limits](JOURNEY-OBSERVABILITY-PERFORMANCE.md).

### Fresh live brief evaluation — 2026-11-12

A user-authorized paid `gpt-6-luna` request against composer.v3 generated an
unmodified 16-node/20-edge cart journey in the local demo workspace. Existing
workspace metadata was explicitly discovered/labeled before generation; the brief
used business language. One attempt took 33.90 seconds and reported 3,489 input
plus 3,367 output tokens. Ten of eleven sandbox cases passed. The 49.995-EUR case
reproduced the model's `>49.99` approximation of a >=50 requirement and failed;
publication was correctly blocked by that failed acceptance run. The draft and
sandbox evidence remain available; there is no published version, live execution
or new live delivery. No graph patch or additional paid repair was made.
[Full findings and recorded artifacts](reports/live-brief-cart-2026-11-12.md).

### Ten additional sandbox edges — 2026-11-12

Ten new scenarios on the unchanged live-generated cart draft passed 9/10, with no
new LLM/provider calls. Cases covered duplicate entry, cooldown at seven days
minus one second and exactly seven days, equal event/timer timestamps, consent
revoked between retry attempts, missing personalization, wrong currency, missing
email contact, an unrelated-cart purchase and ambiguous provider 408. The new
failure is a purchase from a different cart ending this journey as converted,
reflecting the definition's unscoped global exit. Together with the prior amount
boundary, two required cases block publication. Missing-value copy repetition
and UNKNOWN detail hidden behind a FAILED inbox label were also recorded. No
production workflow, execution or delivery changed. Case 10's external assertion
field was corrected via persisted readback; this was a harness correction, not a
runtime fix. [Evidence and limits](reports/live-cart-edges-2026-11-12.md).

### Ten distinct journey audit — 2026-11-12

Before the requested correlation fix, ten fresh paid business briefs generated ten
unmodified composer.v3 drafts in separate fictional workspaces. Fifty sandbox
cases ran: 41 passed, nine failed, zero request/execution errors. Six journey types
passed 5/5; webinar, payment recovery, loyalty and lead qualification exposed
channel, terminal outcome, unsupported invoice correlation and threshold gaps.
Ten requests used twelve provider attempts (two bounded repairs), totaling 70,076
reported input/output tokens and 324.58 seconds generation time. No version was
published and no live customer/execution/delivery was created. The correlation
fix is still pending after this audit; these results do not label it implemented.
[Evidence and limitations](reports/varied-journeys-2026-11-12.md).

### Explicit entry-object goal correlation — implemented 2026-11-12

Added optional typed correlation between entry-event and incoming goal-event
identifiers, shared by runtime/simulator and enforced for waiters of that same
goal event. Missing/empty/incompatible IDs fail closed. Original entry properties
persist in sandbox state; production conditionally reads the existing indexed
entry event. Legacy unconfigured versions keep their behavior. The editor exposes
business-labeled scope/identifier selectors; semantic references validate both
event scopes and compatible types; both provider contracts use composer.v4.
Historical schemas/paid artifacts remain unchanged. No migration or new service.

`make check`, isolated runtime/API PostgreSQL packages and all 74 desktop/mobile
cases passed. Fifteen deterministic cart/invoice replay cases passed on separate
reviewed corrections, including all historical extra inbox checks. One initial
build-overlapped legacy lease-recovery test assertion failed before a fresh
isolated package rerun passed; it was not suppressed. No new paid call after the
ten-journey audit, publication, live execution or delivery was made by the replay.
Other channel/outcome/threshold/canvas findings remain open. [Evidence and limits](ENTRY-CORRELATION.md).

### Generation mismatch root-cause inspection — 2026-11-12

Read-only code/fixture inspection plus a temporary unpaid Composer probe confirmed
that wrong channel, wrong completion outcome and approximate numeric threshold
candidates pass generic validation unchanged without an acceptance contract.
Exact >= is representable with existing OR rules, although no direct inclusive
operator exists. Fallback normalization does not rewrite channel/outcome/rules;
layout initially trusts stored model coordinates. No behavior fix is claimed by
this audit. The temporary test was removed; probe source/output remain as evidence.
[Root causes, limits and scoped correction order](reports/generation-rootcause-2026-11-12.md).

### Independent reviewed expectations and layout — 2026-11-12

Added composer.v5 exact inclusive numeric comparisons and independent human-confirmed
channel/outcome/time/eligibility checks for general AI graph actions. The server
checks them with bounded pure simulation, repairs at most once and rejects remaining
mismatches. Creation approval, Apply, validation and publication enforce saved
expectations. Synthetic profile values are excluded from provider requests.
Creation saves partial expectations; updates require confirmation. Newly generated
graphs receive deterministic layout; modifications preserve existing node positions.

Recorded original webinar/loyalty/lead outputs fail the new checks, while separate
reviewed corrections pass. Original paid audit results remain unchanged. Final `make check`, isolated PostgreSQL HTTP integration and all 76 desktop/mobile
browser cases passed; local Docker services were rebuilt and ready. No new
paid request or external deployment. [Behavior, limits and verification](BEHAVIOR-CHECKS.md).

### Authorized live composer.v5 measurement — 2026-11-12

Fresh paid OpenAI gpt-6-luna generation of webinar, loyalty, lead, invoice recovery
and activation journeys passed 25/25 unchanged sandbox cases (previously 16/25 for
these five briefs). Eight provider attempts include three bounded automatic repairs;
usage 26,709 input + 20,863 output = 47,572 tokens. Generation totaled 155.92 s;
complete audit wall time 183.51 s. Final proposals were inspected, approved only as
Drafts and not manually edited. All five workspaces had zero published versions,
live executions, customers or delivery jobs. This is one targeted local run;
remaining catalog scope limitations are documented. [Evidence and limits](reports/reviewed-generation-2026-11-12.md).

### Five additional live journey types — 2026-11-12

User-authorized fresh stock, referral, support, membership and course briefs yielded
15 PASS / 5 FAIL / 5 BLOCKED out of 25 planned cases. Four proposals generated;
support was rejected after both validation attempts. Eight provider completions
reported 28,784 input + 20,795 output = 49,579 tokens, including rejected generation.
Generation totaled 161.87 s; full wall time 202.89 s. Stock omitted email on positive
paths; referral used the wrong ineligible outcome. Their failing Test Lab cases
blocked publication. Membership/course passed 5/5 each. Independent single-example
coverage and missing rejection diagnostics remain open; no application fix claimed.
All five workspaces had zero published versions, live executions, customers or
jobs by actual database readback. [Evidence and limitations](reports/five-new-journeys-2026-11-12.md).

### Shared scoped business contract v2 — 2026-11-13

Replaced ambiguous global interpretation of representative counts with a versioned
human-owned clause contract: initial predicates, channel consent, timed events,
identity relationships and per-situation outcomes. Alternative clauses use the
same validator for arbitrary supported journey types; no new named journey/audit
special cases. V2 eligibility explicitly requires the rejected outcome and no
attribute mutation. Contradictory identical examples fail before a provider call.
Private examples/payloads stay local; both adapters receive scoped public rules.
Rejected attempts retain bounded diagnostic category counts rather than private
candidate/message data. New UI forms use v2; v1 requires explicit upgrade and
confirmation. No migration or additional runtime/analytics query. Historical paid
failures remain unchanged; no fresh paid call in this correction.
Final `make check`, isolated HTTP/PostgreSQL integration (85.999 s), all 78
browser cases and local Docker readiness passed. Scoped form screenshot inspected.
[Contract, compatibility, verification and limits](BEHAVIOR-CONTRACT-V2.md).

### Authorized scoped-contract live measurement — 2026-11-13

On source b70cbe1, five fresh OpenAI gpt-6-luna generations with human-reviewed v2
contracts passed the prior 25 external cases unchanged: 25 PASS / 0 FAIL / 0 BLOCKED,
versus previous 15 PASS / 5 FAIL / 5 BLOCKED. Seventeen business clauses were defined
before generation; duplicate events, profile-only changes, cap and consent variants
were also tested outside those fixtures. No per-journey production logic or manual
graph changes. Eight attempts used 30,249 input + 19,345 output = 49,594 tokens;
generation 174.17 s, total audit 216.34 s. Three initial candidates still required
workflow repair (recorded category counts 2/1/2). All five workspaces had zero live
records/jobs/versions by database readback. No publication or deployment.
[Evidence, comparison and remaining limits](reports/scoped-contract-live-2026-11-13.md).

### Product documentation and screenshot refresh — 2026-11-13

Refreshed 21 native 3200 × 2100 captures from the current local UI: 13 main product/
observation views and eight creation/review/canvas views. Creation replays the
recorded passing stock output with its v2 contract through local SSE; no new paid
call or publication. Fixed the capture tool's optional historical Inbox run
assumption by selecting its newly created sandbox run. Production entries remained
72 before/after capture. README leads with six product capabilities and the create → test → publish → observe
flow, followed by current 25/25 live evidence and 78-case browser verification; older audits remain linked
from the validation index. Capture commands, manifests and gallery notes are current.

### Clean Northstar showcase — 2026-11-13

At the user's request, the cluttered local demo was reset into a fresh workspace.
Prior journeys, executions and immutable versions remain archived; old executions
were cancelled and old access revoked. The prefilled login now opens Northstar
Commerce with one newly authored Abandoned Cart Recovery journey and 14 fictional
customers. One shared recipe supplies English copy, profiles, scoped obligations,
Mailpit email, mock SMS and all creation/product captures. Published v1 uses 5s/30s
real waits; the editable draft and five passing saved scenarios use 1h/24h. Capture
refuses additional journeys and uses current-draft regression evidence. No paid
LLM call or external deployment. Fixed fractional-hour contract inputs and covered
the real form submission; demo login verification now uses the signed-in identity
rather than a hardcoded display name. [Setup and capture details](SHOWCASE.md).

Verification for the clean showcase: `make check` passed; all 21 capture paths,
PNG dimensions/manifests and documentation links validated. API readback confirms
one Active journey, 14 customers/executions, five saved scenarios and a ready
publish gate. Capturing preserved the 14 live entry count. The full browser run
passed 76 cases; two old demo-name assertions were updated to use the successful
login identity and then passed 2/2 in a focused rerun. Fractional contract duration
submission is included in the passing scoped-contract browser coverage.

### Visible brief-to-journey product walkthrough — 2026-11-13

README now shows inline brief, generation progress, review diagram and editable
canvas screens. At the user's explicit request, the eight creation images were
replaced with a real OpenAI `gpt-6-luna` generation, not replay. The captured
request used 11,320 tokens and 40.9s including one automatic repair. Human approval
applied it to Draft; no publication or delivery occurred. The operational demo
remains separate. The capture script has an explicit paid opt-in and observes a
cloned stream without replacing the provider response. Earlier failed generation
and capture-tool failures are disclosed in the [capture report](reports/live-creation-capture-2026-11-13.md).
