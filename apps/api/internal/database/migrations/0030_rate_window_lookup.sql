-- The rate window counts calls still in flight, calls answered within the last
-- second and unanswered calls that may yet reach the provider. Reading those by
-- when they were answered touches only the calls that count, instead of every
-- call made in the last 30 seconds.
CREATE INDEX delivery_attempts_provider_answered ON delivery_attempts(workspace_id,provider,completed_at) WHERE provider IS NOT NULL;
CREATE INDEX delivery_attempts_provider_unanswered ON delivery_attempts(workspace_id,provider,started_at) WHERE status='UNKNOWN' OR provider_code=408;
