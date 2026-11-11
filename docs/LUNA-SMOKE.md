# Luna journey generation smoke — 2026-11-11

Source `48cf0d9`. The user explicitly authorized this limited live OpenAI test;
broader provider evaluation remains deferred. This is one generation example,
not a reliability score or a general model comparison.

## Path exercised

An isolated test workspace and a temporary loopback backend used the unchanged
application adapter with `OPENAI_MODEL=gpt-6-luna`. The normal application's AI
configuration was not changed. The browser forwarded actual API responses from
that backend; no generated response fixtures were substituted. Setup attempts
first exposed a panel-toggle mistake in the one-off harness and a proxy Host /
Origin mismatch; neither reached a model. The proxy was corrected to preserve
Host, matching normal reverse-proxy behavior, without changing CSRF enforcement.

The successful Composer HTTP request took 38.849 seconds and returned 12 nodes
and four configuration/personalization warnings. Typed schema and graph/message
validation passed. Generation alone left the draft empty. After inspecting the
diff and acknowledging the warnings, Apply persisted the exact proposed workflow
without manual edits. Reload retained the graph. The journey remained Draft,
with no published version, live customer, execution or delivery record.

The application permits up to two internal provider attempts; this adapter does
not expose per-attempt usage or attempt count. One Composer HTTP request is not
proof of exactly one billable provider attempt. No further generation was
requested after the scenario failures below.

## Canonical recovery tests against the generated draft

| Scenario                           | Result | Observed behavior                                                                                                   |
| ---------------------------------- | ------ | ------------------------------------------------------------------------------------------------------------------- |
| A — purchase at +30m               | PASS   | No message; converted                                                                                               |
| B — purchase after email           | PASS   | One email, no SMS; converted                                                                                        |
| C — no purchase                    | FAIL   | Correct one email + one SMS, but `EXITED` with reason `completed/non-converted`; expected `COMPLETED` / `completed` |
| D — SMS consent denied             | FAIL   | Correct one email and no SMS, but free-form exit reason produced `EXITED`; expected `COMPLETED`                     |
| E — repeat entry within seven days | PASS   | One enrollment, no repeat message                                                                                   |

No assertions were weakened and the generated draft was not edited to make
these runs pass. The typed graph validator allows custom exit reasons; runtime
semantics map normal completion specifically to `completed`. The model's
free-form alternatives therefore pass structural validation but change reporting
state. The one-off test brief itself used the human-facing phrase
`completed/non-converted`; the model copied it as a machine reason. The shared
system prompt did not enumerate canonical exit codes. This is a prompt/schema
contract gap, not evidence of a general Luna limitation. It demonstrates why
graph validity alone is insufficient.

Persisted sandbox message text resolves Alex and 89.90; the SMS includes
COMEBACK10. All four persisted outputs are SIMULATED. Test runs did not create
live customers, deliveries, executions or versions. No publish or real send was
performed. The temporary backend was removed, including its key environment.

## Token accounting limitation

The existing HTTP adapter discards the provider response's `usage` and keeps only
its output text. Exact input/output/reasoning usage for this generation cannot
be recovered from the proposal or elapsed time. The raw provider output is also
normalized before it is returned to the UI.

Two subsequent calls to OpenAI's input-token counting endpoint generated no new
journey. They counted reconstructed content:

| Counted content                                                              | Tokens | Interpretation                                                                                       |
| ---------------------------------------------------------------------------- | -----: | ---------------------------------------------------------------------------------------------------- |
| Reconstructed initial request, including system prompt and typed JSON schema |  1,847 | Token count of the reconstructed request; not recorded original usage                                |
| Reconstructed visible model JSON counted as text input                       |  1,364 | Approximation to visible output; includes input framing and can differ from original JSON formatting |
| Sum of those counts                                                          |  3,211 | Content-size estimate only; not total billable generation usage                                      |

Hidden reasoning, response framing and any internal repair attempt are not
recoverable. No exact total or invoice cost is claimed. See OpenAI's
[token counting guidance](https://developers.openai.com/api/docs/guides/token-counting):
reported output usage includes tokens beyond visible text.

## Follow-up identified, not implemented

- Constrain AI-proposed normal exit codes to supported runtime outcomes, while
  preserving intentional custom exits in the manual editor; cover this mismatch
  with deterministic tests before another live generation.
- Capture sanitized provider usage, actual model and attempt totals in generation
  results so future live calls report real input/output/reasoning consumption.

Detailed generated proposal, scenario assertions, browser screenshots and count
fixtures remain in ignored `.tmp` files. No key, credential or real customer data
was committed. Application code and normal AI settings are unchanged.
