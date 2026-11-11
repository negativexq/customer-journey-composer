# AI acceptance corrections — 2026-11-12

Addresses the [focused audit](AI-WORKFLOW-AUDIT.md). No additional paid generation
was used. The original Luna proposal and failing report remain unchanged.

## Implemented

- `e1d2cfb`: publication uses the latest full acceptance result per scenario for
  the exact workflow. All saved required cases must be run on the current draft;
  failed/missing/pending required cases block publication. Replay supersedes an
  earlier result for the same snapshot. Saved optional cases and exploratory
  runs have explicit behavior; changes to saved requirements are audited.
  Overriding a saved scenario cannot satisfy its original requirement. Migration
  0019 adds sandbox acceptance metadata, database-derived identity and scoped
  indexes. Existing worker jobs/versions are untouched.
- `6b51dbd`: `composer.v2` restricts newly AI-produced exit codes and documents
  their runtime meaning. Manual custom reasons keep their semantics. A selected
  `cart-recovery-eur.v1` behavior contract is included in review, AI validation,
  API validation and pre-launch checks. Missing purchase/profile/consent decisions
  trigger bounded repair or rejection. General workflows warn that structural
  validity alone is not brief compliance.
- `2abce2c`: the contract additionally checks exact recovery scheduling, rendered
  name/amount, coupon and changed personalization inputs. Its 14 deterministic
  cases include the original ten plus currency, SMS-only, cooldown expiry and
  changed profile/cart data. No provider/network calls are made by these checks.
- `be822eb`: OpenAI and Claude adapters preserve reported usage, actual/requested
  models, duration and request IDs. All attempts, including rejected candidates
  and repairs, are accounted for. Unknown totals stay unknown. Sanitized metadata
  is stored in workspace audit logs on both success and failure and shown during
  review; prompts, bodies, private reasoning and secrets are excluded. Mocked
  provider tests verify usage, cache and repair aggregation. The historical smoke
  usage that was discarded cannot be recovered by this fix.

## Actual local results

The [corrected fixture](../tests/fixtures/ai/luna-recovery-corrected.json) is an
explicit reviewed edit of the recorded proposal, **not a newly generated Luna
response**. It adds the early consent gate, first/final profile purchase checks
and canonical completion codes while retaining the original message content.

| Candidate                        | Business scenarios | Publication check              | Actual publication |
| -------------------------------- | ------------------ | ------------------------------ | ------------------ |
| Original recorded Luna output    | 4/10 PASS          | Blocked: 6 acceptance failures | Not called         |
| Reviewed correction              | 10/10 PASS         | Ready after acceptance         | Not called         |
| Canonical reference, same inputs | 10/10 PASS         | Control                        | Not called         |

[Machine-readable evidence](validation/luna-remediation-results.json). Both replay
journeys remain Draft with no versions, live customers/executions or delivery
jobs. Sandbox activity does not change execution/conversion/channel metrics.
Creating the two fixture drafts legitimately changes asynchronous journey-count
metadata; the replay isolates that setup effect from sandbox metric checks.

Verification run locally:

- `make check`: formatting, lint, typecheck/build, Go race/unit and Node tests pass.
- `make test-integration`: all package-isolated PostgreSQL tests pass, including
  acceptance-gate regressions and usage persistence; no paid provider is called.
- Browser suite: 48/48 cases pass on desktop/mobile, including token display and blocked
  acceptance review navigation. The added case verifies failed tests cannot be
  hidden by a passing smoke and Open Test Lab navigates to the correction view.
- `node scripts/test-luna-recovery.mjs --corrected`: 10/10, zero new model calls.
- Original fixture replay remains an intentional acceptance failure (exit 1),
  with readiness false. Assertions were not weakened to turn its six failures green.
- `make test-recovery`: an actual local worker stop/start preserves the durable
  wait and duplicate ingestion creates no second execution.

Code/schema/unit changes do not establish that a fresh future model response
will always satisfy every brief. The behavior contract checks bounded explicit
requirements; other journey types still require reviewed acceptance cases.
Further live generation remains deferred. No external deployment was performed.
