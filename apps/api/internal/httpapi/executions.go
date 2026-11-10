package httpapi

import (
	"context"
	"fmt"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func registerExecutions(api huma.API, pool *pgxpool.Pool) {
	huma.Get(api, "/api/v1/customers/{id}/executions", func(ctx context.Context, in *struct {
		ID    string `path:"id" maxLength:"128"`
		Limit int    `query:"limit" default:"25" minimum:"1" maximum:"100"`
	}) (*Result[[]map[string]any], error) {
		return jsonRow[[]map[string]any](pool.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(e)),'[]'::jsonb) FROM (SELECT id,journey_id,version_id,customer_id,entry_event_id,state,exit_reason,started_at FROM journey_executions WHERE workspace_id=$1 AND customer_id=$2 ORDER BY started_at DESC,id LIMIT $3)e`, who(ctx).WorkspaceID, in.ID, in.Limit))
	})
	huma.Get(api, "/api/v1/events/{id}/executions", func(ctx context.Context, in *struct {
		ID    string `path:"id" maxLength:"128"`
		Limit int    `query:"limit" default:"25" minimum:"1" maximum:"100"`
	}) (*Result[[]map[string]any], error) {
		return jsonRow[[]map[string]any](pool.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(e)),'[]'::jsonb) FROM (SELECT id,journey_id,version_id,customer_id,entry_event_id,state,exit_reason,started_at FROM journey_executions WHERE workspace_id=$1 AND entry_event_id=$2 ORDER BY started_at DESC,id LIMIT $3)e`, who(ctx).WorkspaceID, in.ID, in.Limit))
	})
	huma.Get(api, "/api/v1/executions/{id}", func(ctx context.Context, in *journeyID) (*Result[map[string]any], error) {
		return jsonRow[map[string]any](pool.QueryRow(ctx, `SELECT (to_jsonb(e)-'workspace_id')||jsonb_build_object(
 'steps',COALESCE((SELECT jsonb_agg(to_jsonb(s)-'workspace_id') FROM (SELECT * FROM step_executions WHERE workspace_id=$1 AND execution_id=e.id ORDER BY started_at,id LIMIT 100) s),'[]'::jsonb),
 'jobs',COALESCE((SELECT jsonb_agg(to_jsonb(j)-'workspace_id'-'claim_token') FROM (SELECT * FROM scheduled_jobs WHERE workspace_id=$1 AND execution_id=e.id ORDER BY created_at,id LIMIT 200) j),'[]'::jsonb),
 'deliveries',COALESCE((SELECT jsonb_agg(to_jsonb(d)) FROM (SELECT id,node_id,channel,status,decision,attempts,accepted_at,last_error,cancel_requested FROM delivery_jobs WHERE workspace_id=$1 AND execution_id=e.id ORDER BY created_at,id LIMIT 100)d),'[]'::jsonb),
 'waiters',COALESCE((SELECT jsonb_agg(to_jsonb(w)-'workspace_id') FROM (SELECT * FROM event_waiters WHERE workspace_id=$1 AND execution_id=e.id ORDER BY created_at LIMIT 100) w),'[]'::jsonb))
 FROM journey_executions e WHERE e.workspace_id=$1 AND e.id=$2`, who(ctx).WorkspaceID, in.ID))
	})
	huma.Get(api, "/api/v1/journeys/{id}/executions", func(ctx context.Context, in *struct {
		ID         string    `path:"id" format:"uuid"`
		CustomerID string    `query:"customer_id" maxLength:"128"`
		State      string    `query:"state" enum:"RUNNING,WAITING,COMPLETED,EXITED,FAILED,CANCELLED"`
		Limit      int       `query:"limit" default:"50" minimum:"1" maximum:"200"`
		Before     time.Time `query:"before"`
		BeforeID   string    `query:"before_id"`
	}) (*Result[[]map[string]any], error) {
		args := []any{who(ctx).WorkspaceID, in.ID}
		query := `SELECT id,journey_id,version_id,customer_id,entry_event_id,current_node_id,state,exit_reason,started_at,updated_at,completed_at FROM journey_executions WHERE workspace_id=$1 AND journey_id=$2`
		if in.CustomerID != "" {
			args = append(args, in.CustomerID)
			query += fmt.Sprintf(` AND customer_id=$%d`, len(args))
		}
		if in.State != "" {
			args = append(args, in.State)
			query += fmt.Sprintf(` AND state=$%d`, len(args))
		}
		if !in.Before.IsZero() {
			if in.BeforeID == "" {
				return nil, huma.Error422UnprocessableEntity("before_id is required with before")
			}
			args = append(args, in.Before, in.BeforeID)
			query += fmt.Sprintf(` AND (started_at,id)<($%d,$%d::uuid)`, len(args)-1, len(args))
		}
		args = append(args, in.Limit)
		query += fmt.Sprintf(` ORDER BY started_at DESC,id DESC LIMIT $%d`, len(args))
		return jsonRow[[]map[string]any](pool.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(e)),'[]'::jsonb) FROM (`+query+`) e`, args...))
	})
}
