//go:build integration

package demo

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"github.com/ofk/customer-journey-composer/apps/api/internal/database"
	sim "github.com/ofk/customer-journey-composer/apps/api/internal/simulation"
	"github.com/ofk/customer-journey-composer/apps/api/internal/workflow"
	"golang.org/x/crypto/bcrypt"
	"os"
	"testing"
)

func TestDemoProvisioningIsIdempotentAndEnvironmentGated(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Open(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APP_ENV", "production")
	t.Setenv("DEMO_ENABLED", "true")
	t.Setenv("DEMO_EMAIL", "demo@example.invalid")
	t.Setenv("DEMO_PASSWORD", rand.Text())
	if err = Ensure(ctx, pool); err == nil {
		t.Fatal("demo provisioned in production")
	}
	t.Setenv("APP_ENV", "development")
	for range 2 {
		if err = Ensure(ctx, pool); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	var hash string
	if err = pool.QueryRow(ctx, `SELECT count(*),min(password_hash) FROM users WHERE email='demo@example.invalid'`).Scan(&count, &hash); err != nil || count != 1 {
		t.Fatal("demo account duplicated", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(os.Getenv("DEMO_PASSWORD"))) != nil {
		t.Fatal("demo password mismatch")
	}
}

func TestSeededScenariosAreExecutableAndNeverCreateLiveRecords(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Open(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APP_ENV", "development")
	t.Setenv("DEMO_ENABLED", "true")
	t.Setenv("DEMO_EMAIL", "demo@example.invalid")
	t.Setenv("DEMO_PASSWORD", rand.Text())
	if err = Ensure(ctx, pool); err != nil {
		t.Fatal(err)
	}
	rows, err := pool.Query(ctx, `SELECT s.scenario,w.workflow FROM test_scenarios s JOIN sandbox_workflows w ON w.workspace_id=s.workspace_id AND w.workflow_hash=s.workflow_hash JOIN workspaces ws ON ws.id=s.workspace_id WHERE ws.slug='local-demo'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var raw, flow []byte
		rows.Scan(&raw, &flow)
		var scenario sim.Scenario
		var spec workflow.Workflow
		json.Unmarshal(raw, &scenario)
		json.Unmarshal(flow, &spec)
		state, spec, scenario, e := sim.New(spec, scenario)
		if e != nil {
			t.Fatal(e)
		}
		if e = sim.Advance(spec, scenario, &state, "complete", 0); e != nil || state.Result != "PASS" {
			t.Fatal(scenario.Name, state.Result, e)
		}
		n++
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatal("wrong demo scenario count", n)
	}
	var live int
	pool.QueryRow(ctx, `SELECT count(*) FROM journey_executions e JOIN workspaces w ON w.id=e.workspace_id WHERE w.slug='local-demo'`).Scan(&live)
	if live != 0 {
		t.Fatal("demo fabricated live executions")
	}
}
