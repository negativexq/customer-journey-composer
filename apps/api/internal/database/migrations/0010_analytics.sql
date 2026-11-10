-- Replace the superseded inbox run index; avoid paying for equivalent prefixes.
DROP INDEX test_inbox_run;
CREATE INDEX executions_workspace_recent ON journey_executions(workspace_id,started_at DESC,id DESC);
CREATE INDEX executions_journey_state_recent ON journey_executions(workspace_id,journey_id,state,started_at DESC,id DESC);
CREATE INDEX executions_journey_customer_recent ON journey_executions(workspace_id,journey_id,customer_id,started_at DESC,id DESC);
CREATE TABLE metric_queue (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 workspace_id uuid NOT NULL,
 journey_id uuid NOT NULL,
 version_id uuid NOT NULL,
 cohort_at timestamptz NOT NULL,
 customer_id text,
 deltas jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE metric_totals (
 workspace_id uuid NOT NULL,
 journey_id uuid NOT NULL,
 version_id uuid NOT NULL,
 dimension text NOT NULL,
 dimension_key text NOT NULL,
 metric text NOT NULL,
 value double precision NOT NULL,
 PRIMARY KEY(workspace_id,journey_id,version_id,dimension,dimension_key,metric)
);
CREATE TABLE metric_hourly (
 workspace_id uuid NOT NULL,
 journey_id uuid NOT NULL,
 version_id uuid NOT NULL,
 bucket timestamptz NOT NULL,
 dimension text NOT NULL,
 dimension_key text NOT NULL,
 metric text NOT NULL,
 value double precision NOT NULL,
 PRIMARY KEY(workspace_id,journey_id,bucket,version_id,dimension,dimension_key,metric)
);
-- First entry per journey/customer, never sum daily distinct counts.
CREATE TABLE metric_customers (
 workspace_id uuid NOT NULL,
 journey_id uuid NOT NULL,
 customer_id text NOT NULL,
 PRIMARY KEY(workspace_id,journey_id,customer_id)
);
CREATE TABLE metric_watermarks (
 workspace_id uuid PRIMARY KEY,
 processed_at timestamptz NOT NULL,
 source_at timestamptz NOT NULL
);
CREATE FUNCTION metric_delta(dimension text,dimension_key text,metric text,value double precision) RETURNS jsonb LANGUAGE sql IMMUTABLE AS $$ SELECT jsonb_build_object('dimension',dimension,'key',dimension_key,'metric',metric,'value',value) $$;
CREATE FUNCTION capture_execution_metrics() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE deltas jsonb := '[]';
BEGIN
 IF TG_OP='INSERT' THEN
  deltas:=deltas||metric_delta('execution','','entries',1);
 ELSE
  IF OLD.state=NEW.state AND OLD.exit_reason IS NOT DISTINCT FROM NEW.exit_reason AND OLD.completed_at IS NOT DISTINCT FROM NEW.completed_at THEN RETURN NEW; END IF;
  deltas:=deltas||metric_delta('execution','',OLD.state,-1);
  IF OLD.exit_reason IS NOT NULL THEN deltas:=deltas||metric_delta('exit_reason',OLD.exit_reason,'exits',-1); END IF;
  IF OLD.exit_reason='converted' THEN deltas:=deltas||metric_delta('execution','','converted',-1); END IF;
  IF OLD.completed_at IS NOT NULL THEN deltas:=deltas||metric_delta('execution','','finished_count',-1)||metric_delta('execution','','duration_ms',-greatest(0,extract(epoch from OLD.completed_at-OLD.started_at)*1000)::double precision); END IF;
 END IF;
 deltas:=deltas||metric_delta('execution','',NEW.state,1);
 IF NEW.exit_reason IS NOT NULL THEN deltas:=deltas||metric_delta('exit_reason',NEW.exit_reason,'exits',1); END IF;
 IF NEW.exit_reason='converted' THEN deltas:=deltas||metric_delta('execution','','converted',1); END IF;
 IF NEW.completed_at IS NOT NULL THEN deltas:=deltas||metric_delta('execution','','finished_count',1)||metric_delta('execution','','duration_ms',greatest(0,extract(epoch from NEW.completed_at-NEW.started_at)*1000)::double precision); END IF;
 INSERT INTO metric_queue(workspace_id,journey_id,version_id,cohort_at,customer_id,deltas) VALUES(NEW.workspace_id,NEW.journey_id,NEW.version_id,NEW.started_at,CASE WHEN TG_OP='INSERT' THEN NEW.customer_id END,deltas);
 RETURN NEW;
END $$;
CREATE TRIGGER executions_metrics AFTER INSERT OR UPDATE OF state,exit_reason,completed_at ON journey_executions FOR EACH ROW EXECUTE FUNCTION capture_execution_metrics();
CREATE FUNCTION capture_step_metrics() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE deltas jsonb:='[]'; jid uuid; vid uuid;
BEGIN
 IF TG_OP='INSERT' THEN deltas:=deltas||metric_delta('node',NEW.node_id,'entered',1);
 ELSE
  IF OLD.state=NEW.state AND OLD.completed_at IS NOT DISTINCT FROM NEW.completed_at THEN RETURN NEW; END IF;
  deltas:=deltas||metric_delta('node',OLD.node_id,OLD.state,-1);
  IF OLD.completed_at IS NOT NULL THEN deltas:=deltas||metric_delta('node',OLD.node_id,'finished_count',-1)||metric_delta('node',OLD.node_id,'duration_ms',-greatest(0,extract(epoch from OLD.completed_at-OLD.started_at)*1000)::double precision); END IF;
 END IF;
 deltas:=deltas||metric_delta('node',NEW.node_id,NEW.state,1);
 IF NEW.completed_at IS NOT NULL THEN deltas:=deltas||metric_delta('node',NEW.node_id,'finished_count',1)||metric_delta('node',NEW.node_id,'duration_ms',greatest(0,extract(epoch from NEW.completed_at-NEW.started_at)*1000)::double precision); END IF;
 SELECT journey_id,version_id INTO jid,vid FROM journey_executions WHERE id=NEW.execution_id;
 INSERT INTO metric_queue(workspace_id,journey_id,version_id,cohort_at,deltas) VALUES(NEW.workspace_id,jid,vid,NEW.started_at,deltas);
 RETURN NEW;
END $$;
CREATE TRIGGER steps_metrics AFTER INSERT OR UPDATE OF state,completed_at ON step_executions FOR EACH ROW EXECUTE FUNCTION capture_step_metrics();
CREATE FUNCTION capture_delivery_metrics() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE deltas jsonb:='[]'; jid uuid; vid uuid;
BEGIN
 IF NEW.mode<>'LIVE' THEN RETURN NEW; END IF;
 IF TG_OP='INSERT' THEN deltas:=deltas||metric_delta('channel',NEW.channel,'scheduled',1);
 ELSE
  IF OLD.status=NEW.status AND (OLD.attempts>0)=(NEW.attempts>0) THEN RETURN NEW; END IF;
  deltas:=deltas||metric_delta('channel',OLD.channel,OLD.status,-1);
  IF OLD.attempts>0 THEN deltas:=deltas||metric_delta('channel',OLD.channel,'attempted',-1); END IF;
 END IF;
 deltas:=deltas||metric_delta('channel',NEW.channel,NEW.status,1);
 IF NEW.attempts>0 THEN deltas:=deltas||metric_delta('channel',NEW.channel,'attempted',1); END IF;
 SELECT journey_id,version_id INTO jid,vid FROM journey_executions WHERE id=NEW.execution_id;
 INSERT INTO metric_queue(workspace_id,journey_id,version_id,cohort_at,deltas) VALUES(NEW.workspace_id,jid,vid,NEW.created_at,deltas);
 RETURN NEW;
END $$;
CREATE TRIGGER deliveries_metrics AFTER INSERT OR UPDATE OF status,attempts ON delivery_jobs FOR EACH ROW EXECUTE FUNCTION capture_delivery_metrics();
CREATE FUNCTION capture_journey_metrics() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE deltas jsonb:='[]'; zero_id uuid:='00000000-0000-0000-0000-000000000000';
BEGIN
 IF TG_OP='INSERT' THEN deltas:=deltas||metric_delta('journey','','total',1);
 ELSE
  IF OLD.status=NEW.status THEN RETURN NEW; END IF;
  deltas:=deltas||metric_delta('journey','',OLD.status,-1);
 END IF;
 deltas:=deltas||metric_delta('journey','',NEW.status,1);
 INSERT INTO metric_queue(workspace_id,journey_id,version_id,cohort_at,deltas) VALUES(NEW.workspace_id,zero_id,zero_id,NEW.created_at,deltas);
 RETURN NEW;
END $$;
CREATE TRIGGER journeys_metrics AFTER INSERT OR UPDATE OF status ON journeys FOR EACH ROW EXECUTE FUNCTION capture_journey_metrics();
-- Bootstrap existing records once, using the same INSERT snapshot semantics.
-- Transition tables aren't needed at this MVP write rate; each row appends one compact delta.
INSERT INTO metric_queue(workspace_id,journey_id,version_id,cohort_at,customer_id,deltas)
 SELECT workspace_id,journey_id,version_id,started_at,customer_id,
 jsonb_build_array(metric_delta('execution','','entries',1),metric_delta('execution','',state,1))
 || CASE WHEN exit_reason IS NOT NULL THEN jsonb_build_array(metric_delta('exit_reason',exit_reason,'exits',1)) ELSE '[]'::jsonb END
 || CASE WHEN exit_reason='converted' THEN jsonb_build_array(metric_delta('execution','','converted',1)) ELSE '[]'::jsonb END
 || CASE WHEN completed_at IS NOT NULL THEN jsonb_build_array(metric_delta('execution','','finished_count',1),metric_delta('execution','','duration_ms',greatest(0,extract(epoch from completed_at-started_at)*1000)::double precision)) ELSE '[]'::jsonb END FROM journey_executions;
INSERT INTO metric_queue(workspace_id,journey_id,version_id,cohort_at,deltas)
 SELECT s.workspace_id,e.journey_id,e.version_id,s.started_at,jsonb_build_array(metric_delta('node',s.node_id,'entered',1),metric_delta('node',s.node_id,s.state,1))||CASE WHEN s.completed_at IS NOT NULL THEN jsonb_build_array(metric_delta('node',s.node_id,'finished_count',1),metric_delta('node',s.node_id,'duration_ms',greatest(0,extract(epoch from s.completed_at-s.started_at)*1000)::double precision)) ELSE '[]'::jsonb END FROM step_executions s JOIN journey_executions e ON e.id=s.execution_id;
INSERT INTO metric_queue(workspace_id,journey_id,version_id,cohort_at,deltas)
 SELECT d.workspace_id,e.journey_id,e.version_id,d.created_at,jsonb_build_array(metric_delta('channel',d.channel,'scheduled',1),metric_delta('channel',d.channel,d.status,1))||CASE WHEN d.attempts>0 THEN jsonb_build_array(metric_delta('channel',d.channel,'attempted',1)) ELSE '[]'::jsonb END FROM delivery_jobs d JOIN journey_executions e ON e.id=d.execution_id WHERE d.mode='LIVE';
INSERT INTO metric_queue(workspace_id,journey_id,version_id,cohort_at,deltas)
 SELECT workspace_id,'00000000-0000-0000-0000-000000000000','00000000-0000-0000-0000-000000000000',created_at,jsonb_build_array(metric_delta('journey','','total',1),metric_delta('journey','',status,1)) FROM journeys;
