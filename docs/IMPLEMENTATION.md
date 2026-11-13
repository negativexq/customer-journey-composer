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
