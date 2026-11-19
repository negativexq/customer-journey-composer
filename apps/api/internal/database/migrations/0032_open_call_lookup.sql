-- The rate window's first part counts calls still waiting for an answer. With
-- no index of their own, the planner read every call made in the last 30
-- seconds to find the few still open: 6,700 rows and 2,000 blocks for 11 open
-- calls under load. Answered calls leave this index, so it holds only the open
-- ones, and its predicate covers the whole filter.
CREATE INDEX delivery_attempts_provider_open ON delivery_attempts(workspace_id,provider,started_at) WHERE completed_at IS NULL AND status<>'CANCELLED' AND provider IS NOT NULL;
