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

The worker/runtime is phase 5 work; the phase 4 event pipeline durably schedules its first step. Delivery, simulation, monitoring and analytics follow their planned phases.
