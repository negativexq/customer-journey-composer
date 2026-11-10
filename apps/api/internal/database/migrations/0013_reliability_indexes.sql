CREATE INDEX jobs_workspace_failed ON scheduled_jobs(workspace_id,created_at DESC,id DESC) WHERE state='FAILED';
CREATE INDEX delivery_workspace_failures ON delivery_jobs(workspace_id,created_at DESC,id DESC) WHERE status IN ('FAILED','UNKNOWN');
-- Pending outbox checks after sunset use a single bounded execution lookup.
CREATE INDEX executions_journey_identity ON journey_executions(workspace_id,journey_id,id);
