# Ten distinct live-generated journeys — 2026-11-12

**41/50 behavior cases passed across ten fresh, different LLM-generated journeys.**
Six journeys passed all five supplied cases; four contained business mismatches.
All ten proposals passed graph/message validation and catalog grounding, showing
why those checks alone do not establish brief compliance.

[Raw business briefs, catalogs, proposals, provider usage, scenario inputs and execution traces](../validation/varied-journeys-2026-11-12/result.json).

## Method

The user requested different journey types before implementing the pending
purchase-correlation fix. Source `3faac99`, local API, OpenAI `gpt-6-luna`,
`composer.v3`. Each journey used its own fictional workspace and explicit synthetic
catalog: these event integrations are test definitions, not discovered production
integrations. Briefs used business language; no customer values were passed to
the model. Credentials/session values were kept only in process memory.

One generation request per brief, with the Composer's normal bounded repair:
**10 requests / 12 provider attempts**. Product email/SMS adapters and live
execution were not invoked; after inspection the proposals were applied only to
editable drafts for real Test Lab API evaluation. Every workflow stayed identical
to the model output. No journey was published. Each case used an independent
sandbox customer and virtual clock. Five cases per journey checked enrollments,
accepted logical channel counts, outcome and time. Relevant cases also checked
customer attribute changes and uniqueness per execution/node. Attribute/time
checks are external audit checks; built-in supported assertions are also recorded.

## Per-journey results

| Journey                        | Cases passed | Generation seconds | Provider attempts | Input + output tokens |
| ------------------------------ | -----------: | -----------------: | ----------------: | --------------------: |
| Hesap aktivasyonu              |          5/5 |              28.20 |                 1 |                 5,811 |
| Deneme üyeliği dönüşümü        |          5/5 |              30.78 |                 1 |                 5,604 |
| Webinar katılım hatırlatması   |          1/5 |              30.58 |                 1 |                 5,691 |
| Başarısız ödeme kurtarma       |          4/5 |              36.74 |                 1 |                 7,033 |
| Teslimat sonrası değerlendirme |          5/5 |              36.87 |                 1 |                 5,921 |
| Randevu hatırlatması           |          5/5 |              22.84 |                 1 |                 4,808 |
| Ürün yenileme hatırlatması     |          5/5 |              49.16 |                 2 |                12,725 |
| Sadakat VIP seviyesine geçiş   |          2/5 |              14.84 |                 1 |                 3,839 |
| İade talebi takibi             |          5/5 |              27.66 |                 1 |                 5,587 |
| Demo talebi niteliklendirme    |          4/5 |              46.90 |                 2 |                13,057 |

## Findings

1. **Webinar: wrong channel, 4 failures.** The node named confirmation email had
   actual type SMS and was guarded by email consent. As a result, expected emails
   were absent and an extra SMS could be produced. Runtime SMS consent still
   applies, so this does not establish sending without SMS permission. The
   email-disabled/SMS-enabled case passed because it skipped that wrong node.
2. **Payment: unsupported object correlation, 1 failure.** The brief explicitly
   required only payment of the entry invoice to end the journey. The model
   correctly warned that dynamic comparison with the entry invoice was not
   supported, and omitted global exit rather than converting on any invoice.
   Consequently, paying the correct invoice at +12h did not immediately convert:
   the journey continued to SMS and completed at +72h. The different-invoice
   case passed by continuing. This reinforces the platform correlation gap;
   a warning is not implementation of the missing requested behavior.
3. **Loyalty: wrong completion outcome, 3 failures.** Eligible VIP updates ended
   as converted although the brief requested normal completion. Pending email
   became SKIPPED after the EXITED/converted outcome, so two cases also lacked
   their expected email. VIP updates themselves were correct. This is the
   generated definition selecting the wrong terminal behavior, with the runtime
   applying its configured exit semantics.
4. **Lead: imprecise >= boundary, 1 failure.** The model explicitly warned that it
   approximated >=50 employees and >=10000 budget with >49 and >9999. The budget
   9999.99 case was incorrectly qualified, updated the flag, received email/SMS
   and completed instead of remaining ineligible. A nested greater-than OR equals
   rule could represent an exact threshold with the existing operators; the
   approximation was unnecessary. The known cart threshold issue generalizes.

The passing journeys cover profile-only goal changes at checkpoints, immediate
business-goal events, channel fallback, normal cancellation rather than conversion,
long virtual waits, set/increment attributes, monetary boundaries at 1000 versus
1000.01 and duplicate-event counter idempotency. This is coverage of these concrete
five-case fixtures, not exhaustive certification of each journey type.

## Accounting and isolation

- **34,324 input + 35,752 output = 70,076 tokens**, provider-reported totals.
  Reasoning is part of output, not added again. Two repairs occurred in product
  replenishment and lead qualification. Raw attempts preserve their separate usage.
- Sum of generation HTTP durations: **324.58 seconds**. End-to-end driver wall
  time: **355.85 seconds**, including setup and the review pause.
- Exactly 50 scenarios executed: 41 PASS, 9 FAIL, zero execution/HTTP errors.
- All ten journeys remained Draft with zero published versions, live executions,
  live customers and live delivery jobs in their isolated workspaces.
- The four journeys with failed engine assertions returned publication not ready.
  The six passing journeys returned ready for their recorded acceptance cases,
  but were not published. Ready does not certify untested business requirements.

No automatic graph correction, extra ad-hoc paid retry or live-provider send was
performed. The pending correlation change follows this audit; original failed
outputs and cases should remain preserved for regression. Other channel/outcome/
boundary findings require their own scoped follow-ups.
