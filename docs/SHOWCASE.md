# Northstar product demo and screenshots

The gallery follows one newly authored story: **Northstar Commerce / Abandoned
Cart Recovery**. It uses 14 fictional customers, Alex Morgan's €89.90 cart and a
consistent Northstar email/SMS throughout. The shared recipe is
[showcase-recipe.mjs](../scripts/showcase-recipe.mjs).

## Start clean

```sh
make up
npm run demo:reset
```

Reset is explicit and limited to the development-only demo account on a loopback
API. Existing demo journeys are stopped/archived, pending executions are cancelled,
old API keys revoked and sessions ended. Their records and immutable versions stay
in the archived workspace. A fresh `local-demo` workspace uses the same prefilled
login. Other workspaces are untouched. A reset journal with identifiers only is
written under ignored `.tmp/clean-showcase`.

The fresh workspace contains **one journey**, not old scratch names, repeated
campaigns or previous AI audits. `make demo` reuses the current fixtures without
resetting them. Captures reject unexpected additional journeys rather than
silently photographing a cluttered workspace.

## One journey, two cadences

The intended draft waits **one hour**, sends the email, waits up to **24 hours** for
a purchase of the entry cart, then sends an eligible SMS and completes. A matching
purchase exits immediately as converted. Entry requires EUR amount >=50; reentry
has a seven-day cooldown. Channel consent is enforced by delivery policy.

The published local v1 uses accelerated real timers: **5 seconds / 30 seconds**.
The draft retains **1 hour / 24 hours**. Existing executions use immutable v1 while
the editable draft remains separate. Email goes only to local **Mailpit SMTP**;
SMS uses the **mock provider**. Seeding refuses another SMTP host/port. No external
message or paid LLM request is made.

Twelve initial customers demonstrate early purchase, purchase after email,
completion, email opt-out and SMS opt-out. Two later entrants populate the same
journey. Production-mode counts come from real local API events and workers,
never inserted analytics totals. Real waits may finish before a later capture;
completed paths and their history remain inspectable. Repeated seeds retain
original event timestamps and deduplication keys.

Five saved sandbox scenarios cover the full path, early purchase, purchase after
email, SMS opt-out and an unrelated cart purchase. Each must pass against the
current draft before publish review is ready. They use virtual time, not live
provider calls. Inbox previews are selected from a fresh run of the saved Alex
scenario, using current-draft regression mode.

## Capture

```sh
make install
npx playwright install chromium
npm run screenshots
```

The main gallery has 13 images: Overview, Journeys, Builder, Test Lab, email/SMS
Inbox, Live Monitor, node/customer drilldowns, journey/workspace Analytics,
Customers and publish review. It verifies exactly one correctly named journey
and unchanged live entry count during capture. Publish review is acknowledged
without activating the draft.

The creation gallery has eight images: method, brief, scoped contract, generation
feedback, contract/path review and builder start/focus. A separate fictional
Northstar workspace uses the same newly authored recipe and a **local SSE replay**
of its proposal. Provider requests are blocked, no live generation is claimed and
the result stays Draft. This keeps preview creation out of the user's clean demo.

All 21 PNGs are native **2×** captures, 3200 pixels wide. Most use a 1600 × 1050 CSS viewport; the single-row journey list uses 1600 × 760 to avoid excessive empty space.
Images are not composited or patched. Only normal UI navigation, focus and scroll
are used; fonts/layout/data are rendered by the application. Creation and product
manifests record source and capture details.

[Product manifest](images/manifest.json) · [Creation manifest](images/creation/manifest.json).

Use `npm run screenshots:product` or `npm run screenshots:creation` for one gallery.
`.tmp/product-demo.json` contains fixture identifiers/counts, never passwords,
sessions, CSRF values or provider keys. Optional `CJC_API_URL`/`CJC_WEB_URL` must be
loopback origins. This is a local development showcase, not an external deployment.
