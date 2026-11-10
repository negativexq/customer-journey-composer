ALTER TABLE journey_executions ADD COLUMN trace_parent text NOT NULL DEFAULT '' CHECK(char_length(trace_parent)<=128);
