//go:build integration

package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/database"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/messaging"
)

const channelTestPhone = "+905551112233"

type channelTest struct {
	t         *testing.T
	ctx       context.Context
	pool      *pgxpool.Pool
	now       *clock
	workspace string
	sender    *Service
	sms       *Mock
}

// channelSetup makes a workspace that may test-send SMS and a sender whose SMS
// provider answers as the test says.
func channelSetup(t *testing.T, codes ...int) *channelTest {
	t.Helper()
	ctx := context.Background()
	pool, err := database.Open(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	c := &channelTest{t: t, ctx: ctx, pool: pool, now: &clock{now: time.Now().UTC().Truncate(time.Microsecond)}}
	if err := pool.QueryRow(ctx, `INSERT INTO workspaces(name,slug) VALUES('Channel tests',$1) RETURNING id::text`, "test-"+randomID()).Scan(&c.workspace); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO workspace_settings(workspace_id,test_allowlist) VALUES($1,$2)`, c.workspace, []string{channelTestPhone}); err != nil {
		t.Fatal(err)
	}
	c.sender = New(pool, c.now)
	c.sms = &Mock{Codes: codes}
	c.sender.Providers["mock_sms"] = c.sms
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `UPDATE delivery_jobs SET status='CANCELLED',claim_token=NULL,lease_until=NULL WHERE workspace_id=$1 AND status IN ('SCHEDULED','CLAIMED','DISPATCHING')`, c.workspace)
	})
	return c
}

func (c *channelTest) queue(n int) []string {
	c.t.Helper()
	ids := []string{}
	body, _ := json.Marshal(messaging.Message{Channel: "sms", To: channelTestPhone, Text: "Hi"})
	for range n {
		var id string
		if err := c.pool.QueryRow(c.ctx, `INSERT INTO delivery_jobs(workspace_id,mode,channel,provider,idempotent_provider,logical_key,message,decision,status,due_at,created_at) VALUES($1,'TEST','sms','mock_sms',true,$2,$3,'{}','SCHEDULED',$4,$4) RETURNING id::text`, c.workspace, "test:"+randomID(), body, c.now.Now()).Scan(&id); err != nil {
			c.t.Fatal(err)
		}
		ids = append(ids, id)
	}
	return ids
}

// claim returns this workspace's next claimable message, or nil when its
// channel is held. Another test's leftovers are cancelled on the way.
func (c *channelTest) claim() *Job {
	c.t.Helper()
	for range 50 {
		job, err := c.sender.Claim(c.ctx)
		if errors.Is(err, ErrGated) {
			continue
		}
		if err != nil {
			c.t.Fatal(err)
		}
		if job == nil {
			return nil
		}
		if job.WorkspaceID == c.workspace {
			return job
		}
		_, _ = c.pool.Exec(c.ctx, `UPDATE delivery_jobs SET status='CANCELLED',claim_token=NULL WHERE id=$1`, job.ID)
	}
	c.t.Fatal("no claim settled")
	return nil
}

func (c *channelTest) send() *Job {
	c.t.Helper()
	job := c.claim()
	if job == nil {
		c.t.Fatal("expected a message to go out")
	}
	if err := c.sender.Dispatch(c.ctx, *job); err != nil {
		c.t.Fatal(err)
	}
	return job
}

func (c *channelTest) delivery(id string) (status string, attempts, free int, due time.Time) {
	c.t.Helper()
	if err := c.pool.QueryRow(c.ctx, `SELECT status,attempts,free_attempts,due_at FROM delivery_jobs WHERE id=$1`, id).Scan(&status, &attempts, &free, &due); err != nil {
		c.t.Fatal(err)
	}
	return
}

func (c *channelTest) notifications() []string {
	c.t.Helper()
	var kinds []string
	if err := c.pool.QueryRow(c.ctx, `SELECT COALESCE(array_agg(kind ORDER BY created_at,id),'{}') FROM notifications WHERE workspace_id=$1`, c.workspace).Scan(&kinds); err != nil {
		c.t.Fatal(err)
	}
	return kinds
}

func TestA429PausesTheChannelForEveryWorkerWithoutUsingAnAttempt(t *testing.T) {
	c := channelSetup(t, 429)
	c.sms.RetryAfter = 5 * time.Second
	c.queue(2)
	job := c.send()
	status, attempts, free, due := c.delivery(job.ID)
	if status != "SCHEDULED" || attempts != 1 || free != 1 || !due.Equal(c.now.Now().Add(5*time.Second)) {
		t.Fatal("a 429 waits for Retry-After at no cost", status, attempts, free, due)
	}
	if next := c.claim(); next != nil {
		t.Fatal("the other message waits for the pause too", next.ID)
	}
	c.now.now = c.now.now.Add(6 * time.Second)
	c.send()
	c.send()
	if status, attempts, free, _ := c.delivery(job.ID); status != "ACCEPTED" || attempts-free != 1 {
		t.Fatal("the throttled message went out on its first counted attempt", status, attempts, free)
	}
	if got := c.notifications(); len(got) != 0 {
		t.Fatal("a short pause raises no alert", got)
	}
}

func TestARefusedKeyLocksTheChannelUntilAProbeIsAccepted(t *testing.T) {
	c := channelSetup(t, 401)
	ids := c.queue(2)
	c.send()
	if next := c.claim(); next != nil {
		t.Fatal("nothing goes out while the key is refused")
	}
	c.now.now = c.now.now.Add(31 * time.Second)
	probe := c.claim()
	if probe == nil || probe.TestedLock == nil {
		t.Fatal("one call tests the credentials", probe)
	}
	if next := c.claim(); next != nil {
		t.Fatal("only one probe goes out per period")
	}
	if err := c.sender.Dispatch(c.ctx, *probe); err != nil {
		t.Fatal(err)
	}
	c.send()
	for _, id := range ids {
		if status, attempts, free, _ := c.delivery(id); status != "ACCEPTED" || attempts-free != 1 {
			t.Fatal("refused credentials cost the messages nothing", status, attempts, free)
		}
	}
	if got := c.notifications(); len(got) != 2 || got[0] != "channel.credentials_refused" || got[1] != "channel.credentials_accepted" {
		t.Fatal(got)
	}
}

func TestTheBreakerHoldsAFailingProviderAndProbesItBack(t *testing.T) {
	c := channelSetup(t, 503, 503, 503, 503, 503, 503)
	ids := c.queue(breakerMinCalls + 1)
	for range breakerMinCalls {
		c.send()
	}
	if next := c.claim(); next != nil {
		t.Fatal("an open breaker holds the channel")
	}
	// After the first wait one probe fails: the breaker waits twice as long and
	// the probing message keeps its attempts.
	c.now.now = c.now.now.Add(breakerFirst + time.Second)
	failed := c.send()
	if !failed.BreakerProbe {
		t.Fatal("the first call after the wait is a probe")
	}
	if _, attempts, free, _ := c.delivery(failed.ID); free != 1 || attempts-free != failed.Counted()-1 {
		t.Fatal("a failed probe costs nothing", attempts, free)
	}
	c.now.now = c.now.now.Add(breakerFirst + time.Second)
	if next := c.claim(); next != nil {
		t.Fatal("the second wait is doubled")
	}
	c.now.now = c.now.now.Add(2 * breakerFirst)
	for range breakerProbes {
		if job := c.send(); !job.BreakerProbe {
			t.Fatal("probes test the provider before the channel reopens")
		}
	}
	var state string
	c.pool.QueryRow(c.ctx, `SELECT breaker FROM provider_channels WHERE workspace_id=$1 AND provider='mock_sms'`, c.workspace).Scan(&state)
	if state != "closed" {
		t.Fatal("passing probes close the breaker", state)
	}
	if got := c.notifications(); len(got) != 1 || got[0] != "channel.circuit_open" {
		t.Fatal("three passing probes reopen the channel but do not yet clear the alert", got)
	}
	// The alert clears once most calls of the last minute went through.
	c.now.now = c.now.now.Add(alertCloseWindow)
	ids = append(ids, c.queue(breakerMinCalls)...)
	for job := c.claim(); job != nil; job = c.claim() {
		if err := c.sender.Dispatch(c.ctx, *job); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range ids {
		if status, _, _, _ := c.delivery(id); status != "ACCEPTED" {
			t.Fatal("every message went out in the end", id, status)
		}
	}
	if got := c.notifications(); len(got) != 2 || got[1] != "channel.recovered" {
		t.Fatal(got)
	}
}

func TestOnlyAnAnswerToTheCurrentLockOpensIt(t *testing.T) {
	c := channelSetup(t, 401, 200)
	c.queue(3)
	// The first call is out when the second is refused and locks the channel.
	early := c.claim()
	c.send()
	if err := c.sender.Dispatch(c.ctx, *early); err != nil {
		t.Fatal(err)
	}
	var refused *time.Time
	c.pool.QueryRow(c.ctx, `SELECT credentials_refused_at FROM provider_channels WHERE workspace_id=$1 AND provider='mock_sms'`, c.workspace).Scan(&refused)
	if refused == nil {
		t.Fatal("a 200 to a call made before the lock says nothing about the key refused since")
	}
	c.now.now = c.now.now.Add(credentialProbeEvery + time.Second)
	c.send()
	c.pool.QueryRow(c.ctx, `SELECT credentials_refused_at FROM provider_channels WHERE workspace_id=$1 AND provider='mock_sms'`, c.workspace).Scan(&refused)
	if refused != nil {
		t.Fatal("the probe's answer opens the lock it tested")
	}
}

func TestAFullRateWindowHoldsTheChannelBriefly(t *testing.T) {
	c := channelSetup(t)
	if _, err := c.pool.Exec(c.ctx, `INSERT INTO provider_channels(workspace_id,provider,rate_per_second) VALUES($1,'mock_sms',2)`, c.workspace); err != nil {
		t.Fatal(err)
	}
	c.queue(3)
	first, second := c.claim(), c.claim()
	if first == nil || second == nil {
		t.Fatal("two calls fit the window")
	}
	if third := c.claim(); third != nil {
		t.Fatal("the third waits while two are in flight")
	}
	var reason string
	c.pool.QueryRow(c.ctx, `SELECT pause_reason FROM provider_channels WHERE workspace_id=$1 AND provider='mock_sms'`, c.workspace).Scan(&reason)
	if reason != "rate_limit" {
		t.Fatal(reason)
	}
	for _, job := range []*Job{first, second} {
		if err := c.sender.Dispatch(c.ctx, *job); err != nil {
			t.Fatal(err)
		}
	}
	// Answers count for 1.1 s after they come back.
	c.now.now = c.now.now.Add(time.Second)
	if third := c.claim(); third != nil {
		t.Fatal("answered calls still count for their second")
	}
	// The full window also rested the channel briefly.
	c.now.now = c.now.now.Add(rateRest)
	if third := c.claim(); third == nil {
		t.Fatal("the window frees up")
	}
}

func TestASimulatedOutageNeverCallsTheProvider(t *testing.T) {
	c := channelSetup(t)
	if _, err := c.pool.Exec(c.ctx, `INSERT INTO provider_channels(workspace_id,provider,simulate,simulate_until) VALUES($1,'mock_sms','outage',$2)`, c.workspace, c.now.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	id := c.queue(1)[0]
	c.send()
	var response string
	c.pool.QueryRow(c.ctx, `SELECT last_error FROM delivery_jobs WHERE id=$1`, id).Scan(&response)
	if c.sms.calls != 0 || response != "Simulated: the provider is unavailable (503)" {
		t.Fatal(c.sms.calls, response)
	}
	c.now.now = c.now.now.Add(2 * time.Minute)
	c.send()
	if status, _, _, _ := c.delivery(id); status != "ACCEPTED" || c.sms.calls != 1 {
		t.Fatal("the simulation ends on time", status)
	}
}

func TestAHealthyChannelIsAdmittedWithoutWritingIt(t *testing.T) {
	c := channelSetup(t)
	if _, err := c.pool.Exec(c.ctx, `INSERT INTO provider_channels(workspace_id,provider) VALUES($1,'mock_sms')`, c.workspace); err != nil {
		t.Fatal(err)
	}
	version := func() string {
		var xmin string
		if err := c.pool.QueryRow(c.ctx, `SELECT xmin::text FROM provider_channels WHERE workspace_id=$1 AND provider='mock_sms'`, c.workspace).Scan(&xmin); err != nil {
			t.Fatal(err)
		}
		return xmin
	}
	before := version()
	c.queue(5)
	for range 5 {
		c.send()
	}
	// Every claim and every accepted answer left the row as it was, so claims
	// do not queue on its lock and its old versions do not pile up.
	if after := version(); after != before {
		t.Fatalf("channel row rewritten: %s -> %s", before, after)
	}
}

func TestConcurrentClaimsStayWithinTheRate(t *testing.T) {
	c := channelSetup(t)
	const rate = 20
	if _, err := c.pool.Exec(c.ctx, `INSERT INTO provider_channels(workspace_id,provider,rate_per_second) VALUES($1,'mock_sms',$2)`, c.workspace, rate); err != nil {
		t.Fatal(err)
	}
	c.queue(40)
	// Eight claimers race; nothing is answered, so every claim stays in flight.
	claimed := make(chan string, 64)
	errs := make(chan error, 8)
	for range 8 {
		go func() {
			for range 40 {
				job, err := c.sender.Claim(c.ctx)
				if errors.Is(err, ErrGated) {
					continue
				}
				if err != nil {
					errs <- err
					return
				}
				if job == nil {
					break
				}
				if job.WorkspaceID == c.workspace {
					claimed <- job.ID
				}
			}
			errs <- nil
		}()
	}
	for range 8 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	close(claimed)
	n := 0
	for range claimed {
		n++
	}
	if n == 0 || n > rate {
		t.Fatalf("%d calls in flight for a rate of %d", n, rate)
	}
}

func TestTheRateWindowCountsOnlyCallsThatHoldAPlace(t *testing.T) {
	c := channelSetup(t)
	ids := c.queue(1)
	now := c.now.Now()
	ago := func(d time.Duration) *time.Time { at := now.Add(-d); return &at }
	type call struct {
		status    string
		code      *int
		started   time.Duration
		completed *time.Time
		counts    bool
	}
	timeout := 408
	calls := []call{
		{"STARTED", nil, time.Second, nil, true},                              // in flight
		{"STARTED", nil, 40 * time.Second, nil, false},                        // lost long ago
		{"ACCEPTED", nil, 2 * time.Second, ago(500 * time.Millisecond), true}, // answered within its second
		{"ACCEPTED", nil, 3 * time.Second, ago(2 * time.Second), false},       // answered and done
		{"UNKNOWN", nil, 5 * time.Second, ago(4 * time.Second), true},         // may still reach the provider
		{"RETRY", &timeout, 6 * time.Second, ago(5 * time.Second), true},      // timed out, may still arrive
		{"UNKNOWN", nil, 20 * time.Second, ago(19 * time.Second), false},      // past the unanswered hold
		{"CANCELLED", nil, time.Second, nil, false},
	}
	want := 0
	for i, call := range calls {
		if _, err := c.pool.Exec(c.ctx, `INSERT INTO delivery_attempts(workspace_id,delivery_id,attempt,status,provider_code,started_at,completed_at,provider) VALUES($1,$2,$3,$4,$5,$6,$7,'mock_sms')`, c.workspace, ids[0], i+1, call.status, call.code, now.Add(-call.started), call.completed); err != nil {
			t.Fatal(err)
		}
		if call.counts {
			want++
		}
	}
	tx, err := c.pool.Begin(c.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(c.ctx) }()
	got, err := callsInFlight(c.ctx, tx, c.workspace, "mock_sms", now)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("counted %d calls, want %d", got, want)
	}
}

func TestADispatcherWaitsOutHeldChannelsInsteadOfRescanningThem(t *testing.T) {
	c := channelSetup(t)
	if _, err := c.pool.Exec(c.ctx, `INSERT INTO provider_channels(workspace_id,provider,paused_until,pause_reason) VALUES($1,'mock_sms',$2,'rate_limit')`, c.workspace, c.now.Now().Add(rateRest)); err != nil {
		t.Fatal(err)
	}
	if wait, err := c.sender.heldWait(c.ctx); err != nil || wait != 0 {
		t.Fatalf("a held channel with nothing due holds no one: %v %v", wait, err)
	}
	c.queue(3)
	if job := c.claim(); job != nil {
		t.Fatal("a held channel's messages stay where they are")
	}
	wait, err := c.sender.heldWait(c.ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Another workspace may hold its own channel; no wait passes the bound.
	if wait <= 0 || wait > maxHeldWait {
		t.Fatalf("waits %v, bound %v", wait, maxHeldWait)
	}
	c.now.now = c.now.now.Add(rateRest)
	if job := c.claim(); job == nil {
		t.Fatal("the channel takes calls again once its rest is over")
	}
}

func TestAHeldChannelLetsAnotherChannelsLaterMessageGoFirst(t *testing.T) {
	c := channelSetup(t)
	if _, err := c.pool.Exec(c.ctx, `INSERT INTO provider_channels(workspace_id,provider,paused_until,pause_reason) VALUES($1,'mock_sms',$2,'throttled')`, c.workspace, c.now.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	c.queue(3)
	var push string
	if err := c.pool.QueryRow(c.ctx, `INSERT INTO delivery_jobs(workspace_id,mode,channel,provider,idempotent_provider,logical_key,message,decision,status,due_at,created_at) VALUES($1,'TEST','push','mock_push',true,$2,'{}','{}','SCHEDULED',$3,$3) RETURNING id::text`, c.workspace, "test:"+randomID(), c.now.Now().Add(time.Millisecond)).Scan(&push); err != nil {
		t.Fatal(err)
	}
	c.now.now = c.now.now.Add(time.Second)
	job := c.claim()
	if job == nil || job.ID != push {
		t.Fatalf("claimed %+v, want the push message behind the held SMS channel", job)
	}
	if job := c.claim(); job != nil {
		t.Fatal("the held SMS messages keep waiting")
	}
}

// The head claim reads only the earliest due messages once the planner knows
// the queue. Without statistics (a new database, or one analyzed while the queue
// was empty) it may read the whole due queue until autovacuum analyzes it.
func TestTheHeadClaimWalksTheDueIndex(t *testing.T) {
	c := channelSetup(t)
	c.queue(3000)
	if _, err := c.pool.Exec(c.ctx, `ANALYZE delivery_jobs`); err != nil {
		t.Fatal(err)
	}
	var raw []byte
	if err := c.pool.QueryRow(c.ctx, "EXPLAIN (ANALYZE, FORMAT JSON) "+ClaimLiveHead, c.now.Now().Add(time.Second)).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var plan []struct{ Plan map[string]any }
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	visits, seq := 0.0, false
	var walk func(map[string]any)
	walk = func(node map[string]any) {
		if node["Relation Name"] == "delivery_jobs" {
			rows, _ := node["Actual Rows"].(float64)
			loops, _ := node["Actual Loops"].(float64)
			visits += rows * loops
			seq = seq || node["Node Type"] == "Seq Scan"
		}
		if children, ok := node["Plans"].([]any); ok {
			for _, child := range children {
				if child, ok := child.(map[string]any); ok {
					walk(child)
				}
			}
		}
	}
	walk(plan[0].Plan)
	if seq || visits > 32 {
		t.Fatalf("head claim read %v queued messages (sequential scan %v)", visits, seq)
	}
}
