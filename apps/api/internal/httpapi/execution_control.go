package httpapi

import (
	"context"
	"errors"
	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	engine "github.com/ofk/customer-journey-composer/apps/api/internal/runtime"
	"time"
)

func registerExecutionControls(api huma.API, pool *pgxpool.Pool) {
	for _, action := range []string{"cancel", "retry"} {
		huma.Post(api, "/api/v1/executions/{id}/"+action, func(ctx context.Context, in *journeyID) (*Result[map[string]any], error) {
			if err := canEdit(ctx); err != nil {
				return nil, err
			}
			if action == "retry" && who(ctx).Role != "Admin" {
				return nil, huma.Error403Forbidden("Admin role required for recovery")
			}
			tx, err := pool.Begin(ctx)
			if err != nil {
				return nil, dbError(err)
			}
			defer func() { _ = tx.Rollback(context.Background()) }()
			out, err := engine.Control(ctx, tx, who(ctx).WorkspaceID, in.ID, action, time.Now().UTC())
			if err != nil {
				var conflict *engine.ControlConflict
				if errors.As(err, &conflict) {
					return nil, huma.Error409Conflict(conflict.Message)
				}
				return nil, dbError(err)
			}
			if out["unchanged"] != true {
				if err = audit(ctx, tx, "execution."+action, &in.ID, out); err != nil {
					return nil, dbError(err)
				}
			}
			if err = tx.Commit(ctx); err != nil {
				return nil, dbError(err)
			}
			return &Result[map[string]any]{Body: out}, nil
		})
	}
}
