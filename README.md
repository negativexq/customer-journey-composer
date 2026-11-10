# Customer Journey Composer

AI-powered customer journey orchestration: **AI-generated, human-approved, deterministically executed.**

Vue 3 / TypeScript frontend, Go API, PostgreSQL source of truth. The product includes a visual journey builder, persistent execution, messaging, simulation and observability as specified in the [MVP requirements](docs/PRD.md).

**Current scope:** phases 1–4: workspace authentication, journey management, Vue Flow editing/validation, immutable published versions, customer profiles and transactional event ingestion/enrollment. The persistent worker and delivery pipeline are next. [Implementation tracker](docs/IMPLEMENTATION.md).

## Quick start with Docker

Requires Docker Engine with Compose v2, Git and Node.js 24 LTS (Node is used to create the ignored local environment file).

```sh
git clone <your-repository-url>
cd customer-journey-composer
make up
```

For this existing checkout, simply run `make up`. It creates `.env` with a random database password, builds both applications, waits for PostgreSQL, applies migrations, then starts the API and frontend. No AI key is required.

| Service                       | Local address                          |
| ----------------------------- | -------------------------------------- |
| Web                           | http://localhost:5173                  |
| Go API liveness               | http://localhost:8090/healthz          |
| API database/schema readiness | http://localhost:8090/readyz           |
| Interactive API documentation | http://localhost:8090/api/docs         |
| Generated OpenAPI JSON        | http://localhost:8090/api/openapi.json |
| Mailpit inbox                 | http://localhost:8025                  |
| Mailpit SMTP                  | localhost:1025                         |
| PostgreSQL                    | localhost:54329                        |

All exposed ports bind to loopback. Change ports in the ignored `.env` if needed. The default native Vite proxy points to API port 8090; update `apps/web/vite.config.ts` if you change the native API address.

```sh
make down                 # Stop containers; preserve the PostgreSQL volume.
docker compose logs api   # Inspect structured API logs.
```

Create a workspace from the sign-in screen; no default credentials are seeded. The local stack is for development. Production TLS and the remaining reliability/security gates must be completed before public deployment.

## Native development

Requires Go 1.26, Node.js 24 LTS, npm and Docker Compose.

```sh
make setup
make install
make infra
make migrate
make dev-api              # Terminal 1: http://localhost:8090
make dev-web              # Terminal 2: http://localhost:5173
```

Stop a fully containerized stack with `make down` before running native services on the same ports.

## Verification

```sh
make fmt-check
make lint
make typecheck
make test
make test-integration
make build
make up
npx playwright install chromium
make test-e2e
git diff --check
```

`make test` runs Go tests with the race detector and repository script tests. `make test-integration` creates an isolated ephemeral PostgreSQL container with a random password and removes it afterward; it never uses the workspace database. It covers concurrent migrations, idempotent reapplication, checksums, database constraints and reconnection.

`make test-e2e` requires the running stack and Chromium. It checks desktop and mobile layouts, actual API/database readiness, generated OpenAPI, error recovery, JavaScript errors and horizontal overflow. Set `E2E_BASE_URL` to target another local stack. On Linux install the browser with `npx playwright install --with-deps chromium`.

## Repository layout

```text
apps/
  api/
    cmd/server/           Go HTTP API
    cmd/migrate/          Separate migration command
    internal/config/      Validated environment configuration
    internal/database/    PostgreSQL pool and embedded SQL migrations
    internal/httpapi/     Typed Huma endpoints and generated OpenAPI
  web/                    Vue 3 / TypeScript / Vite application
scripts/                  Setup, commit checks and isolated integration runner
tests/e2e/                Playwright desktop/mobile smoke tests
docs/                     Product plan, architecture and development status
```

## AI providers

Both **Anthropic Claude** and **OpenAI** are required in phase 7 behind one provider interface and shared, versioned workflow schema. Keys and model IDs will be configured on the server only. The `.env.example` reserves optional configuration names; adapters are not implemented in phase 1. Keep `AI_PROVIDER=disabled` for now.

Live LLM tests are deferred at the user's request. Unit, integration and simulation tests must be deterministic and must not need paid model requests. AI never publishes or applies workflow changes without human approval.

## Contribution protocol

Follow the phase order in [PRD.md](docs/PRD.md) and use verified, focused Conventional Commits:

```text
feat(runtime): add durable event wait scheduling
fix(delivery): prevent duplicate message jobs
```

Supported types: `feat`, `fix`, `test`, `refactor`, `perf`, `docs`, `ci`, `build`, `chore`. Before committing: `make check` and `git diff --check`. At each phase boundary also run the relevant integration/E2E tests and inspect `git status --short`.

Validate a commit range with `make commitlint RANGE=main..HEAD`. GitHub Actions runs the same checks plus Docker/browser smoke tests. No `.env`, credentials, personal data or provider keys may be committed.

See [API policies](docs/API.md) for authentication, API keys, draft revisions and event ordering/late-event rules.
