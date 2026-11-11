# Local product demo and screenshots

The optional showcase populates only the development-only demo account on a loopback API. It preserves existing journeys and profiles. Default startup seeding remains drafts and isolated sandbox data; live demo activity is created only by the explicit command below.

```sh
make up
make demo
```

The showcase adds 54 fictional customer profiles, 72 local live-mode enrollments, four published demo workflows and a draft campaign. Lifecycle examples include Active, Paused and Sunset. Workflow versions are published through the normal API only after matching full sandbox runs pass. Existing triggers use a `demo_` prefix; contacts use `example.invalid` and fictional phone numbers.

Live showcase actions use **mock SMS only**. Email previews are produced in the isolated simulator. No paid LLM or external email/SMS service is called. Normal manual test email remains available through local Mailpit.

The cart demo uses short real-time waits so it settles locally in seconds. The separate **Abandoned Cart · €50+** draft retains the original 1-hour / 24-hour / 48-hour specification and five A–E virtual-clock scenarios. Long-running onboarding, win-back and follow-up examples populate customer tracking and lifecycle views. Paused and sunset journeys retain their durable work.

Counts and channel outcomes come from stored execution and delivery records, not substituted UI values. Live analytics excludes sandbox runs. Demo activity is created at the actual current time; the screenshots do not invent historical trends, email opens or click rates. Re-running a completed seed preserves the same fixture identities; two repeated runs were verified to retain 54 demo profiles and 72 demo executions.

## Capture the product

```sh
make install
npx playwright install chromium
npm run screenshots
```

The capture script uses the running local web/API stack, signs into the prefilled development demo account, seeds missing fixtures and exercises actual UI controls. Screenshots are 3200 × 2100 PNG files (1600 × 1050 viewport at 2× device scale). Animations and the text caret are disabled only for capture; UI data and content are unchanged.

It captures Overview, Journeys, Builder with its inspector, Test Lab, email/SMS Inbox previews, Live Monitor, Analytics, Customers and publish review. Capturing Test Lab creates one additional isolated sandbox run. The script verifies that production entry counts remain unchanged while taking the screenshots. It opens and acknowledges publish review without activating the original recovery draft.

Images and capture metadata are saved under `docs/images`. `.tmp/product-demo.json` contains only local fixture IDs and counts; passwords, cookies and CSRF tokens are never written. An alternate local setup can set `CJC_API_URL` and `CJC_WEB_URL`; non-loopback origins and credential-bearing URLs are rejected.

This task does not deploy the application. The local Docker stack and demo are development tools.
