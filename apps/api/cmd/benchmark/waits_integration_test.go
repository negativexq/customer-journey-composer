//go:build integration

package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestWaitSamplingRecognizesBlockedBackendWithoutSQLPayload(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	holder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Rollback(context.Background()) }()
	if _, err = holder.Exec(ctx, `SELECT pg_advisory_xact_lock(998877)`); err != nil {
		t.Fatal(err)
	}
	blocked, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	waiting, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); _, _ = blocked.Exec(waiting, `SELECT pg_advisory_xact_lock(998877)`) }()
	defer func() { stop(); <-done; _ = blocked.Rollback(context.Background()) }()
	probe := &waitProbe{Samples: map[string]*waitSample{}}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		probe.sample(ctx, pool, "locked_fixture")
		for _, s := range probe.Samples {
			if s.Type == "Lock" && s.Event == "advisory" && s.BlockedSamples > 0 {
				if probe.Errors != 0 {
					t.Fatal("sampling failed")
				}
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("blocked backend not observed: %+v", probe.report())
}
