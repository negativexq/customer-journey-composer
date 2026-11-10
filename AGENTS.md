# Repository working agreement

Read `docs/PRD.md`, `docs/IMPLEMENTATION.md` and the relevant architecture notes before making changes.

- Implement the twelve phases in order; update the tracker with actual evidence. Do not label planned functionality as implemented.
- Vue 3 + TypeScript + Vue Flow frontend; Go backend; PostgreSQL source of truth. Do not introduce Redis for durable execution state.
- AI must support Anthropic Claude and OpenAI via a shared provider interface and versioned schema. Live LLM tests are deferred by the user. Deterministic unit/integration tests must not call paid providers.
- AI proposes; humans apply and publish; the runtime executes deterministically.
- Preserve workspace boundaries, immutable published versions, idempotent logical actions and isolated sandbox data. Use database constraints for concurrency invariants.
- Commit each independent working slice with an English Conventional Commit. Supported types: feat, fix, test, refactor, perf, docs, ci, build, chore. Avoid vague messages and unrelated changes.
- Before a code commit run `make fmt-check`, `make lint`, `make typecheck`, `make test` and `git diff --check`. At a phase boundary run applicable integration/E2E tests and inspect Git status.
- Report completed work, changed modules, actual checks/results, real commit hash and next work. Never claim remote CI or benchmark results without running them.
- Never commit `.env`, secrets, credentials or personal data. Optional provider keys belong only in the server environment.
