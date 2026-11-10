package httpapi

import (
	"context"
	"encoding/json"
	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/catalog"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/messaging"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
	"strings"
)

func templateSource(ctx context.Context, pool *pgxpool.Pool, id string) (catalog.Template, error) {
	if t, ok := catalog.Find(id); ok {
		return t, nil
	}
	var parsed pgtype.UUID
	if err := parsed.Scan(id); err != nil {
		return catalog.Template{}, huma.Error404NotFound("Template not found")
	}
	out, err := jsonRow[catalog.Template](pool.QueryRow(ctx, `SELECT to_jsonb(t)-'workspace_id'-'created_by' FROM journey_templates t WHERE workspace_id=$1 AND id=$2`, who(ctx).WorkspaceID, id))
	if err != nil {
		return catalog.Template{}, err
	}
	return out.Body, nil
}
func registerTemplates(api huma.API, pool *pgxpool.Pool) {
	huma.Get(api, "/api/v1/templates", func(ctx context.Context, _ *struct{}) (*Result[[]map[string]any], error) {
		out, err := jsonRow[[]map[string]any](pool.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(t)),'[]'::jsonb) FROM (SELECT id,name,description,tags,created_at,false AS builtin FROM journey_templates WHERE workspace_id=$1 ORDER BY created_at DESC,id DESC LIMIT 100)t`, who(ctx).WorkspaceID))
		if err != nil {
			return nil, err
		}
		rows := []map[string]any{}
		for _, t := range catalog.Builtins() {
			rows = append(rows, map[string]any{"id": t.ID, "name": t.Name, "description": t.Description, "tags": t.Tags, "builtin": true, "node_count": len(t.Workflow.Nodes)})
		}
		out.Body = append(rows, out.Body...)
		return out, nil
	})
	huma.Get(api, "/api/v1/templates/{id}", func(ctx context.Context, in *struct {
		ID string `path:"id"`
	}) (*Result[catalog.Template], error) {
		t, err := templateSource(ctx, pool, in.ID)
		if err != nil {
			return nil, err
		}
		return &Result[catalog.Template]{Body: t}, nil
	})
	huma.Post(api, "/api/v1/templates", func(ctx context.Context, in *struct {
		Body struct {
			JourneyID string `json:"journey_id" format:"uuid"`
			Version   int    `json:"version,omitempty" minimum:"0"`
			Name      string `json:"name" minLength:"1" maxLength:"160"`
		}
	}) (*Result[map[string]any], error) {
		if err := canEdit(ctx); err != nil {
			return nil, err
		}
		var raw []byte
		var description string
		var tags []byte
		if err := pool.QueryRow(ctx, `SELECT CASE WHEN $3::int>0 THEN (SELECT workflow FROM journey_versions WHERE workspace_id=$1 AND journey_id=$2 AND version=$3) ELSE draft END,description,tags FROM journeys WHERE workspace_id=$1 AND id=$2`, who(ctx).WorkspaceID, in.Body.JourneyID, in.Body.Version).Scan(&raw, &description, &tags); err != nil {
			return nil, dbError(err)
		}
		var spec workflow.Workflow
		if json.Unmarshal(raw, &spec) != nil || len(messaging.ValidateWorkflow(spec)) > 0 {
			return nil, huma.Error422UnprocessableEntity("Save a valid workflow as a template")
		}
		return jsonRow[map[string]any](pool.QueryRow(ctx, `INSERT INTO journey_templates(workspace_id,name,description,tags,workflow,created_by) VALUES($1,$2,$3,$4,$5,$6) RETURNING jsonb_build_object('id',id,'name',name)`, who(ctx).WorkspaceID, strings.TrimSpace(in.Body.Name), description, tags, raw, who(ctx).ID))
	})
	huma.Post(api, "/api/v1/templates/{id}/create-journey", func(ctx context.Context, in *struct {
		ID   string `path:"id"`
		Body struct {
			Name string `json:"name,omitempty" maxLength:"160"`
		}
	}) (*Result[Journey], error) {
		if err := canEdit(ctx); err != nil {
			return nil, err
		}
		t, err := templateSource(ctx, pool, in.ID)
		if err != nil {
			return nil, err
		}
		name := strings.TrimSpace(in.Body.Name)
		if name == "" {
			name = t.Name
		}
		raw, _ := json.Marshal(t.Workflow)
		return jsonRow[Journey](pool.QueryRow(ctx, `INSERT INTO journeys(workspace_id,name,description,tags,draft) VALUES($1,$2,$3,$4,$5) RETURNING to_jsonb(journeys)-'workspace_id'`, who(ctx).WorkspaceID, name, t.Description, t.Tags, raw))
	})
}
