# Independent journey behavior checks

The varied-journey audit found structurally valid workflows with the wrong
message channel, completion outcome or numeric boundary. Graph validity alone
cannot establish that a workflow follows its brief. Original paid results remain
[41/50](reports/varied-journeys-2026-11-12.md); this correction does not rewrite them.

## Review before generation

General journey generation now requires an independent, human-confirmed example:

- Expected entry event, accepted email/SMS counts and terminal outcome.
- Optional virtual completion duration and up to four numeric eligibility bounds.
- A representative synthetic customer/event, using workspace business labels.

The brief screen saves incomplete expectations so creation can resume later.
Changing a field clears confirmation. Example briefs can suggest expected counts
and timing, but users must choose the entry event and confirm them. The curated
EUR cart contract remains an alternative with its existing richer scenario set.
Manual journeys can add checks in Journey settings.

The example has both email and SMS consent and synthetic contact information.
Users review its eligibility and goal attributes; other consent combinations,
purchases after entry, object correlation, retries and frequency caps still need
separate Test Lab scenarios. One example is not a complete behavioral specification
or an automatic interpretation of arbitrary natural language.

## Enforcement

`composer.v5` supports exact `greater_than_or_equal` and `less_than_or_equal`
operators. Older strict comparisons keep their meaning; inclusive rules no longer
need a one-cent or one-unit approximation.

The server checks caller expectations before contacting either provider. It then
attaches those same expectations to the candidate, independently of the model's
output. Pure virtual-time simulation checks channel counts, outcome and optional
duration. Each eligibility bound is tested at the immediately representable value
below it, exactly on it and immediately above it. Ineligible probes must produce
neither accepted messages nor conversion. The curated EUR contract additionally
checks 49.995 EUR as ineligible.

A mismatch enters the existing bounded repair loop: at most two provider attempts
total. A second mismatch fails generation. Apply, creation approval, validation and
publication recheck saved expectations. Unresolved review drafts can be saved for
correction. Existing manual/legacy workflows without this metadata retain their
previous behavior; the feature does not retrofit independent requirements onto
old published versions.

The API's general journey actions now require `behavior_checks` in the request or
saved workflow, or a named `acceptance_contract`. Clients must collect explicit
expectations before generation. Explain/message-only actions do not require a new
example. See generated `/api/openapi.json` for request types.

## Data and performance boundaries

`behavior_checks` is versioned metadata in the workflow JSON and creation state;
published snapshots and workflow hashes include it. The execution engine does not
run these synthetic checks. No database migration, extra runtime query, queue,
cache or service is introduced.

Metadata is limited to 16 KiB, 50 scalar input values and four thresholds. One
validation runs at most 13 bounded simulations. Synthetic input values are removed
from provider requests, including values stored on an existing draft. Providers
receive only the reviewed event/count/outcome/duration/threshold summary. Users
should still avoid putting personal data in their brief or synthetic inputs.

## Canvas layout

New generated graphs use deterministic Dagre positions instead of model-supplied
coordinates. Modification preserves existing user-positioned nodes and places new
nodes without overlap with those positions. This changes geometry, not branches,
conditions or message content. It is not a guarantee that every large graph has
ideal edge routing; manual layout remains available.

## Local evidence

Deterministic tests replay the recorded webinar, loyalty and lead outputs against
independent expectations: the original wrong channel, wrong outcome and approximate
threshold are rejected. Separately reviewed corrections pass. Tests also exercise
bounded repair/rejection, synthetic-data redaction, approval and publication gates,
resumable drafts and generated geometry in desktop/mobile browsers.

Verification completed locally on November 12, 2026:

- `make check`: formatting, lint, typecheck/build, race-enabled Go and Node tests passed.
- `./scripts/test-integration.sh ./internal/httpapi`: isolated PostgreSQL tests passed (72.332 s).
- `npm run test:e2e -- --workers=2`: all 76 desktop/mobile cases passed (1.1 min).
- Docker API/worker/web rebuilt and local readiness passed; generated canvas screenshot inspected.

No new paid provider call was made for this correction. These tests do not claim a
fresh live `composer.v5` generation, equivalent live Claude quality, hosted CI
success or an external deployment.

## Subsequent authorized live measurement

After the deterministic correction, a fresh user-authorized OpenAI composer.v5
follow-up passed 25/25 cases across five journey types (previously 16/25 for those
same inputs). Three requests needed one automatic repair. Total reported usage
was 47,572 tokens and summed generation time 155.92 seconds. No workflow edits,
publication or real messaging. [Raw outputs, comparison and limits](reports/reviewed-generation-2026-11-12.md).
