DROP INDEX journeys_workspace_updated;
CREATE INDEX journeys_workspace_updated ON journeys(workspace_id,updated_at DESC,id DESC);
CREATE INDEX journeys_workspace_status_updated ON journeys(workspace_id,status,updated_at DESC,id DESC);
CREATE INDEX journeys_workspace_name ON journeys(workspace_id,(lower(name) COLLATE "C"),id);
