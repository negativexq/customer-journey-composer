package httpapi

import (
	"context"
	"fmt"
	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/catalog"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/delivery"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/messaging"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/simulation"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
	"net/mail"
	"os"
	"strconv"
)

type gateIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type gateReader interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func publishIssues(ctx context.Context, db gateReader, id string, w workflow.Workflow) ([]gateIssue, error) {
	issues := []gateIssue{}
	var creation string
	if err := db.QueryRow(ctx, `SELECT creation_step FROM journeys WHERE workspace_id=$1 AND id=$2`, who(ctx).WorkspaceID, id).Scan(&creation); err != nil {
		return nil, dbError(err)
	}
	if creation != "builder" {
		return []gateIssue{{"creation_review", "Finish reviewing the creation proposal before publishing"}}, nil
	}
	for _, i := range append(messaging.ValidateWorkflow(w), catalog.ValidateAcceptance(w)...) {
		issues = append(issues, gateIssue{"workflow", i.Message})
	}
	if len(issues) > 0 {
		return issues, nil
	}
	var passed, failed, pending, missing int
	if err := db.QueryRow(ctx, `WITH latest AS MATERIALIZED (
 SELECT DISTINCT ON(r.acceptance_key) r.acceptance_key,r.scenario_id,r.result,r.finalized
 FROM test_runs r LEFT JOIN test_scenarios s ON s.workspace_id=r.workspace_id AND s.id=r.scenario_id
 WHERE r.workspace_id=$1 AND r.journey_id=$2 AND r.workflow_hash=$3 AND r.mode='FULL' AND r.acceptance AND (r.scenario_id IS NULL OR s.required)
 ORDER BY r.acceptance_key,r.created_at DESC,r.id DESC
 ) SELECT count(*) FILTER(WHERE result='PASS' AND finalized),count(*) FILTER(WHERE result='FAIL' AND finalized),count(*) FILTER(WHERE NOT finalized OR result='PENDING'),
 (SELECT count(*) FROM test_scenarios s WHERE s.workspace_id=$1 AND s.journey_id=$2 AND s.required AND NOT EXISTS(SELECT 1 FROM latest l WHERE l.scenario_id=s.id)) FROM latest`, who(ctx).WorkspaceID, id, simulation.Hash(w)).Scan(&passed, &failed, &pending, &missing); err != nil {
		return nil, dbError(err)
	}
	if passed+failed+pending == 0 || missing > 0 {
		issues = append(issues, gateIssue{"test_required", fmt.Sprintf("Run all required scenarios against this exact workflow (%d saved cases missing)", missing)})
	}
	if failed > 0 {
		issues = append(issues, gateIssue{"test_failed", fmt.Sprintf("%d acceptance scenarios failed; replay and pass each case before publishing", failed)})
	}
	if pending > 0 {
		issues = append(issues, gateIssue{"test_incomplete", fmt.Sprintf("%d acceptance scenarios are unfinished", pending)})
	}

	hasEmail := false
	for _, n := range w.Nodes {
		hasEmail = hasEmail || n.Type == "email"
	}
	if hasEmail {
		smtp := delivery.SMTPFromEnv()
		port, portErr := strconv.Atoi(smtp.Port)
		_, senderErr := mail.ParseAddress(smtp.From)
		// Validation only: no provider/network request is made during publication.
		if smtp.Host == "" || portErr != nil || port < 1 || port > 65535 || senderErr != nil || (os.Getenv("APP_ENV") == "production" && os.Getenv("SMTP_HOST") == "") {
			issues = append(issues, gateIssue{"email_provider", "Configure a valid email provider host, port and sender"})
		}
	}
	return issues, nil
}
func registerPublishChecks(api huma.API, pool *pgxpool.Pool) {
	huma.Get(api, "/api/v1/journeys/{id}/publish-check", func(ctx context.Context, in *journeyID) (*Result[struct {
		Ready  bool        `json:"ready"`
		Issues []gateIssue `json:"issues"`
		Policy string      `json:"policy"`
	}], error) {
		w, err := loadWorkflow(ctx, pool, in.ID)
		if err != nil {
			return nil, err
		}
		issues, err := publishIssues(ctx, pool, in.ID, w)
		if err != nil {
			return nil, err
		}
		return &Result[struct {
			Ready  bool        `json:"ready"`
			Issues []gateIssue `json:"issues"`
			Policy string      `json:"policy"`
		}]{Body: struct {
			Ready  bool        `json:"ready"`
			Issues []gateIssue `json:"issues"`
			Policy string      `json:"policy"`
		}{len(issues) == 0, issues, fmt.Sprintf("Consent is enforced for every send. Workspace frequency caps apply. Global exit: %t. Publishing creates an immutable version.", w.GlobalExit != nil)}}, nil
	})
}
