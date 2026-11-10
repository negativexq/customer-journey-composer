//go:build integration

package database

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestMigrationsAgainstPostgres(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("TEST_DATABASE_URL is required; use make test-integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := Ready(ctx, pool); err == nil {
		t.Fatal("unmigrated database must not be ready")
	}
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for range 4 {
		wg.Go(func() { errs <- Migrate(ctx, pool) })
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent migration failed: %v", err)
		}
	}
	if err := Ready(ctx, pool); err != nil {
		t.Fatalf("migrated database is not ready: %v", err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO workspaces (name, slug) VALUES ('Workspace', 'workspace')"); err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, "INSERT INTO workspaces (name, slug) VALUES ('Another', 'workspace')")
	if err == nil {
		t.Fatal("duplicate workspace slug must be rejected by the database")
	}
	if _, err := pool.Exec(ctx, "UPDATE schema_migrations SET checksum = 'tampered'"); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, pool); err == nil {
		t.Fatal("modified applied migrations must be rejected")
	}
	if err := Ready(ctx, pool); err == nil {
		t.Fatal("checksum mismatch must fail readiness")
	}
	// Restore the test schema and verify the complete migration can be replayed.
	if _, err := pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public"); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	pool.Close()
	// Persistence survives a pool/process boundary; no in-memory schema state.
	reconnected, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer reconnected.Close()
	if err := Ready(ctx, reconnected); err != nil {
		t.Fatal(err)
	}
	if _, err := reconnected.Exec(ctx, "INSERT INTO workspaces (name, slug) VALUES ('', 'invalid')"); err == nil {
		t.Fatal("invalid workspace name must be rejected")
	}
	var version string
	if err := reconnected.QueryRow(ctx, "SELECT version FROM schema_migrations").Scan(&version); err == pgx.ErrNoRows {
		t.Fatal("migration history missing after reconnect")
	} else if err != nil {
		t.Fatal(err)
	}
}
