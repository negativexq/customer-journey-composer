package httpapi

import (
	"testing"
	"time"
)

func TestRequestLimiterRefillAndIsolation(t *testing.T) {
	l := &requestLimiter{}
	now := time.Now()
	for range 2 {
		if !l.allow("workspace-a", 1, 2, now) {
			t.Fatal("initial burst rejected")
		}
	}
	if l.allow("workspace-a", 1, 2, now) {
		t.Fatal("burst exceeded")
	}
	if !l.allow("workspace-b", 1, 2, now) || !l.allow("workspace-a", 1, 2, now.Add(time.Second)) {
		t.Fatal("isolation or refill failed")
	}
}
func TestSignupClosedByDefault(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("PUBLIC_SIGNUP", "")
	if signupEnabled() {
		t.Fatal("production signup open")
	}
	t.Setenv("APP_ENV", "development")
	if !signupEnabled() {
		t.Fatal("development signup closed")
	}
}
