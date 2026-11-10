# Implementation tracker

Source of truth for product scope: [PRD.md](PRD.md).

| Phase                                  | Status   | Evidence / remaining work                                                                                                  |
| -------------------------------------- | -------- | -------------------------------------------------------------------------------------------------------------------------- |
| 1. Repository & Foundation             | Complete | Vue/Go/Docker stack; PostgreSQL migrations; quality checks; integration and browser tests passed locally.                  |
| 2. Authentication & Journey Management | Complete | Sessions, CSRF, RBAC, workspace isolation, revision-guarded drafts and management UI; database and 6 browser tests passed. |
| 3. Visual Canvas & Validation          | Complete | Typed schema, graph/rule validator, immutable snapshots and Vue Flow editor; database and 8 browser tests passed.          |
| 4. Customer & Event Infrastructure     | Pending  | Deduplication and workspace isolation required.                                                                            |
| 5. Persistent Runtime                  | Pending  | PostgreSQL durability and concurrency invariants required.                                                                 |
| 6. Email & SMS                         | Pending  | Mailpit and mock SMS required.                                                                                             |
| 7. AI Journey Composer                 | Pending  | Both Claude and OpenAI; common schema, validation, human approval.                                                         |
| 8. Preview & Test Lab                  | Pending  | Shared deterministic runtime with controlled Clock.                                                                        |
| 9. Test Inbox                          | Pending  | Isolated persisted outputs and traces.                                                                                     |
| 10. Live Monitor & Analytics           | Pending  | Execution-backed metrics with explicit denominators.                                                                       |
| 11. Lifecycle & Reliability            | Pending  | Race, recovery, version, security and observability tests.                                                                 |
| 12. Final Product & Demo               | Pending  | E2E, measured benchmark, demo and deployment verification.                                                                 |

## Accepted decisions

- Develop in the phase order above; make small, verified Conventional Commits.
- PostgreSQL is authoritative; no Redis dependency for durable state.
- AI output is a proposal, never an automatic save or publish.
- Both Anthropic Claude and OpenAI APIs must be supported behind the same provider boundary.
- Live LLM/provider evaluation is deferred at the user's request. No paid LLM request is needed for foundation work or deterministic validation tests.
- Report test execution and benchmark measurements honestly; pending work is not complete.

## Next slice

Phase 4: customer profiles, workspace API keys, validated single/batch event ingestion, deduplication, timelines and transactional enrollment.

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
