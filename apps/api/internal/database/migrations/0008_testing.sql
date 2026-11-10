-- Large immutable workflow JSON is deduplicated across scenarios/replays.
CREATE TABLE sandbox_workflows (
    workspace_id uuid NOT NULL REFERENCES workspaces(id),
    workflow_hash text NOT NULL,
    workflow jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id,workflow_hash)
);
CREATE TRIGGER immutable_sandbox_workflow BEFORE UPDATE ON sandbox_workflows FOR EACH ROW EXECUTE FUNCTION prevent_version_mutation();
CREATE TABLE test_scenarios (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    journey_id uuid NOT NULL,
    version_id uuid,
    draft_revision integer,
    workflow_hash text NOT NULL,
    source_customer_id text,
    name text NOT NULL,
    scenario jsonb NOT NULL,
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (workspace_id,journey_id) REFERENCES journeys(workspace_id,id),
    FOREIGN KEY (workspace_id,journey_id,version_id) REFERENCES journey_versions(workspace_id,journey_id,id),
    FOREIGN KEY (workspace_id,workflow_hash) REFERENCES sandbox_workflows(workspace_id,workflow_hash),
    FOREIGN KEY (workspace_id,created_by) REFERENCES users(workspace_id,id),
    UNIQUE (workspace_id,id)
);
CREATE INDEX scenarios_workspace_recent ON test_scenarios(workspace_id,created_at DESC,id);
CREATE INDEX scenarios_customer ON test_scenarios(workspace_id,source_customer_id) WHERE source_customer_id IS NOT NULL;
CREATE TABLE test_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    journey_id uuid NOT NULL,
    version_id uuid,
    draft_revision integer,
    scenario_id uuid,
    parent_run_id uuid,
    workflow_hash text NOT NULL,
    source_customer_id text,
    mode text NOT NULL CHECK (mode IN ('FULL','SINGLE_NODE')),
    scenario jsonb NOT NULL,
    state jsonb NOT NULL,
    execution_state text NOT NULL,
    result text NOT NULL DEFAULT 'PENDING' CHECK (result IN ('PENDING','PASS','FAIL')),
    finalized boolean NOT NULL DEFAULT false,
    virtual_now timestamptz NOT NULL,
    message_count integer NOT NULL DEFAULT 0,
    revision integer NOT NULL DEFAULT 1,
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (workspace_id,journey_id) REFERENCES journeys(workspace_id,id),
    FOREIGN KEY (workspace_id,journey_id,version_id) REFERENCES journey_versions(workspace_id,journey_id,id),
    FOREIGN KEY (workspace_id,scenario_id) REFERENCES test_scenarios(workspace_id,id),
    FOREIGN KEY (workspace_id,workflow_hash) REFERENCES sandbox_workflows(workspace_id,workflow_hash),
    FOREIGN KEY (workspace_id,created_by) REFERENCES users(workspace_id,id),
    UNIQUE (workspace_id,id),
    FOREIGN KEY (workspace_id,parent_run_id) REFERENCES test_runs(workspace_id,id)
);
CREATE INDEX test_runs_recent ON test_runs(workspace_id,created_at DESC,id);
CREATE INDEX test_runs_journey ON test_runs(workspace_id,journey_id,created_at DESC,id);
CREATE INDEX test_runs_publish_gate ON test_runs(workspace_id,journey_id,workflow_hash,created_at DESC) WHERE result='PASS' AND mode='FULL' AND finalized;
CREATE INDEX test_runs_customer ON test_runs(workspace_id,source_customer_id) WHERE source_customer_id IS NOT NULL;
-- Rendered bodies are stored once, outside the frequently updated run state.
CREATE TABLE test_messages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id),
    test_run_id uuid,
    delivery_id uuid,
    journey_id uuid,
    version_id uuid,
    message_key text,
    execution_index integer,
    customer_id text,
    source_node text,
    node_label text NOT NULL,
    channel text NOT NULL CHECK (channel IN ('email','sms')),
    mode text NOT NULL CHECK (mode IN ('SIMULATED','SINGLE_NODE','REAL_TEST')),
    status text NOT NULL CHECK (status IN ('SIMULATED','BLOCKED','SKIPPED','FAILED','TEST_ACCEPTED')),
    simulated_at timestamptz NOT NULL,
    scheduled_at timestamptz NOT NULL,
    message jsonb NOT NULL,
    decision jsonb NOT NULL,
    attempt_history jsonb NOT NULL DEFAULT '[]',
    provider_response text,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (workspace_id,test_run_id) REFERENCES test_runs(workspace_id,id),
    FOREIGN KEY (workspace_id,delivery_id) REFERENCES delivery_jobs(workspace_id,id),
    FOREIGN KEY (workspace_id,journey_id) REFERENCES journeys(workspace_id,id),
    UNIQUE (test_run_id,message_key),
    UNIQUE (delivery_id),
    UNIQUE (workspace_id,id)
);
CREATE INDEX test_inbox_recent ON test_messages(workspace_id,created_at DESC,id);
CREATE INDEX test_inbox_run ON test_messages(workspace_id,test_run_id,simulated_at,id);
CREATE INDEX test_inbox_customer ON test_messages(workspace_id,customer_id,created_at DESC,id);
CREATE INDEX test_inbox_journey ON test_messages(workspace_id,journey_id,created_at DESC,id);
