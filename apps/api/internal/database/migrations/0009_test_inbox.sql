ALTER TABLE test_messages DROP CONSTRAINT test_messages_status_check;
ALTER TABLE test_messages ADD CONSTRAINT test_messages_status_check CHECK (status IN ('SIMULATED','BLOCKED','SKIPPED','FAILED','TEST_ACCEPTED','TEST_PENDING'));
CREATE INDEX test_inbox_run_recent ON test_messages(workspace_id,test_run_id,created_at DESC,id DESC);
CREATE INDEX test_inbox_channel_recent ON test_messages(workspace_id,channel,created_at DESC,id);
CREATE INDEX test_inbox_status_recent ON test_messages(workspace_id,status,created_at DESC,id);
-- Only explicit TEST delivery jobs enter this inbox. No live delivery is copied.
CREATE FUNCTION mirror_test_delivery() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE inbox_status text;
BEGIN
 IF NEW.mode <> 'TEST' THEN RETURN NEW; END IF;
 inbox_status := CASE NEW.status WHEN 'ACCEPTED' THEN 'TEST_ACCEPTED' WHEN 'BLOCKED' THEN 'BLOCKED' WHEN 'CANCELLED' THEN 'SKIPPED' WHEN 'FAILED' THEN 'FAILED' WHEN 'UNKNOWN' THEN 'FAILED' ELSE 'TEST_PENDING' END;
 IF TG_OP='INSERT' THEN
  INSERT INTO test_messages(workspace_id,delivery_id,customer_id,source_node,node_label,channel,mode,status,simulated_at,scheduled_at,message,decision,provider_response)
  VALUES(NEW.workspace_id,NEW.id,NEW.customer_id,NEW.node_id,'Explicit test send',NEW.channel,'REAL_TEST',inbox_status,COALESCE(NEW.accepted_at,NEW.created_at),NEW.created_at,NEW.message,NEW.decision,NEW.last_error)
  ON CONFLICT(delivery_id) DO NOTHING;
 ELSE
  UPDATE test_messages SET status=inbox_status,simulated_at=COALESCE(NEW.accepted_at,NEW.created_at),decision=NEW.decision,provider_response=NEW.last_error WHERE delivery_id=NEW.id;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER test_delivery_inbox AFTER INSERT OR UPDATE OF status ON delivery_jobs FOR EACH ROW EXECUTE FUNCTION mirror_test_delivery();
INSERT INTO test_messages(workspace_id,delivery_id,customer_id,source_node,node_label,channel,mode,status,simulated_at,scheduled_at,message,decision,provider_response)
SELECT workspace_id,id,customer_id,node_id,'Explicit test send',channel,'REAL_TEST',CASE status WHEN 'ACCEPTED' THEN 'TEST_ACCEPTED' WHEN 'BLOCKED' THEN 'BLOCKED' WHEN 'CANCELLED' THEN 'SKIPPED' WHEN 'FAILED' THEN 'FAILED' WHEN 'UNKNOWN' THEN 'FAILED' ELSE 'TEST_PENDING' END,COALESCE(accepted_at,created_at),created_at,message,decision,last_error FROM delivery_jobs WHERE mode='TEST' ON CONFLICT(delivery_id) DO NOTHING;
