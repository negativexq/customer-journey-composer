CREATE TABLE workspace_settings (
    workspace_id uuid PRIMARY KEY REFERENCES workspaces(id),
    frequency_limit integer NOT NULL DEFAULT 3 CHECK (frequency_limit BETWEEN 1 AND 1000),
    frequency_window_seconds integer NOT NULL DEFAULT 86400 CHECK (frequency_window_seconds BETWEEN 60 AND 2592000),
    test_allowlist jsonb NOT NULL DEFAULT '[]',
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE message_templates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 160),
    channel text NOT NULL CHECK (channel IN ('email','sms')),
    config jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX templates_workspace ON message_templates(workspace_id,updated_at DESC,id);
ALTER TABLE action_outbox ADD CONSTRAINT outbox_workspace_identity UNIQUE (workspace_id,id);
CREATE TABLE delivery_jobs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id),
    execution_id uuid,
    outbox_id uuid,
    node_id text,
    customer_id text,
    mode text NOT NULL DEFAULT 'LIVE' CHECK (mode IN ('LIVE','TEST')),
    channel text NOT NULL CHECK (channel IN ('email','sms')),
    provider text NOT NULL CHECK (provider IN ('smtp','mock_sms','mock_email')),
    idempotent_provider boolean NOT NULL,
    logical_key text NOT NULL,
    request_hash text,
    message jsonb NOT NULL,
    decision jsonb NOT NULL,
    status text NOT NULL CHECK (status IN ('SCHEDULED','CLAIMED','DISPATCHING','ACCEPTED','BLOCKED','SKIPPED','FAILED','UNKNOWN','CANCELLED')),
    due_at timestamptz NOT NULL,
    claim_token uuid,
    lease_until timestamptz,
    attempts integer NOT NULL DEFAULT 0,
    last_error text,
    cancel_requested boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL,
    accepted_at timestamptz,
    FOREIGN KEY (workspace_id,execution_id) REFERENCES journey_executions(workspace_id,id),
    FOREIGN KEY (workspace_id,outbox_id) REFERENCES action_outbox(workspace_id,id),
    FOREIGN KEY (workspace_id,customer_id) REFERENCES customers(workspace_id,customer_id),
    CHECK (mode='TEST' OR (execution_id IS NOT NULL AND customer_id IS NOT NULL AND outbox_id IS NOT NULL)),
    UNIQUE (workspace_id,logical_key),
    UNIQUE (execution_id,node_id),
    UNIQUE (workspace_id,id)
);
CREATE INDEX delivery_due ON delivery_jobs(due_at,id) WHERE status='SCHEDULED';
CREATE INDEX delivery_stale ON delivery_jobs(lease_until,id) WHERE status IN ('CLAIMED','DISPATCHING');
CREATE INDEX delivery_customer_history ON delivery_jobs(workspace_id,customer_id,created_at DESC,id);
CREATE INDEX delivery_frequency ON delivery_jobs(workspace_id,customer_id,status,accepted_at DESC) WHERE mode='LIVE' AND status IN ('SCHEDULED','CLAIMED','DISPATCHING','ACCEPTED','UNKNOWN');
CREATE INDEX delivery_recent ON delivery_jobs(workspace_id,created_at DESC,id);
CREATE INDEX delivery_execution ON delivery_jobs(workspace_id,execution_id,node_id);
CREATE TABLE delivery_attempts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    delivery_id uuid NOT NULL,
    attempt integer NOT NULL CHECK (attempt>0),
    status text NOT NULL CHECK (status IN ('STARTED','ACCEPTED','RETRY','FAILED','UNKNOWN','CANCELLED')),
    provider_code integer,
    provider_id text,
    response text,
    started_at timestamptz NOT NULL,
    completed_at timestamptz,
    FOREIGN KEY (workspace_id,delivery_id) REFERENCES delivery_jobs(workspace_id,id),
    UNIQUE (delivery_id,attempt)
);
