-- What became of a message after the provider accepted it. A delivery's status
-- only ever comes from our own provider calls; reports and tracking land here.
-- The primary key keeps the first report of each kind, so a repeated webhook,
-- a second open or another click never counts twice.
CREATE TABLE delivery_events (
    workspace_id uuid NOT NULL,
    delivery_id uuid NOT NULL,
    kind text NOT NULL CHECK (kind IN ('delivered','bounced','opened','clicked','unsubscribed')),
    occurred_at timestamptz NOT NULL,
    source text NOT NULL CHECK (source IN ('provider','tracking','simulated')),
    detail jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(detail)='object'),
    -- Set once the matching customer event reached the journey runtime.
    ingested_at timestamptz,
    PRIMARY KEY (delivery_id,kind),
    FOREIGN KEY (workspace_id,delivery_id) REFERENCES delivery_jobs(workspace_id,id)
);

-- Each tracked link, open pixel and unsubscribe link of a sent message gets an
-- unguessable code of its own. A click redirects only to the stored URL, so
-- the endpoint is never an open redirect, and a retry resends the same codes.
CREATE TABLE tracking_codes (
    code text PRIMARY KEY CHECK (code ~ '^[A-Za-z0-9]{12}$'),
    workspace_id uuid NOT NULL,
    delivery_id uuid NOT NULL,
    kind text NOT NULL CHECK (kind IN ('open','click','unsubscribe')),
    url text NOT NULL DEFAULT '' CHECK (char_length(url)<=2000),
    uses integer NOT NULL DEFAULT 0,
    FOREIGN KEY (workspace_id,delivery_id) REFERENCES delivery_jobs(workspace_id,id)
);
CREATE INDEX tracking_codes_delivery ON tracking_codes(workspace_id,delivery_id);

-- Engagement joins the delivery's snapshot, so journey insights count it per
-- cohort and version like every other delivery outcome.
CREATE OR REPLACE FUNCTION metric_delivery_snapshot(x delivery_jobs,journey uuid,version uuid,entry_at timestamptz) RETURNS jsonb LANGUAGE plpgsql STABLE AS $$
DECLARE values jsonb; reason text;
BEGIN
 IF x.mode<>'LIVE' THEN RETURN NULL; END IF;
 values:=jsonb_build_array(metric_delta('channel',x.channel,'scheduled',1),metric_delta('channel',x.channel,x.status,1),metric_delta('node_delivery',x.node_id,'scheduled',1),metric_delta('node_delivery',x.node_id,x.status,1));
 IF x.attempts>0 THEN values:=values||metric_delta('channel',x.channel,'attempted',1)||metric_delta('node_delivery',x.node_id,'attempted',1)||metric_delta('node_delivery',x.node_id,'attempts',x.attempts); END IF;
 IF x.status IN ('BLOCKED','SKIPPED','CANCELLED','FAILED','UNKNOWN') THEN
  reason:=CASE WHEN x.status='BLOCKED' THEN COALESCE(NULLIF(x.decision->>'reason',''),'blocked') WHEN x.status IN ('SKIPPED','CANCELLED') THEN COALESCE(NULLIF(x.decision->>'reason',''),lower(x.status)) ELSE lower(x.status) END;
  values:=values||metric_delta('node_reason',x.node_id,reason,1)||metric_delta('channel_reason',x.channel,reason,1);
 END IF;
 values:=values||COALESCE((SELECT jsonb_agg(v) FROM delivery_events e CROSS JOIN LATERAL (VALUES (metric_delta('channel',x.channel,e.kind,1)),(metric_delta('node_delivery',x.node_id,e.kind,1))) t(v) WHERE e.delivery_id=x.id),'[]'::jsonb);
 RETURN metric_snapshot('delivery',x.id,x.workspace_id,journey,version,entry_at,NULL,values);
END $$;

CREATE FUNCTION capture_engagement_metrics() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE d delivery_jobs; jid uuid; vid uuid; entry_at timestamptz;
BEGIN
 SELECT * INTO d FROM delivery_jobs WHERE id=NEW.delivery_id;
 IF d.mode<>'LIVE' OR d.execution_id IS NULL THEN RETURN NEW; END IF;
 SELECT journey_id,version_id,started_at INTO jid,vid,entry_at FROM journey_executions WHERE id=d.execution_id;
 INSERT INTO metric_queue(workspace_id,journey_id,version_id,cohort_at,deltas,snapshot) VALUES(d.workspace_id,jid,vid,d.created_at,jsonb_build_array(metric_delta('channel',d.channel,NEW.kind,1)),metric_delivery_snapshot(d,jid,vid,entry_at));
 RETURN NEW;
END $$;
CREATE TRIGGER delivery_events_metrics AFTER INSERT ON delivery_events FOR EACH ROW EXECUTE FUNCTION capture_engagement_metrics();
