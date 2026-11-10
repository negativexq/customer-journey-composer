ALTER TABLE journeys ADD COLUMN resume_status text NOT NULL DEFAULT 'Active' CHECK(resume_status IN ('Active','Sunset'));
ALTER TABLE scheduled_jobs ADD COLUMN manual_retries integer NOT NULL DEFAULT 0 CHECK(manual_retries BETWEEN 0 AND 3);
ALTER TABLE delivery_jobs ADD COLUMN manual_retries integer NOT NULL DEFAULT 0 CHECK(manual_retries BETWEEN 0 AND 3);
CREATE TABLE audit_logs (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 workspace_id uuid NOT NULL REFERENCES workspaces(id),
 actor_id uuid,
 action text NOT NULL,
 resource_id uuid,
 details jsonb NOT NULL DEFAULT '{}',
 created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(workspace_id,actor_id) REFERENCES users(workspace_id,id)
);
CREATE INDEX audit_workspace_recent ON audit_logs(workspace_id,created_at DESC,id DESC);
CREATE INDEX audit_resource ON audit_logs(workspace_id,resource_id,created_at DESC,id DESC) WHERE resource_id IS NOT NULL;
CREATE INDEX delivery_journey_pending_lookup ON delivery_jobs(workspace_id,execution_id) WHERE mode='LIVE' AND status IN ('SCHEDULED','CLAIMED','DISPATCHING');
