package httpapi

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/analytics"
	"time"
)

func metricSummary(ctx context.Context, pool *pgxpool.Pool, journey string) (*Result[map[string]any], error) {
	return jsonRow[map[string]any](pool.QueryRow(ctx, `SELECT jsonb_build_object('metrics',COALESCE((SELECT jsonb_agg(jsonb_build_object('dimension',dimension,'key',dimension_key,'metric',metric,'value',value)) FROM metric_totals WHERE workspace_id=$1 AND journey_id=$2 AND version_id=$3 AND dimension<>'node'),'[]'::jsonb),'updated_at',(SELECT processed_at FROM metric_watermarks WHERE workspace_id=$1),'source_at',(SELECT source_at FROM metric_watermarks WHERE workspace_id=$1),'semantics','Eventually consistent. Conversion denominator: lifetime logical entries. Provider accepted is not confirmed delivery. Unique customers are lifetime distinct per scope; retries are not logical sends.')`, who(ctx).WorkspaceID, journey, analytics.ZeroID))
}
func registerAnalytics(api huma.API, pool *pgxpool.Pool) {
	huma.Get(api, "/api/v1/journeys/{id}/node-stats", func(ctx context.Context, in *struct {
		ID      string `path:"id" format:"uuid"`
		Version int    `query:"version" minimum:"1"`
	}) (*Result[[]map[string]any], error) {
		return jsonRow[[]map[string]any](pool.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(jsonb_build_object('node_id',dimension_key,'dimension',dimension,'metric',metric,'value',value)),'[]'::jsonb) FROM metric_totals WHERE workspace_id=$1 AND journey_id=$2 AND version_id=(SELECT id FROM journey_versions WHERE workspace_id=$1 AND journey_id=$2 AND version=$3) AND dimension IN ('node','execution')`, who(ctx).WorkspaceID, in.ID, in.Version))
	})
	huma.Get(api, "/api/v1/analytics/comparison", func(ctx context.Context, _ *struct{}) (*Result[[]map[string]any], error) {
		return jsonRow[[]map[string]any](pool.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(r)),'[]'::jsonb) FROM (SELECT j.id,j.name,j.status,COALESCE(m.entries,0) AS entries,COALESCE(m.converted,0) AS converted,COALESCE(m.active,0) AS active FROM (SELECT id,name,status FROM journeys WHERE workspace_id=$1 ORDER BY updated_at DESC,id LIMIT 100)j LEFT JOIN LATERAL (SELECT sum(value) FILTER(WHERE metric='entries') AS entries,sum(value) FILTER(WHERE metric='converted') AS converted,sum(value) FILTER(WHERE metric IN ('RUNNING','WAITING')) AS active FROM metric_totals WHERE workspace_id=$1 AND journey_id=j.id AND version_id=$2 AND dimension='execution')m ON true)r`, who(ctx).WorkspaceID, analytics.ZeroID))
	})

	huma.Get(api, "/api/v1/overview/entries-24h", func(ctx context.Context, _ *struct{}) (*Result[map[string]any], error) {
		now := time.Now().UTC()
		from := now.Add(-24 * time.Hour)
		first := from.Truncate(time.Hour).Add(time.Hour)
		last := now.Truncate(time.Hour)
		return jsonRow[map[string]any](pool.QueryRow(ctx, `SELECT jsonb_build_object('entries',COALESCE((SELECT sum(value) FROM metric_hourly WHERE workspace_id=$1 AND journey_id=$2 AND version_id=$2 AND bucket>=$3 AND bucket<$4 AND dimension='execution' AND metric='entries'),0)+(SELECT count(*) FROM journey_executions WHERE workspace_id=$1 AND started_at>=$5 AND started_at<$3)+(SELECT count(*) FROM journey_executions WHERE workspace_id=$1 AND started_at>=$4 AND started_at<=$6),'from',$5::timestamptz,'until',$6::timestamptz)`, who(ctx).WorkspaceID, analytics.ZeroID, first, last, from, now))
	})
	huma.Get(api, "/api/v1/overview", func(ctx context.Context, _ *struct{}) (*Result[map[string]any], error) {
		return metricSummary(ctx, pool, analytics.ZeroID)
	})
	huma.Get(api, "/api/v1/journeys/{id}/stats", func(ctx context.Context, in *journeyID) (*Result[map[string]any], error) {
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT true FROM journeys WHERE workspace_id=$1 AND id=$2`, who(ctx).WorkspaceID, in.ID).Scan(&exists); err != nil {
			return nil, dbError(err)
		}
		return metricSummary(ctx, pool, in.ID)
	})
	type trendInput struct {
		ID   string `path:"id" format:"uuid"`
		Days int    `query:"days" default:"30" minimum:"1" maximum:"90"`
	}
	huma.Get(api, "/api/v1/journeys/{id}/analytics", func(ctx context.Context, in *trendInput) (*Result[map[string]any], error) {
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT true FROM journeys WHERE workspace_id=$1 AND id=$2`, who(ctx).WorkspaceID, in.ID).Scan(&exists); err != nil {
			return nil, dbError(err)
		}
		return analyticsTrend(ctx, pool, in.ID, in.Days)
	})
	huma.Get(api, "/api/v1/analytics", func(ctx context.Context, in *struct {
		Days int `query:"days" default:"30" minimum:"1" maximum:"90"`
	}) (*Result[map[string]any], error) {
		return analyticsTrend(ctx, pool, analytics.ZeroID, in.Days)
	})
	huma.Get(api, "/api/v1/overview/activity", func(ctx context.Context, _ *struct{}) (*Result[map[string]any], error) {
		// Bounded current/error lists use metadata indexes. Counts come from projections.
		return jsonRow[map[string]any](pool.QueryRow(ctx, `SELECT jsonb_build_object('recent_executions',COALESCE((SELECT jsonb_agg(to_jsonb(e)) FROM (SELECT id,journey_id,customer_id,state,current_node_id,started_at FROM journey_executions WHERE workspace_id=$1 ORDER BY started_at DESC,id DESC LIMIT 10)e),'[]'::jsonb),'failed_executions',COALESCE((SELECT jsonb_agg(to_jsonb(e)) FROM (SELECT id,journey_id,customer_id,state,current_node_id,updated_at FROM journey_executions WHERE workspace_id=$1 AND state='FAILED' ORDER BY updated_at DESC,id DESC LIMIT 10)e),'[]'::jsonb))`, who(ctx).WorkspaceID))
	})
}
func analyticsTrend(ctx context.Context, pool *pgxpool.Pool, journey string, days int) (*Result[map[string]any], error) {
	now := time.Now().UTC()
	end := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	start := end.AddDate(0, 0, -days)
	return jsonRow[map[string]any](pool.QueryRow(ctx, `SELECT jsonb_build_object('from',$4::timestamptz,'until',$5::timestamptz,'timezone','UTC','semantics','Execution trends are entry-date cohorts and reflect their current outcome, including later conversions. Channel trends use logical job creation date. Unique customers are lifetime only; daily distinct counts are not added.','daily',COALESCE((SELECT jsonb_agg(to_jsonb(r) ORDER BY r.day,r.dimension,r.key,r.metric) FROM (SELECT (bucket AT TIME ZONE 'UTC')::date AS day,dimension,dimension_key AS key,metric,sum(value) AS value FROM metric_hourly WHERE workspace_id=$1 AND journey_id=$2 AND version_id=$3 AND bucket>=$4 AND bucket<$5 AND dimension IN ('execution','channel') GROUP BY 1,2,3,4)r),'[]'::jsonb),'updated_at',(SELECT processed_at FROM metric_watermarks WHERE workspace_id=$1))`, who(ctx).WorkspaceID, journey, analytics.ZeroID, start, end))
}
