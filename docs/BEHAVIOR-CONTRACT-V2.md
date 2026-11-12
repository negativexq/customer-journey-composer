# Scoped business behavior contract v2

A journey's business obligations must have a declared scope. A message count from
one example cannot stand for every path. Version 2 separates reviewed rules,
situations/event sequences and expected effects from the generated workflow.
There is no new journey-name registry or repair for individual audit fixtures.

## Contract boundary

`Workflow.behavior_checks.version=2` is human-owned input metadata, separate from
`composer.v5`, the executable graph output schema. The model cannot create or
replace this contract. The same representation is used by both provider adapters,
creation review, Apply, draft validation and publication. The runtime still
executes the validated immutable graph; it does not execute contract clauses.

A contract has one main clause and up to seven alternative clauses. Each declares:

- A situation name and optional initial predicates, using workspace field labels
  in the UI. Predicates are bounded scalar AND conditions.
- Channel consent for that situation and an ordered sequence of subsequent events
  with relative timestamps. No events means none occur, including during timeout.
- Accepted email/SMS counts, terminal outcome and optional total duration.
- Private example inputs/event properties and optional expected profile attributes.

Alternative inputs and consent override the main example. Every clause is
validated independently against the shared simulator. A matched event's count of
one and a timeout's count of zero remain separate obligations. Those numbers do
not authorize removing the brief's other branches.

Eligibility thresholds are business rules rather than example counts. Version 2
requires an explicit `ineligible_outcome` when thresholds are present. Rejected
boundary probes must have that outcome, send no accepted messages and leave the
customer's attributes unchanged. All eligibility thresholds are evaluated together.
The numeric checks still use exact and adjacent representable values.

Initial predicates must agree with the private example before any provider call.
Two clauses giving incompatible counts/outcomes/durations or overlapping expected
attribute values for identical initial state and event sequence are rejected
before generation. This is a concrete contradiction check, not a general logical
satisfiability proof for every possible customer state.

## Event identity relationships

An event clause may explicitly declare:

```json
{
  "event_name": "process_finished",
  "after_seconds": 600,
  "entry_binding": {
    "entry_path": "event.properties.entity_id",
    "event_path": "event.properties.entity_id",
    "operator": "not_equals"
  },
  "properties": { "entity_id": "SYNTHETIC-OTHER" }
}
```

`equals` describes the same object as the entry; `not_equals` describes a different
object. Both private identities must exist and be valid nonempty scalar identifiers.
Missing identity does not mean “different object”. Their relationship is checked
before generation and included in the public contract; the identity values are
not sent to the model.

This describes a contract requirement, not a new runtime feature. Execution still
supports its existing global-exit correlation vocabulary. A contract that cannot
be implemented by supported nodes must fail validation rather than imply that an
unsupported dynamic filter has been implemented.

## Provider data and bounded diagnostics

Providers receive only explicitly reviewed predicates, channel consent, event
names/timing/identity relationships and the expectations for each clause. Private
example inputs, event payloads and expected attribute values stay on the server.
Unspecified initial values are private-example context, never universal rules.
Legacy v1 counts have no public situation scope and are no longer sent as global
instructions. The brief's requirements remain authoritative outside the clauses.

Rejected attempts record bounded **schema/workflow/message/behavior issue counts**
alongside usage. No rejected candidate, arbitrary validation message, customer
value or node label is added to audit accounting. This identifies the failing
validation category without retaining private output. It does not reconstruct
previous rejected paid responses.

## UI, persistence and compatibility

The brief/editor supports situation names, initial conditions, consent, subsequent
events, private payload examples and alternative obligations. Review shows the
separate expected outcomes. Editing any clause clears confirmation; incomplete
contracts can be saved and resumed. Optional `expected_attributes` is supported by
the typed API and validator; the current form does not provide a dedicated result
attribute editor.

New forms start at v2. Saved v1 contracts remain v1 until explicitly upgraded and
reconfirmed. Existing published snapshots and their semantics are unchanged.
No table migration, new runtime query, cache or service is introduced.

The full metadata stays within 16 KiB. There are at most eight clauses, eight
predicates and eight timed events per clause, and four eligibility thresholds.
Validation runs at most **20** bounded simulations (eight clauses plus twelve
boundary probes). This work occurs in design validation, not the execution worker
or analytics read path.

## Verification and remaining limits

Deterministic tests use generic event/field names and both email/SMS to verify
matched-event vs timeout behavior, initial-state alternatives, wrong-channel
rejection, exact ineligible outcomes, no rejected-input mutations, same/different
object scope, contradictory input rejection, privacy and diagnostic categories.
HTTP/PostgreSQL tests enforce saved independent clauses at approval/publication;
browser coverage checks editing, persistence, review and publish blocking.

Local verification on November 13, 2026:

- `make check`: format, lint, typecheck/build, race-enabled Go and 10 Node tests passed.
- `./scripts/test-integration.sh ./internal/httpapi`: isolated PostgreSQL checks passed (85.999 s), including v1/v2 approval and publication enforcement.
- `npm run test:e2e -- --workers=2`: 78/78 desktop/mobile browser cases passed.
- Docker API/worker/web rebuilt and ready; scoped contract form screenshot inspected locally without a provider call.

The contract verifies the obligations actually declared and represented by its
examples. It does not automatically extract a complete specification from arbitrary
natural language or prove that all possible paths are covered. Human review and
additional Test Lab exploration remain necessary. No fresh paid generation is
claimed by this correction, and the earlier paid outputs/results remain unchanged.
