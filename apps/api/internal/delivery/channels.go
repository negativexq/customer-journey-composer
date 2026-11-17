package delivery

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/observability"
)

// ErrGated says the claimed message's channel may not call its provider now:
// it is paused, locked, its breaker is open or its rate window is full. The
// message keeps its place and its attempts; the worker claims something else.
var ErrGated = errors.New("delivery channel is gated")

const (
	// A 429 holds the channel for the provider's Retry-After, within these bounds.
	minThrottle = time.Second
	maxThrottle = 30 * time.Second
	// A refused key is tested by one call this often, across all workers.
	credentialProbeEvery = 30 * time.Second
	// The breaker opens when at least half of at least breakerMinCalls calls in
	// the window failed. The figures fit a demo workspace's traffic.
	breakerWindow   = 30 * time.Second
	breakerMinCalls = 5
	breakerProbes   = 3
	breakerFirst    = 5 * time.Second
	breakerMax      = 30 * time.Second
	// A probe that was claimed and then lost to a crashed worker is handed out
	// again after this long, so the breaker cannot wait for it forever.
	probeLease = time.Minute
	// A call counts against the rate window while in flight and this long after
	// its answer: the provider's second plus room for a step of its clock.
	rateHold = 1100 * time.Millisecond
	// A channel's alert closes once most calls in this window went through.
	alertCloseWindow = time.Minute
	// A call with no answer may still reach the provider later, so it counts
	// for the send timeout plus 2 s, then rateHold more, from when it went out.
	unansweredHold = 10*time.Second + 2*time.Second + rateHold
	// When the window is full the channel rests this long before asking again.
	rateRest = 250 * time.Millisecond
	// Calls that cost no attempt are bounded, so a channel stuck on 429 or 401
	// eventually fails its messages instead of holding them forever.
	maxFreeAttempts = 50
	// A claim admitted without the channel lock is not yet visible to other
	// claims until it commits. Below the rate by this many calls, that cannot
	// overrun the window; closer to the rate, claims take the lock and count
	// exactly. It covers the dispatchers of several workers at once.
	rateHeadroom = 16
)

// openChannel keeps a claim off messages whose channel is held. $2 lists the
// held channels as "workspace/provider", read by heldChannels just before. A
// plain filter keeps the claim walking the due index in order and stopping at
// the first message it may take; a join against provider_channels made the
// planner expect almost nothing to pass, so it fetched and sorted every due
// message on each claim.
const openChannel = `NOT (d.workspace_id::text||'/'||d.provider = ANY($2::text[]))`

// ClaimLive is the claim of the next due live message, for diagnostics that
// explain it. It reads $1 as now and $2 as HeldChannels.
const ClaimLive = `SELECT d.id::text FROM delivery_jobs d JOIN customers c ON c.workspace_id=d.workspace_id AND c.customer_id=d.customer_id WHERE d.status='SCHEDULED' AND d.mode='LIVE' AND d.due_at<=$1 AND (SELECT j.status IN ('Active','Sunset') FROM journey_executions e JOIN journeys j ON j.id=e.journey_id WHERE e.id=d.execution_id) AND ` + openChannel + ` ORDER BY d.due_at,d.id LIMIT 1 FOR UPDATE OF c SKIP LOCKED`

const claimTest = `SELECT d.id::text FROM delivery_jobs d WHERE d.status='SCHEDULED' AND d.mode='TEST' AND d.due_at<=$1 AND ` + openChannel + ` ORDER BY d.due_at,d.id LIMIT 1 FOR UPDATE SKIP LOCKED`

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// HeldChannels lists the channels that may not call their provider at now, as
// "workspace/provider": paused, locked by a refused key, behind an open
// breaker or out of breaker probes.
func HeldChannels(ctx context.Context, db querier, now time.Time) ([]string, error) {
	rows, err := db.Query(ctx, `SELECT g.workspace_id::text||'/'||g.provider FROM provider_channels g WHERE `+heldGate+` IS NOT NULL`, now)
	if err != nil {
		return nil, err
	}
	held, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if held == nil {
		// An empty list, not NULL: NULL would hold every channel.
		held = []string{}
	}
	return held, err
}

// heldGate is when a held channel may be asked again: the latest of the
// reasons that hold it. It reads $1 as now and g as the channel row.
const heldGate = `GREATEST(CASE WHEN g.paused_until>$1 THEN g.paused_until END,CASE WHEN g.credentials_refused_at IS NOT NULL AND g.next_probe_at>$1 THEN g.next_probe_at END,CASE WHEN g.breaker='open' AND g.breaker_until>$1 THEN g.breaker_until END,CASE WHEN g.breaker='probing' AND g.probes_left=0 AND g.updated_at>$1::timestamptz-interval '1 minute' THEN g.updated_at+interval '1 minute' END)`

// maxHeldWait bounds how long a dispatcher sleeps on held channels, so work
// for a channel that is not held waits at most this long behind them.
const maxHeldWait = time.Second

// heldWait says how long a dispatcher that found nothing to claim should wait
// because the messages that are due all sit behind held channels: until the
// first of those channels may be asked again, at most maxHeldWait. It is zero
// when nothing due is held, and the dispatcher then waits for new work as usual.
// Without it, every wake-up would rescan the held messages only to skip them.
func (s *Service) heldWait(ctx context.Context) (time.Duration, error) {
	now := s.Clock.Now()
	var until *time.Time
	err := s.Pool.QueryRow(ctx, `SELECT min(`+heldGate+`) FROM provider_channels g WHERE `+heldGate+` IS NOT NULL AND EXISTS (SELECT 1 FROM delivery_jobs d WHERE d.workspace_id=g.workspace_id AND d.provider=g.provider AND d.status='SCHEDULED' AND d.due_at<=$1)`, now).Scan(&until)
	if err != nil || until == nil {
		return 0, err
	}
	return min(max(until.Sub(now), 10*time.Millisecond), maxHeldWait), nil
}

// FaultOutcome is what a provider answers while the workspace simulates a fault.
func FaultOutcome(fault string) (Outcome, bool) {
	switch fault {
	case "throttle":
		return Outcome{Status: "RETRY", Code: 429, RetryAfter: 5 * time.Second, Signal: "throttled", Response: "Simulated: the provider asked to slow down (429, Retry-After 5 s)"}, true
	case "outage":
		return Outcome{Status: "RETRY", Code: 503, Response: "Simulated: the provider is unavailable (503)"}, true
	case "credentials":
		return Outcome{Status: "RETRY", Code: 401, Signal: "credentials", Response: "Simulated: the provider refused the credentials (401)"}, true
	}
	return Outcome{}, false
}

// admit decides under the channel's row lock whether this call may go out, and
// whether it is one of the breaker's probes or the credential probe.
func admit(ctx context.Context, tx pgx.Tx, workspace, provider string, now time.Time) (breakerProbe bool, testedLock *time.Time, fault string, err error) {
	var rate, probesLeft int
	var paused, refused, nextProbe, breakerUntil, simulateUntil *time.Time
	var state string
	var simulate *string
	var updated time.Time
	const channel = `SELECT rate_per_second,paused_until,credentials_refused_at,next_probe_at,breaker,breaker_until,probes_left,simulate,simulate_until,updated_at FROM provider_channels WHERE workspace_id=$1 AND provider=$2`
	scan := func(row pgx.Row) error {
		return row.Scan(&rate, &paused, &refused, &nextProbe, &state, &breakerUntil, &probesLeft, &simulate, &simulateUntil, &updated)
	}
	// Most claims find a healthy channel well under its rate. They read it
	// without the lock and write nothing, so dispatchers do not queue on one
	// row and the row does not pile up versions that every claim must step
	// over. Anything that could change the channel takes the locked path.
	err = scan(tx.QueryRow(ctx, channel, workspace, provider))
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		if _, err = tx.Exec(ctx, `INSERT INTO provider_channels(workspace_id,provider) VALUES($1,$2) ON CONFLICT DO NOTHING`, workspace, provider); err != nil {
			return
		}
	case err != nil:
		return
	case paused != nil && paused.After(now):
		return false, nil, "", ErrGated
	case state == "closed" && refused == nil && simulate == nil:
		var inFlight int
		if inFlight, err = callsInFlight(ctx, tx, workspace, provider, now); err != nil {
			return
		}
		if inFlight+rateHeadroom < rate {
			return false, nil, "", nil
		}
	}
	if err = scan(tx.QueryRow(ctx, channel+` FOR UPDATE`, workspace, provider)); err != nil {
		return
	}
	if simulate != nil && simulateUntil != nil && simulateUntil.After(now) {
		fault = *simulate
	}
	if paused != nil && paused.After(now) {
		return false, nil, "", ErrGated
	}
	if refused != nil && nextProbe != nil && nextProbe.After(now) {
		return false, nil, "", ErrGated
	}
	switch state {
	case "open":
		if breakerUntil != nil && breakerUntil.After(now) {
			return false, nil, "", ErrGated
		}
		state, probesLeft = "probing", breakerProbes
	case "probing":
		if probesLeft == 0 {
			if now.Sub(updated) < probeLease {
				return false, nil, "", ErrGated
			}
			probesLeft = breakerProbes
		}
	}
	var inFlight int
	if inFlight, err = callsInFlight(ctx, tx, workspace, provider, now); err != nil {
		return
	}
	if inFlight >= rate {
		_, err = tx.Exec(ctx, `UPDATE provider_channels SET paused_until=$3,pause_reason='rate_limit',updated_at=$4 WHERE workspace_id=$1 AND provider=$2`, workspace, provider, now.Add(rateRest), now)
		if err == nil {
			err = ErrGated
		}
		return false, nil, "", err
	}
	if state == "probing" {
		breakerProbe = true
		probesLeft--
	}
	// The probe carries the lock it tests, so only an answer to that lock opens it.
	testedLock = refused
	var next *time.Time
	if refused != nil {
		at := now.Add(credentialProbeEvery)
		next = &at
	}
	_, err = tx.Exec(ctx, `UPDATE provider_channels SET breaker=$3,probes_left=$4,next_probe_at=COALESCE($5,next_probe_at),simulate=CASE WHEN simulate_until<=$6 THEN NULL ELSE simulate END,simulate_until=CASE WHEN simulate_until<=$6 THEN NULL ELSE simulate_until END,updated_at=$6 WHERE workspace_id=$1 AND provider=$2`, workspace, provider, state, probesLeft, next, now)
	return
}

// callsInFlight counts the calls that hold a place in the channel's rate window:
// those not yet answered, those answered within rateHold, and those whose answer
// was unknown or a timeout, which may still reach the provider for
// unansweredHold. The three parts are disjoint on completed_at and each reads
// its own index, so the count touches only the calls that count.
func callsInFlight(ctx context.Context, tx pgx.Tx, workspace, provider string, now time.Time) (n int, err error) {
	err = tx.QueryRow(ctx, `SELECT
  (SELECT count(*) FROM delivery_attempts WHERE workspace_id=$1 AND provider=$2 AND completed_at IS NULL AND started_at>$3::timestamptz-interval '30 seconds' AND status<>'CANCELLED')
 +(SELECT count(*) FROM delivery_attempts WHERE workspace_id=$1 AND provider=$2 AND completed_at>$4 AND started_at>$3::timestamptz-interval '30 seconds' AND status<>'CANCELLED')
 +(SELECT count(*) FROM delivery_attempts WHERE workspace_id=$1 AND provider=$2 AND (status='UNKNOWN' OR provider_code=408) AND completed_at<=$4 AND started_at>$5 AND status<>'CANCELLED')`, workspace, provider, now, now.Add(-rateHold), now.Add(-unansweredHold)).Scan(&n)
	return
}

func gateLabel(breakerProbe, credentialProbe bool) *string {
	var gate string
	switch {
	case breakerProbe && credentialProbe:
		gate = "both"
	case breakerProbe:
		gate = "probe"
	case credentialProbe:
		gate = "credential_probe"
	default:
		return nil
	}
	return &gate
}

// answer is what a provider's answer says about the channel.
type answer int

const (
	answerAccepted answer = iota
	answerRejected
	answerFailed
	answerThrottled
	answerCredentials
)

func classify(out Outcome) answer {
	switch {
	case out.Signal == "throttled":
		return answerThrottled
	case out.Signal == "credentials":
		return answerCredentials
	case out.Status == "ACCEPTED":
		return answerAccepted
	case out.Status == "RETRY" || out.Status == "UNKNOWN":
		return answerFailed
	}
	return answerRejected
}

// free reports whether this call should cost the message no attempt: it told
// us about the channel, not about the message.
func (j Job) free(a answer) bool {
	if j.Free >= maxFreeAttempts {
		return false
	}
	return a == answerThrottled || a == answerCredentials || a == answerFailed && j.BreakerProbe
}

// retryAfter is how long the message waits before it may be claimed again.
func retryAfter(a answer, out Outcome, counted int) time.Duration {
	switch a {
	case answerThrottled:
		return min(max(out.RetryAfter, minThrottle), maxThrottle)
	case answerCredentials:
		return credentialProbeEvery
	}
	base := time.Duration(RetryDelay(counted)) * time.Second
	// Up to half again at random, so messages that failed together come back apart.
	return base + time.Duration(rand.Int64N(int64(base/2)+1))
}

// observe records what the answer says about the channel and raises or clears
// its alerts. It runs in the transaction that records the attempt.
func observe(ctx context.Context, tx pgx.Tx, job Job, channel string, a answer, out Outcome, at time.Time) error {
	var refused, failing *time.Time
	var state string
	var wait, passed int
	const channelRow = `SELECT credentials_refused_at,breaker,breaker_wait_seconds,probes_passed,failing_since FROM provider_channels WHERE workspace_id=$1 AND provider=$2`
	read := func(lock string) error {
		return tx.QueryRow(ctx, channelRow+lock, job.WorkspaceID, job.Provider).Scan(&refused, &state, &wait, &passed, &failing)
	}
	if err := read(""); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	// Most answers leave a healthy channel as it is: read it without the lock,
	// and lock only when this answer may change it. A failure on a closed
	// breaker counts recent calls first; only enough failures to open it need
	// the lock, and a concurrent failure that is not yet committed can open it
	// at most one call later.
	if !job.BreakerProbe && job.TestedLock == nil && refused == nil && state == "closed" {
		switch a {
		case answerAccepted, answerRejected:
			if failing == nil {
				return nil
			}
		case answerFailed:
			calls, failed, err := recentCalls(ctx, tx, job, at.Add(-breakerWindow))
			if err != nil || calls < breakerMinCalls || failed*2 < calls {
				return err
			}
		}
	}
	if err := read(" FOR UPDATE"); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	update := func(set string, args ...any) error {
		_, err := tx.Exec(ctx, `UPDATE provider_channels SET `+set+`,updated_at=$3 WHERE workspace_id=$1 AND provider=$2`, append([]any{job.WorkspaceID, job.Provider, at}, args...)...)
		return err
	}
	label := channelLabel(channel)
	// A probe whose answer said nothing about the outage is handed back.
	giveBack := func() error {
		if job.BreakerProbe && state == "probing" {
			return update(`probes_left=probes_left+1`)
		}
		return nil
	}
	switch a {
	case answerThrottled:
		observability.ObserveChannel(job.Provider, "throttled")
		if err := update(`paused_until=GREATEST(COALESCE(paused_until,$4),$4),pause_reason='throttled'`, at.Add(retryAfter(a, out, 0))); err != nil {
			return err
		}
		return giveBack()
	case answerCredentials:
		if refused == nil {
			observability.ObserveChannel(job.Provider, "credentials_locked")
			if err := update(`credentials_refused_at=$3,next_probe_at=$4`, at.Add(credentialProbeEvery)); err != nil {
				return err
			}
			if err := notify(ctx, tx, job.WorkspaceID, "channel.credentials_refused", "critical", label+" provider refused the credentials",
				fmt.Sprintf("Nothing is sent on %s until a test call every 30 seconds is accepted. Messages keep their attempts.", label), at); err != nil {
				return err
			}
		}
		return giveBack()
	case answerAccepted, answerRejected:
		// A late answer to a call made before the lock, or to an older lock's
		// probe, says nothing about the key refused now.
		if refused != nil && a == answerAccepted && job.TestedLock != nil && job.TestedLock.Equal(*refused) {
			observability.ObserveChannel(job.Provider, "credentials_unlocked")
			if err := update(`credentials_refused_at=NULL,next_probe_at=NULL`); err != nil {
				return err
			}
			if err := notify(ctx, tx, job.WorkspaceID, "channel.credentials_accepted", "success", label+" provider accepted the credentials again", "Sending resumed on "+label+".", at); err != nil {
				return err
			}
		}
		if job.BreakerProbe && state == "probing" {
			if passed+1 < breakerProbes {
				return update(`probes_passed=probes_passed+1`)
			}
			state = "closed"
			if err := update(`breaker='closed',breaker_until=NULL,breaker_wait_seconds=0,probes_left=0,probes_passed=0`); err != nil {
				return err
			}
		}
		if failing == nil || state != "closed" {
			return nil
		}
		// The alert closes only once most calls of the last minute went through:
		// three passing probes against a provider failing 70% of the time would
		// otherwise ring the bell on every cycle.
		calls, failed, err := recentCalls(ctx, tx, job, at.Add(-alertCloseWindow))
		if err != nil || calls < breakerMinCalls || failed*2 >= calls {
			return err
		}
		observability.ObserveChannel(job.Provider, "alert_closed")
		if err := update(`failing_since=NULL`); err != nil {
			return err
		}
		return notify(ctx, tx, job.WorkspaceID, "channel.recovered", "success", label+" provider is answering again",
			fmt.Sprintf("%d of the last %d calls on %s went through.", calls-failed, calls, label), at)
	}
	// A failure: a failed probe reopens the breaker for twice as long.
	if job.BreakerProbe && state == "probing" {
		next := min(max(time.Duration(wait)*time.Second*2, breakerFirst), breakerMax)
		return update(`breaker='open',breaker_until=$4,breaker_wait_seconds=$5,probes_left=0,probes_passed=0`, at.Add(next), int(next/time.Second))
	}
	if state != "closed" {
		return nil
	}
	// This attempt is already recorded in the same transaction.
	calls, failed, err := recentCalls(ctx, tx, job, at.Add(-breakerWindow))
	if err != nil || calls < breakerMinCalls || failed*2 < calls {
		return err
	}
	observability.ObserveChannel(job.Provider, "circuit_opened")
	if err := update(`breaker='open',breaker_until=$4,breaker_wait_seconds=$5,probes_left=0,probes_passed=0,failing_since=COALESCE(failing_since,$3)`, at.Add(breakerFirst), int(breakerFirst/time.Second)); err != nil {
		return err
	}
	if failing != nil {
		// The alert from the last time it opened is still open.
		return nil
	}
	return notify(ctx, tx, job.WorkspaceID, "channel.circuit_open", "critical", label+" provider is failing",
		fmt.Sprintf("%d of the last %d calls failed. %s waits without using messages' attempts and tries %d test calls in %d seconds.", failed, calls, label, breakerProbes, int(breakerFirst/time.Second)), at)
}

// recentCalls counts the calls that reached the provider since a time, and
// how many of them failed; 429s and 401s say nothing about failure.
func recentCalls(ctx context.Context, tx pgx.Tx, job Job, since time.Time) (calls, failed int, err error) {
	err = tx.QueryRow(ctx, `SELECT count(*),count(*) FILTER (WHERE status IN ('RETRY','UNKNOWN')) FROM delivery_attempts WHERE workspace_id=$1 AND provider=$2 AND started_at>$3 AND signal IS NULL AND status IN ('ACCEPTED','RETRY','UNKNOWN','FAILED')`, job.WorkspaceID, job.Provider, since).Scan(&calls, &failed)
	return
}

func channelLabel(channel string) string {
	switch channel {
	case "sms":
		return "SMS"
	case "push":
		return "Push"
	}
	return "Email"
}

func notify(ctx context.Context, tx pgx.Tx, workspace, kind, severity, title, body string, at time.Time) error {
	_, err := tx.Exec(ctx, `INSERT INTO notifications(workspace_id,kind,severity,title,body,created_at) VALUES($1,$2,$3,$4,$5,$6)`, workspace, kind, severity, title, body, at)
	return err
}
