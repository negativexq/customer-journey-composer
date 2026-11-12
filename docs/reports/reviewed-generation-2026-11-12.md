# Live generation after independent behavior checks — 2026-11-12

**Five fresh `composer.v5` journeys passed 25/25 sandbox behavior cases.**
The same five briefs and scenario inputs previously passed 16/25. Three fresh
requests required one automatic repair each; two passed on the first attempt.
No generated workflow was manually edited.

[Raw briefs, independent expectations, catalogs, proposals, per-attempt usage, scenarios, Inbox messages and traces](../validation/reviewed-generation-2026-11-12/result.json).

## Method

Source `ec017b1`, local Docker API, OpenAI `gpt-6-luna`. The user explicitly
requested this paid follow-up after the deterministic correction. Selected the
three prior channel/outcome/threshold failures, the invoice-correlation case and
activation as a control. This is a targeted single-run comparison, not a repeated
or randomized estimate of overall model reliability.

Each brief used a separate fictional workspace. Human-reviewed expectations were
fixed before generation from the brief and prior independent test cases: entry
event, accepted email/SMS counts, normal outcome, duration and representative
synthetic inputs. Loyalty also required points >=500; lead required company size

> =50 and budget >=10000. Expectations were not inferred from the new graphs.
> Synthetic input values were excluded from provider requests by the application.

The API performed its normal bounded validation/repair. Final node types, rules,
edges, exit/reentry settings and warnings were inspected before approving an
editable Draft for Test Lab. Twenty-five scenario inputs are identical to the
matching previous audit. No changes to model-produced workflow JSON were made,
including geometry; this measures the API and simulator rather than a new canvas
usability test. The prior 76/76 browser run covers the separate layout behavior.

## Results

| Journey                 | Previous cases | New cases | Generation seconds | Provider attempts | Input tokens | Output tokens | Total tokens |
| ----------------------- | -------------: | --------: | -----------------: | ----------------: | -----------: | ------------: | -----------: |
| Webinar reminders       |            1/5 |   **5/5** |              42.71 |                 2 |        7,188 |         4,941 |       12,129 |
| Loyalty VIP update      |            2/5 |   **5/5** |              21.93 |                 2 |        6,257 |         2,653 |        8,910 |
| Lead qualification      |            4/5 |   **5/5** |              31.06 |                 2 |        7,827 |         5,490 |       13,317 |
| Failed invoice recovery |            4/5 |   **5/5** |              31.78 |                 1 |        2,795 |         4,115 |        6,910 |
| Account activation      |            5/5 |   **5/5** |              28.44 |                 1 |        2,642 |         3,664 |        6,306 |
| **Total**               |      **16/25** | **25/25** |         **155.92** |             **8** |   **26,709** |    **20,863** |   **47,572** |

Total wall time was **183.51 seconds**, including workspace/catalog preparation,
review wait and sandbox checks. Summed generation request time was **155.92
seconds**; average **31.18 seconds per brief**. The 25 local sandbox case operations
summed to **0.324 seconds**; this tiny sequential audit is not a runtime load test.

All eight provider attempts returned HTTP 200 and reported usage. Three initial
candidates were not accepted by validation; the second candidates passed. The
accounting record does not retain rejected candidates or individual validation
issues, so their exact initial defects cannot be reconstructed from this report.
No extra paid retry was made to investigate them.

Token counts include both attempts where repair occurred. Provider-reported
reasoning counts are already part of output usage and are not added again. The
matched earlier run used 35,431 tokens; this run used 12,141 more (+34.3%). Prompt,
schema and sampling differences mean that increase cannot be attributed solely to
behavior validation. This is token accounting, not a billing-price calculation.

## What the cases verified

- **Webinar:** email confirmation, participation profile update, cancellation as
  normal completion, missing SMS consent and email-disabled/SMS-enabled paths.
- **Loyalty:** exactly 500 points qualifies, 499 does not, missing points do not
  qualify, VIP attribute still updates without email consent, duplicate entry
  does not create a second logical action.
- **Lead:** both exact thresholds qualify; 49 employees and budget 9999.99 fail;
  meeting event exits at +12h; email-disabled customers retain the permitted SMS path.
- **Invoice:** unpaid flow, already-paid profile, matching invoice event exits at
  +12h, different invoice event does not exit, and missing SMS consent.
- **Activation:** inactive completion, SMS-only path, no-channel exit, immediate
  activation event and a later profile-only activation check.

Assertions checked accepted logical channel counts, enrollment count, outcome,
virtual completion time and uniqueness per execution/node. Loyalty and lead also
checked profile attribute changes. Test Inbox messages were verified as simulated
and scoped to their run. All five journeys remained Draft with zero published
versions, live executions, live customers and delivery jobs in their audit workspaces.

## Remaining limits

The result validates these five final outputs and 25 scenarios. It does not prove
arbitrary brief compliance, all branches, production concurrency or live Claude
quality. The original ten-journey audit remains 41/50; the other five journey types
were not regenerated in this follow-up.

The invoice model explicitly warned that `customer.attributes.invoice_paid` is a
customer-level field: identifier correlation protects incoming payment events,
but the integration must still supply the intended invoice's status for profile
checks. Webinar and lead cancellation/goal events remain customer-scoped because
their supplied catalogs contain no per-registration/request identifiers. Those
catalog limitations were reviewed and are not hidden by the passing cases.

Temporary example URLs and provider configuration warnings remain unresolved for
publication. No journey was published, no external message was sent and no external
deployment was performed. Local results do not claim hosted CI success.
