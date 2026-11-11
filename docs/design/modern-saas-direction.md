# Modern SaaS design direction

User direction received 11 November 2026. This is an implementation guide and staged backlog, not a claim that every proposed module exists. The current task finishes temporary-access recovery, measured runtime optimization and contextual navigation, followed by the explicitly requested Vue Flow migration and canvas readability fixes. Deployment and live LLM evaluation remain deferred.

## One identity

Light-first, dense and calm. Linear informs visual hierarchy and restrained navigation; Attio informs tables, filters and record inspection; Customer.io informs journey composition. Notion informs the AI brief/review experience and PostHog informs explicit funnel/cohort semantics. Reference proportions describe visual emphasis, not copied code or assets.

- Neutral white/gray surfaces; violet reserved for selected state, AI and primary actions. Semantic node icons and status text accompany color.
- One UI font family; Inter/Geist are the requested candidates. The existing system font remains until a font slice verifies self-hosted files, license, Turkish characters and loading cost.
- Body 13–14px, headings 20–24px, 4px grid/8px rhythm, radii 6–10px, 120–180ms transitions and reduced-motion support. Retain compact 16px navigation icons and 34px rows from the earlier user request. Touch targets may exceed desktop control heights.
- Sidebar roughly 224px with a future collapsed state; consistent 52–56px topbar and shared page headers. Current compact navigation must not grow vertically.

## Product boundaries

Journeys describe behavior, Content describes reusable messages and Audience describes eligible customers. Navigation should reflect those concepts as their actual modules become available. Do not invent working segments, scheduling triggers, opened/clicked statistics or provider credentials from a visual mockup. All preview values in the supplied plan are illustrative.

## Priority surfaces

1. Builder: canvas gets the available width. Elements has a fixed 220px desktop width and is an overlay on mobile. Palette and inspector can be toggled independently; no selected node means no right inspector. Node cards summarize type, configuration and errors, with forms in the inspector. Edge insertion preserves the existing port and requires an explicit continuation output for new branching nodes. Copy/duplicate, undo/redo, zoom, pan and auto-layout remain.
2. AI Brief: broad, focused input → readable proposed plan/assumptions → Apply/Discard → editable draft. Existing Claude/OpenAI proposals are human-approved. Workspace-aware generation requires a validated schema/catalog without live customer PII; it is a separate remaining feature, not implied by the existing brief starter. Node-level message actions already exist; a redesigned dedicated AI creation surface remains.
3. Test Inbox: connected message list and preview with readable eligibility/attempts. Simulation never calls live providers. Explicit Send Test uses an allowed recipient and a separately identified result. Preserve this distinction in every view.

## Delivery map

| Stage                      | Current state / next work                                                                                                                            |
| -------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| Shared shell/tokens/tables | Compact light shell and common controls exist; broader semantic-token coverage, collapsible sidebar and typography evaluation remain.                |
| Journeys list/detail       | Bounded search/status views, URL navigation and draft/live context exist; saved views, ownership/trigger/channel filters and row preview remain.     |
| Canvas/inspector           | Vue Flow editor, contextual inspector, explicit branch insertion and atomic undo exist. Further node hover actions remain.                           |
| AI brief/review            | Provider adapters, typed proposal, diff, assumptions/warnings and approval exist; dedicated brief surface and workspace catalog remain.              |
| Content/Inbox              | Email/SMS studio, personalized previews and isolated inbox exist; standalone Content module and template UX remain.                                  |
| Simulation/preflight       | Virtual clock, fault injection, assertions, run-scoped inbox and publish gates exist.                                                                |
| Analytics                  | Real projected counts, scoped/cohort trends, branch reach and monitor metrics exist; richer drill-down and a dedicated canvas analytics mode remain. |
| Command/theme              | Journey command search exists; broader resource search, shortcut help and complete dark theme remain.                                                |
| A/B/collaboration          | Outside current MVP scope; separate future work.                                                                                                     |

## Dependency decision

The official [Vue Flow 2 announcement](https://next.vueflow.dev/whats-new/2026-08-06) and [migration guide](https://next.vueflow.dev/learn/troubleshooting/migrate-to-v2) confirm the new `@xyflow/vue` package and breaking changes in store scope, bindings, immutable nodes/edges and styling. At inspection, the [new documentation homepage](https://next.vueflow.dev/) shows `2.0.0-next.21`; npm metadata reports preview tags (`latest: 2.0.0-next.0`, `next: 2.0.0-next.21`). At the user’s explicit request, the repository is migrated from `@vue-flow/core` 1.48.x to pinned `@xyflow/vue` 2.0.0-next.21. All old Vue Flow add-ons are removed. Builder uses a scoped provider, managed `v-model` and immutable application updates; Monitor keeps an isolated read-only flow. The custom node consumes library props without forwarding native `draggable` to HTML. Visual target handles have a stable input ID without changing the persisted workflow schema. The preview-version pin is deliberate and future updates require regression checks.

The canvas explicitly uses a light color scheme instead of following OS dark mode. Shared controls use the app’s stroked icons, readable 38px desktop/44px mobile buttons and a zoom percentage. Tests cover real drag-to-connect, ports, undo, node settings, dark OS mode, wide Elements and separate builder/monitor behavior. The complete suite passes 46 desktop/mobile tests.

Tailwind, Reka UI, Lucide Vue, ECharts and Pinia are proposed tools, not dependencies already installed. Adopt a package only for a concrete need and migrate existing behavior with regression checks. Current native dialogs, accessible selects, icons and deterministic workflow schema remain functional.
