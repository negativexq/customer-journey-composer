# Journey observation screenshots

Refreshed November 13, 2026 from the running local application and its new
Northstar Commerce workspace and Abandoned Cart Recovery journey. Native screenshots at 1600 × 1050 CSS pixels, 2× display
scale (**3200 × 2100**). Counts and paths come from persisted published-version
execution records. No analytics values or geometry are substituted.

- [Node and message breakdown](node-breakdown.png)
- [Customer path and execution timeline](customer-path.png)
- [Journey analytics](analytics.png)

The main capture command seeds missing local fixtures and creates one isolated
sandbox run for Test Lab/Inbox previews. It checks that production entry totals
are unchanged across capture. It does not generate AI output or publish the
recovery draft. Live showcase messages use local Mailpit SMTP and mock SMS. Published v1 uses
accelerated timers; the editable draft keeps the business cadence.

Projection timestamps may precede browser refresh when no new data arrives.
Provider accepted is not proof of delivery; these are synthetic demo records,
not load-test results. [Manifest](../manifest.json) · [Capture method](../../SHOWCASE.md).
