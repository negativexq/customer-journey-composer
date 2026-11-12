-- The monitor checks due work inside one workspace. Keep global jobs_due for
-- workers and avoid probing every waiting execution on a healthy monitor poll.
CREATE INDEX jobs_workspace_due ON scheduled_jobs(workspace_id,due_at,execution_id) WHERE state='SCHEDULED';
