//go:build integration

package delivery

import (
	"slices"
	"testing"
	"time"
)

// batchedWorkspaces seeds n workspaces, one customer each with an email and an
// SMS, on channels with room for batched calls, and schedules their messages.
func batchedWorkspaces(t *testing.T, n int) (*Service, *Mock, []string) {
	t.Helper()
	var sender *Service
	mock := &Mock{}
	workspaces := []string{}
	for range n {
		ctx, pool, now, workspace, _ := seeded(t, true)
		for _, provider := range []string{"smtp", "mock_sms"} {
			if _, err := pool.Exec(ctx, `INSERT INTO provider_channels(workspace_id,provider,rate_per_second) VALUES($1,$2,1000)`, workspace, provider); err != nil {
				t.Fatal(err)
			}
		}
		workspaces = append(workspaces, workspace)
		sender = New(pool, now)
	}
	sender.Providers["smtp"], sender.Providers["mock_sms"] = mock, mock
	sender.Clock.(*clock).now = sender.Clock.Now().Add(time.Second)
	for range 10 {
		if _, err := sender.Schedule(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	return sender, mock, workspaces
}

// ours keeps this test's messages and hands any other test's back untouched.
func ours(t *testing.T, s *Service, jobs []Job, workspaces []string) []Job {
	t.Helper()
	kept := []Job{}
	for _, job := range jobs {
		if slices.Contains(workspaces, job.WorkspaceID) {
			kept = append(kept, job)
			continue
		}
		if _, err := s.Pool.Exec(t.Context(), `UPDATE delivery_jobs SET status='CANCELLED',claim_token=NULL,lease_until=NULL WHERE id=$1`, job.ID); err != nil {
			t.Fatal(err)
		}
	}
	return kept
}

func TestBatchedDeliverySendsEachMessageOnceAndKeepsTheFence(t *testing.T) {
	sender, mock, workspaces := batchedWorkspaces(t, 3)
	ctx := t.Context()
	jobs := ours(t, sender, mustClaim(t, sender, 8), workspaces)
	if len(jobs) != 3 {
		t.Fatalf("claimed %d messages, want one per customer (3)", len(jobs))
	}
	// Consent revoked between claim and dispatch stops that customer's send.
	revoked := jobs[1].WorkspaceID
	if _, err := sender.Pool.Exec(ctx, `UPDATE customers SET consents='{"email":false,"sms":false}' WHERE workspace_id=$1`, revoked); err != nil {
		t.Fatal(err)
	}
	if err := sender.DispatchBatch(ctx, jobs); err != nil {
		t.Fatal(err)
	}
	if mock.calls != 2 {
		t.Fatalf("provider calls after the first batch: %d, want 2", mock.calls)
	}
	for range 20 {
		more := ours(t, sender, mustClaim(t, sender, 8), workspaces)
		if len(more) == 0 {
			break
		}
		if err := sender.DispatchBatch(ctx, more); err != nil {
			t.Fatal(err)
		}
	}
	for _, workspace := range workspaces {
		var accepted, blocked, attempts, extra int
		if err := sender.Pool.QueryRow(ctx, `SELECT count(*) FILTER(WHERE status='ACCEPTED'),count(*) FILTER(WHERE status='BLOCKED' AND decision->>'reason'='consent_revoked'),count(*) FILTER(WHERE attempts=1),count(*) FILTER(WHERE status NOT IN ('ACCEPTED','BLOCKED')) FROM delivery_jobs WHERE workspace_id=$1`, workspace).Scan(&accepted, &blocked, &attempts, &extra); err != nil {
			t.Fatal(err)
		}
		want := [2]int{2, 0}
		if workspace == revoked {
			want = [2]int{0, 2}
		}
		if accepted != want[0] || blocked != want[1] || attempts != 2 || extra != 0 {
			t.Fatalf("workspace revoked=%v: accepted=%d blocked=%d single_attempt=%d other=%d", workspace == revoked, accepted, blocked, attempts, extra)
		}
		var answered int
		if err := sender.Pool.QueryRow(ctx, `SELECT count(*) FROM delivery_attempts WHERE workspace_id=$1 AND status='ACCEPTED' AND completed_at IS NOT NULL`, workspace).Scan(&answered); err != nil {
			t.Fatal(err)
		}
		if answered != want[0] {
			t.Fatalf("workspace revoked=%v: %d answered attempts, want %d", workspace == revoked, answered, want[0])
		}
	}
	if mock.calls != 4 {
		t.Fatalf("provider calls: %d, want 4", mock.calls)
	}
}

// A channel without room for a batch, here the default 50 calls/s, is claimed
// one message at a time through the locked admission path.
func TestBatchedClaimFallsBackOnAChannelWithoutRoom(t *testing.T) {
	ctx, pool, now, workspace, _ := seeded(t, true)
	sender := New(pool, now)
	now.now = now.now.Add(time.Second)
	if _, err := sender.Schedule(ctx); err != nil {
		t.Fatal(err)
	}
	jobs := ours(t, sender, mustClaim(t, sender, 8), []string{workspace})
	if len(jobs) != 1 {
		t.Fatalf("claimed %d messages, want 1", len(jobs))
	}
	// Only the locked admission path creates the missing channel row.
	var channels int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM provider_channels WHERE workspace_id=$1`, workspace).Scan(&channels); err != nil || channels != 1 {
		t.Fatalf("channel rows %d (%v): the claim did not take the locked path", channels, err)
	}
}

func mustClaim(t *testing.T, s *Service, n int) []Job {
	t.Helper()
	for range 20 {
		jobs, err := s.ClaimBatch(t.Context(), n)
		if err == ErrGated {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		return jobs
	}
	t.Fatal("claim stayed gated")
	return nil
}
