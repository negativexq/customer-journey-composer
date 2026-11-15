package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	engine "github.com/negativexq/customer-journey-composer/apps/api/internal/runtime"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

type audienceStatus struct {
	Enabled   bool                 `json:"enabled"`
	Version   int                  `json:"version,omitempty"`
	NextRunAt *time.Time           `json:"next_run_at,omitempty"`
	Runs      []engine.AudienceRun `json:"runs"`
}

// currentAudience loads the active version of a journey whose trigger enrolls
// an audience. ok is false for event-triggered or unpublished journeys.
func currentAudience(ctx context.Context, pool *pgxpool.Pool, journey string) (version string, number int, spec workflow.Workflow, anchor time.Time, ok bool, err error) {
	var raw []byte
	err = pool.QueryRow(ctx, `SELECT v.id::text,v.version,v.workflow,GREATEST(v.created_at,COALESCE((SELECT max(r.slot_at) FROM audience_runs r WHERE r.version_id=v.id AND r.trigger='SCHEDULE'),v.created_at)) FROM journeys j JOIN journey_versions v ON v.workspace_id=j.workspace_id AND v.id=j.current_version_id WHERE j.workspace_id=$1 AND j.id=$2 AND j.status='Active'`, who(ctx).WorkspaceID, journey).Scan(&version, &number, &raw, &anchor)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", 0, spec, anchor, false, nil
	}
	if err != nil {
		return "", 0, spec, anchor, false, err
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		return "", 0, spec, anchor, false, err
	}
	return version, number, spec, anchor, spec.Start().Config.IsAudience(), nil
}

func registerAudience(api huma.API, pool *pgxpool.Pool) {
	// The live journey's audience schedule and its latest runs.
	huma.Get(api, "/api/v1/journeys/{id}/audience", func(ctx context.Context, in *journeyID) (*Result[audienceStatus], error) {
		out := audienceStatus{Runs: []engine.AudienceRun{}}
		version, number, spec, anchor, ok, err := currentAudience(ctx, pool, in.ID)
		if err != nil {
			return nil, dbError(err)
		}
		if !ok {
			return &Result[audienceStatus]{Body: out}, nil
		}
		out.Enabled, out.Version = true, number
		next := workflow.AudienceSlotAfter(spec.Start().Config, anchor)
		if now := time.Now(); next.Before(now) {
			next = workflow.AudienceSlotAfter(spec.Start().Config, now.Add(-time.Second))
		}
		out.NextRunAt = &next
		rows, err := pool.Query(ctx, `SELECT id::text,slot_at,trigger,state,checked,matched,enrolled FROM audience_runs WHERE workspace_id=$1 AND journey_id=$2 AND version_id=$3 ORDER BY started_at DESC,id LIMIT 10`, who(ctx).WorkspaceID, in.ID, version)
		if err != nil {
			return nil, dbError(err)
		}
		out.Runs, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (engine.AudienceRun, error) {
			var r engine.AudienceRun
			err := row.Scan(&r.ID, &r.SlotAt, &r.Trigger, &r.State, &r.Checked, &r.Matched, &r.Enrolled)
			return r, err
		})
		if err != nil {
			return nil, dbError(err)
		}
		return &Result[audienceStatus]{Body: out}, nil
	})
	// Enroll the audience now instead of waiting for the next scheduled run.
	huma.Post(api, "/api/v1/journeys/{id}/audience/runs", func(ctx context.Context, in *journeyID) (*Result[engine.AudienceRun], error) {
		if who(ctx).Role != "Admin" {
			return nil, huma.Error403Forbidden("Admin role required to enroll an audience")
		}
		version, _, spec, _, ok, err := currentAudience(ctx, pool, in.ID)
		if err != nil {
			return nil, dbError(err)
		}
		if !ok {
			return nil, huma.Error409Conflict("Publish a journey that enters an audience first")
		}
		now := time.Now().UTC().Truncate(time.Microsecond)
		run, err := engine.RunAudience(ctx, pool, who(ctx).WorkspaceID, in.ID, version, spec, now, "MANUAL", now)
		if errors.Is(err, engine.ErrAudienceRunExists) {
			return nil, huma.Error409Conflict("An audience run just started; try again")
		}
		if err != nil {
			return nil, dbError(err)
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return nil, dbError(err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()
		if err := audit(ctx, tx, "journey.audience_run", &in.ID, map[string]any{"run_id": run.ID, "matched": run.Matched, "enrolled": run.Enrolled}); err != nil {
			return nil, dbError(err)
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, dbError(err)
		}
		return &Result[engine.AudienceRun]{Body: run}, nil
	})
}
