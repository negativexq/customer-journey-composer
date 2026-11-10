CREATE TABLE step_executions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    execution_id uuid NOT NULL,
    node_id text NOT NULL,
    node_type text NOT NULL,
    state text NOT NULL CHECK (state IN ('SCHEDULED','RUNNING','WAITING','SUCCEEDED','SKIPPED','FAILED')),
    output jsonb NOT NULL DEFAULT '{}',
    started_at timestamptz NOT NULL,
    completed_at timestamptz,
    FOREIGN KEY (workspace_id,execution_id) REFERENCES journey_executions(workspace_id,id),
    UNIQUE (execution_id,node_id)
);
CREATE INDEX steps_execution_trace ON step_executions(workspace_id,execution_id,started_at,id);
CREATE TABLE event_waiters (
    workspace_id uuid NOT NULL,
    execution_id uuid NOT NULL,
    node_id text NOT NULL,
    customer_id text NOT NULL,
    event_name text NOT NULL,
    deadline timestamptz NOT NULL,
    state text NOT NULL DEFAULT 'WAITING' CHECK (state IN ('WAITING','MATCHED','TIMED_OUT','CANCELLED')),
    matched_event_id text,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (execution_id,node_id),
    FOREIGN KEY (workspace_id,execution_id) REFERENCES journey_executions(workspace_id,id),
    FOREIGN KEY (workspace_id,customer_id) REFERENCES customers(workspace_id,customer_id)
);
CREATE INDEX waiters_event_match ON event_waiters(workspace_id,customer_id,event_name) INCLUDE (execution_id,node_id,deadline,created_at) WHERE state='WAITING';
CREATE TABLE action_outbox (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    execution_id uuid NOT NULL,
    node_id text NOT NULL,
    channel text NOT NULL CHECK (channel IN ('email','sms')),
    payload jsonb NOT NULL,
    state text NOT NULL DEFAULT 'PENDING' CHECK (state IN ('PENDING','PROCESSED','CANCELLED')),
    created_at timestamptz NOT NULL,
    FOREIGN KEY (workspace_id,execution_id) REFERENCES journey_executions(workspace_id,id),
    UNIQUE (execution_id,node_id)
);
CREATE INDEX outbox_pending ON action_outbox(created_at,id) WHERE state='PENDING';
CREATE INDEX outbox_execution_pending ON action_outbox(workspace_id,execution_id) WHERE state='PENDING';
ALTER TABLE journey_executions ADD COLUMN exit_event_name text;
UPDATE journey_executions e SET exit_event_name=v.workflow->'global_exit'->>'event_name' FROM journey_versions v WHERE e.version_id=v.id;
CREATE INDEX executions_global_exit ON journey_executions(workspace_id,customer_id,exit_event_name) INCLUDE(id,version_id) WHERE state IN ('RUNNING','WAITING');
