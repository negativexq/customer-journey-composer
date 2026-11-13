# Real AI creation capture — November 13, 2026

The eight [creation screenshots](../images/creation/README.md) show a real OpenAI
`gpt-6-luna` response to the displayed natural-language abandoned-cart brief.
This is a product walkthrough, not a new broad reliability benchmark.

## Captured result

- Application stream and normal server-side schema, graph, messaging and scoped behavior validation.
- 40,945 ms from clicking Generate to receiving the proposal; provider attempts total 40,848 ms.
- 7,154 input + 4,166 output = **11,320 provider-reported tokens**. Reasoning tokens are included in output.
- First attempt rejected with one workflow validation issue; second accepted. No manual graph/content correction.
- Generated graph: 11 steps and 12 connections. Same-cart purchase exit, eligibility, consent branches, waits, email and SMS.
- Normal review checkbox and approval applied the proposal to an editable draft.
- Zero published versions and zero delivery jobs, checked through the application API.
- Native 3200 × 2100 screenshots; no UI text, counts or geometry patched.

[Exact generated proposal, provider counters and screenshot manifest](../images/creation/manifest.json).

## Earlier attempts in this capture session

| Request          | Outcome                                                                              | Reported tokens |
| ---------------- | ------------------------------------------------------------------------------------ | --------------: |
| First            | Two provider attempts rejected by behavior validation; draft unchanged               |          13,671 |
| Second           | Accepted in 30.3s; screenshot tool could not read the response body after navigation |           6,402 |
| Third            | Capture observer setup failed; browser closed and request was cancelled              |         Unknown |
| Fourth, pictured | Accepted after one automatic repair; all eight images captured                       |          11,320 |

Known usage across completed requests is **31,393 tokens**, excluding the cancelled
request whose provider usage was not returned. This is not an exact total charge.
[Sanitized application audit records](../validation/live-creation-capture-2026-11-13/attempts.json)
preserve accepted/rejected status and all returned counters. The first failure's
precise behavior mismatch was not retained by the original capture tool; only the
behavior-validation category is available. It is not presented as diagnosed or fixed.

The capture observer now reads a clone of the browser fetch stream while leaving
the application consumer and actual network response intact. Failed capture work
never substitutes an authored graph for the provider result. The earlier replay
pictures have been replaced. The operational demo gallery still uses its separately
authored, populated fixture; these creation pictures use the generated draft.
