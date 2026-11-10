//go:build integration

package delivery

import (
	"context"
	"github.com/ofk/customer-journey-composer/apps/api/internal/messaging"
	"sync/atomic"
	"testing"
	"time"
)

type heldProvider struct {
	started chan struct{}
	release chan struct{}
	first   atomic.Bool
}

func (*heldProvider) Idempotent() bool { return true }
func (p *heldProvider) Send(ctx context.Context, _ messaging.Message, _ string) Outcome {
	if p.first.CompareAndSwap(false, true) {
		close(p.started)
		select {
		case <-p.release:
		case <-ctx.Done():
			return Outcome{Status: "RETRY", Code: 408}
		}
	}
	return Outcome{Status: "ACCEPTED", Code: 200}
}
func TestSlowProviderDoesNotHoldOtherDispatchSlots(t *testing.T) {
	ctx, pool, now, workspace, _ := seeded(t, true)
	workspaces := []string{workspace}
	for range 7 {
		_, _, _, w, _ := seeded(t, true)
		workspaces = append(workspaces, w)
	}
	sender := New(pool, now)
	if _, err := sender.Schedule(ctx); err != nil {
		t.Fatal(err)
	}
	provider := &heldProvider{started: make(chan struct{}), release: make(chan struct{})}
	sender.Providers["smtp"] = provider
	sender.Providers["mock_sms"] = provider
	running, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); _ = sender.Run(running) }()
	defer func() { cancel(); <-done }()
	select {
	case <-provider.started:
	case <-time.After(3 * time.Second):
		t.Fatal("provider was not started")
	}
	deadline := time.Now().Add(2 * time.Second)
	accepted := 0
	for time.Now().Before(deadline) {
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM delivery_jobs WHERE workspace_id=ANY($1::uuid[]) AND status='ACCEPTED'`, workspaces).Scan(&accepted); err != nil {
			t.Fatal(err)
		}
		if accepted >= 8 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if accepted < 8 {
		t.Fatalf("one slow provider held otherwise free dispatch slots: only %d accepted while it was blocked", accepted)
	}
	close(provider.release)
}
