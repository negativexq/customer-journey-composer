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

Refreshed November 13, 2026 at 1600 × 1050 CSS pixels, scale 2 (3200 × 2100).
The fictional workspace uses the recorded passing stock workflow and scoped v2
contract. Generation lifecycle is a local streamed replay, not a new provider call.
The demo journey name/description are prepared metadata; normal application
layout, reference checks and approval render the graph. The result remains Draft.

[Brief and examples](../images/creation/brief.png) ·
[Scoped contract](../images/creation/contract.png) ·
[Generation feedback](../images/creation/generating.png) ·
[Review and expected outcomes](../images/creation/review.png) ·
[Path diagram](../images/creation/review-diagram.png) ·
[Gallery and manifest](../images/creation/README.md).

The 60-case verification above belongs to the initial creation implementation.
The later shared-contract correction passed 78/78 desktop/mobile cases;
[scope and evidence](../BEHAVIOR-CONTRACT-V2.md).

## Canvas readability follow-up

The editor opens at the trigger at readable zoom. Go to start and Focus selected
step provide orientation without squeezing a long workflow into a tiny overview.
Selected nodes highlight adjacent connections; other nodes/edges fade. Connected
steps and edge controls jump to the original target. Shared exits optionally use
short labeled references, preserving the exact node and edge identities; turning
the option off restores the full links. These references are not new runtime nodes.

Conditions show grouped rules, event waits include their timeout, and message
summaries use readable variable labels without changing stored templates.
Orthogonal edges have arrowheads and nearby branch labels. Insert controls appear
on hover/focus. Auto layout measures actual cards and remains undoable; this is
not a guarantee that every arbitrary graph has zero edge crossings. No new layout
dependency or runtime behavior was introduced.

[Readable initial canvas](../images/creation/canvas-start.png) ·
[Selected connections and navigation](../images/creation/canvas-focus.png)

At this earlier canvas follow-up, the complete browser suite passed **64/64** desktop/mobile cases, including
short exit navigation without graph mutation, Auto layout undo and failed
composition restore without overwriting a saved brief. `make check` passes again.
The earlier complete PostgreSQL integration run remains applicable: this
follow-up changes frontend presentation only. No additional paid model call was
made. The earlier 60-case result above records the creation slice before these
four additional browser cases. The later shared-contract run reached **78/78**; see the verification link above.
