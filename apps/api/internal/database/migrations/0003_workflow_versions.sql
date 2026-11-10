CREATE TABLE journey_versions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    journey_id uuid NOT NULL,
    version integer NOT NULL CHECK (version > 0),
    workflow jsonb NOT NULL,
    published_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (workspace_id, journey_id) REFERENCES journeys(workspace_id, id),
    FOREIGN KEY (workspace_id, published_by) REFERENCES users(workspace_id, id),
    UNIQUE (workspace_id, journey_id, version),
    UNIQUE (workspace_id, journey_id, id)
);
ALTER TABLE journeys ADD COLUMN current_version_id uuid;
ALTER TABLE journeys ADD CONSTRAINT current_version_workspace_fk FOREIGN KEY (workspace_id,id,current_version_id) REFERENCES journey_versions(workspace_id,journey_id,id);
CREATE FUNCTION prevent_version_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'Published journey versions are immutable';
END;
$$;
CREATE TRIGGER immutable_version BEFORE UPDATE OR DELETE ON journey_versions FOR EACH ROW EXECUTE FUNCTION prevent_version_mutation();
