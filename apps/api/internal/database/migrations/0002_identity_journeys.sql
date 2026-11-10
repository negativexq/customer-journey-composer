CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id),
    email text NOT NULL UNIQUE CHECK (email = lower(email)),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    password_hash text NOT NULL,
    role text NOT NULL DEFAULT 'Admin' CHECK (role IN ('Admin', 'Editor', 'Viewer')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, id)
);
CREATE TABLE sessions (
    token_hash text PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    csrf_token text NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_expiry ON sessions(expires_at);
CREATE TABLE journeys (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 160),
    description text NOT NULL DEFAULT '',
    tags jsonb NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(tags) = 'array'),
    status text NOT NULL DEFAULT 'Draft' CHECK (status IN ('Draft','Active','Paused','Sunset','Completed','Archived')),
    draft jsonb NOT NULL DEFAULT '{"schema_version":1,"nodes":[],"edges":[]}',
    revision integer NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, id)
);
CREATE INDEX journeys_workspace_updated ON journeys(workspace_id, updated_at DESC);
