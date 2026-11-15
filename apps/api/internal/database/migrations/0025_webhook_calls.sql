-- One row per webhook step an execution reaches. The step writes the row in
-- its own transaction; a dispatcher sends it afterwards. UNIQUE(execution_id,
-- node_id) makes the logical call idempotent however often the step is retried.
CREATE TABLE webhook_calls (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    execution_id uuid NOT NULL,
    node_id text NOT NULL,
    customer_id text NOT NULL,
    url text NOT NULL,
    payload jsonb NOT NULL,
    state text NOT NULL DEFAULT 'PENDING' CHECK (state IN ('PENDING','RECORDED','SENT','FAILED')),
    attempts integer NOT NULL DEFAULT 0,
    response_status integer,
    detail text,
    created_at timestamptz NOT NULL,
    completed_at timestamptz,
    FOREIGN KEY (workspace_id,execution_id) REFERENCES journey_executions(workspace_id,id),
    UNIQUE (execution_id,node_id)
);
CREATE INDEX webhook_calls_pending ON webhook_calls(created_at,id) WHERE state='PENDING';
CREATE INDEX webhook_calls_execution ON webhook_calls(workspace_id,execution_id);
