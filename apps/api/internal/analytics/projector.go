// Package analytics projects transactional metric deltas asynchronously. Read
// requests never rebuild aggregates or scan execution/message JSON history.
package analytics

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"time"
)

const ZeroID = "00000000-0000-0000-0000-000000000000"
const batchRows = `jsonb_to_recordset($1::jsonb) AS b(workspace_id uuid,journey_id uuid,version_id uuid,cohort_at timestamptz,customer_id text,deltas jsonb,created_at timestamptz)`
const expanded = `SELECT b.workspace_id,b.journey_id,b.version_id,date_trunc('hour',b.cohort_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC' AS bucket,d->>'dimension' AS dimension,d->>'key' AS dimension_key,d->>'metric' AS metric,(d->>'value')::double precision AS value FROM ` + batchRows + ` CROSS JOIN LATERAL jsonb_array_elements(b.deltas) d`

// Drain is atomic: queue deletion and summaries commit together. A crash rolls
// back both. Projector replicas serialize briefly, avoiding counter lock inversions;
// producers never acquire this advisory lock. Bounded JSON batches avoid repeated
// temporary-table DDL/catalog churn and require no persistent staging indexes.
func Drain(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var locked bool
	if err = tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(716294803)`).Scan(&locked); err != nil || !locked {
		return 0, err
	}
	var batch []byte
	err = tx.QueryRow(ctx, `WITH claimed AS (SELECT id FROM metric_queue ORDER BY id LIMIT 1000 FOR UPDATE SKIP LOCKED),removed AS (DELETE FROM metric_queue q USING claimed c WHERE q.id=c.id RETURNING q.*) SELECT COALESCE(jsonb_agg(to_jsonb(r)),'[]'::jsonb) FROM removed r`).Scan(&batch)
	if err != nil {
		return 0, err
	}
	var rows []json.RawMessage
	if err = json.Unmarshal(batch, &rows); err != nil {
		return 0, err
	}
	n := len(rows)
	if n == 0 {
		return 0, tx.Commit(ctx)
	}
	_, err = tx.Exec(ctx, `WITH candidates AS (SELECT workspace_id,journey_id,customer_id FROM `+batchRows+` WHERE customer_id IS NOT NULL UNION SELECT workspace_id,'00000000-0000-0000-0000-000000000000'::uuid,customer_id FROM `+batchRows+` WHERE customer_id IS NOT NULL), added AS (INSERT INTO metric_customers SELECT * FROM candidates ON CONFLICT DO NOTHING RETURNING workspace_id,journey_id) INSERT INTO metric_totals(workspace_id,journey_id,version_id,dimension,dimension_key,metric,value) SELECT workspace_id,journey_id,'00000000-0000-0000-0000-000000000000','execution','','unique_customers',count(*) FROM added GROUP BY workspace_id,journey_id ON CONFLICT(workspace_id,journey_id,version_id,dimension,dimension_key,metric) DO UPDATE SET value=metric_totals.value+excluded.value`, batch)
	if err != nil {
		return 0, err
	}
	_, err = tx.Exec(ctx, `WITH expanded AS (`+expanded+`), totals AS (INSERT INTO metric_totals SELECT workspace_id,COALESCE(journey_id,'00000000-0000-0000-0000-000000000000'),COALESCE(version_id,'00000000-0000-0000-0000-000000000000'),dimension,dimension_key,metric,sum(value) FROM expanded WHERE dimension<>'journey' GROUP BY GROUPING SETS((workspace_id,journey_id,version_id,dimension,dimension_key,metric),(workspace_id,journey_id,dimension,dimension_key,metric),(workspace_id,dimension,dimension_key,metric)) HAVING sum(value)<>0 AND (dimension<>'node' OR grouping(version_id)=0) ON CONFLICT(workspace_id,journey_id,version_id,dimension,dimension_key,metric) DO UPDATE SET value=metric_totals.value+excluded.value RETURNING 1), journeys AS (INSERT INTO metric_totals SELECT workspace_id,journey_id,version_id,dimension,dimension_key,metric,sum(value) FROM expanded WHERE dimension='journey' GROUP BY workspace_id,journey_id,version_id,dimension,dimension_key,metric HAVING sum(value)<>0 ON CONFLICT(workspace_id,journey_id,version_id,dimension,dimension_key,metric) DO UPDATE SET value=metric_totals.value+excluded.value RETURNING 1) INSERT INTO metric_hourly SELECT workspace_id,COALESCE(journey_id,'00000000-0000-0000-0000-000000000000'),COALESCE(version_id,'00000000-0000-0000-0000-000000000000'),bucket,dimension,dimension_key,metric,sum(value) FROM expanded WHERE dimension IN ('execution','channel') GROUP BY GROUPING SETS((workspace_id,journey_id,version_id,bucket,dimension,dimension_key,metric),(workspace_id,journey_id,bucket,dimension,dimension_key,metric),(workspace_id,bucket,dimension,dimension_key,metric)) HAVING sum(value)<>0 ON CONFLICT(workspace_id,journey_id,bucket,version_id,dimension,dimension_key,metric) DO UPDATE SET value=metric_hourly.value+excluded.value`, batch)
	if err != nil {
		return 0, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO metric_watermarks SELECT workspace_id,clock_timestamp(),max(created_at) FROM `+batchRows+` GROUP BY workspace_id ON CONFLICT(workspace_id) DO UPDATE SET processed_at=excluded.processed_at,source_at=greatest(metric_watermarks.source_at,excluded.source_at)`, batch)
	if err != nil {
		return 0, err
	}
	return n, tx.Commit(ctx)
}
func Run(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) {
	for ctx.Err() == nil {
		n, err := Drain(ctx, pool)
		if err != nil && ctx.Err() == nil {
			log.Error("analytics projection failed")
		}
		if err == nil && n == 1000 {
			continue
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
