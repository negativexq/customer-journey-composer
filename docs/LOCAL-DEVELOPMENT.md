# Local development

The product overview and populated demo are in the [README](../README.md). These commands operate on the local development stack; no public deployment is configured.

## Docker setup

Requires Docker with Compose v2, Git and Node.js 24+ (supported range: 24–26).

```sh
make up
make demo
```

`make up` generates an ignored `.env` with random local credentials, starts PostgreSQL, applies checksummed migrations, builds the API/worker/web and waits for readiness. No AI key is required. Demo login is prefilled in development. See [the showcase guide](SHOWCASE.md) for fixture boundaries and screenshot reproduction.

| Service           | Default local address                  |
| ----------------- | -------------------------------------- |
| Web               | http://localhost:5173                  |
| API liveness      | http://localhost:8090/healthz          |
| API readiness     | http://localhost:8090/readyz           |
| API docs          | http://localhost:8090/api/docs         |
| OpenAPI           | http://localhost:8090/api/openapi.json |
| Mailpit UI / SMTP | http://localhost:8025 / localhost:1025 |
| PostgreSQL        | localhost:54329                        |

Ports bind to loopback. Override local ports through ignored `.env`; update the native Vite proxy in `apps/web/vite.config.ts` if the API port changes.

```sh
make down                 # Preserve the PostgreSQL volume.
docker compose logs api   # Structured application logs.
```

## Native development

Requires Go 1.26.11+ in the supported 1.26 series, Node.js 24–26 and Docker.

```sh
make setup
make install
make infra
make migrate
make dev-api              # Terminal 1.
make dev-worker           # Terminal 2.
make dev-web              # Terminal 3.
```

Stop containers using the same ports before starting native services.

## Messaging and AI

SMTP values stay in server environment configuration. Native development uses `SMTP_HOST`/`SMTP_PORT`; Compose uses `SMTP_DOCKER_HOST`/`SMTP_SERVER_PORT`, defaulting to Mailpit. Configure sender/TLS/authentication as described in [security policies](SECURITY.md). In-app settings manage frequency caps and the test-recipient allowlist, not secrets.

Claude/OpenAI keys and model IDs are optional server settings; see [AI configuration](AI.md). The default provider is disabled. Live LLM evaluation is deferred, and deterministic tests do not call paid providers.

Generated demo credentials are exposed only by the development-only demo endpoint. Production mode refuses an enabled demo account. Production owners are bootstrapped with the admin binary using environment secrets; see [SECURITY.md](SECURITY.md).

## Verification

```sh
make check
make test-integration
make build
npx playwright install chromium
make test-e2e
make test-recovery
make audit
make scan-images
git diff --check
```

`make check` includes formatting, lint, typecheck, Go race tests and repository script tests. Integration tests create package-isolated ephemeral databases and never reset the development database. Browser tests create fictional workspaces against the running stack. Recovery testing stops/restarts only this project's worker.

Set `E2E_BASE_URL` for another local web port. Linux runners use `npx playwright install --with-deps chromium`.

## Telemetry

```sh
make observe-up
make test-telemetry
```

Use `make observe-up` when restarting with telemetry so the API/worker retain the observability Compose overrides. The base Compose configuration disables trace export by default.

[Journey operations dashboard](http://localhost:3005/d/journey-operations). [Signals, alerts and configuration](OBSERVABILITY.md).

## Repository

```text
apps/api/cmd/             API, worker, migration, admin and benchmark commands
apps/api/internal/        Runtime, delivery, policy, analytics and API modules
apps/web/                 Vue application and shared UI components
scripts/                  Setup, verification, demo and capture tools
tests/e2e/                Desktop/mobile browser coverage
docs/                     Product, architecture, measurements and screenshots
```

Use focused English Conventional Commits and run the checks required by [AGENTS.md](../AGENTS.md). Commit-range validation is available through `make commitlint RANGE=main..HEAD`.
