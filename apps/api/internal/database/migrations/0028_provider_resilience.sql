-- How each workspace's provider account is doing. Every worker reads and
-- changes it under a row lock, so a 429 pause, a refused key or an open breaker
-- holds the channel for all of them at once. PostgreSQL stays the only store.
CREATE TABLE provider_channels (
    workspace_id uuid NOT NULL REFERENCES workspaces(id),
    provider text NOT NULL CHECK (provider IN ('smtp','mock_sms','mock_email')),
    -- Calls the provider account accepts in one second.
    rate_per_second integer NOT NULL DEFAULT 50 CHECK (rate_per_second BETWEEN 1 AND 1000),
    -- A 429's Retry-After, or a full rate window: nothing is claimed until then.
    paused_until timestamptz,
    pause_reason text CHECK (pause_reason IN ('throttled','rate_limit')),
    -- A 401 locks the channel; one call tests the credentials every 30 seconds.
    credentials_refused_at timestamptz,
    next_probe_at timestamptz,
    -- The circuit breaker: open while the provider fails, then a few probes.
    breaker text NOT NULL DEFAULT 'closed' CHECK (breaker IN ('closed','open','probing')),
    breaker_until timestamptz,
    breaker_wait_seconds integer NOT NULL DEFAULT 0 CHECK (breaker_wait_seconds BETWEEN 0 AND 3600),
    probes_left integer NOT NULL DEFAULT 0 CHECK (probes_left>=0),
    probes_passed integer NOT NULL DEFAULT 0 CHECK (probes_passed>=0),
    -- Set while the channel's alert is open; it closes only once most calls
    -- of the last minute went through, so a flapping breaker cannot flap the bell.
    failing_since timestamptz,
    -- Demo-only: the mock providers act out a fault until this time.
    simulate text CHECK (simulate IN ('throttle','outage','credentials')),
    simulate_until timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id,provider),
    CHECK ((simulate IS NULL)=(simulate_until IS NULL))
);

-- Attempts carry their provider so the rate window and the breaker read one
-- index; a probe is marked so its failure costs the message nothing.
ALTER TABLE delivery_attempts
    ADD COLUMN provider text,
    ADD COLUMN gate text CHECK (gate IN ('probe','credential_probe','both')),
    -- What the answer said about the channel rather than the message.
    ADD COLUMN signal text CHECK (signal IN ('throttled','credentials'));
UPDATE delivery_attempts a SET provider=d.provider FROM delivery_jobs d WHERE d.id=a.delivery_id;
CREATE INDEX delivery_attempts_provider_recent ON delivery_attempts(workspace_id,provider,started_at DESC) WHERE provider IS NOT NULL;

-- Calls that said something about the channel, not the message (a 429, a 401,
-- a failed probe), do not use up the message's attempts.
ALTER TABLE delivery_jobs ADD COLUMN free_attempts integer NOT NULL DEFAULT 0 CHECK (free_attempts>=0);

-- The bell. Rows are written by the worker for channel alerts and by a trigger
-- for what teammates do to journeys; each user keeps how far they have read.
CREATE TABLE notifications (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id),
    kind text NOT NULL CHECK (char_length(kind) BETWEEN 1 AND 80),
    severity text NOT NULL CHECK (severity IN ('info','success','warning','critical')),
    title text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 300),
    body text NOT NULL DEFAULT '' CHECK (char_length(body)<=1000),
    journey_id uuid,
    actor_id uuid,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notifications_recent ON notifications(workspace_id,created_at DESC,id DESC);
ALTER TABLE users ADD COLUMN notifications_seen_at timestamptz NOT NULL DEFAULT now();

CREATE FUNCTION notify_journey_audit() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE journey text; actor text; verb text; title text; body text := ''; severity text := 'info';
BEGIN
 IF NEW.action NOT IN ('journey.publish','journey.pause','journey.resume','journey.sunset','journey.complete','journey.archive','journey.audience_run') THEN RETURN NEW; END IF;
 SELECT name INTO journey FROM journeys WHERE id=NEW.resource_id AND workspace_id=NEW.workspace_id;
 IF journey IS NULL THEN RETURN NEW; END IF;
 SELECT name INTO actor FROM users WHERE id=NEW.actor_id AND workspace_id=NEW.workspace_id;
 actor := COALESCE(actor,'Someone');
 IF NEW.action='journey.publish' THEN
  title := actor||' published '||journey;
  body := 'Version '||COALESCE(NEW.details->>'version','?')||' is live.';
  severity := 'success';
 ELSIF NEW.action='journey.audience_run' THEN
  title := journey||' enrolled '||COALESCE(NEW.details->>'enrolled','0')||' customers';
  body := COALESCE(NEW.details->>'matched','0')||' matched the audience; '||actor||' ran it.';
 ELSE
  verb := CASE NEW.action WHEN 'journey.pause' THEN 'paused' WHEN 'journey.resume' THEN 'resumed' WHEN 'journey.sunset' THEN 'stopped new entries to' WHEN 'journey.complete' THEN 'completed' ELSE 'archived' END;
  title := actor||' '||verb||' '||journey;
  IF NEW.action='journey.pause' THEN severity := 'warning'; END IF;
 END IF;
 INSERT INTO notifications(workspace_id,kind,severity,title,body,journey_id,actor_id,created_at)
 VALUES(NEW.workspace_id,NEW.action,severity,left(title,300),left(body,1000),NEW.resource_id,NEW.actor_id,NEW.created_at);
 RETURN NEW;
END $$;
CREATE TRIGGER audit_logs_notify AFTER INSERT ON audit_logs FOR EACH ROW EXECUTE FUNCTION notify_journey_audit();
