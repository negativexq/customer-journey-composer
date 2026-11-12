# Five additional live journey types — 2026-11-12

**15 cases passed, 5 failed and 5 were blocked by rejected generation.**
Five fresh briefs produced four testable proposals; the support escalation request
was rejected after both bounded attempts. Of the 20 executed sandbox cases, 15
passed. The denominator is **25 planned cases**, not 25 executed cases.

[Raw briefs, catalogs, pre-generation independent expectations, planned cases, proposals, usage, Inbox outputs and traces](../validation/five-new-journeys-2026-11-12/result.json).

## Method

Source `634dac5` (application behavior from `ec017b1`), local Docker API, OpenAI
`gpt-6-luna`, `composer.v5`. The user explicitly authorized five more different
scenarios. All five journey types are new to the earlier ten-type audit and the
previous five-brief follow-up: stock availability, referral rewards, support
escalation, membership recovery and course completion.

Each used a fresh fictional workspace and business-language brief with a synthetic
catalog. Independent expectations and all five per-journey case inputs were fixed
before generation. Counts, outcomes, durations, numeric bounds and example values
were not derived from the generated graph. Synthetic profile values remained out
of provider input through the application's existing redaction.

Final graphs, edges and warnings were inspected before approval. The stock email
omission and referral outcome mismatch were known at that point; those drafts
were deliberately approved only for isolated negative testing. No graph or message
was edited, and no publication was attempted. The rejected support request was
not regenerated beyond the normal two-attempt limit.

## Results and accounting

| Journey                         | Cases                            | Generation seconds | Attempts | Input tokens | Output tokens | Total tokens |
| ------------------------------- | -------------------------------- | -----------------: | -------: | -----------: | ------------: | -----------: |
| Stock availability notification | 2 PASS / 3 FAIL                  |              20.89 |        1 |        2,664 |         2,157 |        4,821 |
| Referral reward eligibility     | 3 PASS / 2 FAIL                  |              27.80 |        2 |        7,254 |         3,684 |       10,938 |
| Support ticket escalation       | 5 BLOCKED                        |              48.97 |        2 |        8,206 |         6,874 |       15,080 |
| Membership recovery             | 5 PASS                           |              22.94 |        1 |        2,777 |         2,810 |        5,587 |
| Course completion               | 5 PASS                           |              41.27 |        2 |        7,883 |         5,270 |       13,153 |
| **Total**                       | **15 PASS / 5 FAIL / 5 BLOCKED** |         **161.87** |    **8** |   **28,784** |    **20,795** |   **49,579** |

Total wall time was **202.89 seconds**, including setup, review wait and tests.
Summed generation request time was **161.87 seconds**. The 20 sandbox case
operations summed to **0.257 seconds**; this sequential local check is not a load
benchmark. All eight provider completions returned HTTP 200 and reported usage;
the application accepted four final proposals and returned HTTP 422 for support.
Two proposals passed on the first attempt, two after repair, and one was rejected.

The rejected support request's **15,080 tokens are included** in the total.
Reasoning counters are already within output usage and are not added again.
These are provider-reported token counts, not a billing-price calculation.

## Confirmed failures

### Stock: the representative timeout case hides the positive branch

The reviewed example had `in_stock=false`, no subsequent stock event, zero emails
and completion at 72 hours. That expectation is correct for the timeout path.
The final graph passed that example, but contained **no email node at all**.
Both the in-stock branch and the matched-event branch led directly to completion.
The model explicitly warned that it omitted email steps because the reviewed
`email_count` was zero despite the brief asking for email.

Consequently, stock arriving at +6h, stock already available at entry and duplicate
stock notifications all failed their expected one-email assertion. Timeout and
no-email-consent cases passed. The independent example did not describe or test
the positive event path. The current provider summary supplies counts/outcome/time
without the synthetic example's state or event sequence, and the model applied
that path-specific zero count across the graph.

The final graph was not silently repaired for this report. After all tests, its
publication check returned `ready=false` with **3 failed acceptance scenarios**.

### Referral: numeric bounds work; the ineligible exit reason is wrong

Exactly three referrals with reward cost 200 qualifies. Cost 200.001 and two
referrals correctly produce no message and do not mutate reward attributes.
However, both ineligible paths end as `completed`, whereas the brief and external
cases require `condition_not_met`. These two exit-reason assertions failed.

The valid path, attribute update without email consent and duplicate-event
counter protection passed. This is not another inclusive-boundary arithmetic
failure: the generated rules correctly use `>=3` and `<=200`.

The generic checker tests ineligible boundary probes for no accepted messages and
no conversion; it does **not** require a particular ineligible exit reason. Its
eligible example therefore passes while this analytics-relevant classification
error survives generation. After the external cases, publication returned
`ready=false` with **2 failed acceptance scenarios**.

### Support: generation is safely rejected, but detailed diagnostics are missing

Both provider responses succeeded at the transport level and neither candidate
was accepted. The API returned the existing generic “did not pass schema and
workflow validation after two attempts” error. No proposal could be approved, so
all five behavior cases remain blocked rather than reported as pass or fail.

Current audit accounting retains usage and accepted flags, but not rejected
candidate graphs or per-attempt validation issues. This evidence cannot establish
which schema, workflow, message or behavioral rule caused rejection. No additional
paid request was made to guess or reconstruct the cause.

The planned cases remain in the raw artifact: unresolved ticket escalation,
matching resolved event, unrelated ticket event, profile-only resolution and no
SMS consent.

## Passing journeys

**Membership recovery: 5/5.** Seven-day timeout produces the second email; a
restored-membership event converts at +48h; account deletion exits normally at
+24h; a profile-only restoration at hour 167 converts at the timeout; a 30-day
frequency limit of one blocks the second email with recorded `frequency_cap`
reason while the journey completes. Waiting for an event, timeout branching and
publication prerequisites passed for these supplied cases.

**Course completion: 5/5.** Low progress receives email then SMS; exactly 50%
progress skips SMS; completion of the entry course converts at +12h; another
course's completion does not exit the journey; the SMS path remains available
without email consent. The model generated explicit `course_id` correlation.

## Isolation, publication and limits

Actual database readback across **all five** audit workspaces, including rejected
support, found one draft journey each, zero published versions, zero live
executions, zero live customers and zero delivery jobs. There were 20 sandbox runs
and none in the rejected workspace. Accepted workflows were read back unchanged;
Inbox records were scoped to each simulated run. No real message or deployment.

Stock and referral remained blocked by their failing Test Lab cases. Membership
and course publication checks were ready but were not published. A warning alone
is still insufficient to establish arbitrary brief compliance. One independent
example cannot cover alternative branches or specify every rejected outcome.

The previous 25/25 result remains valid for its recorded five outputs and cases;
this new result demonstrates its generalization limit. Neither run measures live
Claude quality or production concurrency. Temporary URLs, real integration/provider
configuration and invoice/course profile scoping still require review for real use.

Next corrections should give expected results explicit scenario scope, cover at
least positive and alternate event paths, require the reviewed ineligible outcome
where specified, and retain bounded sanitized validation diagnostics per attempt.
No application fix is claimed by this measurement.
