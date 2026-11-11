# AI generation feedback and journey review

Research and proposed implementation — November 12, 2026. **Not implemented.**
This document records a source-code review and competitor documentation research,
not a usability study or a claim that the proposed improvements already ship.
No paid provider call or application-data change was made for this research.

## Observed problems in this application

- `AIPanel.vue` awaits one JSON response, changes the Compose button label and
  exposes Cancel. Its busy state is not communicated to the canvas. There is no
  elapsed time, visible processing panel or server-stage stream.
- `httpapi/ai.go` returns only the final proposal/error. `ai/composer.go` already
  has real provider, validation and bounded-repair boundaries but does not expose
  these as progress events. A frontend timer cannot know these stages.
- `WorkflowCanvas.vue` accepts model-produced coordinates. Explicit Auto layout
  uses Dagre with fixed 224 × 112 node assumptions and node-to-node edges; branch
  ports and semantic continuation/exit paths are not supplied to the layout.
- `InsertableEdge.vue` independently draws Bezier paths and a permanent insertion
  button for every editable edge. There is no selected-path focus treatment.
- `WorkflowNode.vue` shows a condition's direct `rule.path`; grouped rules fall
  back to “Configure conditions” even when correctly configured. Wait-for-event
  summaries show the event name but omit the timeout.
- The saved Trial Conversion snapshot inspected for this review has 16 nodes,
  24 edges and a shared converted exit with seven incoming edges. Four edges
  span at least 1,000 vertical coordinate units. Node origins plus the current
  nominal dimensions occupy approximately 842 × 2,660 units. These are graph
  coordinate measurements, not viewport screenshots or measured user behavior.
  Fit-all necessarily makes a tall workflow small in a typical editor viewport.

## Relevant product patterns

| Reference                                                                                                         | Documented pattern                                                                                   | Proposed application here                                                                      |
| ----------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| [Customer.io workflow builder](https://docs.customer.io/messaging/send/workflows/builder/)                        | Top-to-bottom canvas; contextual block actions; Review sidebar; zoom-to-fit and Zen mode             | Consistent reading direction, contextual controls and a dedicated review surface               |
| [Intercom workflow builder](https://www.intercom.com/help/en/articles/6611595-using-the-workflows-builder)        | Organized paths, path list, recenter, connector “Go to” controls and Tidy up                         | Named path navigation and target jumps instead of making the user follow every long connection |
| [n8n Canvas Groups](https://docs.n8n.io/build/understand-workflows/workflow-components/canvas-groups)             | Named groups, optional descriptions and collapse/expand with personal visibility preferences         | Later add named visual sections with strict connection-boundary rules                          |
| [n8n Assistant](https://docs.n8n.io/build/ways-of-building-workflows/n8n-assistant)                               | Review the plan and generated workflow, check configuration, then test before production             | Show a readable journey outline beside the proposed graph before Apply                         |
| [Carbon progress bar](https://www.carbondesignsystem.com/building-blocks/core/components/progress-bar/guidelines) | Indeterminate progress when completion cannot be measured; explicit success/error and nearby context | Persistent processing card with actual stage and elapsed time, without fabricated percentages  |

These recommendations are our synthesis. The documentation does not establish
that competitors use the exact generation stepper or connector representation
proposed below. No authenticated competitor workspace was inspected.

## Generation experience

Show a processing card immediately inside the workspace, linked to the active
Composer request. Keep the brief visible. On an empty canvas, replace the empty
instruction with the processing card; on an existing journey, use a compact
banner so the existing draft stays inspectable. Do not show a fake generated
graph or animate invented nodes as if the model had returned them.

Example state, with real event-driven status rather than timed transitions:

```text
Creating your journey                                  00:28 elapsed
✓ Request accepted
● Generating proposal · attempt 1
○ Checking workflow and message configuration
○ Ready for your review

Your draft has not changed.                            Cancel request
```

- Use an indeterminate activity indicator, not 62% or a completion countdown.
  The few recorded 38–47s trials are not a reliable ETA distribution.
- Emit only observable lifecycle events: request accepted, provider attempt
  started, validation started, repair needed, retry waiting, review ready,
  failed or cancelled. An attempt can return to generation after validation;
  represent this explicitly rather than treating the stages as monotonic.
- During the provider wait, say “Waiting for the AI provider”; do not claim the
  model is currently analyzing the brief or composing SMS. There is no such
  internal telemetry and no need to expose private reasoning.
- After a longer wait, add a neutral elapsed-time message without claiming a
  healthy connection unless a server heartbeat was received.
- Preserve brief/error details and a user-triggered retry on failure. No hidden
  retry that starts another paid Composer request. Show ready-for-review on
  success, then switch to the proposal outline; never auto-Apply or auto-Publish.
- Cancel propagates context cancellation but cannot promise reversal of provider
  billing. Distinguish a disconnected client from confirmed server cancellation.
- Use an accessible status region for stage changes, a quiet timer not announced
  every second, keyboard-reachable Cancel and reduced-motion support.

### Technical shape

First add honest immediate local feedback while the existing JSON request is
pending. That version can display “Request sent / Waiting for proposal” only;
validation/repair stages require backend events and must not be simulated.

For full stages, retain the existing JSON endpoints and add an opt-in streaming
POST path using authenticated fetch streaming. Factor the shared orchestration
so accounting, workspace active-request guards, errors and time limits remain
identical. A bounded observer on Composer emits safe lifecycle events. Emit the
proposal only after schema/graph/message validation and accounting succeed.

The stream must not contain keys, raw provider bodies, live customer payloads or
reasoning. Disable proxy buffering for that path; verify flushing, timeouts,
client abort and heartbeat cleanup. Broken streams must not trigger automatic
paid resubmission. Provider output itself need not be streamed to implement this.
No new database table, Redis, worker service or additional LLM call is needed.
Navigation/reload recovery is a separate durable-generation-job feature; do not
promise it for a request-scoped stream. Warn before leaving during generation.

## Readable review and canvas

1. **Review first:** display an outline generated deterministically from the
   validated graph, not another AI call. Include trigger, global exit, reentry,
   waits, readable conditions and message/skip branches. Outline rows select the
   exact canvas node. Shared nodes appear as named references to avoid exponential
   path expansion; cap traversal and distinguish visual paths from test results.
2. **Readable viewport:** offer “Start”, “Fit all” and “Focus selection”. Open a
   long journey at its trigger at a readable zoom; keep Fit all as an overview.
   Temporarily hide the palette/inspector when reviewing. A single zoom value
   cannot make both all 16 nodes and their text fit a short viewport.
3. **Useful node summaries:** flatten simple rule groups into readable summaries,
   e.g. `login_count < 3 AND email consent`, and show `subscription_started · up
to 3 days` for a wait. Longer expressions get a count and inspectable full rule.
4. **Branch navigation:** show stable Yes/No and Matched/Timeout labels close to
   source ports, arrowheads and visible focus states. Selecting a node/edge
   highlights its immediate connections and fades unrelated ones. “Go to target”
   centers the destination. Reveal insert controls on hover, selection or keyboard
   focus instead of keeping a plus button on every connection.
5. **Layout:** keep the continuation spine aligned, reserve clear branch lanes and
   route long/shared-target edges around nodes. Match actual node dimensions;
   compute layout on explicit organize/new-proposal placement, not each keystroke.
   Preserve user positions on reload and support undo of layout.
6. **Shared exits:** in Review mode, consider short terminal references such as
   `Converted → Abonelik dönüşümü` beside each branch, with a jump to the original
   node and an explicit shared-target label. These are presentation references,
   never extra workflow nodes or altered transitions. Edit mode must retain the
   actual graph. Global exit remains a visible journey-wide rule.

For general port-aware routing, [ELK Layered](https://eclipse.dev/elk/reference/algorithms/org-eclipse-elk-layered.html)
supports orthogonal routing and port constraints. Evaluate it against the Trial
Conversion, recovery and onboarding fixtures before adding a dependency. Merely
switching layout engines does not fix review comprehension or shared exits. If
adopted, render its routed edge sections; laying out nodes with ELK and drawing
independent Bezier edges would discard much of the routing benefit. Lazy-load
and measure main-thread/bundle impact. This is an option, not a benchmark claim.

## Proposed delivery order and acceptance

| Slice | Scope                                                            | Acceptance evidence needed                                                                                                                                                    |
| ----- | ---------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | Immediate canvas/panel processing feedback and elapsed time      | Delayed mocked request shows feedback immediately; Cancel/error/success states work; brief retained; no extra generation request                                              |
| 2     | Real backend lifecycle streaming                                 | Controlled providers expose generation, validation, repair and timeout stages; proxy flush tested; both adapters share lifecycle handling; disconnect/tenant/accounting tests |
| 3     | Review outline, useful rule/wait summaries, focus and navigation | Reviewer can find the login threshold, all three waits, consent skips and converted targets in Trial Conversion without tracing long lines; keyboard/mobile checks            |
| 4     | Port-aware organization and shared-exit review references        | Baseline screenshot comparison; no node/label overlap in representative fixtures; exact IDs/config/edges preserved; organize undo and reload behavior verified                |
| Later | Named collapsible sections                                       | Explicit group boundary and shared-node behavior defined before implementation                                                                                                |

Use deterministic fixtures and delayed mock responses for these UX tests. Paid
regeneration is unnecessary. Record screenshots at normal desktop sizes as well
as mobile; a large screenshot alone is not readability evidence. Check fmt,
lint, typecheck, unit and relevant browser/integration tests before code commits.
