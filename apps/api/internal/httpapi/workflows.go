package httpapi

import (
	"context"
	"encoding/json"
	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/messaging"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

type Validation struct {
	Valid  bool             `json:"valid"`
	Issues []workflow.Issue `json:"issues"`
}
type Version struct {
	ID        string            `json:"id"`
	Version   int               `json:"version"`
	Workflow  workflow.Workflow `json:"workflow"`
	CreatedAt string            `json:"created_at"`
}

func loadWorkflow(ctx context.Context, pool *pgxpool.Pool, id string) (workflow.Workflow, error) {
	var data []byte
	err := pool.QueryRow(ctx, `SELECT draft FROM journeys WHERE workspace_id=$1 AND id=$2`, who(ctx).WorkspaceID, id).Scan(&data)
	if err != nil {
		return workflow.Workflow{}, dbError(err)
	}
	var w workflow.Workflow
	if err := json.Unmarshal(data, &w); err != nil {
		return w, huma.Error422UnprocessableEntity("Invalid workflow document")
	}
	return w, nil
}

type VersionMetadata struct {
	ID        string `json:"id"`
	Version   int    `json:"version"`
	CreatedAt string `json:"created_at"`
}

func registerWorkflows(api huma.API, pool *pgxpool.Pool) {
	huma.Post(api, "/api/v1/journeys/{id}/validate", func(ctx context.Context, in *journeyID) (*Result[Validation], error) {
		w, err := loadWorkflow(ctx, pool, in.ID)
		if err != nil {
			return nil, err
		}
		issues := messaging.ValidateWorkflow(w)
		return &Result[Validation]{Body: Validation{len(issues) == 0, issues}}, nil
	})
	huma.Post(api, "/api/v1/workflows/validate", func(ctx context.Context, in *struct{ Body workflow.Workflow }) (*Result[Validation], error) {
		issues := messaging.ValidateWorkflow(in.Body)
		return &Result[Validation]{Body: Validation{len(issues) == 0, issues}}, nil
	})
	huma.Post(api, "/api/v1/journeys/{id}/publish", func(ctx context.Context, in *struct {
		ID   string `path:"id" format:"uuid"`
		Body struct {
			Revision int `json:"revision" minimum:"1"`
		}
	}) (*Result[Version], error) {
		if who(ctx).Role != "Admin" {
			return nil, huma.Error403Forbidden("Admin role required to publish")
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return nil, dbError(err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()
		var data []byte
		var revision int
		var status string
		err = tx.QueryRow(ctx, `SELECT draft,revision,status FROM journeys WHERE workspace_id=$1 AND id=$2 FOR UPDATE`, who(ctx).WorkspaceID, in.ID).Scan(&data, &revision, &status)
		if err != nil {
			return nil, dbError(err)
		}
		if revision != in.Body.Revision || status == "Archived" {
			return nil, huma.Error409Conflict("Journey changed or archived; reload before publishing")
		}
		var w workflow.Workflow
		if err := json.Unmarshal(data, &w); err != nil {
			return nil, huma.Error422UnprocessableEntity("Invalid workflow")
		}
		if issues := messaging.ValidateWorkflow(w); len(issues) > 0 {
			return nil, huma.Error422UnprocessableEntity("Fix workflow validation errors before publishing")
		}
		gates, err := publishIssues(ctx, tx, in.ID, w)
		if err != nil {
			return nil, err
		}
		if len(gates) > 0 {
			return nil, huma.Error422UnprocessableEntity(gates[0].Message)
		}
		out, err := jsonRow[Version](tx.QueryRow(ctx, `INSERT INTO journey_versions(workspace_id,journey_id,version,workflow,published_by) SELECT $1,$2,COALESCE(max(version),0)+1,$3,$4 FROM journey_versions WHERE workspace_id=$1 AND journey_id=$2 RETURNING jsonb_build_object('id',id,'version',version,'workflow',workflow,'created_at',created_at)`, who(ctx).WorkspaceID, in.ID, data, who(ctx).ID))
		if err != nil {
			return nil, err
		}
		_, err = tx.Exec(ctx, `UPDATE journeys SET current_version_id=$3,entry_event_name=$4,status='Active',revision=revision+1,updated_at=now() WHERE workspace_id=$1 AND id=$2`, who(ctx).WorkspaceID, in.ID, out.Body.ID, w.Start().Config.EventName)
		if err != nil {
			return nil, dbError(err)
		}
		if err := audit(ctx, tx, "journey.publish", &in.ID, map[string]any{"version": out.Body.Version, "version_id": out.Body.ID}); err != nil {
			return nil, dbError(err)
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, dbError(err)
		}
		return out, nil
	})
	huma.Get(api, "/api/v1/journeys/{id}/versions", func(ctx context.Context, in *journeyID) (*Result[[]VersionMetadata], error) {
		return jsonRow[[]VersionMetadata](pool.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(v) ORDER BY v.version DESC),'[]'::jsonb) FROM (SELECT id,version,created_at FROM journey_versions WHERE workspace_id=$1 AND journey_id=$2 ORDER BY version DESC LIMIT 100) v`, who(ctx).WorkspaceID, in.ID))
	})
	huma.Get(api, "/api/v1/journeys/{id}/versions/{version}", func(ctx context.Context, in *struct {
		ID      string `path:"id" format:"uuid"`
		Version int    `path:"version" minimum:"1"`
	}) (*Result[Version], error) {
		return jsonRow[Version](pool.QueryRow(ctx, `SELECT jsonb_build_object('id',id,'version',version,'workflow',workflow,'created_at',created_at) FROM journey_versions WHERE workspace_id=$1 AND journey_id=$2 AND version=$3`, who(ctx).WorkspaceID, in.ID, in.Version))
	})

}
