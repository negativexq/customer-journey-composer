-- Acceptance state belongs to sandbox tests, not production execution tables.
ALTER TABLE test_scenarios ADD COLUMN required boolean NOT NULL DEFAULT true;
ALTER TABLE test_runs ADD COLUMN acceptance boolean NOT NULL DEFAULT true;
ALTER TABLE test_runs ADD COLUMN acceptance_key text;
UPDATE test_runs SET acceptance_key=COALESCE('saved:'||scenario_id::text,'inline:'||encode(sha256(convert_to(scenario::text,'UTF8')),'hex'));
ALTER TABLE test_runs ALTER COLUMN acceptance_key SET NOT NULL;
CREATE FUNCTION set_test_acceptance_key() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 NEW.acceptance_key:=COALESCE('saved:'||NEW.scenario_id::text,'inline:'||encode(sha256(convert_to(NEW.scenario::text,'UTF8')),'hex'));
 RETURN NEW;
END $$;
CREATE TRIGGER test_acceptance_key BEFORE INSERT OR UPDATE OF scenario,scenario_id ON test_runs FOR EACH ROW EXECUTE FUNCTION set_test_acceptance_key();
CREATE INDEX test_runs_acceptance ON test_runs(workspace_id,journey_id,workflow_hash,acceptance_key,created_at DESC,id DESC) INCLUDE(scenario_id,result,finalized) WHERE mode='FULL' AND acceptance;
CREATE INDEX scenarios_required ON test_scenarios(workspace_id,journey_id,id) WHERE required;
