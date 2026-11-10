# Customer Journey Composer

AI-powered customer journey orchestration: **AI-generated, human-approved, deterministically executed.**

Vue 3 / TypeScript frontend, Go API, PostgreSQL source of truth. The product includes a visual journey builder, persistent execution, messaging, simulation and observability as specified in the [MVP requirements](docs/PRD.md).

**Implemented:** workspace authentication/RBAC, Vue Flow editing and immutable journey versions, idempotent event ingestion, durable runtime/timers/event waits/global exits, SMTP/Mailpit and mock SMS with consent/caps/retries, Claude/OpenAI proposals with human approval, isolated virtual-clock Test Lab/Test Inbox, live monitoring, projected analytics and audited lifecycle/recovery. Templates and generated local demo data are included. Live LLM evaluation remains deferred. Final demo/deployment artifacts are tracked in the [implementation tracker](docs/IMPLEMENTATION.md).

**Measured performance:** 10,000 active executions; queue/runtime optimizations reduce mock-send drain from 176.7s to 87.5–89.5s from a shared timer deadline. The [performance report](docs/PERFORMANCE.md) includes loaded API latency, PostgreSQL plans/costs, rate-limit effects, raw results and remaining opportunities.

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

The local development login comes prefilled with a generated demo account: click **Sign in**. Demo seeding is disabled in production; production signup is closed by default and the `admin` command bootstraps an owner using environment inputs. See [security and deployment policy](docs/SECURITY.md). This Compose stack binds to loopback and does not configure public TLS.

## Operational telemetry

`make observe-up` adds a local Prometheus/Grafana/Tempo/Collector stack. Open [Journey operations](http://localhost:3005/d/journey-operations). `make test-telemetry` validates metrics, alert rules and a durable API-to-provider trace. See [telemetry setup and signal semantics](docs/OBSERVABILITY.md).

## Native development

Requires Go 1.26.10 or newer in the supported 1.26 series, Node.js 24 LTS, npm and Docker Compose.

```sh
make setup
make install
make infra
make migrate
make dev-api              # Terminal 1: http://localhost:8090
make dev-worker           # Terminal 2: durable step processing
make dev-web              # Terminal 3: http://localhost:5173
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
make test-recovery
make benchmark
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

Both **Anthropic Claude** and **OpenAI** are implemented behind one provider interface and shared, versioned workflow schema. Keys and model IDs are configured on the server only. The `.env.example` reserves optional configuration names; adapters are implemented and covered by local contract tests. Keep `AI_PROVIDER=disabled` until live generation is requested; see [AI configuration](docs/AI.md).

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

`make test-recovery` verifies a real worker process restart against the local Docker stack, using a new isolated workspace. It stops and restarts only this project's worker.

SMTP settings stay in server environment variables. Native development uses `SMTP_HOST`/`SMTP_PORT`; Compose uses `SMTP_DOCKER_HOST`/`SMTP_SERVER_PORT` (defaults: `mailpit:1025`). Configure `SMTP_FROM`, optional `SMTP_USER`/`SMTP_PASSWORD`, and `SMTP_REQUIRE_TLS=true` when using an authenticated external SMTP server. In-app Settings manages caps and the explicit test-recipient allowlist; secrets are never returned to the browser.

Local demo login is prefilled automatically after `make up`: click **Sign in**. `make setup` generates its password into ignored `.env`; no fixed password is stored in Git. Demo provisioning and the no-store credential endpoint require both `APP_ENV=development` and `DEMO_ENABLED=true`. Set `DEMO_ENABLED=false` outside local development. Production mode refuses an enabled demo account.
