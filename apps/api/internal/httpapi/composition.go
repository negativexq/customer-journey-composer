package httpapi

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	composer "github.com/negativexq/customer-journey-composer/apps/api/internal/ai"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/catalog"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/messaging"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

type CompositionState struct {
	BehaviorChecks      *workflow.BehaviorChecks `json:"behavior_checks,omitempty"`
	ConfirmedReferences []string                 `json:"confirmed_references,omitempty" maxItems:"500"`
	Step                string                   `json:"step" enum:"details,method,brief,review,builder"`
	Method              string                   `json:"method,omitempty" enum:"ai,scratch"`
	Provider            string                   `json:"provider,omitempty" enum:"openai,claude"`
	Contract            string                   `json:"acceptance_contract,omitempty" enum:"cart-recovery-eur.v1"`
	Brief               string                   `json:"brief,omitempty" maxLength:"4000"`
	Changes             string                   `json:"changes,omitempty" maxLength:"4000"`
	Editing             bool                     `json:"editing,omitempty"`
	Generating          bool                     `json:"generating,omitempty"`
	Proposal            *composer.Proposal       `json:"proposal,omitempty"`
}
type CompositionRecord struct {
	Journey Journey          `json:"journey"`
	State   CompositionState `json:"state"`
}
type compositionWrite struct {
	ID   string `path:"id" format:"uuid"`
	Body struct {
		Revision    int              `json:"revision" minimum:"1"`
		Name        string           `json:"name" minLength:"1" maxLength:"160"`
		Description string           `json:"description,omitempty" maxLength:"4000"`
		State       CompositionState `json:"state"`
	}
}
type compositionApproval struct {
	ID   string `path:"id" format:"uuid"`
	Body struct {
		Revision int `json:"revision" minimum:"1"`
	}
}
type compositionReader interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func readComposition(ctx context.Context, db compositionReader, id string) (*Result[CompositionRecord], error) {
	return jsonRow[CompositionRecord](db.QueryRow(ctx, `SELECT jsonb_build_object('journey',to_jsonb(j)-'workspace_id','state',COALESCE(c.state,'{}'::jsonb)||jsonb_build_object('step',j.creation_step)) FROM journeys j LEFT JOIN journey_compositions c ON c.workspace_id=j.workspace_id AND c.journey_id=j.id WHERE j.workspace_id=$1 AND j.id=$2`, who(ctx).WorkspaceID, id))
}
func lockComposition(ctx context.Context, tx pgx.Tx, id string, revision int) error {
	var current int
	var status, step string
	if err := tx.QueryRow(ctx, `SELECT revision,status,creation_step FROM journeys WHERE workspace_id=$1 AND id=$2 FOR UPDATE`, who(ctx).WorkspaceID, id).Scan(&current, &status, &step); err != nil {
		return dbError(err)
	}
	if current != revision {
		return huma.Error409Conflict("This journey changed in another tab. Reload before saving; your local brief has been kept.")
	}
	if status != "Draft" || step == "builder" {
		return huma.Error409Conflict("This journey has already left creation. Open the builder to edit it.")
	}
	return nil
}
func registerComposition(api huma.API, pool *pgxpool.Pool) {
	huma.Get(api, "/api/v1/journeys/{id}/composition", func(ctx context.Context, in *journeyID) (*Result[CompositionRecord], error) {
		return readComposition(ctx, pool, in.ID)
	})
	huma.Put(api, "/api/v1/journeys/{id}/composition", func(ctx context.Context, in *compositionWrite) (*Result[CompositionRecord], error) {
		if err := canEdit(ctx); err != nil {
			return nil, err
		}
		in.Body.Name = strings.TrimSpace(in.Body.Name)
		if in.Body.Name == "" {
			return nil, huma.Error422UnprocessableEntity("Journey name required")
		}
		state := in.Body.State
		if state.Step == "builder" {
			return nil, huma.Error422UnprocessableEntity("Use explicit approval to open the builder")
		}
		if state.Step == "review" && state.Proposal == nil {
			return nil, huma.Error422UnprocessableEntity("Review requires a proposal")
		}
		if state.Proposal != nil {
			// Draft review remains editable even when business expectations fail.
			// Approval and publication revalidate the independent expectations.
			if issues := messaging.ValidateWorkflow(state.Proposal.Workflow); len(issues) > 0 {
				return nil, huma.Error422UnprocessableEntity("The proposed workflow is not valid")
			}
		}
		if state.BehaviorChecks != nil {
			raw, _ := json.Marshal(state.BehaviorChecks)
			if len(raw) > 16384 || len(state.BehaviorChecks.Inputs) > 50 || len(state.BehaviorChecks.Thresholds) > 4 {
				return nil, huma.Error422UnprocessableEntity("Independent checks exceed the draft limits")
			}
		}

		raw, err := json.Marshal(state)
		if err != nil || len(raw) > 900000 {
			return nil, huma.Error422UnprocessableEntity("Composition exceeds the size limit")
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return nil, dbError(err)
		}
		defer tx.Rollback(ctx)
		if err = lockComposition(ctx, tx, in.ID, in.Body.Revision); err != nil {
			return nil, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO journey_compositions(workspace_id,journey_id,state) VALUES($1,$2,$3) ON CONFLICT(workspace_id,journey_id) DO UPDATE SET state=EXCLUDED.state`, who(ctx).WorkspaceID, in.ID, raw)
		if err != nil {
			return nil, dbError(err)
		}
		_, err = tx.Exec(ctx, `UPDATE journeys SET creation_step=$3,name=$4,description=$5,revision=revision+1,updated_at=now() WHERE workspace_id=$1 AND id=$2`, who(ctx).WorkspaceID, in.ID, state.Step, in.Body.Name, in.Body.Description)
		if err != nil {
			return nil, dbError(err)
		}
		result, err := readComposition(ctx, tx, in.ID)
		if err != nil {
			return nil, err
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, dbError(err)
		}
		return result, nil
	})
	huma.Post(api, "/api/v1/journeys/{id}/composition/approve", func(ctx context.Context, in *compositionApproval) (*Result[Journey], error) {
		if err := canEdit(ctx); err != nil {
			return nil, err
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return nil, dbError(err)
		}
		defer tx.Rollback(ctx)
		if err = lockComposition(ctx, tx, in.ID, in.Body.Revision); err != nil {
			return nil, err
		}
		record, err := readComposition(ctx, tx, in.ID)
		if err != nil {
			return nil, err
		}
		state := record.Body.State
		var draft any = map[string]any{"schema_version": 1, "nodes": []any{}, "edges": []any{}}
		if state.Method == "ai" {
			if state.Step != "review" || state.Proposal == nil || state.Editing || state.Generating {
				return nil, huma.Error422UnprocessableEntity("Review the completed proposal before approval")
			}
			if state.Proposal.SchemaVersion == composer.SchemaVersion && state.Proposal.Workflow.BehaviorChecks == nil && state.BehaviorChecks == nil && state.Proposal.Workflow.AcceptanceContract == "" {
				return nil, huma.Error422UnprocessableEntity("Independent behavior checks are required before approving this AI proposal")
			}
			if state.Contract == "" && state.BehaviorChecks != nil {
				state.Proposal.Workflow.BehaviorChecks = state.BehaviorChecks
			}
			if issues := append(messaging.ValidateWorkflow(state.Proposal.Workflow), catalog.ValidateAcceptance(state.Proposal.Workflow)...); len(issues) > 0 {
				return nil, huma.Error422UnprocessableEntity("The proposal needs correction before approval")
			}
			var catalogWorkspace string
			if err = tx.QueryRow(ctx, `SELECT id FROM workspaces WHERE id=$1 FOR SHARE`, who(ctx).WorkspaceID).Scan(&catalogWorkspace); err != nil {
				return nil, dbError(err)
			}
			if err = groundingRequired(ctx, tx, state.Proposal.Workflow, state.ConfirmedReferences); err != nil {
				return nil, err
			}
			draft = state.Proposal.Workflow
		} else if state.Method != "scratch" {
			return nil, huma.Error422UnprocessableEntity("Choose a creation method")
		}
		raw, err := json.Marshal(draft)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("Invalid proposal")
		}
		result, err := jsonRow[Journey](tx.QueryRow(ctx, `UPDATE journeys SET draft=$3,creation_step='builder',revision=revision+1,updated_at=now() WHERE workspace_id=$1 AND id=$2 RETURNING to_jsonb(journeys)-'workspace_id'`, who(ctx).WorkspaceID, in.ID, raw))
		if err != nil {
			return nil, err
		}
		if err = audit(ctx, tx, "journey.creation_approved", &in.ID, map[string]any{"method": state.Method}); err != nil {
			return nil, dbError(err)
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, dbError(err)
		}
		return result, nil
	})
}
