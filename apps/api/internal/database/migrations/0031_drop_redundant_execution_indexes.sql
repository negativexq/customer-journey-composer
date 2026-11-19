-- Every runtime step updates journey_executions, and no update can be HOT while
-- state changes, so each one writes every index on the table. These three serve
-- no query another index does not serve as well:
-- executions_reentry is the leading part of executions_journey_customer_recent;
-- no query orders a journey's executions by start without a further key, which
-- executions_journey existed for; the only state-filtered journey read, the
-- active-or-waiting check before archiving, matches executions_one_active, so
-- executions_journey_state_recent only added an entry on every state change.
DROP INDEX executions_reentry;
DROP INDEX executions_journey;
DROP INDEX executions_journey_state_recent;
