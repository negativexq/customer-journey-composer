# Five live business-brief audits — 2026-11-12

Five paid OpenAI generation requests were authorized by the user and run against
local API commit `8fa8ca5`, with model `gpt-6-luna`, structured output
`composer.v2` and catalog report `semantic.v1`. Each used a fresh fictional
workspace, a curated data catalog and a Turkish business brief containing no
technical field/event identifiers. Outputs were not manually repaired.

All five produced graph-valid proposals. Four passed reference grounding and
opened Drafts through the approval API as an isolated test; the fifth was blocked.
This was a scripted API audit, not evidence of a person reviewing the UI or of a
published/live campaign. Four unchanged workflows were then copied into separate
sandbox workspaces for 29 behavior checks: **22 passed, 7 failed**. All production
customer, execution, delivery and published-version counts stayed zero in the
fresh test workspaces.

[Recorded briefs, catalogs, outputs, usage and sandbox results](semantic-live-five-2026-11-12.json)
contain fictional test data and no provider key, cookie, password or session.

## Generation measurements

| Brief                                          | API seconds | Provider attempts | Input tokens | Output tokens | Total | Audit result                                                                 |
| ---------------------------------------------- | ----------: | ----------------: | -----------: | ------------: | ----: | ---------------------------------------------------------------------------- |
| Sepet kurtarma                                 |      26.648 |                 1 |         2593 |          2467 |  5060 | 7/10 sandbox checks; purchase-state and threshold defects                    |
| Hesap etkinleştirme                            |      46.705 |                 2 |         6417 |          5085 | 11502 | 6/6 sandbox checks passed                                                    |
| Denemeden aboneliğe                            |      36.447 |                 1 |         2454 |          3761 |  6215 | 6/7; missing personalization fallback fails                                  |
| Teslimat sonrası memnuniyet                    |      55.476 |                 2 |         7392 |          6996 | 14388 | 3/6; refunds count as conversions and email opt-out ends SMS path            |
| Eksik sadakat verisi ve belirsiz etkinleştirme |      15.792 |                 1 |         2395 |          1591 |  3986 | Ambiguity blocked; missing requirement remains warning-only after resolution |

There were **7 provider attempts** for 5 briefs: onboarding and post-purchase each
needed one automatic validation repair. Reported usage across every attempt is
**21,251 input + 19,900 output = 41,151 tokens**. Output includes 7,882 reasoning
tokens; input includes 4,452 cached input tokens. These breakdowns are not added a
second time. [OpenAI token accounting](https://developers.openai.com/api/docs/guides/token-counting)
explains generated-output accounting. No dollar cost is estimated.

AI request wall times sum to **181.068 seconds**. The sequential generation
pipeline, including fictional workspace/catalog setup and API checks, took
**182.647 seconds**. These timings exclude build/verification, manual inspection
and the later sandbox audit; they are not a throughput benchmark.

## Findings

### High — cancellation and conversion share one global-exit outcome

The post-purchase brief explicitly says refunds must stop messages immediately
and must not count as conversion. AI selected `order_refunded` as global exit.
At +24h and +100h the sandbox correctly stopped the execution but set
`exit_reason=converted`. Both checks failed. Production
`runtime/event_transitions.go` and sandbox `simulation/engine.go` hard-code that
outcome; this is a runtime/schema limitation, not a catalog mismatch.

Add an explicit outcome to cancellation/goal exit rules, with a backward-compatible
default for existing published definitions. Until supported, a cancellation-only
requirement must be an unresolved blocking requirement rather than a guessed
conversion goal. Both production and sandbox need the same tests.

### High — omitted requirements can survive as warnings

The VIP brief needs loyalty score, which the catalog lacks. AI did not invent a
field or substitute another attribute. It explicitly warned and omitted the VIP
email/SMS behavior. Two activation events share the same business label, and the
catalog correctly marked the chosen event ambiguous; original approval returned
HTTP 422.

A separate deterministic probe confirmed the ambiguity explicitly. Grounding then
became ready and approval returned HTTP 200, despite the loyalty requirement still
being absent. No campaign was published. This demonstrates the current boundary:
reference grounding cannot detect an omitted requirement that has no reference.
Use versioned, structured unresolved requirements with a server approval gate;
warnings alone are insufficient for missing required behavior.

### High — cart goal checks are missing before messages

Purchase events at +30m and +2h correctly stop messages through global exit. But
when only the profile purchase state changes at +30m, both email and SMS still
appear and conversion is checked only at +73h. A profile purchase change at +24h
also fails to suppress SMS at +25h. These two checks failed because the generated
graph omitted current-profile checks after waits and before sends.

The catalog correctly matched the field; it cannot prove that the graph checks it
at the required times. Goal/exit behavior needs independently reviewed acceptance
cases, including profile changes without a matching event.

### Medium — personalization fallback is incomplete

The trial journey's welcome email uses the product name without a fallback, even
though the brief asks for fallback wording. With product/first name absent, the
sandbox fails at hour zero with `simulation_failed`; no expected messages are
produced. The other six trial behavior checks pass. Field existence in a catalog
does not mean every customer has a value. Validate missing-variable behavior
before accepting generated messages.

### Medium — threshold is approximated incorrectly

“50 EUR and above” became `amount > 49.99`. This works for the tested two-decimal
amounts, including exactly 50, but admits 49.995. Event ingestion currently does
not enforce two-decimal EUR precision, so the below-50 boundary test fails. Use an
exact greater-than-or-equal expression or an explicitly validated money contract;
do not silently assume decimal precision.

### Medium — email opt-out incorrectly closes an allowed SMS path

The post-purchase graph combines “not refunded” and “email consent” into one
condition whose false branch exits. A customer with email=false and SMS=true
therefore exits at +72h with no SMS instead of continuing to the SMS stage. That
check failed. Consent must gate each channel without cutting off another allowed
path unless the brief explicitly requests that behavior.

### Content — cart SMS omits the cart amount

The cart brief asks for the customer's name and cart amount in messages. Email
contains both, while SMS contains the name and discount code but no amount. This
was found by inspecting the generated/rendered content, outside the 29 execution
checks. A graph can be structurally valid and still miss a message requirement.

## What passed and what remains

The complete onboarding scenario passed all six event, profile, consent and timing
checks. Both providers have deterministic catalog unit coverage; this paid run
used OpenAI only. Catalog reference matching, ambiguous-label blocking, event
exit timing, immutable draft replay and sandbox isolation worked in the tested
cases. None of this establishes that every business brief is implemented correctly.

No fixes to the above behavioral findings, extra paid retries or production
publication were performed in this audit. Prioritize exit outcomes and blocking
unresolved requirements first, then goal checks, fallback validation, exact numeric
boundaries and channel-path acceptance cases. Keep the recorded outputs for
unpaid deterministic regression tests rather than repeatedly regenerating them.

## Replay without paid generation

With the local stack running, run `node scripts/replay-semantic-audit.mjs`.
Optionally set `CJC_API_URL` to another loopback API origin. The script reads these
recorded outputs, creates fresh fictional workspaces and sandbox runs, and writes
its results to `.tmp/semantic-audit-replay/replay.json`. It refuses AI, publish and
real test-send routes and never reads a provider key. Exit code 1 currently means
the recorded behavioral defects are still reproduced. It is an explicit diagnostic,
not part of deterministic CI's passing-test command.
