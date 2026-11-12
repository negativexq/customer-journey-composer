# Workspace vocabulary and brief grounding

The semantic layer translates business language into explicit workflow references
at design time. A marketer can write “When a customer activates their account,
stop the reminders” without naming a database column. Administrators describe
what the workspace's actual events and fields mean in **Settings → Workspace data**.
Both AI providers receive the same server-loaded metadata catalog.

## User flow

1. Define events, custom profile fields and event fields. Give each a business
   name, optional synonyms, explanation and scalar type. Definitions can document
   a planned integration, but do not create events or populate customer data.
2. Alternatively, explicitly discover recent data. Review the imported names and
   types, then edit their meaning. Discovery preserves curated definitions and
   disabled entries; conflicting observed types are marked mixed.
3. Write an ordinary business brief. A workspace with no enabled event definitions
   is rejected before any provider call. Briefs, workflow specifications and catalog
   metadata are sent; discovery never sends customer values to providers.
4. Review the diagram and **Data used by this plan**. Unknown/disabled references,
   conflicting field types and incompatible scalar comparisons block approval.
   Shared business labels or aliases require an explicit selection. A fallback
   value does not establish that a custom field exists.
5. Choose the intended workspace definitions. Mapping changes update event names,
   rules, message variables, fallbacks and attribute writes together. They do not
   modify customers. No model call is needed. Choices persist with the creation
   draft and survive refresh. Changing an entry event may require a second field
   mapping because event fields are scoped to an event.
6. Approve the plan to open the editable draft. The server recomputes grounding
   from its own catalog under the approval transaction; a client-supplied report
   is never authoritative. Publishing and acceptance tests remain separate.

The builder's AI panel also presents and gates its proposed changes with the same
mapping review. Manual workflow editing remains available: the catalog is not a
runtime authorization system or a restriction on editing an integration contract.

## Data and API

Migration `0021_data_catalog.sql` introduces `workspace_data_catalog`, keyed by
`(workspace_id,key)`. A key identifies an event, custom profile field, or field of
a specific event. Enabled built-in contact/consent fields are supplied separately.
Custom profile paths use `customer.attributes.*`; event fields use
`event.properties.*`. Context-only references require a workflow correction; they
cannot be invented as customer attributes. Technical details are collapsible in
Review. The deterministic report has version `semantic.v1`; provider structured
output remains `composer.v2`, with the report computed by the server afterward.

Authenticated endpoints:

- `GET /api/v1/data-catalog`: current workspace metadata.
- `PUT /api/v1/data-catalog`: bounded upsert; Editor/Admin only.
- `POST /api/v1/data-catalog/discover`: explicit bounded discovery/import.
- `POST /api/v1/data-catalog/resolve`: analyze or remap a proposed workflow using
  enabled definitions of the same kind and event scope; never saves a journey.

Writes serialize with `FOR NO KEY UPDATE` on the workspace row (compatible with
runtime foreign-key `KEY SHARE` checks), enforce a 1,000-entry cap and audit counts
without storing catalog contents in audit details. Approval holds a shared
workspace lock so a catalog edit cannot race its grounding check. Existing
published versions and running executions retain their compiled references.

## Cost and performance limits

There are no catalog reads, AI calls, discovery scans or new indexes in ingestion,
worker, provider-dispatch or analytics paths. Normal catalog access is an indexed
workspace lookup. Discovery runs only on an explicit request: at most 100 recent
events and 100 recent customers, using the existing recent-row indexes, with a
five-second deadline. Payloads over 32 KiB are skipped; scalar leaves are extracted
to depth four, at most 100 keys per object and 500 definitions per discovery.
Arrays and null-only fields are not inferred. This is a sample, not a complete
historical schema inventory. No values or sample payloads persist in the catalog.

Enabled provider metadata is capped at 96 KiB; oversized catalogs must be narrowed
or descriptions shortened before generation. Candidate suggestions are bounded;
the Review selector can inspect the full enabled catalog. The existing two-attempt
provider bound remains unchanged. No vector database, embedding calls, additional
service or runtime dependency is introduced.

## Deliberate limits

Exact paths, labels, synonyms and types ground the model; they do not prove that
it understood every business requirement. A model can omit a requested behavior
or choose an available but unintended definition. Human review and behavior tests
remain necessary. Matching references are not a claim that a planned integration
is deployed or that every customer has a value. Consent and frequency policies
remain enforced by the existing runtime.

Condition/message event properties are checked against the entry event's declared
fields; incoming waiter/global-exit filters use their own event scope. This does
not implement full data-flow type inference across branches where the runtime's
latest event can change. Complex arrays, dynamic context, computed business
metrics, connectors and automatic schema drift alerts are follow-up work.
