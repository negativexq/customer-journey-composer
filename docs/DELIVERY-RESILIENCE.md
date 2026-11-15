# Delivery resilience

What the delivery worker does when a provider call does not end in a plain acceptance, how each case affects a message's five attempts, and what the people running the workspace see. The structure and most of the rules follow the sister project Smart Campaign Composer's `docs/errors-and-retries.md`. The difference is where the shared state lives: there it is Redis, here it is a `provider_channels` row per workspace and provider in PostgreSQL. Each worker reads and changes the row under a row lock, so every worker sees a pause, a refused key or an open breaker at once.

Each workspace sends through its own provider account. Its rate, pauses and breaker are its own, and one workspace's outage or simulation never holds another's messages.

## At a glance

| What happened                                     | Next status                                   | Uses an attempt | What the channel does                                                          |
| ------------------------------------------------- | --------------------------------------------- | --------------- | ------------------------------------------------------------------------------ |
| Accepted                                          | `ACCEPTED`                                    | Yes             | Goes on                                                                        |
| 429                                               | `SCHEDULED` after `Retry-After`               | No              | Every worker pauses the channel for `Retry-After`: 1 s if absent, 30 s at most |
| 401, or SMTP 530/534/535                          | `SCHEDULED` after 30 s                        | No              | Locked; one call every 30 s tests the credentials                              |
| Other 4xx, SMTP 5xx                               | `FAILED`                                      | Yes             | Goes on                                                                        |
| 5xx, 408, connection failure                      | `SCHEDULED` with backoff and jitter           | Yes             | Counts toward the breaker                                                      |
| No answer after the message may have gone through | `UNKNOWN` (SMTP) or retried (idempotent mock) | Yes             | Counts toward the breaker, and toward the rate window for 13.1 s               |
| Failed probe while the breaker is testing         | `SCHEDULED` with backoff                      | No              | Breaker stays open; its wait doubles up to 30 s                                |
| The rate window is full                           | Not claimed                                   | No              | Rests 250 ms, then asks again                                                  |

An attempt is one call that may have reached the provider. A message has 5. Calls that cost no attempt are bounded at 50 per message, so a channel stuck on 429 or 401 for hours ends its messages in the end instead of holding them forever.

## Backoff

A message that gets a 5xx, a 408 or a connection failure waits 2, 4, 8, 16 and then 32 s, with a random extra of up to half the wait. Messages that failed together therefore do not all come back together. Every retry carries the same idempotency key.

## 429: the provider asks to slow down

A 429 is about the channel, not the message, so it uses no attempt. The worker writes `paused_until` on the channel for the provider's `Retry-After`, bounded between 1 and 30 s, and never shortens a pause already running. Claims skip a paused channel's messages, so another channel's messages go first. A 429 does not count toward the breaker: a busy provider is not a failing one.

## 401: the provider refuses the credentials

Treated as a rejection, an expired key would fail every message within minutes. Instead:

- The first refusal locks the channel (`credentials_refused_at`), raises a critical notification and holds every worker. Refusals that arrive while it is locked change nothing.
- Every 30 s, one call tests the key. The claim that hands out the probe moves `next_probe_at` forward under the row lock, so there is one probe across all workers.
- The probe carries the lock it tests. Only an accepted answer to the current lock's probe opens it. A late 200 to a call made before the lock, or to an older lock's probe, leaves the newer lock in place.
- When the probe is accepted, its message is sent like any other, the lock opens and a notification says so.
- SMTP login refusals (530, 534, 535) are treated the same way. A 403 stays a rejection of the message, because some providers use it for a refused recipient.

## The circuit breaker

- **Opening.** With at least 5 calls in the last 30 s and at least half of them failed (5xx, 408, connection failure or `UNKNOWN`; 429s and 401s are left out), the breaker opens for 5 s. The figures fit a demo workspace's traffic; the sister project uses 20 calls in 10 s for a send of thousands.
- **While open.** No message on the channel is claimed, so none uses an attempt.
- **Probing.** After the wait, three probes go out. A probe that fails uses no attempt and reopens the breaker with the wait doubled, up to 30 s. Three passing probes close it.
- **Lost probes.** A probe lost to a crashed worker is handed out again after a minute, so the breaker cannot wait for it forever.

## Alerts and the notification bell

A provider failing 70% of the time would pass three probes now and then, so an alert that closed with the breaker would ring the bell on every cycle. The alert therefore has its own state (`failing_since`):

- It opens with the first breaker trip, as a critical notification.
- Later trips while it is open add nothing.
- It closes, with a success notification, only once at least 5 calls in the last minute went through with fewer than half failing.

The bell also lists what teammates do to journeys: publishing, pausing, resuming, stopping new entries, completing, archiving and audience runs. These come from a trigger on `audit_logs`, so every write path is covered in the same transaction. A person's own actions arrive already read. The count of unread notifications is kept per user (`notifications_seen_at`) and clears when the bell is opened. The bell checks every 15 s while the tab is visible.

## The rate limit

Each channel has `rate_per_second`, 50 by default and editable in Settings → Messaging. A claim counts the channel's calls under its row lock:

- calls in flight;
- calls answered in the last 1.1 s (the provider's second plus 100 ms of clock margin);
- calls with no answer in the last 13.1 s (send timeout plus 2 s, plus 1.1 s), because a call with no answer may still reach the provider later.

If the window is full, the channel rests 250 ms and the claim moves on to other messages. Blocked messages (consent withdrawn, recipient changed) never count, because they never left.

## Simulating a provider problem

Settings → Messaging → Channels lets an Admin make a channel's provider act out a 429, an outage (503) or a refused key (401) for 1 to 30 minutes. While it runs, no call reaches the provider: the worker records the simulated answer in its place, and every rule above applies as if the provider had sent it. The card shows the channel's state, its calls, acceptances, failures and held calls over 10 minutes, and its last problem, refreshed every 5 s. The change is audited.

## Content guards

- **Discount code.**
  - An email or SMS step may set `discount_code`: 3 to 30 capital letters, digits and hyphens, so it reads the same in every language and cannot break an SMS.
  - Messages write `{{ offer.code }}`, which is filled in at preview and at send time, so the code cannot be mistyped.
  - An email always shows the code in a box of its own below its body, unless the body already writes it. Its plain text gets a line with the same label. The label and the unsubscribe footer follow the customer's profile `locale` (Turkish, English, German, French or Spanish; anything else gets English), as in the sister project. The message copy itself is the author's.
  - Validation refuses an SMS that sets a code and never shows it, and any message that writes `{{ offer.code }}` without a code.
- **Longest case.** An SMS preview is also measured as sent to the worst-off customer:
  - Each `customer.attributes.*` variable is filled with the costliest value among the workspace's customers who accept SMS, or the fallback when it costs more. A non-GSM letter such as "ş" turns the whole SMS into Unicode, so the costliest value is not always the longest.
  - Links are counted at their tracked length.
  - The preview warns when that case needs more segments than the profile being previewed.

## Where to see it

- **Settings → Messaging → Channels:** each channel's state, rate, recent calls and last problem, and the simulation controls.
- **The notification bell:** channel alerts and their clearing, and teammates' journey changes.
- **Delivery details:** every attempt with its provider code and response. The attempts table also records `provider`, `gate` (`probe`, `credential_probe`) and `signal` (`throttled`, `credentials`).
- **Metrics:** `journey_channel_events_total{provider,event}` counts `throttled`, `credentials_locked`, `credentials_unlocked`, `circuit_opened` and `alert_closed`, without workspace labels.

## Tests

- `internal/delivery/channels_test.go`: classifying answers, which calls are free, Retry-After bounds, jitter spread, simulated faults and SMTP login refusals.
- `internal/delivery/channels_integration_test.go`, against PostgreSQL. It covers:
  - a 429 pausing the channel for every claim at no cost;
  - a 401 locking until one probe per 30 s is accepted;
  - only the current lock's probe opening it;
  - the breaker opening, a failed probe doubling the wait, probes reopening the channel, and the alert closing only after a good minute;
  - a full rate window counting answers for 1.1 s;
  - a simulated outage never calling the provider and ending on time.
- `internal/httpapi/channels_integration_test.go`: the channels API and its validation, the bell's unread count (a teammate's publish is unread, one's own is not) and the longest-case preview.
- `internal/messaging/discount_test.go`: code filling, the email code box and the validation rules.
- `tests/e2e/channel-resilience.spec.ts`, in the browser: a simulated refused key holding a real test SMS without using its attempt, then the channel card and the bell.

## Not covered

- An unreachable provider uses an attempt here; the sister project rests the channel 5 s without one.
- A long outage does not pause journeys. The sister project pauses a campaign after 10 minutes without progress; a journey run here keeps its messages queued, and the bell and channel card say why.
- The rate window is enforced per workspace and provider by the claim, not measured at the provider. No load run has measured it.
- Real providers were not called. SMTP is checked against a local test server, and a real provider's idempotency must be checked before retries are trusted.
