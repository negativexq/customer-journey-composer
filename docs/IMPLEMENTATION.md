# Implementation tracker

Source of truth for product scope: [PRD.md](PRD.md).

| Phase                                  | Status                         | Evidence / remaining work                                                                                                                        |
| -------------------------------------- | ------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| 1. Repository & Foundation             | Complete                       | Vue/Go/Docker stack; PostgreSQL migrations; quality checks; integration and browser tests passed locally.                                        |
| 2. Authentication & Journey Management | Complete                       | Sessions, CSRF, RBAC, workspace isolation, revision-guarded drafts and management UI; database and 6 browser tests passed.                       |
| 3. Visual Canvas & Validation          | Complete                       | Typed schema, graph/rule validator, immutable snapshots and Vue Flow editor; database and 8 browser tests passed.                                |
| 4. Customer & Event Infrastructure     | Complete                       | Profiles, scoped API keys, idempotent single/batch ingestion and atomic enrollment; database and 10 browser tests passed.                        |
| 5. Persistent Runtime                  | Complete                       | Fenced worker, durable timers/waiters, global exit, shared pure decisions, outbox; race/recovery/version tests and actual Docker restart passed. |
| 6. Email & SMS                         | Complete                       | Safe rendering, channel studio, SMTP/Mailpit and mock SMS, durable deliveries/attempts, policy and retry tests; 14 browser tests passed.         |
| 7. AI Journey Composer                 | Complete (live tests deferred) | Shared schema, Claude/OpenAI adapters, bounded repair, diff/approval; local provider contracts and 18 browser tests passed.                      |
| 8. Preview & Test Lab                  | Complete                       | Virtual clock, snapshots, replay, event/attribute sequences, assertions and fault injection; 20 browser tests passed.                            |
| 9. Test Inbox                          | Complete                       | Three-panel inbox, scoped filters, payload/trace/attempts, replay comparison and actual test-send distinction; 20 browser tests.                 |
| 10. Live Monitor & Analytics           | Pending                        | Execution-backed metrics with explicit denominators.                                                                                             |
| 11. Lifecycle & Reliability            | Pending                        | Race, recovery, version, security and observability tests.                                                                                       |
| 12. Final Product & Demo               | Pending                        | E2E, measured benchmark, demo and deployment verification.                                                                                       |

## Accepted decisions

- Develop in the phase order above; make small, verified Conventional Commits.
- PostgreSQL is authoritative; no Redis dependency for durable state.
- AI output is a proposal, never an automatic save or publish.
- Both Anthropic Claude and OpenAI APIs must be supported behind the same provider boundary.
- Live LLM/provider evaluation is deferred at the user's request. No paid LLM request is needed for foundation work or deterministic validation tests.
- Report test execution and benchmark measurements honestly; pending work is not complete.

## Next slice

Phase 10: asynchronous analytics projections, bounded live monitoring and measured query-plan verification.

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
