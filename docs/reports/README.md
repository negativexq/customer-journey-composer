# Live validation history

Each report records its own source revision, inputs, outputs and limits. Later
fixes and fresh generations do not rewrite earlier failed results. These are
local, targeted evaluations, not universal quality or production readiness claims.

| Date         | Scope                                          | Recorded outcome                                   | Evidence                                                                              |
| ------------ | ---------------------------------------------- | -------------------------------------------------- | ------------------------------------------------------------------------------------- |
| Nov 13, 2026 | Five fresh journeys, scoped contract v2        | 25/25 external cases; 49,594 tokens; three repairs | [Contracts, outputs and comparison](scoped-contract-live-2026-11-13.md)               |
| Nov 12, 2026 | Five additional journey types, v1              | 15 PASS / 5 FAIL / 5 BLOCKED                       | [Stock/referral gaps and rejected support request](five-new-journeys-2026-11-12.md)   |
| Nov 12, 2026 | Five fresh journeys with independent v1 checks | 25/25 cases, previously 16/25 for matched inputs   | [Outputs and accounting](reviewed-generation-2026-11-12.md)                           |
| Nov 12, 2026 | Ten different journey types                    | 41/50 cases                                        | [Channel/outcome/threshold/correlation findings](varied-journeys-2026-11-12.md)       |
| Nov 12, 2026 | Fresh business-language cart brief             | 10/11 cases                                        | [Fractional-boundary failure and publication gate](live-brief-cart-2026-11-12.md)     |
| Nov 12, 2026 | Ten more cart edge cases, no new generation    | 9/10 cases                                         | [Unrelated-cart conversion and other edges](live-cart-edges-2026-11-12.md)            |
| Nov 12, 2026 | Five business-language briefs                  | 22/29 checks                                       | [Earlier semantic audit](semantic-live-five-2026-11-12.md)                            |
| Nov 12, 2026 | Recovery live trial                            | 10/10 cases; one automatic repair                  | [Brief, workflow and raw results](../validation/luna-live-2026-11-12/README.md)       |
| Nov 12, 2026 | Onboarding live trial                          | 10/10 cases; one provider attempt                  | [Brief, workflow and raw results](../validation/luna-onboarding-2026-11-12/README.md) |

The latest live model was OpenAI `gpt-6-luna`. Claude and OpenAI share the provider
interface and deterministic contracts, but equivalent live Claude quality has not
been measured. All these journeys remained Draft; messages used the isolated
simulator. No external deployment was performed.

[Current contract](../BEHAVIOR-CONTRACT-V2.md) · [Implementation tracker](../IMPLEMENTATION.md)
· [Product README](../../README.md).
