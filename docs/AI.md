# AI composer contracts

The AI layer proposes draft changes. It cannot save a journey, publish a version, change consent/cap/provider rules or execute arbitrary nodes. The server never loads customer profiles or event payloads for generation. Only the explicit prompt and workflow draft are sent; single-node/message actions send only the selected node.

## Providers and configuration

Two server-side HTTP adapters implement the same interface:

- OpenAI Responses API with `text.format`, strict JSON Schema, bounded output tokens and `store: false`. See [official OpenAI structured output documentation](https://developers.openai.com/api/docs/guides/structured-outputs?api-mode=responses).
- Claude Messages API with `output_config.format` and the same schema. See [Claude structured output documentation](https://platform.claude.com/docs/en/build-with-claude/structured-outputs).

Set `OPENAI_API_KEY`/`OPENAI_MODEL` and/or `ANTHROPIC_API_KEY`/`ANTHROPIC_MODEL` in the ignored server environment. Choose a model available to your account that supports structured output; the application does not silently substitute model IDs. `AI_PROVIDER=disabled` is the default. Set it to `claude` or `openai` to enable live generation and choose the initial UI selection; a configured alternative remains selectable. Recreate the API container after environment changes.

Docker gives AI credentials only to the API, and SMTP credentials only to the delivery worker. Keys, raw provider error bodies and prompts are not logged or returned by settings. Provider requests cannot redirect to a different endpoint. Refusals and incomplete responses are explicit failures. Ambiguous transport failures are not automatically retried; explicit transient HTTP failures can retry once, respecting short Retry-After values. There are at most two model attempts total, including graph repair, and one in-flight AI request per workspace.

## Versioned output and validation

[composer.v6.json](schemas/composer.v6.json) is the shared journey output contract. v6 adds the wait step's `message_node` (wait for an open or click on an earlier message) and a message's `discount_code`. Tests compare its content to the Go schema generator. Generate it with:

```sh
cd apps/api
go run ./cmd/schema > ../../docs/schemas/composer.v6.json
cd ../..
npm run fmt
```

The provider-compatible subset uses closed objects with required properties, typed per-node configuration, explicit nulls and non-recursive rule definitions. Claude does not accept recursive schemas, so AI condition groups are unrolled to two group levels. The human editor and execution validator support eight levels. Fallbacks cross the AI boundary as `{path,value}` arrays and normalize to the execution schema's map; this keeps arbitrary fallback paths compatible with closed structured-output objects. Scalar attribute/condition values are supported by the AI schema. The runtime schema permits broader human-authored JSON values.

Outputs pass local JSON Schema validation, typed decoding, graph validation and message syntax/sanitization validation. Applied migrations, runtime logic and allowed node types are outside the model's control. Oversized prompts/specifications and responses are rejected. Invalid output gets one bounded correction attempt and is never exposed as an applicable proposal.

Generated message text is then compared with the request, without a model, as in the sister project Smart Campaign Composer ([factcheck](../apps/api/internal/factcheck/factcheck.go)):

- **Refused.** A link, discount code or number that the request, the journey's name and description, and the messages a person already has do not contain is sent back to the model with the reason, like a schema failure. Examples are a different URL, "Son 3 gün!", a bigger percentage or an invented code. A length of time may change unit: "24 saat" passes for "1 gün". A code the request names is set as the step's `discount_code` and written as `{{ offer.code }}`, never typed out.
- **Noted.** A link, code or discount percentage the brief asks for that no message carries is returned in `not_done`. The review page shows it under "In the brief, not in the draft", and approval asks for the usual acknowledgement. These are notes, because only the person knows whether the outcome is acceptable.
- **Scope.** Only steps the model wrote or rewrote in this answer are checked; a person's edits may say anything. The reader knows closed patterns, so it catches these disagreements and proves nothing else. The ten reviewed live journeys of 2026-11-15 pass it unchanged, a test that keeps it from refusing good drafts. A percentage counts as a discount only near a word for one, so "%20'nin altındaysa" beside "indirim oranı" is read as an offer and can raise a note that does not apply.

The response includes added/removed/updated nodes, condition/message changes, connection changes and exit/reentry changes. Explain actions do not propose mutations. Message actions deep-copy the input and can improve a node in an incomplete draft without pretending the graph is publishable.

## Human approval

The panel captures the current draft, shows the proposal and warnings, and offers Apply/Discard. Warnings require acknowledgement. If the human edits the draft while a proposal is open, Apply is disabled until the proposal is discarded/regenerated. Apply updates the editable draft and its ordinary revision-guarded autosave. Publish remains a separate Admin action.

## Independent behavior and verification

General graph generation requires a human-confirmed [scoped behavior contract](BEHAVIOR-CONTRACT-V2.md) (or a saved v1 example) or the curated
acceptance contract before a provider call. Expected channel counts, outcome,
optional duration and exact eligibility boundaries are validated with pure
simulation and rechecked on Apply/approval/publication. Private example values and event payloads
are stripped from provider input; explicitly reviewed predicates and event
relationships describe each clause to the model. Legacy counters are not global
requirements. Rejected attempts retain only bounded validation category counts. [Contract, API change and limits](BEHAVIOR-CHECKS.md).

Automated tests use local provider fixtures; no deterministic suite calls paid
providers. Earlier user-authorized live OpenAI audits and their original failures
are preserved in the [varied-journey report](reports/varied-journeys-2026-11-12.md).
A subsequent authorized fresh composer.v5 OpenAI follow-up passed 25/25 cases
across five different journeys with 47,572 reported tokens; three requests needed
automatic repair. [Outputs, comparison and limits](reports/reviewed-generation-2026-11-12.md).
Live Claude quality and broader prompt coverage remain unverified.

Five additional journey types subsequently produced 15 passing, five failing and
five blocked cases. The new failures show that a representative example does not
cover alternative event paths or require the ineligible exit reason. Support was
rejected after both attempts, with detailed validation diagnostics unavailable.
Failing Test Lab cases blocked publication. [New findings and evidence](reports/five-new-journeys-2026-11-12.md).

An authorized live v2 contract follow-up on those same five briefs passed all
25 unchanged external cases, including variants not encoded as contract fixtures.
Eight attempts used 49,594 tokens and 174.17 seconds of generation; three first
candidates still failed workflow validation and needed repair. No graph edits,
publication or real messaging. [Fresh outputs, comparison and limits](reports/scoped-contract-live-2026-11-13.md).
