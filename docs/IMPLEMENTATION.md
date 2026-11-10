# Implementation tracker

Source of truth for product scope: [PRD.md](PRD.md).

| Phase | Status | Evidence / remaining work |
| --- | --- | --- |
| 1. Repository & Foundation | In progress | Empty repository inspected; initial requirements captured. |
| 2. Authentication & Journey Management | Pending | Depends on phase 1. |
| 3. Visual Canvas & Validation | Pending | Vue Flow required. |
| 4. Customer & Event Infrastructure | Pending | Deduplication and workspace isolation required. |
| 5. Persistent Runtime | Pending | PostgreSQL durability and concurrency invariants required. |
| 6. Email & SMS | Pending | Mailpit and mock SMS required. |
| 7. AI Journey Composer | Pending | Both Claude and OpenAI; common schema, validation, human approval. |
| 8. Preview & Test Lab | Pending | Shared deterministic runtime with controlled Clock. |
| 9. Test Inbox | Pending | Isolated persisted outputs and traces. |
| 10. Live Monitor & Analytics | Pending | Execution-backed metrics with explicit denominators. |
| 11. Lifecycle & Reliability | Pending | Race, recovery, version, security and observability tests. |
| 12. Final Product & Demo | Pending | E2E, measured benchmark, demo and deployment verification. |

## Accepted decisions

- Develop in the phase order above; make small, verified Conventional Commits.
- PostgreSQL is authoritative; no Redis dependency for durable state.
- AI output is a proposal, never an automatic save or publish.
- Both Anthropic Claude and OpenAI APIs must be supported behind the same provider boundary.
- Live LLM/provider evaluation is deferred at the user's request. No paid LLM request is needed for foundation work or deterministic validation tests.
- Report test execution and benchmark measurements honestly; pending work is not complete.

## Next slice

Finish phase 1: Vue/TypeScript shell, Go API, health/readiness and generated OpenAPI, PostgreSQL migration runner, local Docker stack, checks and CI. Then begin phase 2 with workspace authentication and persisted journey CRUD.
