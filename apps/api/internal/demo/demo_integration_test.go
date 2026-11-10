//go:build integration

package demo

import (
	"context"
	"crypto/rand"
	"github.com/ofk/customer-journey-composer/apps/api/internal/database"
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
