-- Event inspection follows the ingestion foreign key, independently of journey.
-- Keep payloads out of this index; writes add only one small entry per enrollment.
CREATE INDEX executions_entry_event ON journey_executions(workspace_id,entry_event_id,started_at DESC,id);
