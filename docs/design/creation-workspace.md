# Resumable journey creation workspace

Implemented locally on November 12, 2026. This replaces the creation dialog with
an actual workspace page; it does not deploy the application externally.

## Product flow

`Details → Creation method → Brief → Review → Builder`

- Details opens at `/journeys/new`. Next creates a Draft in the Method step.
- Scratch explicitly applies an empty draft and opens the builder.
- AI opens a dedicated Brief page with four editable examples. Selecting an
  example does not generate anything. Example briefs do not tell AI “never
  publish”: proposal/apply/publish permissions are enforced by the product.
- Generation replaces the brief form with one submitted-brief summary and real
  lifecycle feedback. The same brief is not duplicated in an input and chat.
- Review shows a readable box-and-arrow design path, nearby alternate branches,
  an All steps view, warnings and explicit approval. Branch selection changes
  presentation only, not workflow data and not a simulation result.
- Request changes uses the last proposal as input, saves the revision brief and
  presents the updated proposal for another review.
- Approve saves the exact persisted proposal to the editable draft, with a
  preparation progress bar, then opens Builder. Publication is separate.

The stepper highlights the active stage with a moving underline and slightly
larger text. Reduced-motion disables the transition.

## Persistence and safety

Migration `0020_journey_composition.sql` adds a small `journeys.creation_step`
field and a separate `journey_compositions` table. Journey lists project only
the step, not the large brief/proposal JSON. Composition reads use the composite
workspace/journey primary key. Production runtime and analytics queries do not
read composition state and no runtime scheduling table is changed.

- `GET /api/v1/journeys/{id}/composition`: restore the current step and plan.
- `PUT /api/v1/journeys/{id}/composition`: save metadata, brief, provider/method,
  changes and latest proposal with optimistic revision fencing.
- `POST /api/v1/journeys/{id}/composition/approve`: atomically apply the saved
  proposal and move to Builder; validate the graph and audit approval.

Writes require Editor/Admin rights and a Draft still in creation. The shared
journey revision fences other tabs and approval. Proposal persistence does not
apply it to the execution draft. Publish checks block unfinished creation.
Composition data follows journey retention and is removed by cascading journey
deletion; archiving preserves it. It may contain user-authored text and is not
written into request logs. It never contains server provider credentials.

Brief/Review drafts reopen in their saved step from Journeys or a direct URL,
including after reload. Failed approval retains Review. Generating is recorded
before the request starts: reload restores the brief and last saved proposal,
marks the interrupted request and does **not** automatically make another paid
request. Generation remains request-scoped, not a durable resumable model job.

The new page serializes autosaves and waits for outstanding saves before
requesting generation or approval. Restore failures prevent writes until a
successful reload, so an unavailable composition cannot be replaced with a blank
local state. In-flight/unsaved work has a browser unload warning.

## Actual verification

- `make check`: formatting, lint, typecheck/build, Go race/unit and Node tests pass.
- Complete `make test-integration`: all package-isolated PostgreSQL suites pass.
  New cases exercise restore, revision conflict, workspace/RBAC isolation,
  publication blocking and atomic approval without a published version.
- Complete browser suite: **60/60** desktop/mobile cases pass. New cases include
  saved Brief and Review resume, correction against the previous proposal,
  explicit approval, preparation progress, failed approval retry and interrupted
  generation without automatic resubmission.
- Local API/worker/web rebuilt; migration applied; no new paid model request or
  real external delivery used for this work. These are local checks, not a claim
  about the latest hosted CI run.

## Screenshots

Captured from the running application at 1440 × 1040 CSS pixels, scale 1.5
(2160 × 1560 output). Fictional isolated workspace. Generation lifecycle is a
local streamed fixture; the workflow is a recorded Trial Conversion graph,
not a newly generated provider response. No UI text or geometry was substituted.

[Brief and examples](../images/creation/brief.png) ·
[Generation feedback](../images/creation/generating.png) ·
[Review diagram](../images/creation/review.png)

Advanced canvas connection focus and readable node summaries are a subsequent
slice; the creation review diagram does not replace the editable workflow graph.
