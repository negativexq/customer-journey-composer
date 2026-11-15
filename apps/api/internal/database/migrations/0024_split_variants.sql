-- Split steps leave through ports "a" and "b". Edge traffic is now counted for
-- every output a step reports instead of a fixed list, so new step types do not
-- need another projection change.
CREATE OR REPLACE FUNCTION metric_step_snapshot(x step_executions,journey uuid,version uuid,entry_at timestamptz) RETURNS jsonb LANGUAGE plpgsql STABLE AS $$
DECLARE values jsonb;
BEGIN
 values:=jsonb_build_array(metric_delta('node',x.node_id,'entered',1),metric_delta('node',x.node_id,x.state,1));
 IF x.completed_at IS NOT NULL THEN values:=values||metric_delta('node',x.node_id,'finished_count',1)||metric_delta('node',x.node_id,'duration_ms',greatest(0,extract(epoch from x.completed_at-x.started_at)*1000)::double precision); END IF;
 IF x.state='SUCCEEDED' AND COALESCE(x.output->>'port','')<>'' THEN values:=values||metric_delta('edge',x.node_id||':'||(x.output->>'port'),'traversed',1); END IF;
 RETURN metric_snapshot('step',x.id,x.workspace_id,journey,version,entry_at,NULL,values);
END $$;
CREATE INDEX steps_workspace_type_node ON step_executions(workspace_id,node_type,node_id) WHERE node_type='split';
