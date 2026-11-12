package httpapi

import (
	"context"
	"fmt"
	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
	"time"
)

type insightScope struct {
	ID      string `path:"id" format:"uuid"`
	Version int    `query:"version" default:"0" minimum:"0"`
	Days    int    `query:"days" default:"30" minimum:"0" maximum:"90"`
}

func registerInsights(api huma.API, pool *pgxpool.Pool) {
	huma.Get(api, "/api/v1/journeys/{id}/insights", func(ctx context.Context, in *insightScope) (*Result[map[string]any], error) {
		var status, version string
		if err := pool.QueryRow(ctx, `SELECT j.status,COALESCE((SELECT id::text FROM journey_versions v WHERE v.workspace_id=j.workspace_id AND v.journey_id=j.id AND v.version=$3),'') FROM journeys j WHERE j.workspace_id=$1 AND j.id=$2`, who(ctx).WorkspaceID, in.ID, in.Version).Scan(&status, &version); err != nil {
			return nil, dbError(err)
		}
		if in.Version > 0 && version == "" {
			return nil, huma.Error404NotFound("Published version not found")
		}
		until := time.Now().UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
		from := time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
		if in.Days > 0 {
			from = until.AddDate(0, 0, -in.Days)
		}
		// Aggregate facts are already entry-cohort scoped, including message outcomes.
		return jsonRow[map[string]any](pool.QueryRow(ctx, `WITH scoped AS MATERIALIZED (
   SELECT * FROM journey_cohort_metrics WHERE workspace_id=$1 AND journey_id=$2 AND cohort_day>=$3::date AND cohort_day<$4::date AND ($5='' OR version_id=NULLIF($5,'')::uuid)
  ) SELECT jsonb_build_object(
   'scope',jsonb_build_object('version',$6::int,'days',$7::int,'from',CASE WHEN $7::int=0 THEN NULL ELSE $3::timestamptz END,'until',$4::timestamptz,'basis','entry_cohort','timezone','UTC'),
   'metrics',COALESCE((SELECT jsonb_agg(to_jsonb(m)) FROM (SELECT dimension,dimension_key AS key,metric,sum(value) AS value FROM scoped GROUP BY 1,2,3)m),'[]'::jsonb),
   'daily',COALESCE((SELECT jsonb_agg(to_jsonb(d) ORDER BY d.day) FROM (SELECT cohort_day AS day,metric,sum(value) AS value FROM scoped WHERE dimension='execution' AND metric IN ('entries','converted') GROUP BY 1,2)d),'[]'::jsonb),
   'unique_customers',(SELECT count(DISTINCT customer_id) FROM journey_cohort_members WHERE workspace_id=$1 AND journey_id=$2 AND cohort_day>=$3::date AND cohort_day<$4::date AND ($5='' OR version_id=NULLIF($5,'')::uuid)),
   'coverage_complete',NOT EXISTS(SELECT 1 FROM metric_snapshot_backfill WHERE NOT complete),
   'projector_last_success',(SELECT last_success FROM metric_projection_health WHERE id),
   'updated_at',(SELECT processed_at FROM metric_watermarks WHERE workspace_id=$1),
   'source_at',(SELECT source_at FROM metric_watermarks WHERE workspace_id=$1),
   'queue_oldest_at',(SELECT created_at FROM metric_queue ORDER BY id LIMIT 1),
   'journey_status',$8::text,
   'overdue',(SELECT count(*) FROM journey_executions e WHERE workspace_id=$1 AND journey_id=$2 AND state='WAITING' AND started_at>=$3 AND started_at<$4 AND ($5='' OR version_id=NULLIF($5,'')::uuid) AND $8::text IN ('Active','Sunset') AND EXISTS(SELECT 1 FROM scheduled_jobs j WHERE j.workspace_id=e.workspace_id AND j.execution_id=e.id AND j.state='SCHEDULED' AND j.due_at<clock_timestamp()-interval '30 seconds'))
  )`, who(ctx).WorkspaceID, in.ID, from, until, version, in.Version, in.Days, status))
	})
	huma.Get(api, "/api/v1/journeys/{id}/messages", func(ctx context.Context, in *struct {
		ID       string    `path:"id" format:"uuid"`
		Version  int       `query:"version" minimum:"0"`
		Node     string    `query:"node_id" maxLength:"80"`
		Channel  string    `query:"channel" enum:"email,sms"`
		Status   string    `query:"status" enum:"SCHEDULED,CLAIMED,ACCEPTED,BLOCKED,SKIPPED,FAILED,CANCELLED,UNKNOWN"`
		Reason   string    `query:"reason" maxLength:"160"`
		From     time.Time `query:"from"`
		Until    time.Time `query:"until"`
		Before   time.Time `query:"before"`
		BeforeID string    `query:"before_id"`
		Limit    int       `query:"limit" default:"25" minimum:"1" maximum:"100"`
	}) (*Result[[]map[string]any], error) {
		args := []any{who(ctx).WorkspaceID, in.ID}
		query := `SELECT d.id,d.execution_id,e.customer_id,e.version_id,d.node_id,d.channel,d.status,d.decision,d.attempts,d.created_at,d.last_error,e.started_at FROM journey_executions e JOIN delivery_jobs d ON d.workspace_id=e.workspace_id AND d.execution_id=e.id WHERE e.workspace_id=$1 AND e.journey_id=$2 AND d.mode='LIVE'`
		add := func(condition string, value any) {
			args = append(args, value)
			query += fmt.Sprintf(condition, len(args))
		}
		if in.Version > 0 {
			add(` AND e.version_id=(SELECT id FROM journey_versions WHERE workspace_id=$1 AND journey_id=$2 AND version=$%d)`, in.Version)
		}
		if in.Node != "" {
			add(` AND d.node_id=$%d`, in.Node)
		}
		if in.Channel != "" {
			add(` AND d.channel=$%d`, in.Channel)
		}
		if in.Status != "" {
			add(` AND d.status=$%d`, in.Status)
		}
		if in.Reason != "" {
			add(` AND CASE WHEN d.status='BLOCKED' THEN COALESCE(NULLIF(d.decision->>'reason',''),'blocked') WHEN d.status IN ('SKIPPED','CANCELLED') THEN COALESCE(NULLIF(d.decision->>'reason',''),lower(d.status)) ELSE lower(d.status) END=$%d`, in.Reason)
		}
		if !in.From.IsZero() {
			add(` AND e.started_at>=$%d`, in.From)
		}
		if !in.Until.IsZero() {
			add(` AND e.started_at<$%d`, in.Until)
		}
		if !in.Before.IsZero() {
			if in.BeforeID == "" {
				return nil, huma.Error422UnprocessableEntity("before_id is required")
			}
			args = append(args, in.Before, in.BeforeID)
			query += fmt.Sprintf(` AND (d.created_at,d.id)<($%d,$%d::uuid)`, len(args)-1, len(args))
		}
		args = append(args, in.Limit)
		query += fmt.Sprintf(` ORDER BY d.created_at DESC,d.id DESC LIMIT $%d`, len(args))
		return jsonRow[[]map[string]any](pool.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(m)),'[]'::jsonb) FROM (`+query+`)m`, args...))
	})
}

// Shared indexed scope for contributing execution records.
func executionScope(query string, args []any, version int, node, port, exit string, current, overdue bool, from, until time.Time) (string, []any) {
	add := func(condition string, value any) {
		args = append(args, value)
		query += fmt.Sprintf(condition, len(args))
	}
	if version > 0 {
		add(` AND version_id=(SELECT id FROM journey_versions WHERE workspace_id=$1 AND journey_id=$2 AND version=$%d)`, version)
	}
	if !from.IsZero() {
		add(` AND started_at>=$%d`, from)
	}
	if !until.IsZero() {
		add(` AND started_at<$%d`, until)
	}
	if exit != "" {
		add(` AND exit_reason=$%d`, exit)
	}
	if node != "" {
		if current {
			add(` AND current_node_id=$%d`, node)
		} else {
			args = append(args, node)
			cond := fmt.Sprintf(` AND EXISTS(SELECT 1 FROM step_executions s WHERE s.workspace_id=journey_executions.workspace_id AND s.execution_id=journey_executions.id AND s.node_id=$%d`, len(args))
			if port != "" {
				args = append(args, port)
				cond += fmt.Sprintf(` AND s.state='SUCCEEDED' AND s.output->>'port'=$%d`, len(args))
			}
			query += cond + `)`
		}
	}
	if overdue {
		query += ` AND state='WAITING' AND EXISTS(SELECT 1 FROM journeys j WHERE j.workspace_id=journey_executions.workspace_id AND j.id=journey_executions.journey_id AND j.status IN ('Active','Sunset')) AND EXISTS(SELECT 1 FROM scheduled_jobs j WHERE j.workspace_id=journey_executions.workspace_id AND j.execution_id=journey_executions.id AND j.state='SCHEDULED' AND j.due_at<clock_timestamp()-interval '30 seconds')`
	}
	return strings.TrimSpace(query), args
}
