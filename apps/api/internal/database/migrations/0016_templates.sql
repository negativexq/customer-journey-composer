CREATE TABLE journey_templates (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 workspace_id uuid NOT NULL REFERENCES workspaces(id),
 name text NOT NULL CHECK(char_length(name) BETWEEN 1 AND 160),
 description text NOT NULL DEFAULT '',
 tags jsonb NOT NULL DEFAULT '[]',
 workflow jsonb NOT NULL,
 created_by uuid NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(workspace_id,created_by) REFERENCES users(workspace_id,id)
);
CREATE INDEX journey_templates_recent ON journey_templates(workspace_id,created_at DESC,id DESC);
CREATE TABLE demo_seed_state(workspace_id uuid PRIMARY KEY REFERENCES workspaces(id),version integer NOT NULL,created_at timestamptz NOT NULL DEFAULT now());
