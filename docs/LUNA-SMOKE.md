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

## Ten-scenario replay — 2026-11-11

The user requested ten scenarios. This replay tests the **same recorded generated
workflow**, not ten new model generations. The proposal is preserved in
[the fixture](../tests/fixtures/ai/luna-recovery-2026-11-11.json). No model call,
token-count endpoint or paid provider was invoked during this replay.

Each input/assertion set was run through the actual persisted Test Lab API twice:
once on the unchanged generated draft and once on the canonical approved
`abandoned-cart-eur` template. The canonical control passes 10/10; the generated
flow passes 4/10. This is one workflow's acceptance result, not a model-wide
success rate. [Raw assertions and outcomes](validation/luna-recovery-ten-scenarios.json)
include expected/actual values, virtual durations and isolation checks.

| #   | Scenario                                           | Generated | Reference | Interpretation                                                                         |
| --- | -------------------------------------------------- | --------- | --------- | -------------------------------------------------------------------------------------- |
| 1   | Purchase at +30m                                   | PASS      | PASS      | No message; converted                                                                  |
| 2   | Purchase after email                               | PASS      | PASS      | One email, no SMS; converted                                                           |
| 3   | No purchase                                        | FAIL      | PASS      | Correct email/SMS counts; wrong normal-completion code and state                       |
| 4   | SMS consent denied                                 | FAIL      | PASS      | No SMS, as required; wrong terminal state                                              |
| 5   | Repeat entry within seven days                     | PASS      | PASS      | Repeat enrollment blocked                                                              |
| 6   | EUR 49.99                                          | PASS      | PASS      | No enrollment or message                                                               |
| 7   | Exactly EUR 50                                     | FAIL      | PASS      | Qualifies and sends correctly; wrong normal-completion code/state                      |
| 8   | Both consents denied                               | FAIL      | PASS      | No messages, but exits at +25h instead of immediately and uses a different exit reason |
| 9   | Profile already marks current cart purchased       | FAIL      | PASS      | Generated flow sends two messages; expected none and converted                         |
| 10  | Profile changes to purchased at +72h without event | FAIL      | PASS      | Generated final branch ignores the profile change; expected converted                  |

Three failures share the earlier completion-code issue. The larger suite also
confirms missing behavior: the early/final current-cart profile checks and the
immediate no-channel exit. Fixing exit strings alone cannot make all ten pass.

The generated workflow is checked for semantic equality before/after replay
(JSONB object-key ordering is ignored; node/edge array order and all values are
preserved). Both journeys remain Draft. There are no published versions, live
customers/executions/delivery jobs or changes to production analytics. Only
sandbox runs and SIMULATED Inbox messages are persisted in the isolated workspace.

Reproduce against the running local stack:

```sh
node scripts/test-luna-recovery.mjs
```

Default report: `.tmp/luna-ten-scenarios.json`. The command intentionally returns
exit code 1 when any acceptance scenario fails; in this recorded run, that is
the expected signal for the six generated-flow failures, not a harness crash.
It is not included in the always-green unit-test command. No application fix or
manual edit of the generated proposal is part of this replay.
