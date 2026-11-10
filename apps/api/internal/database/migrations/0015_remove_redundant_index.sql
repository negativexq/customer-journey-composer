-- Existing executions_journey already covers workspace/journey/id lookups.
-- Avoid another B-tree write per enrollment for an infrequent drain check.
DROP INDEX executions_journey_identity;
