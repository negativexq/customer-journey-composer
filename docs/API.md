# API usage and data policies

The running server generates OpenAPI at `/api/openapi.json` and interactive documentation at `/api/docs`. Only implemented endpoints appear there.

## Authentication and workspaces

Create a workspace with `POST /api/v1/auth/register` (`name`, `workspace`, `email`, `password`) or sign in with `POST /api/v1/auth/login` (`email`, `password`). Passwords require 12–72 bytes and are bcrypt-hashed. No default account is seeded. Registration creates an independent workspace and its Admin.

Authentication uses an HttpOnly, SameSite=Strict, 24-hour session cookie. Server storage contains only the session token hash. The authentication response and `GET /api/v1/auth/me` include `csrf_token`; send it as `X-CSRF-Token` on authenticated mutations. Logout revokes the session. Cross-site mutation requests are rejected. Set `COOKIE_SECURE=true` behind HTTPS in deployment; local HTTP development leaves it false.

Workspace identity comes from the authenticated session or API key. Client-supplied workspace IDs never grant access. Viewer cannot mutate; Editor can edit drafts/profiles and ingest events; Admin can publish and manage API keys.

## Drafts and publication

Draft updates send the last observed `revision`; stale concurrent changes receive 409 rather than overwriting newer data. Drafts may be incomplete, but graph/configuration validation must pass before publication. Publish also checks revision and requires Admin. Published snapshots are immutable at the database level.

Journey/version lists return bounded metadata. Fetch a journey by ID for its draft and `/journeys/{id}/versions/{version}` for a specific published workflow. Lists do not fetch every large workflow JSON blob.

## Customers and events

Create a customer before ingesting events. `customer_id` may be supplied (for example `cust_123`) or generated. Customer IDs and external IDs are workspace-scoped. Consent defaults to denied. Custom attributes and event properties are limited to 64 KiB each.

`POST /api/v1/events` accepts `event_id`, `customer_id`, `event_name`, `occurred_at` (RFC3339), and optional `properties`. Successful results contain `status` and newly started execution IDs.

- `ACCEPTED`: persisted and evaluated against published Active journeys.
- `DUPLICATE`: the same workspace/event ID and canonical payload; no new execution/action. Duplicate count is incremented.
- Reusing an event ID with a different payload returns 409.
- `LATE`: more than 24 hours old; stored for the timeline, never drives enrollment or event-wait/exit processing.
- More than five minutes in the future: rejected with 422.
- Unknown or cross-workspace customer: rejected with 422.

Occurrence timestamps are canonicalized to UTC for deduplication. Arrival order controls transitions; events are not retrospectively replayed into already completed steps. Event metadata lists and customer timelines are bounded; fetch `/events/{id}` for an individual payload.

Batch ingestion accepts `{ "events": [...] }` at `/api/v1/events/batch`, maximum 100 items. Envelope/schema and timestamp/size validation occurs before processing. Each valid item is independently transactional; item-level processing failures are returned as `REJECTED` with an error, alongside successful items. This is a best-effort batch, not an all-or-nothing database transaction. Retry using the same event IDs.

## API keys

Admins create, list and revoke keys under `/api/v1/settings/api-keys`. Plaintext is returned once on creation; only its hash is stored. Keys authorize **only** `POST /events` and `POST /events/batch`, in the key's workspace. They cannot read profiles or administer journeys.

Send `Authorization: Bearer <key>` from your ingestion service. Keep keys on the server and out of browser code, source control and logs. Session-cookie calls instead require CSRF headers.

## Enrollment and durability

Event persistence, enrollment and the first scheduled job are committed atomically. Database uniqueness prevents duplicate event IDs, duplicate event-based enrollment and more than one active execution per customer/journey. Enrollment uses the current immutable version and its reentry policy; later edits do not rewrite that snapshot.

The phase 5 worker executes persisted steps, waits, conditions, attribute updates and exits. Message steps render content and create a unique transactional outbox entry. The phase 6 sender enforces policy, creates durable delivery jobs and records provider attempts. Read `/executions/{id}` for the trace and `/journeys/{id}/executions` for bounded, filterable execution records. Paused journeys do not progress; Sunset journeys drain existing executions.

Event/timeout races serialize on the customer/execution state; only one branch wins. Global exit ignores events that predate the entry event. Event waiters accept matching events only if their occurrence is after waiter creation and they arrive by the deadline. After resume, overdue timers are immediately eligible; elapsed wall time is not added back.

## Messages, providers and preview

`POST /messages/preview` renders a typed channel configuration against an existing customer and/or isolated overrides. It never changes the live profile or calls a provider. HTML values are escaped and sanitized, unknown expressions are rejected, and missing attributes, used values, links and SMS encoding/segment counts are returned. UI HTML previews use a sandboxed iframe with a restrictive CSP.

Manage message templates under `/message-templates` (metadata list, individual detail, creation). Applying a template copies its content into the draft; later template edits cannot change an already published workflow snapshot.

Admins configure `/settings/messaging`: `frequency_limit`, `frequency_window_seconds`, and explicit `test_allowlist`. Live sends need current channel consent and a valid address/phone. Pending and unknown outcomes reserve a frequency-cap slot; accepted sends are counted by actual acceptance time. Blocked/failed records and retry attempts do not inflate send totals. Frequency lookups are customer-scoped, indexed and stop at the configured limit.

`POST /messages/test-send` needs a 16–128-character `Idempotency-Key`, `{preview, recipient}` and an allowlisted recipient. The same key/payload returns the same job; a different payload returns 409. These jobs have mode TEST and do not enroll or modify customers or reserve live cap slots. SMTP test email reaches Mailpit by default; test SMS uses the mock provider. Consent is a live-send gate; explicit, allowlisted test recipients are a separate decision.

`GET /deliveries` lists bounded metadata; `GET /deliveries/{id}` returns rendered content, eligibility and attempt history. ACCEPTED means provider acceptance, not confirmed delivery. Transient failures retry at most five attempts with 2/4/8/16-second backoff. SMTP connection failures before acceptance can retry; a lost acceptance response or expired non-idempotent dispatch becomes UNKNOWN and is not automatically resent. Mock providers support stable idempotency keys.

Global exit/failure cancels SCHEDULED or CLAIMED deliveries before dispatch. A dispatch already committed as in-flight can finish; `cancel_requested` records a later exit. External acceptance cannot be revoked transactionally. Consent, execution state and recipient identity are rechecked before committing dispatch, and stale claim tokens cannot dispatch another logical send.

## AI draft proposals

`POST /ai/journeys/generate`, `/ai/journeys/modify` and `/ai/messages/generate` accept `provider`, `action`, `prompt`, current `name`/`description`/`workflow`, and `node_id` for node/message actions. Supported actions are generate, modify, add_step, explain_journey, explain_node, improve_message, generate_message and fix_validation.

These endpoints are Editor/Admin-only and return validated proposals with a typed workflow, explanation, warnings and diff. They do not write drafts or publish. Configuration is visible without secrets at `/settings/ai`. Generation defaults to disabled; the selected provider key and compatible model must be configured on the server. Requests are bounded to two provider attempts and one active request per workspace. The browser rejects stale proposals and requires explicit Apply plus warning acknowledgement. Details and deferred live verification are in [AI.md](AI.md).
