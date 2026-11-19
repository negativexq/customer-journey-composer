package analytics

import (
	"context"
	"github.com/jackc/pgx/v5"
)

// Baselines are lower priority than every real transition snapshot, regardless
// of enqueue time. Cursor backfill therefore needs no locks on runtime rows.
// more reports whether a source kind still has rows to backfill.
func enqueueBackfill(ctx context.Context, tx pgx.Tx) (more bool, err error) {
	var kind, cursor string
	err = tx.QueryRow(ctx, `SELECT source_kind,after_id::text FROM metric_snapshot_backfill WHERE NOT complete ORDER BY CASE source_kind WHEN 'execution' THEN 0 WHEN 'step' THEN 1 ELSE 2 END LIMIT 1`).Scan(&kind, &cursor)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var query string
	switch kind {
	case "execution":
		query = `SELECT x.id,metric_execution_snapshot(x) AS snapshot FROM journey_executions x WHERE x.id>$1::uuid ORDER BY x.id LIMIT 200`
	case "step":
		query = `SELECT x.id,metric_step_snapshot(x,e.journey_id,e.version_id,e.started_at) AS snapshot FROM step_executions x JOIN journey_executions e ON e.id=x.execution_id WHERE x.id>$1::uuid ORDER BY x.id LIMIT 200`
	case "delivery":
		query = `SELECT x.id,metric_delivery_snapshot(x,e.journey_id,e.version_id,e.started_at) AS snapshot FROM delivery_jobs x JOIN journey_executions e ON e.id=x.execution_id WHERE x.id>$1::uuid ORDER BY x.id LIMIT 200`
	}
	var last string
	var count int
	err = tx.QueryRow(ctx, `WITH batch AS MATERIALIZED (`+query+`),queued AS (INSERT INTO metric_queue(workspace_id,journey_id,version_id,cohort_at,deltas,snapshot,baseline) SELECT (snapshot->>'workspace_id')::uuid,(snapshot->>'journey_id')::uuid,(snapshot->>'version_id')::uuid,(snapshot->>'cohort_day')::date AT TIME ZONE 'UTC','[]',snapshot,true FROM batch WHERE snapshot IS NOT NULL RETURNING 1) SELECT COALESCE((SELECT id::text FROM batch ORDER BY id DESC LIMIT 1),$1::text),(SELECT count(*) FROM batch)`, cursor).Scan(&last, &count)
	if err != nil {
		return false, err
	}
	_, err = tx.Exec(ctx, `UPDATE metric_snapshot_backfill SET after_id=$2,complete=$3 WHERE source_kind=$1`, kind, last, count < 200)
	return err == nil, err
}

// Pick only the newest snapshot per source in a batch. Subtract the previously
// projected snapshot, then add the new one in the same queue-drain transaction.
const projectSnapshots = `WITH candidates AS MATERIALIZED (
 SELECT DISTINCT ON (snapshot->>'workspace_id',snapshot->>'kind',snapshot->>'id') snapshot,id,baseline
 FROM jsonb_to_recordset($1::jsonb) AS b(id bigint,snapshot jsonb,baseline boolean)
 WHERE snapshot IS NOT NULL ORDER BY snapshot->>'workspace_id',snapshot->>'kind',snapshot->>'id',baseline,id DESC
), eligible AS MATERIALIZED (
 SELECT c.*,f.snapshot AS previous FROM candidates c LEFT JOIN metric_snapshot_facts f
 ON f.workspace_id=(c.snapshot->>'workspace_id')::uuid AND f.source_kind=c.snapshot->>'kind' AND f.source_id=(c.snapshot->>'id')::uuid
 WHERE f.source_id IS NULL OR (NOT c.baseline AND (f.baseline OR c.id>f.sequence))
), changes AS (
 SELECT snapshot,1 AS sign FROM eligible UNION ALL SELECT previous,-1 FROM eligible WHERE previous IS NOT NULL
), expanded AS (
 SELECT (snapshot->>'workspace_id')::uuid AS workspace_id,(snapshot->>'journey_id')::uuid AS journey_id,(snapshot->>'cohort_day')::date AS cohort_day,(snapshot->>'version_id')::uuid AS version_id,v->>'dimension' AS dimension,v->>'key' AS dimension_key,v->>'metric' AS metric,(v->>'value')::double precision*sign AS value
 FROM changes CROSS JOIN LATERAL jsonb_array_elements(snapshot->'values') v
), totals AS (
 INSERT INTO journey_cohort_metrics SELECT workspace_id,journey_id,cohort_day,version_id,dimension,dimension_key,metric,sum(value) FROM expanded GROUP BY 1,2,3,4,5,6,7 HAVING sum(value)<>0
 ON CONFLICT(workspace_id,journey_id,cohort_day,version_id,dimension,dimension_key,metric) DO UPDATE SET value=journey_cohort_metrics.value+EXCLUDED.value RETURNING 1
), members AS (
 INSERT INTO journey_cohort_members SELECT DISTINCT (snapshot->>'workspace_id')::uuid,(snapshot->>'journey_id')::uuid,(snapshot->>'cohort_day')::date,(snapshot->>'version_id')::uuid,snapshot->>'customer_id' FROM eligible WHERE snapshot->>'kind'='execution' AND snapshot->>'customer_id' IS NOT NULL ON CONFLICT DO NOTHING RETURNING 1
)
INSERT INTO metric_snapshot_facts(workspace_id,source_kind,source_id,sequence,baseline,snapshot)
SELECT (snapshot->>'workspace_id')::uuid,snapshot->>'kind',(snapshot->>'id')::uuid,id,baseline,snapshot FROM eligible
ON CONFLICT(workspace_id,source_kind,source_id) DO UPDATE SET sequence=EXCLUDED.sequence,baseline=EXCLUDED.baseline,snapshot=EXCLUDED.snapshot`
