# Abandoned Cart · €50+ — demo contract

The `abandoned-cart-eur` built-in creates a Draft. It includes an AI brief and five editable sandbox scenarios, but loading either never calls a model or publishes a journey. Existing built-ins and existing immutable versions are preserved.

Entry is `cart_abandoned`, currency `EUR`, and numeric amount >= 50 (including exactly 50). The shop must emit this event; the composer does not infer abandonment from browsing inactivity. Example properties:

```json
{ "cart_id": "cart_demo", "amount": 89.9, "amount_display": "89.90", "currency": "EUR" }
```

`amount` controls qualification; `amount_display` is the store-formatted value used in the email and SMS. The customer supplies `attributes.first_name` and current-cart `attributes.cart_purchased`; this flag is not lifetime purchase history. Missing required personalization fails safely. Configure your actual cart URL and coupon before publishing: COMEBACK10 is example content, not an automatically created shop coupon.

- No permitted channel: exit with no messages.
- Wait one hour, check current-cart purchase, then send an email only with email consent.
- If email is not permitted but SMS is, skip the email and retain the 24-hour purchase window.
- A purchase event exits as converted immediately while active.
- At the 24-hour timeout, send SMS only with SMS consent; otherwise finish without conversion.
- After SMS, wait up to 48 hours for purchase, then check the current-cart flag once more and finish converted or without conversion.
- Wait windows follow the workflow clock; delivery retries do not extend them.

## Seven-day rule

The example prevents **repeat enrollment for seven days since the last enrollment**, while allowing email and SMS within the same execution. The customer's last qualifying enrollment is checked under the existing customer lock, with the active-execution unique constraint retained. This differs from a one-message-per-seven-days limit, which would block this example's intended second message. Workspace-wide consent and frequency limits are also checked by normal delivery policy before dispatch. No new index, unbounded history scan or per-node reporting query was introduced.

## A–E scenarios

| Scenario | Sequence                          | Assertions                                              |
| -------- | --------------------------------- | ------------------------------------------------------- |
| A        | Purchase at +30m                  | No messages; converted                                  |
| B        | Purchase at +5h                   | One email, no SMS; converted                            |
| C        | No purchase, both consents        | One email and one SMS; completed without conversion     |
| D        | No SMS consent                    | One email, no SMS; completed                            |
| E        | Purchase at +30m, new cart at +1d | One enrollment total; no messages from the repeat entry |

E builds the prior enrollment inside the same isolated simulation; it does not fabricate a live customer history. Additional deterministic tests cover below-threshold and non-EUR carts, exactly €50, no consent, SMS-only consent, re-entry after eight days, and final conversion through a current-cart attribute update.

Test Lab loads these examples from the canonical template API. The embedded Test Inbox filters by the current run ID and follows virtual-clock revisions. It fetches message detail on selection and keeps sanitized sandboxed HTML rendering; there is no per-node API polling. All outputs retain SIMULATED / BLOCKED / FAILED semantics; simulated content is not labeled delivered. Existing standalone Inbox actions remain available.
