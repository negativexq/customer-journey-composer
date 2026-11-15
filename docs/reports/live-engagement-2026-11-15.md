# Ten live journeys with engagement waits and discount codes — 2026-11-15

The user authorized a live run. The local Docker stack ran with `AI_PROVIDER=openai` and `OPENAI_MODEL=gpt-6-luna`. The key was read from the macOS keychain into the `docker compose` process environment only. It is in no file, log or evidence. Source revision: `08311aa` (schema `composer.v6`).

Raw evidence: [result.json](../validation/live-engagement-2026-11-15/result.json). Harness: [scripts/live-engagement-scenarios.mjs](../../scripts/live-engagement-scenarios.mjs).

## What each run did

Each scenario:

1. registered a fresh workspace and defined its events and fields in the data catalog;
2. sent a Turkish business brief and a human-written v2 behavior contract to `POST /ai/journeys/generate`;
3. saved the proposal to a Draft without edits;
4. ran every contract case and the held-out cases in Test Lab;
5. read the rendered messages from the test inbox;
6. ran the publish check;
7. counted the workspace's rows in PostgreSQL.

Nothing was published, sent or entered by a live customer.

## Results

| Journey                   | What it exercises                                               | Provider attempts | Generation | Cases (contract + held out) |
| ------------------------- | --------------------------------------------------------------- | ----------------- | ---------- | --------------------------- |
| Cart abandonment, SEPET15 | Code in email and SMS, cart-scoped purchase exit, ≥ 50 boundary | 1                 | 28.4 s     | 4 + 3                       |
| Welcome, wait for a click | `message_clicked` wait on the welcome email (`message_node`)    | 1                 | 19.8 s     | 3 + 2                       |
| Win-back, GERIDON20       | Code in email, customer-wide goal exit                          | 1                 | 28.2 s     | 3 + 1                       |
| Viewed product, wait open | `message_opened` wait, SMS only with consent                    | 2                 | 40.9 s     | 4 + 1                       |
| Review request            | Order-scoped goal exit; another order's review ignored          | 1                 | 18.7 s     | 3 + 1                       |
| Loyalty gold              | ≥ 1000 boundary, `tier` attribute update                        | 1                 | 19.2 s     | 2 + 2                       |
| Price drop                | SMS, else email, else ineligible; ≥ 20 boundary                 | 1                 | 13.4 s     | 4 + 1                       |
| Invoice reminder          | Invoice-scoped exit, staged email then SMS                      | 2                 | 40.0 s     | 3 + 2                       |
| Webinar, wait SMS click   | `message_clicked` wait on the SMS, not the earlier email        | 2                 | 49.0 s     | 3 + 1                       |
| Back in stock, STOK10 SMS | SMS-only with code, 14-day event wait                           | 1                 | 34.4 s     | 3 + 1                       |

All ten passed:

- 47/47 Test Lab cases: 32 from the contract and 15 held out. A case passes when the email count, SMS count and outcome match, every message with a code shows it, and the expected attribute is set.
- Every duration stated in the contract matched.
- Every publish check was ready.
- Every static check passed:
  - `discount_code` exactly as briefed, and none invented;
  - SMS text writes `{{ offer.code }}`;
  - each engagement wait sets `message_node` to the right message step;
  - no business-event wait sets `message_node`.

Usage: 13 provider calls. Tokens: 66,091 input, 29,060 output, 14,017 of them reasoning. Generation took 13.4–49.0 s, median 28.3 s. Three proposals failed graph validation once and passed after the automatic repair. The whole run took 298 s.

Database readback for every workspace: 1 Draft journey, 0 versions, 0 live executions, 0 customers, 0 delivery jobs, and only test runs.

Held-out cases worth naming:

- **Cart.** An order for another cart does not convert. An amount of exactly 50 qualifies.
- **Welcome.** An open without a click sends the reminder.
- **Viewed product.** A click without an open still sends the SMS.
- **Webinar.** A click on the earlier email does not satisfy the wait for the SMS click.
- **Loyalty.** 999.5 stays out.

## What the run found

- **The email's "Your code" line is English.** The plain-text part of a coded email ends with "Your code: SEPET15" under Turkish copy, and the HTML code box has an English label. The renderer adds both in a fixed language.
- **Copy.** The webinar reminder email promises a link "by SMS". Customers without SMS consent get no SMS, so for them that sentence is wrong. The contract and cases do not cover message wording.
- **The first run's failures were the harness's, not the model's.** Before this run, one attempt recorded 5 of 10 scenarios failing:
  - Four contracts were refused before any provider call: a bound event's example lacked the entry's identity.
  - The loyalty contract wrote the expected attribute as `customer.attributes.tier` instead of `tier`. No graph could satisfy it, and the provider was called twice.
  - The code check read sandbox state, which keeps no message bodies.

  All three were fixed in the harness, and the full set was regenerated. Those first-run generations cost provider calls and are not included in the totals above.

## Limits

- One run per scenario on one model. This does not measure repeatability or Claude.
- The Draft was saved through the draft API, not through the review screen's approve button.
- Engagement events in Test Lab are synthetic. No real email was opened and no link was clicked.
- The contract cases are human-written and finite. Unlisted states and wording still need review.
