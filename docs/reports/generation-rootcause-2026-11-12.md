# Why valid proposals violated their briefs — 2026-11-12

Inspected source `9532fcb` and the recorded ten-journey audit. The main gap is
**brief-to-behavior verification**, not an observed channel conversion inside the
Composer. Three controlled, unpaid stub probes confirm that contradictory but
structurally valid outputs are accepted unchanged on the first attempt without a
selected acceptance contract. A fourth check confirms existing OR rules can
represent the exact >= boundary.

[Probe source](../validation/generation-rootcause-2026-11-12/probe.go.txt).
[Actual test output](../validation/generation-rootcause-2026-11-12/probe.log).
[Recorded paid outputs and scenarios](../validation/varied-journeys-2026-11-12/result.json).

## What the evidence establishes

| Symptom                             | Recorded definition / code path                                                                                              | Why it passed                                                                                                              |
| ----------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------- |
| Email became SMS                    | Webinar `confirmation_email` already has type `sms`, even though its label and copy say email                                | A valid SMS config satisfies the schema; the label and original brief are not a typed channel constraint                   |
| Normal completion became conversion | Loyalty `complete_no_email` and `complete_emailed` explicitly have reason `converted`                                        | Converted is a legal enum value; no general contract ties the terminal outcome to the requested business goal              |
| Threshold rounded down              | Lead uses budget >9999 for a >=10000 brief                                                                                   | Comparator/value are individually legal; no generic check compares the resulting predicate with the brief's exact boundary |
| Overlapping canvas                  | Stored positions are assigned directly to Vue Flow; cards are 224px wide and some generated x positions differ by only 200px | Graph validation checks finite coordinates, not collision/readability; existing Dagre layout is an explicit button action  |

`HTTPProvider.Generate` extracts the returned output text. `Composer.normalize`
converts fallback arrays into maps; it does not change node types, terminal
reasons or rule boundaries. Typed decoding then assigns the workflow directly.
The controlled probes check that the submitted mismatching node type survives
that path. These are saved post-normalization proposals rather than archived
complete provider wire responses; the original model's internal reason for its
choice is not available and is not inferred.

## Verification boundary

`Composer.Compose` performs JSON schema validation, typed decoding, workflow /
message validation and `catalog.ValidateAcceptance`. The latter immediately
returns no issues when no acceptance contract is selected; currently the only
named contract is `cart-recovery-eur.v1`. All ten varied audit journeys had no
contract selected. They were explicitly tested later against independent cases.

After validation reports no issues, the attempt is marked accepted and returned,
with a warning to define business acceptance scenarios. Repair occurs only if
these validations report an issue. It therefore never starts for a wrong but
legal channel or outcome. The lead journey had a separate first-attempt rejection;
its accepted second attempt still contained the imprecise threshold. Schema
repair is not semantic repair.

The workspace semantic layer validates vocabulary, field references, types and
ambiguities. It does not prove that the chosen channel, business goal or timing
implements the natural-language request. The three unpaid probes call the Composer
with a stub and no catalog; they isolate this missing brief-verification step.
The original paid outputs additionally had ready catalog grounding, yet still
failed the independent business scenarios.

## Why the loyalty email disappeared

`converted` produces execution state EXITED, while normal `completed` produces
COMPLETED. Delivery eligibility rejects EXITED executions. The loyalty fixture
queues an email and immediately exits as converted, so the sandbox records the
pending email as SKIPPED / execution_terminated. This is the configured terminal
semantics interacting with asynchronous delivery, not evidence that an email
provider failed. A generated wrong outcome can change both metrics and messages.

## Numeric operators and layout

The exposed comparator vocabulary contains equals, greater_than and less_than,
without direct greater-than-or-equal or less-than-or-equal. This makes exact
translation less convenient, but does not force an approximation: an OR group
of >10000 and ==10000 correctly rejects 9999.99 and accepts 10000/10000.01 in the
probe. Adding explicit inclusive operators is a bounded, testable improvement;
blindly replacing every >9999 with >=10000 would rewrite potentially intentional
workflows and is not proposed.

The application already has Dagre layout with measured card sizes. Its normal
initialization trusts stored positions and adjusts the viewport; it does not run
Dagre automatically. Applying deterministic display layout on first generated
preview/import can remove the model's unnecessary responsibility for geometry.
User-arranged coordinates should have an explicit preservation policy.

## Recommended correction order

1. Add exact inclusive comparators through the shared workflow/operator schema,
   runtime, simulator and editor, retaining old definitions' semantics.
2. Validate channel, normal-versus-goal outcome, predicates and timing against
   **independently reviewed business constraints**. The current graph itself
   must not be the source of its expected test results. Generic templates/saved
   business cases can extend the existing acceptance mechanism; human approval
   remains necessary for ambiguous intent.
3. Feed specific semantic failures into the existing bounded repair loop and
   block unresolved required behavior, rather than merely appending a warning.
4. Use the existing layout engine for generated graph display, with a clear
   distinction between generated geometry and a user's arranged draft.

A second model reviewing its own output is not an independent correctness
contract. Model quality, language, prompt changes and larger models were not
compared in this audit; no claim is made about which model upgrade would remove
these failures. This investigation made no paid call, production-code change,
publication or live send. Findings and a reproducible temporary probe are recorded
without installing tests that treat acceptance of bad output as desired behavior.
