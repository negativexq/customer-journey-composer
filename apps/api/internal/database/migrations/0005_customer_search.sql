-- Prefix search uses a B-tree, not a write-heavy GIN over arbitrary attributes.
CREATE INDEX customers_email_prefix ON customers(workspace_id,lower(email) text_pattern_ops) WHERE email IS NOT NULL;
