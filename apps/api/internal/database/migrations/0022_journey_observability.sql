-- Entry-cohort snapshots are projected asynchronously. Existing lifetime metrics
-- remain compatible; no historical scan or synchronous backfill in this migration.
ALTER TABLE metric_queue ADD COLUMN snapshot jsonb;
ALTER TABLE metric_queue ADD COLUMN baseline boolean NOT NULL DEFAULT false;
CREATE TABLE metric_snapshot_facts (
 workspace_id uuid NOT NULL, source_kind text NOT NULL, source_id uuid NOT NULL,
 sequence bigint NOT NULL, baseline boolean NOT NULL, snapshot jsonb NOT NULL,
 PRIMARY KEY(workspace_id,source_kind,source_id)
);
CREATE TABLE journey_cohort_metrics (
 workspace_id uuid NOT NULL, journey_id uuid NOT NULL, cohort_day date NOT NULL,
 version_id uuid NOT NULL, dimension text NOT NULL, dimension_key text NOT NULL,
 metric text NOT NULL, value double precision NOT NULL,
 PRIMARY KEY(workspace_id,journey_id,cohort_day,version_id,dimension,dimension_key,metric)
);
CREATE TABLE journey_cohort_members (
 workspace_id uuid NOT NULL,journey_id uuid NOT NULL,cohort_day date NOT NULL,
 version_id uuid NOT NULL,customer_id text NOT NULL,
 PRIMARY KEY(workspace_id,journey_id,cohort_day,version_id,customer_id)
);
CREATE TABLE metric_snapshot_backfill (
 source_kind text PRIMARY KEY CHECK(source_kind IN ('execution','step','delivery')),
 after_id uuid NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000',
 complete boolean NOT NULL DEFAULT false
);
INSERT INTO metric_snapshot_backfill(source_kind) VALUES('execution'),('step'),('delivery');
CREATE TABLE metric_projection_health (id boolean PRIMARY KEY DEFAULT true CHECK(id),last_success timestamptz NOT NULL);
-- Actual list/drilldown access paths: version/cohort, visited node and message source.
CREATE INDEX executions_journey_version_recent ON journey_executions(workspace_id,journey_id,version_id,started_at DESC,id DESC);
CREATE INDEX steps_workspace_node_execution ON step_executions(workspace_id,node_id,execution_id) INCLUDE(state);
CREATE INDEX delivery_execution_node_recent ON delivery_jobs(workspace_id,execution_id,node_id,created_at DESC,id DESC) WHERE mode='LIVE';

CREATE FUNCTION metric_snapshot(kind text,source uuid,workspace uuid,journey uuid,version uuid,entry_at timestamptz,customer text,metrics jsonb) RETURNS jsonb LANGUAGE sql STABLE AS $$
 SELECT jsonb_build_object('kind',kind,'id',source,'workspace_id',workspace,'journey_id',journey,'version_id',version,'cohort_day',(entry_at AT TIME ZONE 'UTC')::date,'customer_id',customer,'values',metrics)
$$;
CREATE FUNCTION metric_execution_snapshot(x journey_executions) RETURNS jsonb LANGUAGE plpgsql STABLE AS $$
DECLARE values jsonb;
BEGIN
 values:=jsonb_build_array(metric_delta('execution','','entries',1),metric_delta('execution','',x.state,1));
 IF x.exit_reason IS NOT NULL THEN
  values:=values||metric_delta('exit_reason',x.exit_reason,'exits',1);
  IF x.state='EXITED' THEN values:=values||metric_delta('node_exit',x.current_node_id,x.exit_reason,1); END IF;
 END IF;
 IF x.exit_reason='converted' THEN values:=values||metric_delta('execution','','converted',1); END IF;
 IF x.completed_at IS NOT NULL THEN values:=values||metric_delta('execution','','finished_count',1)||metric_delta('execution','','duration_ms',greatest(0,extract(epoch from x.completed_at-x.started_at)*1000)::double precision); END IF;
 IF x.state IN ('RUNNING','WAITING') THEN values:=values||metric_delta('node_current',x.current_node_id,x.state,1); END IF;
 RETURN metric_snapshot('execution',x.id,x.workspace_id,x.journey_id,x.version_id,x.started_at,x.customer_id,values);
END $$;
CREATE FUNCTION metric_step_snapshot(x step_executions,journey uuid,version uuid,entry_at timestamptz) RETURNS jsonb LANGUAGE plpgsql STABLE AS $$
DECLARE values jsonb;
BEGIN
 values:=jsonb_build_array(metric_delta('node',x.node_id,'entered',1),metric_delta('node',x.node_id,x.state,1));
 IF x.completed_at IS NOT NULL THEN values:=values||metric_delta('node',x.node_id,'finished_count',1)||metric_delta('node',x.node_id,'duration_ms',greatest(0,extract(epoch from x.completed_at-x.started_at)*1000)::double precision); END IF;
 IF x.state='SUCCEEDED' AND x.output->>'port' IN ('started','completed','true','false','matched','timeout','next') THEN values:=values||metric_delta('edge',x.node_id||':'||(x.output->>'port'),'traversed',1); END IF;
 RETURN metric_snapshot('step',x.id,x.workspace_id,journey,version,entry_at,NULL,values);
END $$;
CREATE FUNCTION metric_delivery_snapshot(x delivery_jobs,journey uuid,version uuid,entry_at timestamptz) RETURNS jsonb LANGUAGE plpgsql STABLE AS $$
DECLARE values jsonb; reason text;
BEGIN
 IF x.mode<>'LIVE' THEN RETURN NULL; END IF;
 values:=jsonb_build_array(metric_delta('channel',x.channel,'scheduled',1),metric_delta('channel',x.channel,x.status,1),metric_delta('node_delivery',x.node_id,'scheduled',1),metric_delta('node_delivery',x.node_id,x.status,1));
 IF x.attempts>0 THEN values:=values||metric_delta('channel',x.channel,'attempted',1)||metric_delta('node_delivery',x.node_id,'attempted',1)||metric_delta('node_delivery',x.node_id,'attempts',x.attempts); END IF;
 IF x.status IN ('BLOCKED','SKIPPED','CANCELLED','FAILED','UNKNOWN') THEN
  reason:=CASE WHEN x.status='BLOCKED' THEN COALESCE(NULLIF(x.decision->>'reason',''),'blocked') WHEN x.status IN ('SKIPPED','CANCELLED') THEN COALESCE(NULLIF(x.decision->>'reason',''),lower(x.status)) ELSE lower(x.status) END;
  values:=values||metric_delta('node_reason',x.node_id,reason,1)||metric_delta('channel_reason',x.channel,reason,1);
 END IF;
 RETURN metric_snapshot('delivery',x.id,x.workspace_id,journey,version,entry_at,NULL,values);
END $$;

CREATE OR REPLACE FUNCTION capture_execution_metrics() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE deltas jsonb := '[]';
BEGIN
 IF TG_OP='INSERT' THEN
  deltas:=deltas||metric_delta('execution','','entries',1);
 ELSE
  IF OLD.state=NEW.state AND OLD.exit_reason IS NOT DISTINCT FROM NEW.exit_reason AND OLD.completed_at IS NOT DISTINCT FROM NEW.completed_at THEN
    IF OLD.current_node_id=NEW.current_node_id THEN RETURN NEW; END IF;
    INSERT INTO metric_queue(workspace_id,journey_id,version_id,cohort_at,deltas,snapshot) VALUES(NEW.workspace_id,NEW.journey_id,NEW.version_id,NEW.started_at,'[]',metric_execution_snapshot(NEW));
    RETURN NEW;
   END IF;
  deltas:=deltas||metric_delta('execution','',OLD.state,-1);
  IF OLD.exit_reason IS NOT NULL THEN deltas:=deltas||metric_delta('exit_reason',OLD.exit_reason,'exits',-1); END IF;
  IF OLD.exit_reason='converted' THEN deltas:=deltas||metric_delta('execution','','converted',-1); END IF;
  IF OLD.completed_at IS NOT NULL THEN deltas:=deltas||metric_delta('execution','','finished_count',-1)||metric_delta('execution','','duration_ms',-greatest(0,extract(epoch from OLD.completed_at-OLD.started_at)*1000)::double precision); END IF;
 END IF;
 deltas:=deltas||metric_delta('execution','',NEW.state,1);
 IF NEW.exit_reason IS NOT NULL THEN deltas:=deltas||metric_delta('exit_reason',NEW.exit_reason,'exits',1); END IF;
 IF NEW.exit_reason='converted' THEN deltas:=deltas||metric_delta('execution','','converted',1); END IF;
 IF NEW.completed_at IS NOT NULL THEN deltas:=deltas||metric_delta('execution','','finished_count',1)||metric_delta('execution','','duration_ms',greatest(0,extract(epoch from NEW.completed_at-NEW.started_at)*1000)::double precision); END IF;
 INSERT INTO metric_queue(workspace_id,journey_id,version_id,cohort_at,customer_id,deltas,snapshot) VALUES(NEW.workspace_id,NEW.journey_id,NEW.version_id,NEW.started_at,CASE WHEN TG_OP='INSERT' THEN NEW.customer_id END,deltas,metric_execution_snapshot(NEW));
 RETURN NEW;
END $$;

CREATE OR REPLACE FUNCTION capture_step_metrics() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE deltas jsonb:='[]'; jid uuid; vid uuid; entry_at timestamptz;
BEGIN
 IF TG_OP='INSERT' THEN deltas:=deltas||metric_delta('node',NEW.node_id,'entered',1);
 ELSE
  IF OLD.state=NEW.state AND OLD.completed_at IS NOT DISTINCT FROM NEW.completed_at THEN RETURN NEW; END IF;
  deltas:=deltas||metric_delta('node',OLD.node_id,OLD.state,-1);
  IF OLD.completed_at IS NOT NULL THEN deltas:=deltas||metric_delta('node',OLD.node_id,'finished_count',-1)||metric_delta('node',OLD.node_id,'duration_ms',-greatest(0,extract(epoch from OLD.completed_at-OLD.started_at)*1000)::double precision); END IF;
 END IF;
 deltas:=deltas||metric_delta('node',NEW.node_id,NEW.state,1);
 IF NEW.completed_at IS NOT NULL THEN deltas:=deltas||metric_delta('node',NEW.node_id,'finished_count',1)||metric_delta('node',NEW.node_id,'duration_ms',greatest(0,extract(epoch from NEW.completed_at-NEW.started_at)*1000)::double precision); END IF;
 SELECT journey_id,version_id,started_at INTO jid,vid,entry_at FROM journey_executions WHERE id=NEW.execution_id;
 INSERT INTO metric_queue(workspace_id,journey_id,version_id,cohort_at,deltas,snapshot) VALUES(NEW.workspace_id,jid,vid,NEW.started_at,deltas,metric_step_snapshot(NEW,jid,vid,entry_at));
 RETURN NEW;
END $$;

CREATE OR REPLACE FUNCTION capture_delivery_metrics() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE deltas jsonb:='[]'; jid uuid; vid uuid; entry_at timestamptz;
BEGIN
 IF NEW.mode<>'LIVE' THEN RETURN NEW; END IF;
 IF TG_OP='INSERT' THEN deltas:=deltas||metric_delta('channel',NEW.channel,'scheduled',1);
 ELSE
  IF OLD.status=NEW.status AND OLD.attempts=NEW.attempts AND OLD.decision IS NOT DISTINCT FROM NEW.decision THEN RETURN NEW; END IF;
  deltas:=deltas||metric_delta('channel',OLD.channel,OLD.status,-1);
  IF OLD.attempts>0 THEN deltas:=deltas||metric_delta('channel',OLD.channel,'attempted',-1); END IF;
 END IF;
 deltas:=deltas||metric_delta('channel',NEW.channel,NEW.status,1);
 IF NEW.attempts>0 THEN deltas:=deltas||metric_delta('channel',NEW.channel,'attempted',1); END IF;
 SELECT journey_id,version_id,started_at INTO jid,vid,entry_at FROM journey_executions WHERE id=NEW.execution_id;
 INSERT INTO metric_queue(workspace_id,journey_id,version_id,cohort_at,deltas,snapshot) VALUES(NEW.workspace_id,jid,vid,NEW.created_at,deltas,metric_delivery_snapshot(NEW,jid,vid,entry_at));
 RETURN NEW;
END $$;

DROP TRIGGER executions_metrics ON journey_executions;
CREATE TRIGGER executions_metrics AFTER INSERT OR UPDATE OF state,exit_reason,completed_at,current_node_id ON journey_executions FOR EACH ROW EXECUTE FUNCTION capture_execution_metrics();
DROP TRIGGER deliveries_metrics ON delivery_jobs;
CREATE TRIGGER deliveries_metrics AFTER INSERT OR UPDATE OF status,attempts,decision ON delivery_jobs FOR EACH ROW EXECUTE FUNCTION capture_delivery_metrics();
