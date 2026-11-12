# Live measurement with scoped business contract v2 — 2026-11-13

**25/25 unchanged external cases passed across five fresh journeys.**
The previous run on the same briefs/cases produced 15 PASS, 5 FAIL and 5 BLOCKED.
All five final outputs were approved only as editable drafts and tested without
manual graph or message changes.

[Raw briefs, contracts, manifests, proposals, per-attempt usage/diagnostics, external cases and Inbox traces](../validation/scoped-contract-live-2026-11-13/result.json).

## What changed

Application source `b70cbe1`, OpenAI `gpt-6-luna`, graph schema `composer.v5`,
human-owned behavior contract **v2**. Measured locally November 13 in
Europe/Istanbul; raw UTC timestamps are November 12, 21:35:18–21:38:55.

The change is the [shared contract](../BEHAVIOR-CONTRACT-V2.md), not a special repair
for stock, referral or support. Public clauses declare initial state predicates,
consent, subsequent event timing/identity relationships and expected outcomes.
Private examples stay on the server. Numeric eligibility has an explicit rejected
outcome and no attribute mutation. The production validator contains no new
journey-name or audit-fixture branches.

For this measurement, reviewed obligations were defined from the briefs before
generation. There were **17 clauses** across five contracts: stock 4, referral 2,
support 4, membership 3 and course 4. Referral additionally used >=3 valid referrals,
<=200 reward cost and `condition_not_met` for ineligible inputs. Course progress
was correctly a conditional reminder band, not a global entry-eligibility limit.
No obligation was derived from the newly generated graph.

The **25 external case inputs and all five briefs are unchanged** from the prior
[five-type audit](five-new-journeys-2026-11-12.md). They were not simply copied in
full into the new contract. Examples not encoded as contract fixtures included
duplicate availability/entry events, profile-only support resolution and membership
restoration, the workspace frequency cap and the course's SMS-only consent path.
A manifest records contract/case hashes and those external variants.

This is a targeted follow-up on known requirements/cases, not a blinded,
randomized or repeated measurement. Passing it does not prove unseen arbitrary
brief compliance or isolate a statistically causal effect of the contract alone.

## Results

| Journey             | Previous                         | New         | Generation seconds | Attempts | Input tokens | Output tokens | Total tokens |
| ------------------- | -------------------------------- | ----------- | -----------------: | -------: | -----------: | ------------: | -----------: |
| Stock availability  | 2/5                              | **5/5**     |              19.51 |        1 |        2,993 |         2,143 |        5,136 |
| Referral rewards    | 3/5                              | **5/5**     |              30.94 |        2 |        7,226 |         3,170 |       10,396 |
| Support escalation  | 5 BLOCKED                        | **5/5**     |              45.18 |        2 |        7,928 |         4,810 |       12,738 |
| Membership recovery | 5/5                              | **5/5**     |              26.83 |        1 |        3,047 |         2,996 |        6,043 |
| Course completion   | 5/5                              | **5/5**     |              51.72 |        2 |        9,055 |         6,226 |       15,281 |
| **Total**           | **15 PASS / 5 FAIL / 5 BLOCKED** | **25 PASS** |         **174.17** |    **8** |   **30,249** |    **19,345** |   **49,594** |

Generation request time totaled **174.17 seconds**; average **34.83 seconds per
brief**. Full wall time including setup, graph review wait and sandbox checks was
**216.34 seconds**. The 25 sandbox case operations summed to **0.328 seconds**;
this is not a production load or latency benchmark.

Total usage includes all eight provider attempts and their repairs. All returned
HTTP 200 with usage available. Reasoning counters are already part of output
usage and are not added again. Previous usage was 49,579 tokens; the new total is
15 tokens higher. Input/output/cache composition changed, so this does not establish
equal billing cost. Generation was 12.30 seconds longer in this single run; no
latency improvement is claimed.

## What the outputs did

- **Stock:** kept separate already-in-stock and matched-event email nodes, with a
  no-message timeout branch. All three previous missing-email cases passed,
  including duplicate availability events.
- **Referral:** generated exact >=3 / <=200 rules, incremented the reward count
  once, retained the attribute update without email consent and exited ineligible
  customers as `condition_not_met`. Both previous classification failures passed.
- **Support:** produced a correlated normal global exit for the entry ticket,
  waited four hours and rechecked profile state before priority escalation/SMS.
  Same ticket, different ticket, profile-only resolution and no-SMS cases passed.
- **Membership:** retained restoration-event and timeout paths, normal account
  deletion, a profile-only return check and accepted-message accounting under the
  frequency cap. The cap case recorded one blocked second email.
- **Course:** used <50 for the SMS reminder, checked completion after both waits,
  and correlated completion events to the entry course. Exactly 50%, a different
  course and an SMS-only customer passed.

## Rejections still occur, now with diagnostic categories

First-attempt acceptance remained **2/5**. Referral, support and course needed one
automatic repair each. Their rejected first attempts recorded respectively **2,
1 and 2 workflow issues**. Schema/message/behavior counts were zero for those
attempts. This means workflow validation rejected them before downstream behavior
validation; it does not establish that their untested behavior was already correct.

The new accounting identifies the failure category without retaining rejected
candidate content or private error values. Exact rejected graph defects cannot
be reconstructed from these counters. No extra paid request was made to do so.

## Isolation and limits

Final graphs/edges/exits and warnings were inspected before approval. Readback
confirmed every draft equals its original returned workflow. Test Inbox outputs
were scoped to simulated runs. Database readback across all five workspaces found
**25 test runs, zero published versions, zero live executions, zero live customers
and zero delivery jobs**. All final publication checks were ready, but no journey
was published. No external message or deployment occurred.

The model still flagged real deployment/integration limits: temporary URLs,
provider settings, customer-scoped account events without account identifiers and
course profile attributes whose per-course freshness must be supplied by the
integration. Support timing was verified with virtual relative waits, not a real
wall-clock SLA under production queue load.

The result is evidence for these five final outputs, supplied contracts and external
cases. It does not prove a complete contract was automatically extracted from the
brief, every branch is covered, first-attempt reliability improved, production
concurrency is correct under new load, or equivalent live Claude quality. Earlier
paid reports remain unchanged. The contract correction had already passed
`make check`, isolated HTTP/PostgreSQL integration and 78/78 browser tests before
this explicitly authorized paid follow-up.
