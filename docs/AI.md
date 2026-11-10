# AI composer contracts

The AI layer proposes draft changes. It cannot save a journey, publish a version, change consent/cap/provider rules or execute arbitrary nodes. The server never loads customer profiles or event payloads for generation. Only the explicit prompt and workflow draft are sent; single-node/message actions send only the selected node.

## Providers and configuration

Two server-side HTTP adapters implement the same interface:

- OpenAI Responses API with `text.format`, strict JSON Schema, bounded output tokens and `store: false`. See [official OpenAI structured output documentation](https://developers.openai.com/api/docs/guides/structured-outputs?api-mode=responses).
- Claude Messages API with `output_config.format` and the same schema. See [Claude structured output documentation](https://platform.claude.com/docs/en/build-with-claude/structured-outputs).

Set `OPENAI_API_KEY`/`OPENAI_MODEL` and/or `ANTHROPIC_API_KEY`/`ANTHROPIC_MODEL` in the ignored server environment. Choose a model available to your account that supports structured output; the application does not silently substitute model IDs. `AI_PROVIDER=disabled` is the default. Set it to `claude` or `openai` to enable live generation and choose the initial UI selection; a configured alternative remains selectable. Recreate the API container after environment changes.

Docker gives AI credentials only to the API, and SMTP credentials only to the delivery worker. Keys, raw provider error bodies and prompts are not logged or returned by settings. Provider requests cannot redirect to a different endpoint. Refusals and incomplete responses are explicit failures. Ambiguous transport failures are not automatically retried; explicit transient HTTP failures can retry once, respecting short Retry-After values. There are at most two model attempts total, including graph repair, and one in-flight AI request per workspace.

## Versioned output and validation

[composer.v1.json](schemas/composer.v1.json) is the shared journey output contract. Tests compare its content to the Go schema generator. Generate it with:

```sh
cd apps/api
go run ./cmd/schema > ../../docs/schemas/composer.v1.json
cd ../..
npm run fmt
```

The provider-compatible subset uses closed objects with required properties, typed per-node configuration, explicit nulls and non-recursive rule definitions. Claude does not accept recursive schemas, so AI condition groups are unrolled to two group levels. The human editor and execution validator support eight levels. Fallbacks cross the AI boundary as `{path,value}` arrays and normalize to the execution schema's map; this keeps arbitrary fallback paths compatible with closed structured-output objects. Scalar attribute/condition values are supported by the AI schema. The runtime schema permits broader human-authored JSON values.

Outputs pass local JSON Schema validation, typed decoding, graph validation and message syntax/sanitization validation. Applied migrations, runtime logic and allowed node types are outside the model's control. Oversized prompts/specifications and responses are rejected. Invalid output gets one bounded correction attempt and is never exposed as an applicable proposal.

The response includes added/removed/updated nodes, condition/message changes, connection changes and exit/reentry changes. Explain actions do not propose mutations. Message actions deep-copy the input and can improve a node in an incomplete draft without pretending the graph is publishable.

## Human approval

The panel captures the current draft, shows the proposal and warnings, and offers Apply/Discard. Warnings require acknowledgement. If the human edits the draft while a proposal is open, Apply is disabled until the proposal is discarded/regenerated. Apply updates the editable draft and its ordinary revision-guarded autosave. Publish remains a separate Admin action.

## Verification status

No live LLM request has been made, as explicitly requested by the user. Automated tests use local HTTP fixtures to verify both provider wire contracts, strict schema handling, malformed/unsupported outputs, one-repair limits, refusals, input immutability and fallback normalization. Browser tests use clearly named contract fixtures to verify discard, acknowledgement, apply, stale-proposal rejection and absence of automatic publication.

Live model access, schema acceptance by the actual provider, natural-language quality, model-specific latency and the final live prompt-to-journey demonstration remain deferred. Enable and evaluate those only when live LLM testing is requested; deterministic tests and normal development do not require API keys.
