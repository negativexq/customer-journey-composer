# Interface design

See the [current competitor review and prioritized component backlog](UX-RESEARCH.md) for the next interface changes; that research distinguishes proposed components from implemented behavior below.

The interface uses one native system UI font family, readable body text, restrained heading weights, and a violet/slate palette, a grouped workspace navigation, compact journey actions and a canvas-first builder. Metrics come from persisted workspace records; empty states do not invent activity, uplift or conversion results.

Workflow interaction references were reviewed from the official [Customer.io builder guide](https://docs.customer.io/messaging/send/workflows/builder/), [Braze Canvas components](https://www.braze.com/docs/user_guide/messaging/canvas/canvas_components) and [Loops builder](https://loops.so/docs/loop-builder). Their product images were used for visual research only; no product screenshots, marks or assets are shipped in the application.

## Components and behavior

- Interface text uses a 12px minimum, 13–14px controls/body text, and clear secondary-text contrast. Code and payloads retain monospace. There are no downloaded font assets or decorative letter spacing.
- Shared icons, status badges, metric cards and an actual-data trend chart establish a common visual language.
- Journey creation uses a focused dialog; journey lists use bounded server pagination and search.
- The builder keeps metadata collapsed, the step library on the left and contextual node settings on the right. AI remains a proposal drawer with Apply/Discard and separate publication.
- Test Lab exposes customer fields, event rows and expected outcomes before advanced JSON. Virtual clock controls and execution results share one workspace.
- Test Inbox separates filtering, message selection and isolated rendering. Provider acceptance and simulated output remain distinct.
- Analytics journey selection is searchable. Period presets show actual UTC date ranges and support keyboard selection, Escape and focus restoration. Date selection controls the trend only; summary cards explicitly show all-time totals.
- Selection controls have generous value space, padding and a distinct disclosure target. Reporting selectors are at least 290px wide on desktop and fill available width on mobile. Native form selects retain platform behavior with a consistent minimum 44px height.

## Navigation

Workspace pages use stable URL paths. Journey URLs identify the journey and its active tab; Preview/Test also preserves its subsection. Reload restores the authenticated route, and browser back/forward updates the same state. Journey status filters are reflected in the query string. Protected deep links resume after sign-in; inaccessible journey IDs show an error and return to the journey list. No customer data or credentials are stored in navigation URLs.

## Verification

Browser coverage checks existing editor, AI approval, version, lifecycle, preview, virtual clock and inbox behavior on desktop and mobile. Reporting tests additionally exercise journey search, keyboard period selection, the resulting API request and focus restoration. Screenshots are inspected against the running Docker build, including open menus and mobile widths.
