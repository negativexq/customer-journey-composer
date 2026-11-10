package httpapi

import (
	"context"
	"fmt"
	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ofk/customer-journey-composer/apps/api/internal/delivery"
	"github.com/ofk/customer-journey-composer/apps/api/internal/messaging"
	"github.com/ofk/customer-journey-composer/apps/api/internal/simulation"
	"github.com/ofk/customer-journey-composer/apps/api/internal/workflow"
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
	for _, i := range messaging.ValidateWorkflow(w) {
		issues = append(issues, gateIssue{"workflow", i.Message})
	}
	if len(issues) > 0 {
		return issues, nil
	}
	var passed bool
	if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM test_runs WHERE workspace_id=$1 AND journey_id=$2 AND workflow_hash=$3 AND result='PASS' AND mode='FULL' AND finalized LIMIT 1)`, who(ctx).WorkspaceID, id, simulation.Hash(w)).Scan(&passed); err != nil {
		return nil, dbError(err)
	}
	if !passed {
		issues = append(issues, gateIssue{"test_required", "Complete a passing full Test Lab run for this exact workflow before publishing"})
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
