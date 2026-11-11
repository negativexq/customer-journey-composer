//go:build integration

package httpapi

import (
	"context"
	composer "github.com/negativexq/customer-journey-composer/apps/api/internal/ai"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/database"
	"os"
	"strings"
	"testing"
)

func TestGenerationAuditStoresOnlySanitizedAttemptMetadata(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Open(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var identity Identity
	if err = pool.QueryRow(ctx, `INSERT INTO workspaces(name,slug) VALUES('Usage test',$1) RETURNING id::text`, randomToken()[:15]).Scan(&identity.WorkspaceID); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO users(workspace_id,name,email,password_hash) VALUES($1,'Owner','usage@example.invalid','!disabled') RETURNING id::text`, identity.WorkspaceID).Scan(&identity.ID); err != nil {
		t.Fatal(err)
	}
	report := &composer.GenerationReport{Attempts: []composer.Attempt{{Provider: "openai", ProviderResult: composer.ProviderResult{Output: []byte("DO_NOT_STORE_PRIVATE_OUTPUT"), Model: "reported", Usage: &composer.Usage{InputTokens: 100, OutputTokens: 20}}, ProviderSucceeded: true}}}
	if err = recordGeneration(ctx, pool, identity, "openai", "generate", report, false); err != nil {
		t.Fatal(err)
	}
	var details string
	if err = pool.QueryRow(ctx, `SELECT details::text FROM audit_logs WHERE workspace_id=$1 AND action='ai.generation'`, identity.WorkspaceID).Scan(&details); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(details, "input_tokens") || strings.Contains(details, "DO_NOT_STORE_PRIVATE_OUTPUT") {
		t.Fatal("accounting missing or sensitive output stored")
	}
}
