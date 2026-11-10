CREATE TABLE api_keys (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id),
    created_by uuid NOT NULL,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    token_hash text NOT NULL UNIQUE,
    prefix text NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (workspace_id,created_by) REFERENCES users(workspace_id,id)
);
CREATE INDEX api_keys_workspace ON api_keys(workspace_id,created_at DESC);
CREATE TABLE customers (
    workspace_id uuid NOT NULL REFERENCES workspaces(id),
    customer_id text NOT NULL DEFAULT gen_random_uuid()::text CHECK (char_length(customer_id) BETWEEN 1 AND 128),
    external_id text,
    email text,
    phone text,
    locale text NOT NULL DEFAULT 'en',
    timezone text NOT NULL DEFAULT 'UTC',
    attributes jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(attributes)='object'),
    consents jsonb NOT NULL DEFAULT '{"email":false,"sms":false}',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id,customer_id)
);
CREATE UNIQUE INDEX customers_external_id ON customers(workspace_id,external_id) WHERE external_id IS NOT NULL;
CREATE INDEX customers_created ON customers(workspace_id,created_at DESC,customer_id DESC);
CREATE TABLE customer_events (
    workspace_id uuid NOT NULL,
    event_id text NOT NULL CHECK (char_length(event_id) BETWEEN 1 AND 128),
    customer_id text NOT NULL,
    event_name text NOT NULL CHECK (event_name ~ '^[a-z][a-z0-9_]{0,79}$'),
    occurred_at timestamptz NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now(),
    properties jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(properties)='object'),
    payload_hash text NOT NULL,
    status text NOT NULL CHECK (status IN ('ACCEPTED','LATE')),
    duplicate_count integer NOT NULL DEFAULT 0,
    PRIMARY KEY (workspace_id,event_id),
    FOREIGN KEY (workspace_id,customer_id) REFERENCES customers(workspace_id,customer_id)
);
CREATE INDEX events_customer_timeline ON customer_events(workspace_id,customer_id,occurred_at DESC,event_id DESC);
CREATE INDEX events_recent ON customer_events(workspace_id,received_at DESC,event_id DESC);
ALTER TABLE journeys ADD COLUMN entry_event_name text;
UPDATE journeys j SET entry_event_name=(SELECT n->'config'->>'event_name' FROM journey_versions v CROSS JOIN LATERAL jsonb_array_elements(v.workflow->'nodes') n WHERE v.id=j.current_version_id AND n->>'type'='event_trigger' LIMIT 1);
CREATE INDEX journeys_entry_matching ON journeys(workspace_id,entry_event_name) INCLUDE (id,current_version_id) WHERE status='Active';
CREATE TABLE journey_executions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    journey_id uuid NOT NULL,
    version_id uuid NOT NULL,
    customer_id text NOT NULL,
    entry_event_id text NOT NULL,
    current_node_id text NOT NULL,
    state text NOT NULL DEFAULT 'RUNNING' CHECK (state IN ('RUNNING','WAITING','COMPLETED','EXITED','FAILED','CANCELLED')),
    context jsonb NOT NULL DEFAULT '{}',
    exit_reason text,
    started_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    FOREIGN KEY (workspace_id,journey_id,version_id) REFERENCES journey_versions(workspace_id,journey_id,id),
    FOREIGN KEY (workspace_id,customer_id) REFERENCES customers(workspace_id,customer_id),
    FOREIGN KEY (workspace_id,entry_event_id) REFERENCES customer_events(workspace_id,event_id),
    UNIQUE (workspace_id,journey_id,entry_event_id),
    UNIQUE (workspace_id,id)
);
CREATE UNIQUE INDEX executions_one_active ON journey_executions(workspace_id,journey_id,customer_id) WHERE state IN ('RUNNING','WAITING');
CREATE INDEX executions_reentry ON journey_executions(workspace_id,journey_id,customer_id,started_at DESC);
CREATE INDEX executions_customer ON journey_executions(workspace_id,customer_id,started_at DESC,id);
CREATE INDEX executions_journey ON journey_executions(workspace_id,journey_id,started_at DESC,id);
CREATE TABLE scheduled_jobs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    execution_id uuid NOT NULL,
    node_id text NOT NULL,
    kind text NOT NULL DEFAULT 'step' CHECK (kind IN ('step','timer')),
    due_at timestamptz NOT NULL DEFAULT now(),
    state text NOT NULL DEFAULT 'SCHEDULED' CHECK (state IN ('SCHEDULED','CLAIMED','SUCCEEDED','CANCELLED','FAILED')),
    claim_token uuid,
    lease_until timestamptz,
    attempts integer NOT NULL DEFAULT 0,
    last_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (workspace_id,execution_id) REFERENCES journey_executions(workspace_id,id),
    UNIQUE (execution_id,node_id,kind)
);
CREATE INDEX jobs_due ON scheduled_jobs(due_at,id) WHERE state='SCHEDULED';
CREATE INDEX jobs_stale_lease ON scheduled_jobs(lease_until,id) WHERE state='CLAIMED';
CREATE INDEX jobs_execution ON scheduled_jobs(workspace_id,execution_id) WHERE state IN ('SCHEDULED','CLAIMED');
