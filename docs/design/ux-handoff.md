# Customer Journey Composer — Product UX / UI Handoff

**Status:** Design concept and clickable frontend prototype; not a deployed application.

## Product promise

A marketing team can describe a customer journey in natural language, review the AI's proposal, configure every step visually, simulate synthetic customers without contacting live providers, publish a validated immutable journey revision, and observe aggregate and customer-level outcomes.

**Primary loop:** Journeys → AI Studio → Proposal Review → Visual Builder → Test Lab → Publish / Preflight → Live Monitor → Analytics.

## Inspiration and original references

| Source                                                                                                                             | Reference                           | Adopt                                            | Avoid                                         |
| ---------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------- | ------------------------------------------------ | --------------------------------------------- |
| [Inspora — Infinite FAQ](https://www.inspora.design/posts/infinite-faq)                                                            | AI chat + suggested prompts         | Guided creation, prompt suggestions              | Chat-only UX with hidden side effects         |
| [Inspora — Interactive Cards](https://www.inspora.design/posts/interactive-cards)                                                  | Motion-based data visualization     | Clear data-card hierarchy and micro-interactions | 3D decorative charts for operational data     |
| [Inspora — Session Progress and Recovery Timeline](https://www.inspora.design/posts/session-progress-and-recovery-timeline)        | Timeline + progress steps           | Customer execution trace                         | Wellness-specific visuals and color semantics |
| [External Dribbble — No-Code Email Campaign Flow Builder](https://dribbble.com/shots/26921808-No-Code-Email-Campaign-Flow-Builder) | Workflow canvas and right inspector | Builder layout, editable step inspector          | Copying visual assets or branding             |

Inspora is a curated inspiration gallery, not the author of every referenced work. The external Dribbble concept is not an Inspora post.

## Navigation and route map

| Page / route suggestion | Core content                                                                                             | Primary action             |
| ----------------------- | -------------------------------------------------------------------------------------------------------- | -------------------------- |
| `/overview`             | Live journey summary, conversion and engagement KPIs, recent journeys                                    | Create with AI             |
| `/journeys`             | Filterable list (Draft / Live / Paused / Archived), owner, version, last change, entrants and conversion | Create Journey             |
| `/journeys/new`         | Choose AI-generated, template or blank                                                                   | Generate proposal          |
| `/journeys/:id/builder` | Node canvas + element palette + selected-node inspector                                                  | Save draft / Test          |
| `/journeys/:id/test`    | Synthetic persona selector, virtual clock, detailed trace, Test Inbox                                    | Run Simulation             |
| `/journeys/:id/publish` | Version diff, preflight, warnings, explicit publish action                                               | Publish immutable revision |
| `/journeys/:id/monitor` | Live canvas overlay, per-step metrics and customer run inspector                                         | Inspect run                |
| `/analytics`            | Entry/exit funnel, conversion, messaging reliability, node drop-off, filters                             | Drill down                 |
| `/templates`            | Journey templates and previews                                                                           | Use Template               |

This is a route suggestion; the delivered HTML uses client-side page switching rather than real URL routing.

## Primary interaction sequence

1. **Create**: `Create Journey` offers AI, template or blank. Prompt examples are visible by default. The marketing goal is written in plain language.
2. **AI proposal**: Before touching the draft, render parsed intent: trigger, target audience, branch predicates, wait windows, channels, exits, personalization tokens, and any unknowns. Present a summary of assumptions plus unresolved questions.
3. **Review and apply**: Render an editable schema diff. `Apply to draft` is the only action that mutates the draft. Undo and version history are available.
4. **Build**: A three-column editor shows elements on the left, a pannable/zoomable canvas in the center, and context-sensitive settings on the right. The top bar holds status, version, test, publish, and AI edit.
5. **Test**: Select synthetic profile and event timeline, fast-forward virtual time, inspect per-node path, and preview exactly what email/SMS would render. Distinguish `generated`, `skipped`, `suppressed`, `rejected`, and `mock delivered`.
6. **Preflight**: Validate graph structure, consent and suppression configuration, time-zone policy, missing templates and variables, frequency caps, node exit conditions, duplicate-send protection, and external provider limits. Render hard blockers separately from warnings.
7. **Publish**: Require review of immutable version and confirmation. Publishing doesn't silently overwrite running instances: existing customers follow an explicit versioning policy.
8. **Observe**: Live canvas overlays aggregate entrances, success, waiting and errors per node. A run inspector traces one anonymized synthetic/test customer or an authorized real customer, subject to privacy permissions.
9. **Optimize**: Analytics separates _delivery_ from _business conversion_, supports cohorts and time windows, and allows drilling into failed or low-performing nodes.

## Canvas and node composition

**Visual language:** primarily light theme; soft gray canvas dotted at low contrast; small rounded white cards with thin borders; accent reserved for selected step, focused action and AI suggestion. Use semantic colors for condition, wait, send and failure. Avoid heavy glow, excessive gradients or distracting animation.

**Node inventory:** Trigger (event/schedule), Filter (audience), Condition (true/false), Wait (duration/until event), Email, SMS, Webhook (later), Goal/Conversion, Exit. Cards show icon, kind, label, summary, validation state, and compact execution metrics in Monitor mode. Branch labels must be explicit and mutually intelligible.

**Builder states:** no selection → inspector empty state; selected node → editable fields; invalid node → inline error with field focus; unsaved edits → dirty indication and Save/Discard; production Live version → read-only by default with `Create new draft` action. Existing-running-instance changes must never be ambiguous.

## Test Lab: scenarios and expected outcome

| Synthetic customer | Trigger / events                      | Email          | SMS            | Explanation         |
| ------------------ | ------------------------------------- | -------------- | -------------- | ------------------- |
| Alex               | Abandoned; no purchase through +25h   | Mock delivered | Mock delivered | Follows NO branches |
| Taylor             | Abandoned; purchases at +5h           | Mock delivered | Skipped        | Exits before SMS    |
| Jordan             | Abandoned; opted out of both channels | Suppressed     | Suppressed     | No provider call    |
| Morgan             | Abandoned; purchases before +1h       | Skipped        | Skipped        | Exits early         |

Each run records a chronological trace with node ID, reason, evaluated predicate, virtual timestamp, selected branch, message rendering result, mock provider response and idempotency key (redacted by default). **Never mark a message `sent` when only an HTML preview was generated.**

## UI acceptance criteria

- Every node can be selected; right inspector clearly reflects its type and configuration.
- Nodes remain legible at standard desktop widths; pan/zoom/minimap affordances are discoverable.
- AI generation always has a review step and never automatically publishes.
- Template variables have live previews and explicit fallbacks when unavailable.
- Test Lab works with no real customer data or provider credentials.
- Consent and frequency-cap suppression is visible in run traces.
- Preflight clearly separates simulated visual indicators from actual engine-enforced checks.
- Published journey has a version ID, publish actor/time, and a rollback/new-draft policy.
- Analytics exposes numerators, denominators, attribution window and measurement date range.
- Empty, loading, error, no-permission and zero-results states are designed for each page.
- Primary desktop target 1440px+, tablet read-only/monitor mode; mobile message previews work but canvas authoring may be restricted.

## Engineering integration notes (not implemented in HTML)

Suggested frontend slices: `JourneysPage`, `JourneyWizard`, `AIProposalReview`, `JourneyCanvas`, `NodePalette`, `NodeInspector`, `MessageEditor`, `TestLab`, `TraceTimeline`, `TestInbox`, `PreflightChecklist`, `LiveMonitor`, `AnalyticsPage`.

Suggested types: `Journey`, `JourneyDraft`, `PublishedJourneyVersion`, `JourneyNode`, `JourneyEdge`, `NodeConfig`, `JourneyExecution`, `NodeExecution`, `SyntheticCustomer`, `MessagePreview`, `DeliveryAttempt`, `AnalyticsSummary`.

Canonical graph schema, node validation, and state transitions belong in the backend; frontend must not be a second independent source of truth. A journey may be generated by Claude but should be validated as typed structured data before editing, simulating or publishing.

Provider channel throughput (e.g. <=100 calls/sec) must be controlled by delivery workers, not only UI knobs. Ensure idempotent dispatch and reconcilable retries; external exactly-once delivery cannot be guaranteed unless the provider participates in deduplication. Consent and frequency-cap controls require concurrency-safe decisions at the point of dispatch, not merely when messages enter a queue.

## Suggested implementation order / Conventional Commits

1. `feat(journey-ui): add workspace shell and journeys list`
2. `feat(journey-ui): add visual canvas and typed node inspector`
3. `feat(journey-ai): add proposal review and draft diff`
4. `feat(journey-test): add virtual-time run inspector`
5. `feat(journey-test): add sandbox inbox and message previews`
6. `feat(journey-publish): add preflight and version confirmation`
7. `feat(journey-observe): add node status overlay and analytics`
8. `test(journey-ui): cover creation, branching, suppression and publish flows`
9. `style(journey-ui): polish responsive states and accessibility`

## Delivered prototype limitations

The HTML is a self-contained **frontend demonstration** using synthetic data, with interactive navigation, AI proposal review flow, editable selected-node names/details, deterministic testing, email/SMS mock previews, a publish-state modal, live-monitor and analytics examples. It intentionally does **not** connect to Claude, a Go backend, a database, real SMS/email providers or live events. Node drag-and-drop, adding/removing/rewiring graph edges, real persistence and real preflight rules are **not** implemented. Reopening the HTML resets the demo to default data.
