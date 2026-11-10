package demo

import (
	"crypto/rand"
	"testing"
)

func TestDemoRequiresExplicitDevelopmentAndCredentials(t *testing.T) {
	env := map[string]string{}
	get := func(k string) string { return env[k] }
	c, err := Load(get)
	if err != nil || c.Enabled {
		t.Fatal("demo enabled by default")
	}
	env["DEMO_ENABLED"] = "true"
	env["APP_ENV"] = "production"
	if _, err = Load(get); err == nil {
		t.Fatal("demo accepted in production")
	}
	env["APP_ENV"] = "development"
	env["DEMO_EMAIL"] = "demo@example.invalid"
	if _, err = Load(get); err == nil {
		t.Fatal("blank password accepted")
	}
	// Generated environment credentials are validated without making HTTP calls.
	env["DEMO_PASSWORD"] = rand.Text()
	if _, err = Load(get); err != nil {
		t.Fatal(err)
	}
}
