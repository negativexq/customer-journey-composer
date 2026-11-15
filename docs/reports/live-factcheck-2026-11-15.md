# Ten live journeys with the brief fact check — 2026-11-15

The user authorized a second live run, to measure the fact check added in `a3b6cee` on real generations. The local Docker stack ran with `AI_PROVIDER=openai` and `OPENAI_MODEL=gpt-6-luna`. The key was read from the macOS keychain into the `docker compose` process environment only. It is in no file, log or evidence. After the run the API was restarted with `AI_PROVIDER=disabled` and no key. Source revision: `f896fb4` (schema `composer.v6`).

Raw evidence: [result.json](../validation/live-factcheck-2026-11-15/result.json). The harness and the ten scenarios are the same as in the [first run](live-engagement-2026-11-15.md). The harness now also records each proposal's `not_done` notes.

## Results

All ten passed, as in the first run:

- 47/47 Test Lab cases: 32 from the contract and 15 held out.
- Every publish check was ready; every static check passed.
- Database readback for every workspace: 1 Draft journey, 0 versions, 0 live executions, 0 customers, 0 delivery jobs, and only test runs.

| Measure                 | First run (`08311aa`, no fact check) | This run (`f896fb4`, fact check) |
| ----------------------- | ------------------------------------ | -------------------------------- |
| Provider calls          | 13                                   | 14                               |
| Input / output tokens   | 66,091 / 29,060                      | 70,355 / 26,947                  |
| Reasoning tokens        | 14,017                               | 14,151                           |
| Generation, median      | 28.3 s                               | 23.5 s                           |
| Whole run               | 298 s                                | 240 s                            |
| Repairs caused by facts | —                                    | 0                                |
| `not_done` notes        | —                                    | 0 in all ten                     |

Four proposals needed a repair: three for graph validation (winback, review request, webinar) and one for the behavior contract (back in stock). None was for facts. Every final proposal passed the fact check, so every link, code and number in the generated messages was in the brief, and every link, code and discount the briefs asked for was in some message.

## What the run found

- **The fact check cost nothing on these briefs.** It refused no generated text and caused no repair call. The one extra call against the first run is a behavior repair, which is the model's variance between runs, not the check.
- **The webinar email no longer promises an SMS.** In the first run it told customers a link would come "by SMS", which was wrong for customers without SMS consent. This run's email does not; that is one sample, not a fix.
- **The code label follows the test customer's language.** The cart email ends with "Your discount code: SEPET15" under Turkish copy. The harness creates test customers without a locale, so they get the default `en`. A customer with `tr-TR` gets "İndirim kodun". The back-in-stock SMS shows "İndirim kodun" because the model wrote that text itself.

## Limits

- One run per scenario on one model; Claude was not measured.
- The briefs state every fact the messages need, so this run shows the check does not refuse good output. It does not show how often the check catches an invented fact; the paired refusals in `internal/factcheck` and `internal/ai` tests cover that without a provider.
- Engagement events in Test Lab are synthetic.
