# Security and operations policy

## Identity and isolation

Sessions are random, hashed at rest, HttpOnly and SameSite=Strict, with CSRF checks on mutations. Use COOKIE_SECURE=true behind HTTPS. Authentication work is bounded by an eight-request semaphore. Public workspace signup is closed unless APP_ENV=development or PUBLIC_SIGNUP=true. Production owners are bootstrapped with the `admin` binary using ADMIN_EMAIL, ADMIN_PASSWORD and WORKSPACE_NAME from a secret manager/environment, not command-line password arguments. Admins create workspace users and assign Admin/Editor/Viewer roles. The last Admin cannot be demoted; concurrent role changes serialize on the workspace row.

Every resource query uses the authenticated workspace. Ingestion API keys are hashed, revocable and restricted to event POSTs; they cannot read customers, change journeys or invoke AI. Provider keys stay in the API process; SMTP credentials stay in the worker. Both AI providers only return proposals, never save/publish. Customer profiles/event payloads are not fetched for AI requests. Live LLM evaluation remains deferred.

The generated local demo login is intentionally public only in development. Production startup refuses an enabled demo account. The credential endpoint is no-store. Do not enable development mode on a public deployment. The local Compose ports bind to loopback.

## Bounded work and content

HTTP requests have operation deadlines; AI has its separate bounded budget. In-memory token buckets bound per-workspace interactive traffic (10/s, burst 200), ingestion (500/s, burst 1,000), AI (one token per six seconds, burst two), and authentication per direct network peer. Authentication is relaxed for local development. Buckets retain no raw peer identities, cap memory at 4,096 keys and expire inactive entries. These are per-process limits; deploy edge limits for a shared proxy or multiple API replicas. Workspace consent/frequency policies remain transactionally enforced across workers in PostgreSQL.

Message variables cannot execute code. HTML is sanitized and dynamic values escaped; previews use sandboxed iframes and restrictive CSP. Test recipients must be allowlisted. Sandbox simulations have no provider handle and never schedule live delivery jobs. Preview overrides never modify source customer profiles.

Application logs omit URLs/query strings, credentials, payloads, customer IDs, message bodies and raw database errors. Audit records contain actor/resource UUIDs, action names and bounded operational facts, never passwords, API keys, allowlisted addresses or message content. Publishing, lifecycle changes, execution cancel/recovery, user changes, messaging policy and API key changes persist their audit in the same transaction. Draft autosaves and read-only previews are not administrative audit events.

## Lifecycle and provider safety

Publishing requires graph/message validation and a finalized PASS from a full sandbox run matching the exact workflow hash. Named test snapshots cannot be substituted across workspaces. Missing personalization is blocked at runtime unless a fallback exists. Consent is checked at planning and dispatch; frequency reservations are workspace-wide and include pending/unknown logical sends. Provider settings are validated before publishing; no provider call occurs at publication.

Paused journeys accept no new enrollments and execute no new node/provider actions. Timer deadlines retain their original UTC time; overdue work becomes eligible on resume. Events may resolve waiters or global exit while paused, preserving customer events without sending messages. Resume restores the previous Active/Sunset policy. Sunset closes enrollment and drains existing work. Completion/archive require no active executions or pending outbox/delivery actions. In-flight provider calls cannot be recalled; cancel_requested records that boundary explicitly.

Execution cancellation fences claims and cancels pending jobs/waiters/outbox/deliveries. Manual runtime recovery is Admin-only, limited to three recoveries, preserves the immutable version and refuses a node with an existing logical action or another active enrollment. Prior retry counts are retained in the audit. Provider retries remain bounded and keep attempt history. UNKNOWN SMTP outcomes require operator/provider reconciliation; there is no blind manual or automatic resend button.

## Retention and erasure policy

Recommended operational windows are 30 days for sandbox inputs/messages, 90 days for raw events, 180 days for terminal execution/message history and 365 days for administrative audit metadata. Backups should expire within 30 days. These are documented operating defaults, not an automatic deletion job: this MVP does not silently delete data. Workspace owners must approve their retention window before a deployment enables maintenance cleanup.

Before removing event/execution history, preserve all rows referenced by active executions, durable waits, pending outbox/delivery jobs or unresolved UNKNOWN outcomes. Delete dependent detail rows in foreign-key order, in bounded batches outside interactive traffic. Preserve anonymous aggregate counters independently; deleting history must not subtract historical performance. Published versions remain immutable and referenced versions are retained.

A customer erasure request is handled by an authorized operator: stop new ingestion for that identity, cancel active executions, reconcile in-flight sends, then remove/redact contacts, attributes, consents, event properties, rendered bodies/personalization, step outputs, test scenario/run/profile copies and test messages that reference the source customer. Include metric queue/customer-identity projections and backups. Opaque runtime/aggregate facts may remain only after unlinking identifying keys. Archive is not erasure. No erasure endpoint claims to complete this multi-source operation in the MVP; deployments must use an audited maintenance procedure and verify the result across all stores. Static workflow content must not contain live customer PII.

## Telemetry and dependency checks

OpenTelemetry traces use route templates and opaque execution/delivery UUIDs, never raw routes, customer properties or message content. W3C trace context is persisted at enrollment and restored by workers/delivery jobs. Export is disabled when collector endpoints are blank; configured OTLP exports use bounded batches and timeouts. Endpoint URLs cannot embed credentials/query strings; OTLP headers can come from environment secrets. Use OTEL_TRACES_SAMPLER_ARG (0–1, default 0.1) and OTEL_SDK_DISABLED=true to control export. See the official [Go exporter documentation](https://opentelemetry.io/docs/languages/go/exporters/).

Run `make audit` for reachable Go vulnerabilities and npm advisories, and `make scan-images` after building images. CI includes these checks. A local check does not imply GitHub-hosted CI ran. Scan results are dated observations, not a permanent guarantee. Go is pinned to 1.26.11 after the hosted audit identified GO-2026-6859 in 1.26.10 (`net/http` HTTP/1 server crash); see the official [release history](https://go.dev/doc/devel/release#go1.26.0).
