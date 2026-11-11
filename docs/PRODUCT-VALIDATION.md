# Product validation — November 12, 2026

**Status: a locally running MVP with verified core flows.** The evidence below
separates fresh paid AI generation from deterministic product checks. This is a
record of executed local tests, not an external deployment or production sign-off.

## Fresh AI-generated journeys

Both trials used the application's OpenAI adapter, `gpt-6-luna` and
`composer.v2`. The browser submitted the brief, displayed the proposal and its
warnings, and applied the reviewed output to a Draft. Persisted graphs matched
the proposals exactly, without manual graph edits. Validation, sandbox scenarios,
Test Inbox rendering and reload persistence were checked through the application.

| Trial                          | Generated graph     | Provider attempts | Scenario result | Input / output tokens | Composer HTTP time |
| ------------------------------ | ------------------- | ----------------- | --------------- | --------------------- | ------------------ |
| Abandoned cart recovery        | 17 nodes / 18 edges | 2                 | 10/10 PASS      | 7,094 / 6,657         | 47.169s            |
| SaaS onboarding and activation | 19 nodes / 19 edges | 1                 | 10/10 PASS      | 1,954 / 4,141         | 38.219s            |

- [Recovery report](validation/luna-live-2026-11-12/README.md): exact brief,
  accepted output, automatic repair accounting, screenshots, assertions,
  personalization, consent, conversion exits and reentry checks. The first
  candidate failed application validation; its exact rejected body and validation
  issues were not retained. The accepted candidate was applied unchanged.
- [Onboarding report](validation/luna-onboarding-2026-11-12/README.md): exact
  brief, output, screenshots and ten cases covering activation events, profile-only
  updates, channel consent, terminal timing and once-only enrollment. It used
  General workflow mode with explicit reviewed cases; there is no automatic
  onboarding-specific behavior contract.

Total reported usage is **19,846 tokens** across three provider requests:
9,048 input and 10,798 output. Reasoning tokens are already included in output.
Composer time is the generation HTTP round trip, not the entire review/test
session. Each linked report records the full timing boundaries and limitations.

Both journeys remained Draft with no published versions, live customers,
production executions or delivery jobs in their isolated trial workspaces.
Inbox messages were SIMULATED; neither Publish nor Send Test was invoked.
The trials made paid model calls but no real email/SMS deliveries.

## Deterministic product verification

The [implementation tracker](IMPLEMENTATION.md) and
[AI remediation evidence](AI-REMEDIATION.md) record the preceding executed checks:

| Area                     | Recorded local evidence                                                                                                                         |
| ------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| Code quality             | `make check`: formatting, lint, typecheck/build, Go race/unit and Node tests passed                                                             |
| Database and runtime     | `make test-integration`: isolated PostgreSQL suites passed, including event deduplication, concurrency, acceptance gating and sandbox isolation |
| Browser flows            | 48/48 desktop/mobile cases passed, including canvas persistence, review navigation, explicit publishing and token display                       |
| Recovery                 | Actual worker stop/start preserved a durable wait; duplicate ingestion created no second execution                                              |
| Channel adapters         | Local Mailpit SMTP and mock SMS exercised; provider acceptance is not confirmed external delivery                                               |
| Publication safety       | Failing required acceptance cases block publication; published versions remain immutable                                                        |
| Monitoring and analytics | Execution tracking, workspace boundaries, sandbox metric isolation and persisted metric counts covered by recorded checks                       |

The original recorded recovery output passed only 4/10 cases. It remains a
historical failure; the separately reviewed correction and the fresh live
recovery generation each passed 10/10. These are different artifacts, not a
rewritten result. See the [audit and corrections](AI-REMEDIATION.md).

## What these results establish

A user can generate a journey, review and apply it, edit the canvas, validate it,
run isolated virtual-time scenarios and inspect generated messages. Separate
local deterministic checks exercise controlled publishing, persistent execution,
restart recovery and observability. AI proposes; a person applies and publishes;
the runtime follows the saved workflow.

Remaining limits:

- These two live model outputs and twenty scenario runs do not guarantee all
  future briefs will be generated correctly. Acceptance review remains necessary.
- Both live trials used OpenAI. Claude has a shared adapter and deterministic
  tests, but equivalent fresh live Claude generation is not demonstrated here.
- Live trials did not publish or exercise real external recipient delivery.
  Production infrastructure, credentials and deployment are outside this report.
- Earlier load measurements have their own revisions and limits; these trials
  are not a new performance benchmark. See [performance evidence](PERFORMANCE.md).
- Local checks are not a claim that the latest remote CI run passed.

This report adds documentation only. No new model call, application change or
deployment was made while preparing it.
