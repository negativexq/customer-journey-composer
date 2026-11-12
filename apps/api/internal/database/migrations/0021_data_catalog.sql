-- Design-time metadata only. No discovery work or indexes on hot runtime tables.
CREATE TABLE workspace_data_catalog (
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    key text NOT NULL CHECK (char_length(key) BETWEEN 1 AND 400),
    kind text NOT NULL CHECK (kind IN ('event','customer_field','event_field')),
    event_name text NOT NULL DEFAULT '',
    path text NOT NULL DEFAULT '',
    label text NOT NULL CHECK (char_length(label) BETWEEN 1 AND 160),
    description text NOT NULL DEFAULT '' CHECK (char_length(description)<=1000),
    data_type text NOT NULL CHECK (data_type IN ('event','string','number','boolean','mixed')),
    aliases jsonb NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(aliases)='array' AND jsonb_array_length(aliases)<=12),
    enabled boolean NOT NULL DEFAULT true,
    source text NOT NULL CHECK (source IN ('declared','observed')),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id,key)
);
