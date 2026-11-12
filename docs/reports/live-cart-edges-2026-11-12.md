# Ten additional cart-journey edge cases — 2026-11-12

**9/10 new scenarios passed** on the exact, unmodified output from the
[preceding paid brief trial](live-brief-cart-2026-11-12.md). No new LLM call, graph
patch, publication or real provider send was made. These ten cases are different
from the original eleven: combined, **19/21** pass with two distinct business
issues (the earlier fractional amount boundary and this cross-cart purchase).

[Full scenario inputs, run IDs, assertions, rendered messages and traces](../validation/live-cart-edges-2026-11-12/result.json).

Tested source `5c706af`; the local application used its real authenticated Test Lab
and Test Inbox APIs. The original workflow was compared before and after testing.
The ten-case driver took **1.21 seconds** including login/browser startup and
HTTP operations, with days advanced virtually. Later authoritative readback of
case 10 is separate from that timing. All profiles/contact details are fictional.

## Results

|   # | Scenario                                          | Result   | Observed behavior                                                        |
| --: | ------------------------------------------------- | -------- | ------------------------------------------------------------------------ |
|   1 | Aynı entry event aynı anda iki kez                | **PASS** | One enrollment; one email and one SMS; duplicate recorded                |
|   2 | Yedi gün sınırından bir saniye önce yeniden giriş | **PASS** | Second entry rejected at 7 days minus 1 second                           |
|   3 | Tam yedi gün dolduğunda yeniden giriş             | **PASS** | Second entry accepted at exactly 7 days; two emails and two SMS total    |
|   4 | Satın alma ile bir saatlik timer aynı anda        | **PASS** | Converted at +1h; no message scheduled                                   |
|   5 | Email retry beklerken izin geri çekilir           | **PASS** | Email blocked as consent_revoked after one 429 attempt; retry cancelled  |
|   6 | Ad ve biçimlendirilmiş tutar eksik                | **PASS** | Fallbacks render; no unresolved variable or missing-attribute errors     |
|   7 | Tutar yeterli ama para birimi USD                 | **PASS** | No enrollment or messages for USD                                        |
|   8 | Email izni var ama adres yok                      | **PASS** | Email invalid_recipient with zero attempts; permitted SMS continues      |
|   9 | Başka sepet için gelen satın alma                 | **FAIL** | Unrelated purchase incorrectly converts at +2h and suppresses SMS        |
|  10 | Provider sonucu belirsiz: 408                     | **PASS** | Exactly one UNKNOWN/408 email attempt; no automatic retry; SMS continues |

Every scenario also checks accepted email/SMS counts, enrollment count, expected
virtual completion time, and no duplicate logical messages. An additional check
uses `(execution_index,source_node)` rather than only message IDs. Case 1 verifies
the DUPLICATE trace; cases 5/8/10 inspect persisted attempts and eligibility reason.
Case 3 deliberately expects two executions' messages rather than counting a
legitimate second enrollment as duplication.

## Business issue: purchase correlation

Case 9 starts cart `LLM-DEMO-1` and receives a purchase at +2h for
`A-DIFFERENT-CART`. We expect the original still-abandoned cart to continue:
email at +1h, SMS at +25h, normal completion at +73h. Actual: email at +1h,
then **converted at +2h**, no SMS.

The generated global exit listens to the customer's purchase event without a
cart correlation filter. The runtime deterministically follows that definition;
this is a workflow/goal-scoping gap, not evidence that it ignored its specification.
The model had already disclosed the missing cart correlation as a warning.
The original brief talks about “this cart” but did not spell out the correlation
rule; this edge case makes the business expectation explicit. A product choosing
“any customer purchase counts” would intentionally use different assertions.

This draft is therefore unsuitable for a per-cart conversion claim until purchase
scope is explicit. Do not silently rewrite the runtime or substitute the current
customer's entire purchase history. Entry-object correlation must be represented
and validated in the specification/available rule semantics before relying on it.
The existing one-active-execution-per-customer/journey invariant by itself does
not establish purchase-to-cart identity.

## Additional quality observations

- Missing personalization is technically handled, but the generated copy becomes
  “Merhaba Merhaba” and repeats “sepetinizdeki ürünler … sepetinizde”. Case 6 passes
  fallback safety, not editorial quality. Message review should improve those
  fallbacks before publication.
- A sandbox provider 408 is recorded as attempt status UNKNOWN with exactly one
  attempt, correctly avoiding automatic resend after ambiguous acceptance. The
  inbox's top-level status is FAILED; a user must inspect the attempt/response to
  see the uncertainty. This is a presentation limitation, not a second send.
- The initial external extra assertion for case 10 read `attempt.code` instead
  of the API's `attempt.provider_code`. It was corrected and re-evaluated against
  the authoritative persisted inbox message via GET. The stored engine assertions
  already passed. No second scenario execution, workflow change or paid call was
  used to turn this harness mistake into a product finding. The raw artifact
  records the correction.

## Isolation and limits

The journey remains Draft, with no published version or live execution. The live
delivery count did not increase, and all messages belong to SIMULATED runs. The
publication check still returns `ready=false`: the prior threshold failure and
new cross-cart failure are both required acceptance failures.

The simultaneous timestamp case validates the simulator's deterministic ordering
(event before timer), not a concurrent PostgreSQL race. Duplicate events and
cooldown here are also sandbox behavior, not additional concurrent ingestion or
worker-crash verification. Existing integration tests address those separately.
There is no claim that a mock UNKNOWN outcome proves a real provider's behavior.

The failed runs and successful runs remain inspectable under
[the saved local draft](http://localhost:5173/journeys/df88e290-d6e0-489a-bea8-8b42f1c843d1).
Next corrections are the exact >=50 threshold and explicit purchase-object
correlation, with the original failing scenarios retained for regression.
