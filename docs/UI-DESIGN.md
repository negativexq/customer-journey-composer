# Interface design

See the [current competitor review and prioritized component backlog](UX-RESEARCH.md) for the next interface changes; that research distinguishes proposed components from implemented behavior below.

The [updated SaaS design direction](design/modern-saas-direction.md) records the latest user preferences and separates shipped behavior from staged work.

The interface uses one native system UI font family, readable body text, restrained heading weights, and a violet/slate palette, a grouped workspace navigation, compact journey actions and a canvas-first builder. Metrics come from persisted workspace records; empty states do not invent activity, uplift or conversion results.

Workflow interaction references were reviewed from the official [Customer.io builder guide](https://docs.customer.io/messaging/send/workflows/builder/), [Braze Canvas components](https://www.braze.com/docs/user_guide/messaging/canvas/canvas_components) and [Loops builder](https://loops.so/docs/loop-builder). Their product images were used for visual research only; no product screenshots, marks or assets are shipped in the application.

## Components and behavior

The user-provided [UX handoff](design/ux-handoff.md) and [four-screen overview](design/design-overview.png) guide the current implementation. Builder now uses pinned `@xyflow/vue` 2.0.0-next.21 with scoped flow state and immutable application edits. It has grouped elements, independently collapsible panels, an inspector that opens only for a selected step and explicit draft/published-version context. Delay and event timeout fields use seconds/minutes/hours/days while persisting the existing seconds schema. Journey settings are opened from More actions to preserve canvas space. Published-version metadata is obtained through an indexed join on the existing detail query, with no per-node API requests.

- Elements stays 220px wide on desktop and opens over the canvas on mobile. Canvas stays light in dark OS mode, with shared 38px/44px controls, explicit stroke icons and a zoom percentage.
- Interface text uses a 12px minimum, 13–14px controls/body text, and clear secondary-text contrast. Code and payloads retain monospace. There are no downloaded font assets or decorative letter spacing.
- Shared icons, status badges, metric cards and an actual-data trend chart establish a common visual language.
- Journey creation uses a focused dialog; journey lists use bounded server pagination and search.
- Connection “+” controls open a keyboard-accessible insertion dialog. The original source branch is preserved; new Condition/Wait for Event nodes require an explicit continuation output, and their other output stays unconnected until the user configures it. Insertion is one undo step.
- The builder keeps metadata collapsed, the step library on the left and contextual node settings on the right. AI remains a proposal drawer with Apply/Discard and separate publication.
- Test Lab exposes customer fields, event rows and expected outcomes before advanced JSON. Virtual clock controls, execution results and a run-scoped inline Test Inbox share one workspace. The €50+ recovery template includes five editable scenario presets; switching journey tabs preserves the current run.
- Test Inbox separates filtering, message selection and isolated rendering. Eligibility decisions and attempt history use readable summaries with advanced payload inspection. Provider acceptance and simulated output remain distinct.
- Publishing opens a review dialog with server-derived graph, matching full-test and email configuration gates. Acknowledgement is required before the version is published. Consent, frequency caps and customer personalization are explicitly runtime decisions.
- Live Monitor reuses builder node cards, displays actual node metrics and highlights the selected customer path. The inspector presents transitions, condition outcomes, pending work and delivery attempts. Tab changes preserve the selected customer while hidden monitors stop polling; late responses cannot replace a newer customer selection.
- Analytics node reach bars use version entries as their denominator; alternative branches are labeled “Not reached” rather than implying linear funnel drop-off. Analytics journey selection is searchable. Period presets show actual UTC date ranges and support keyboard selection, Escape and focus restoration. Date selection controls the trend only; summary cards explicitly show all-time totals.
- Selection controls have generous value space, padding and a distinct disclosure target. Reporting selectors are at least 290px wide on desktop and fill available width on mobile. Native form selects retain platform behavior with a consistent minimum 44px height.

## Navigation

Workspace pages use stable URL paths. Journey URLs identify the journey and its active tab; Preview/Test also preserves its subsection. Reload restores the authenticated route, and browser back/forward updates the same state. Journey status filters are reflected in the query string. Protected deep links resume after sign-in; inaccessible journey IDs show an error and return to the journey list. Customer selections, activity inspector records and Live Monitor execution/delivery selections also survive reload and browser back/forward. Only resource identifiers enter navigation URLs; contact fields, attributes, rendered messages and credentials do not. Customer/event/execution/message inspections share a lazy-loaded accessible drawer with native Escape and focus behavior. Hidden detail requests are aborted or fenced against stale results. Temporary session and journey-load failures keep the current URL with an explicit retry; only an anonymous 401 shows sign-in, and only inaccessible 403/404 journey IDs fall back to the list.

## Verification

Browser coverage checks existing editor, AI approval, version, lifecycle, preview, virtual clock and inbox behavior on desktop and mobile. Reporting tests additionally exercise journey search, keyboard period selection, the resulting API request and focus restoration. Screenshots are inspected against the running Docker build, including open menus and mobile widths.
