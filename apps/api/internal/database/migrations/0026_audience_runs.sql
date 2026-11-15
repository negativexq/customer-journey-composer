-- One row per audience entry run of a published version: scheduled at a slot
-- or started by hand. UNIQUE(version_id,slot_at) lets only one worker claim a
-- scheduled slot; each enrolled customer also gets an entry event whose ID is
-- derived from the run, so a retried run never enrolls anyone twice.
CREATE TABLE audience_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    journey_id uuid NOT NULL,
    version_id uuid NOT NULL,
    slot_at timestamptz NOT NULL,
    trigger text NOT NULL CHECK (trigger IN ('SCHEDULE','MANUAL')),
    state text NOT NULL DEFAULT 'RUNNING' CHECK (state IN ('RUNNING','DONE','FAILED')),
    checked integer NOT NULL DEFAULT 0,
    matched integer NOT NULL DEFAULT 0,
    enrolled integer NOT NULL DEFAULT 0,
    detail text,
    started_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    FOREIGN KEY (workspace_id,journey_id,version_id) REFERENCES journey_versions(workspace_id,journey_id,id),
    UNIQUE (version_id,slot_at)
);
CREATE INDEX audience_runs_journey ON audience_runs(workspace_id,journey_id,started_at DESC);
CREATE INDEX audience_runs_schedule ON audience_runs(version_id,slot_at DESC) WHERE trigger='SCHEDULE';
