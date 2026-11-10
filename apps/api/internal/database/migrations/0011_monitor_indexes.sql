CREATE INDEX executions_workspace_failed ON journey_executions(workspace_id,updated_at DESC,id DESC) WHERE state='FAILED';
