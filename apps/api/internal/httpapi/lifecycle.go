package httpapi

import (
	"context"
	"fmt"
	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func audit(ctx context.Context, tx pgx.Tx, action string, resource *string, details map[string]any) error {
	if details == nil {
		details = map[string]any{}
	}
	_, err := tx.Exec(ctx, `INSERT INTO audit_logs(workspace_id,actor_id,action,resource_id,details) VALUES($1,$2,$3,$4,$5)`, who(ctx).WorkspaceID, who(ctx).ID, action, resource, details)
	return err
}
func lifecycleTarget(state, resume, action string) (string, error) {
	switch action {
	case "pause":
		if state == "Paused" {
			return state, nil
		}
		if state == "Active" || state == "Sunset" {
			return "Paused", nil
		}
	case "resume":
		if state == "Active" || state == "Sunset" {
			return state, nil
		}
		if state == "Paused" {
			return resume, nil
		}
	case "sunset":
		if state == "Sunset" {
			return state, nil
		}
		if state == "Active" || state == "Paused" {
			return "Sunset", nil
		}
	case "complete":
		if state == "Completed" {
			return state, nil
		}
		if state == "Sunset" || state == "Paused" {
			return "Completed", nil
		}
	case "archive":
		if state == "Archived" {
			return state, nil
		}
		if state == "Draft" || state == "Completed" {
			return "Archived", nil
		}
	}
	return "", fmt.Errorf("cannot %s a %s journey", action, state)
}
func registerLifecycle(api huma.API, pool *pgxpool.Pool) {
	for _, action := range []string{"pause", "resume", "sunset", "complete", "archive"} {
		huma.Post(api, "/api/v1/journeys/{id}/"+action, func(ctx context.Context, in *struct {
			ID   string `path:"id" format:"uuid"`
			Body struct {
				Revision int `json:"revision" minimum:"1"`
			}
		}) (*Result[Journey], error) {
			if who(ctx).Role != "Admin" {
				return nil, huma.Error403Forbidden("Admin role required")
			}
			tx, err := pool.Begin(ctx)
			if err != nil {
				return nil, dbError(err)
			}
			defer func() { _ = tx.Rollback(context.Background()) }()
			var state, resume string
			var revision int
			if err = tx.QueryRow(ctx, `SELECT status,resume_status,revision FROM journeys WHERE workspace_id=$1 AND id=$2 FOR UPDATE`, who(ctx).WorkspaceID, in.ID).Scan(&state, &resume, &revision); err != nil {
				return nil, dbError(err)
			}
			target, err := lifecycleTarget(state, resume, action)
			if err != nil {
				return nil, huma.Error409Conflict(err.Error())
			}
			if target != state {
				if revision != in.Body.Revision {
					return nil, huma.Error409Conflict("Journey changed; reload before changing lifecycle")
				}
				if target == "Completed" || target == "Archived" {
					var pending bool
					if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM journey_executions WHERE workspace_id=$1 AND journey_id=$2 AND state IN ('RUNNING','WAITING')) OR EXISTS(SELECT 1 FROM journey_executions e JOIN delivery_jobs d ON d.execution_id=e.id WHERE e.workspace_id=$1 AND e.journey_id=$2 AND d.mode='LIVE' AND d.status IN ('SCHEDULED','CLAIMED','DISPATCHING')) OR EXISTS(SELECT 1 FROM journey_executions e JOIN action_outbox o ON o.execution_id=e.id WHERE e.workspace_id=$1 AND e.journey_id=$2 AND o.state='PENDING')`, who(ctx).WorkspaceID, in.ID).Scan(&pending); err != nil {
						return nil, dbError(err)
					}
					if pending {
						return nil, huma.Error409Conflict("Active executions or pending actions remain; sunset and drain before completing")
					}
				}
				if target == "Paused" {
					resume = state
				}
				if _, err = tx.Exec(ctx, `UPDATE journeys SET status=$3,resume_status=$4,revision=revision+1,updated_at=now() WHERE workspace_id=$1 AND id=$2`, who(ctx).WorkspaceID, in.ID, target, resume); err != nil {
					return nil, dbError(err)
				}
				if err = audit(ctx, tx, "journey."+action, &in.ID, map[string]any{"from": state, "to": target}); err != nil {
					return nil, dbError(err)
				}
			}
			out, err := jsonRow[Journey](tx.QueryRow(ctx, `SELECT to_jsonb(j)-'workspace_id' FROM journeys j WHERE workspace_id=$1 AND id=$2`, who(ctx).WorkspaceID, in.ID))
			if err != nil {
				return nil, err
			}
			if err = tx.Commit(ctx); err != nil {
				return nil, dbError(err)
			}
			return out, nil
		})
	}
	huma.Get(api, "/api/v1/audit-logs", func(ctx context.Context, in *struct {
		Limit int `query:"limit" default:"50" minimum:"1" maximum:"200"`
	}) (*Result[[]map[string]any], error) {
		if who(ctx).Role != "Admin" {
			return nil, huma.Error403Forbidden("Admin role required")
		}
		return jsonRow[[]map[string]any](pool.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(a)),'[]'::jsonb) FROM (SELECT id,actor_id,action,resource_id,details,created_at FROM audit_logs WHERE workspace_id=$1 ORDER BY created_at DESC,id DESC LIMIT $2)a`, who(ctx).WorkspaceID, in.Limit))
	})
}
