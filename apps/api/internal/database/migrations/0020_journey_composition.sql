ALTER TABLE journeys ADD COLUMN creation_step text NOT NULL DEFAULT 'builder'
    CHECK (creation_step IN ('details','method','brief','review','builder'));

-- Large brief/proposal data is loaded only on the creation page, never in lists
-- or execution/analytics queries. The composite key enforces workspace isolation.
CREATE TABLE journey_compositions (
    workspace_id uuid NOT NULL,
    journey_id uuid NOT NULL,
    state jsonb NOT NULL CHECK (jsonb_typeof(state) = 'object' AND octet_length(state::text) <= 1048576),
    PRIMARY KEY (workspace_id, journey_id),
    FOREIGN KEY (workspace_id, journey_id) REFERENCES journeys(workspace_id, id) ON DELETE CASCADE
);
