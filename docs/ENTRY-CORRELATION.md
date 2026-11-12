# Entry-object correlation — implemented 2026-11-12

Purchase goals can now explicitly require the incoming event to refer to the
object that started this execution. This corrects the platform limitation found
in the [cart edge audit](reports/live-cart-edges-2026-11-12.md) and the
[ten-journey audit](reports/varied-journeys-2026-11-12.md). It does not silently
reinterpret historical customer-wide goals or repair every model-generated draft.

## Specification and behavior

```json
{
  "global_exit": {
    "event_name": "invoice_paid",
    "reason": "converted",
    "correlation": {
      "entry_path": "event.properties.invoice_id",
      "event_path": "event.properties.paid_invoice_id"
    }
  }
}
```

Paths can differ between the entry and incoming event. They must be bounded,
valid event property paths. Equality uses non-empty string identities or numeric
identities; missing, null, blank, object, boolean or incompatible types do not
match. There is no expression execution, template evaluation or coercion of
string "1" into numeric 1. Correlation and existing global-exit filters must both
pass for global exit. Other conditions retain their existing semantics.

An explicit correlated goal also scopes Wait for Event nodes listening for that
**same goal event**. Otherwise an unrelated purchase could fail global exit but
still take a waiter's matched path. Other wait events retain their existing
filter behavior; this is not a general per-node event-correlation feature.

No correlation (including old published definitions) preserves the original
customer-wide event behavior. Published versions remain immutable. A human must
explicitly select correlation in a new draft/version; this change does not edit
old definitions, existing conversion counts or active execution snapshots.

## Runtime and data access

Production reads the original entry event's persisted properties, scoped by
workspace and execution entry-event ID. The global-exit query already joined that
record for event ordering. It now projects properties only when correlation is
configured. The waiter query conditionally uses the same indexed identity lookup.
There is no extra per-execution application/database round trip, new table,
migration, reporting scan, Redis state or language/service rewrite. This access
path description is not a new 10,000-execution performance measurement.

Event handling retains customer/execution locks and cancellation fences. An
unrelated event leaves timers and waiters alone; a matching event follows normal
atomic exit/cancellation behavior. The simulator uses the same comparison helper,
persists the entry properties separately from the current/resumed event and resets
that identity on enrollment. Its state round-trip test ensures a current event
cannot replace the stored entry identity.

## Editor and AI

Journey settings exposes **Exit matching** with customer-wide versus entry-object
choices. Entry/incoming identifier selectors use workspace business labels through
Reka Select. Missing catalog choices direct users to Workspace data; saved unknown
fields remain inspectable. Selected labels also update after asynchronous catalog
loading, fixing stale fallback labels on reload. Review shows the actual exit
outcome and whether entry-object matching is selected.

Semantic analysis includes both identity fields in their separate event scopes;
missing/disabled references and incompatible identity types cannot be confirmed
away. Mapping a field rewrites the corresponding path without mutating the source
proposal. The common Claude/OpenAI schema is **composer.v4**, with explicit nullable
correlation and instructions for object-scoped goals. Historical composer.v3 and
paid-output artifacts remain unchanged. Both adapters' schema paths were tested
with deterministic stubs; no additional paid call was made after the preceding
ten-journey audit. Fresh live v4 generation has not been verified here.

## Verification and evidence

- `make check` passed: formatting, Go vet, ESLint, TypeScript and unit/race tests.
- Isolated PostgreSQL runtime and HTTP API integration packages passed.
- **74/74 desktop/mobile browser tests passed**, including selecting identities,
  persistence/reload, switching back to customer-wide scope and matching versus
  unrelated purchases while waiting for the goal event.
- Unit cases cover matching/different/missing/null/blank/typed/scalar identifiers,
  invalid paths, semantic scopes/types, both provider schema contracts and
  persisted simulation state. Existing legacy and concurrency tests remain in
  the integration package.
- The first integration run overlapped a container build and failed the existing
  `TestBatchedReadsRespectPauseAndLeaseExpiryDuringCustomerLockWait` recovery
  assertion. A fresh isolated rerun passed (runtime 4.45s, HTTP API 57.32s).
  That test was not modified or suppressed; timing sensitivity remains a test
  reliability follow-up. The passing rerun is not represented as remote CI.

[Recorded deterministic replay and traces](validation/entry-correlation-2026-11-12/result.json).
[Inspected settings screenshot](validation/entry-correlation-2026-11-12/settings.png).

Separate reviewed drafts apply only the scoped goal correction to the preserved
paid outputs. **All 15 cases passed**: ten cart edges and five invoice scenarios.
Historical extra message/attempt/fallback checks were also verified against the
persisted inbox records. Correct invoice payment now exits at +12h; another
invoice's payment continues to normal completion. An unrelated cart purchase no
longer converts the cart journey. The original paid proposals were preserved.

- [cart corrected local draft](http://localhost:5173/journeys/ff2bece9-9e1d-4d12-9b28-73c533804c01) — 10 cases passed.
- [invoice corrected local draft](http://localhost:5173/journeys/1adc9310-eb34-4898-a00e-8875ac66ce07) — 5 cases passed.

Both corrected drafts remain unpublished, with zero live executions and no new
live delivery jobs. This fixes event identity, not the known fractional amount
approximation in the original cart trigger. The corrected cart replay does not
include that failing fractional-threshold case, so 15/15 does not claim the whole
cart draft is publish-ready for every original requirement.

## Remaining findings

Wrong channel selection, wrong normal/converted outcomes, approximate >= numeric
thresholds and model-provided overlapping canvas positions remain separate
problems. Object-level profile facts such as a customer's invoice-paid boolean
also require correct integration scoping; matching event IDs does not magically
turn a customer-level boolean into per-invoice storage. Neither graph validity
nor a warning proves brief compliance. The original failing business fixtures
remain preserved for regression.
