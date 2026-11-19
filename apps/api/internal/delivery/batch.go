package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/observability"
)

// claimHeadBatch claims up to $2 of the earliest due live messages when no
// channel is held, locking their customers like ClaimLiveHead. It reads $1 as
// now.
const claimHeadBatch = `SELECT d.id::text,d.workspace_id::text,d.customer_id,d.provider FROM (SELECT id,workspace_id,customer_id,execution_id,provider,mode,due_at FROM delivery_jobs WHERE status='SCHEDULED' AND due_at<=$1 ORDER BY due_at,id LIMIT ` + claimHeadSQL + `) d JOIN customers c ON c.workspace_id=d.workspace_id AND c.customer_id=d.customer_id WHERE d.mode='LIVE' AND (SELECT j.status IN ('Active','Sunset') FROM journey_executions e JOIN journeys j ON j.id=e.journey_id WHERE e.id=d.execution_id) ORDER BY d.due_at,d.id LIMIT $2 FOR UPDATE OF c SKIP LOCKED`

// ClaimBatch claims up to n due live messages in one transaction. It claims
// several only on the path where every message's channel is healthy and well
// under its rate; anything else (held channels, breaker probes, simulated
// faults, a channel near its rate, test messages) goes through Claim, one
// message at a time, exactly as before.
func (s *Service) ClaimBatch(ctx context.Context, n int) ([]Job, error) {
	if n > 1 {
		jobs, ok, err := s.claimHealthy(ctx, n)
		if err != nil || ok {
			return jobs, err
		}
	}
	job, err := s.Claim(ctx)
	if job == nil || err != nil {
		return nil, err
	}
	return []Job{*job}, nil
}

// claimHealthy is ClaimBatch's batched path. ok is false when the claim must
// be made by Claim instead; nothing was changed then.
func (s *Service) claimHealthy(ctx context.Context, n int) (jobs []Job, ok bool, err error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	now := s.Clock.Now()
	held, err := HeldChannels(ctx, tx, now)
	if err != nil || len(held) > 0 {
		return nil, false, err
	}
	rows, err := tx.Query(ctx, claimHeadBatch, now, n)
	if err != nil {
		return nil, false, err
	}
	type head struct{ id, workspace, customer, provider string }
	heads, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (head, error) {
		var h head
		return h, row.Scan(&h.id, &h.workspace, &h.customer, &h.provider)
	})
	if err != nil || len(heads) < 2 {
		return nil, false, err
	}
	// One message per customer, as when each claim locked its own customer.
	seen := map[[2]string]bool{}
	perChannel := map[[2]string]int{}
	ids := []string{}
	for _, h := range heads {
		key := [2]string{h.workspace, h.customer}
		if seen[key] {
			continue
		}
		seen[key] = true
		ids = append(ids, h.id)
		perChannel[[2]string{h.workspace, h.provider}]++
	}
	for channel, count := range perChannel {
		healthy, err := admitMany(ctx, tx, channel[0], channel[1], now, count)
		if err != nil || !healthy {
			return nil, false, err
		}
	}
	rows, err = tx.Query(ctx, `UPDATE delivery_jobs SET status='CLAIMED',claim_token=gen_random_uuid(),lease_until=$2::timestamptz+interval '30 seconds',attempts=attempts+1 WHERE id=ANY($1::uuid[]) AND status='SCHEDULED' RETURNING id::text,workspace_id::text,COALESCE(customer_id,''),COALESCE(execution_id::text,''),claim_token::text,provider,logical_key,mode,attempts,free_attempts,message,COALESCE((SELECT trace_parent FROM journey_executions e WHERE e.id=delivery_jobs.execution_id),'')`, ids, now)
	if err != nil {
		return nil, false, err
	}
	jobs, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (Job, error) {
		var job Job
		var raw []byte
		if err := row.Scan(&job.ID, &job.WorkspaceID, &job.CustomerID, &job.ExecutionID, &job.Token, &job.Provider, &job.LogicalKey, &job.Mode, &job.Attempt, &job.Free, &raw, &job.TraceParent); err != nil {
			return job, err
		}
		return job, json.Unmarshal(raw, &job.Message)
	})
	if err != nil {
		return nil, false, err
	}
	var workspaces, deliveries, providers []string
	var attempts []int
	for _, job := range jobs {
		workspaces, deliveries, providers, attempts = append(workspaces, job.WorkspaceID), append(deliveries, job.ID), append(providers, job.Provider), append(attempts, job.Attempt)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO delivery_attempts(workspace_id,delivery_id,attempt,status,started_at,provider,gate) SELECT w,d,a,'STARTED',$5,p,NULL FROM unnest($1::uuid[],$2::uuid[],$3::int[],$4::text[]) AS x(w,d,a,p)`, workspaces, deliveries, attempts, providers, now); err != nil {
		return nil, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, err
	}
	return jobs, true, nil
}

// admitMany admits count calls on a channel without its row lock, which is
// only safe on a healthy channel well under its rate. Each dispatcher may hold
// count uncommitted calls, so the headroom that covers one call per dispatcher
// is scaled by count.
func admitMany(ctx context.Context, tx pgx.Tx, workspace, provider string, now time.Time, count int) (bool, error) {
	var rate int
	var paused, refused *time.Time
	var state string
	var simulate *string
	err := tx.QueryRow(ctx, `SELECT rate_per_second,paused_until,credentials_refused_at,breaker,simulate FROM provider_channels WHERE workspace_id=$1 AND provider=$2`, workspace, provider).Scan(&rate, &paused, &refused, &state, &simulate)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil || state != "closed" || refused != nil || simulate != nil || paused != nil && paused.After(now) {
		return false, err
	}
	inFlight, err := callsInFlight(ctx, tx, workspace, provider, now)
	return inFlight+rateHeadroom*count < rate, err
}

// DispatchBatch sends claimed messages. The live messages pass the dispatch
// fence together: their customers are locked in order and their journeys
// share-locked, each message is checked as Dispatch checks it, and those that
// may go out become DISPATCHING in the same transaction, so a consent change
// or exit that commits first stops the send, and one that commits later does
// not. Provider calls then run concurrently and their answers are recorded in
// one transaction. Test messages go through Dispatch one at a time.
func (s *Service) DispatchBatch(ctx context.Context, jobs []Job) error {
	live := make([]Job, 0, len(jobs))
	for _, job := range jobs {
		if job.Mode == "LIVE" {
			live = append(live, job)
		} else if err := s.Dispatch(ctx, job); err != nil {
			return err
		}
	}
	if len(live) == 1 {
		return s.Dispatch(ctx, live[0])
	}
	if len(live) == 0 {
		return nil
	}
	ctx, finish := observability.TrackOperation(ctx, "delivery_dispatch_batch")
	var err error
	defer func() {
		finish(err)
		if err != nil && !errors.Is(err, context.Canceled) {
			s.Log.ErrorContext(ctx, "delivery batch dispatch failed", "messages", len(live), "error_class", observability.ErrorClass(err))
		}
	}()
	var sending []Job
	if sending, err = s.fence(ctx, live); err != nil || len(sending) == 0 {
		return err
	}
	type answered struct {
		out    Outcome
		called bool
	}
	answers := make([]answered, len(sending))
	var calls sync.WaitGroup
	for i, job := range sending {
		calls.Go(func() {
			out, called := s.call(ctx, job)
			answers[i] = answered{out, called}
		})
	}
	calls.Wait()
	results := make([]settled, len(sending))
	called := make([]bool, len(sending))
	for i, job := range sending {
		results[i], called[i] = settle(ctx, job, answers[i].out), answers[i].called
	}
	err = s.record(sending, results, called)
	return err
}

// fence locks and checks live claimed messages and marks those that may go out
// DISPATCHING, returning them.
func (s *Service) fence(ctx context.Context, jobs []Job) ([]Job, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	now := s.Clock.Now()
	var workspaces, customers, executions []string
	for _, job := range jobs {
		workspaces, customers, executions = append(workspaces, job.WorkspaceID), append(customers, job.CustomerID), append(executions, job.ExecutionID)
	}
	// Customers in a fixed order, so concurrent batches cannot deadlock.
	rows, err := tx.Query(ctx, `SELECT workspace_id::text,customer_id,consents,email,phone,push_token FROM customers WHERE (workspace_id,customer_id) IN (SELECT * FROM unnest($1::uuid[],$2::text[])) ORDER BY workspace_id,customer_id FOR UPDATE`, workspaces, customers)
	if err != nil {
		return nil, err
	}
	people := map[[2]string]recipient{}
	for rows.Next() {
		var workspace, customer string
		var r recipient
		if err := rows.Scan(&workspace, &customer, &r.consents, &r.email, &r.phone, &r.pushToken); err != nil {
			rows.Close()
			return nil, err
		}
		people[[2]string{workspace, customer}] = r
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows, err = tx.Query(ctx, `SELECT e.id::text,e.state,j.status FROM journey_executions e JOIN journeys j ON j.id=e.journey_id WHERE e.id=ANY($1::uuid[]) FOR SHARE OF j`, executions)
	if err != nil {
		return nil, err
	}
	type lifecycle struct{ state, status string }
	lifecycles := map[string]lifecycle{}
	for rows.Next() {
		var id string
		var l lifecycle
		if err := rows.Scan(&id, &l.state, &l.status); err != nil {
			rows.Close()
			return nil, err
		}
		lifecycles[id] = l
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var ids, tokens []string
	byID := map[string]Job{}
	for _, job := range jobs {
		person, found := people[[2]string{job.WorkspaceID, job.CustomerID}]
		l, known := lifecycles[job.ExecutionID]
		if !found || !known {
			// Dispatch fails such a message's fence; recovery takes it back.
			continue
		}
		blocked, held := liveDecision(job, person, l.state, l.status)
		switch {
		case held:
			err = handBack(ctx, tx, job)
		case blocked != "":
			err = block(ctx, tx, job, blocked, now)
		default:
			ids, tokens = append(ids, job.ID), append(tokens, job.Token)
			byID[job.ID] = job
		}
		if err != nil {
			return nil, err
		}
	}
	var sending []Job
	if len(ids) > 0 {
		rows, err = tx.Query(ctx, `UPDATE delivery_jobs d SET status='DISPATCHING' FROM unnest($1::uuid[],$2::uuid[]) AS x(id,token) WHERE d.id=x.id AND d.claim_token=x.token AND d.status='CLAIMED' AND d.lease_until>$3 RETURNING d.id::text`, ids, tokens, now)
		if err != nil {
			return nil, err
		}
		marked, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return nil, err
		}
		for _, id := range marked {
			sending = append(sending, byID[id])
		}
	}
	return sending, tx.Commit(ctx)
}

// record writes the provider answers of a batch in one transaction.
func (s *Service) record(jobs []Job, results []settled, called []bool) error {
	completed, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := s.Pool.Begin(completed)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	finished := s.Clock.Now()
	n := len(jobs)
	ids, tokens, statuses, responses := make([]string, n), make([]string, n), make([]string, n), make([]string, n)
	dues := make([]time.Time, n)
	frees := make([]int, n)
	for i, job := range jobs {
		r := results[i]
		ids[i], tokens[i], statuses[i], responses[i], dues[i] = job.ID, job.Token, r.status, r.out.Response, finished.Add(r.delay)
		if r.free {
			frees[i] = 1
		}
	}
	rows, err := tx.Query(completed, `UPDATE delivery_jobs d SET status=x.status,due_at=x.due,accepted_at=CASE WHEN x.status='ACCEPTED' THEN $7::timestamptz ELSE NULL END,lease_until=NULL,claim_token=NULL,last_error=CASE WHEN x.status='ACCEPTED' THEN NULL ELSE x.response END,free_attempts=d.free_attempts+x.free FROM unnest($1::uuid[],$2::uuid[],$3::text[],$4::timestamptz[],$5::text[],$6::int[]) AS x(id,token,status,due,response,free) WHERE d.id=x.id AND d.claim_token=x.token AND d.status='DISPATCHING' RETURNING d.id::text`, ids, tokens, statuses, dues, responses, frees, finished)
	if err != nil {
		return err
	}
	settledIDs, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	kept := map[string]bool{}
	for _, id := range settledIDs {
		kept[id] = true
	}
	var deliveries, outcomes, providerIDs, answersText []string
	var attempts, codes []int
	var signals []*string
	for i, job := range jobs {
		if !kept[job.ID] {
			continue
		}
		out := results[i].out
		var signal *string
		if out.Signal != "" {
			signal = &out.Signal
		}
		deliveries, attempts, outcomes, codes, providerIDs, answersText, signals = append(deliveries, job.ID), append(attempts, job.Attempt), append(outcomes, out.Status), append(codes, out.Code), append(providerIDs, out.ProviderID), append(answersText, out.Response), append(signals, signal)
	}
	if len(deliveries) > 0 {
		if _, err := tx.Exec(completed, `UPDATE delivery_attempts a SET status=x.status,provider_code=x.code,provider_id=x.provider_id,response=x.response,completed_at=$7,signal=x.signal FROM unnest($1::uuid[],$2::int[],$3::text[],$4::int[],$5::text[],$6::text[],$8::text[]) AS x(delivery,attempt,status,code,provider_id,response,signal) WHERE a.delivery_id=x.delivery AND a.attempt=x.attempt AND a.status='STARTED'`, deliveries, attempts, outcomes, codes, providerIDs, answersText, finished, signals); err != nil {
			return err
		}
	}
	// A healthy channel stays as it is after accepted or rejected answers; read
	// it once per channel, and let observe handle anything else per answer.
	quiet := map[[2]string]bool{}
	for i, job := range jobs {
		if !kept[job.ID] || !called[i] {
			continue
		}
		channel := [2]string{job.WorkspaceID, job.Provider}
		verdict := results[i].verdict
		simple := !job.BreakerProbe && job.TestedLock == nil && (verdict == answerAccepted || verdict == answerRejected)
		if simple {
			healthy, checked := quiet[channel]
			if !checked {
				healthy, err = channelQuiet(completed, tx, job.WorkspaceID, job.Provider)
				if err != nil {
					return err
				}
				quiet[channel] = healthy
			}
			if healthy {
				continue
			}
		}
		if err := observe(completed, tx, job, job.Message.Channel, verdict, results[i].out, finished); err != nil {
			return err
		}
	}
	if err := tx.Commit(completed); err != nil {
		return err
	}
	for i, job := range jobs {
		if !kept[job.ID] || results[i].status != "ACCEPTED" {
			continue
		}
		// The mock providers report delivery the way a real one would by webhook.
		if _, mock := s.Providers[job.Provider].(*Mock); mock {
			kind := simulatedReport(job.Message.Channel, job.Message.To)
			if _, err := RecordEngagement(completed, s.Pool, job.WorkspaceID, job.ID, kind, "simulated", nil, finished, finished); err != nil {
				s.Log.WarnContext(completed, "simulated delivery report failed", "delivery_id", job.ID, "error_class", observability.ErrorClass(err))
			}
		}
	}
	return nil
}

// channelQuiet is whether observe would leave the channel as it is for an
// accepted or rejected answer: closed breaker, no refused key, not failing.
func channelQuiet(ctx context.Context, tx pgx.Tx, workspace, provider string) (bool, error) {
	var refused, failing *time.Time
	var state string
	err := tx.QueryRow(ctx, `SELECT credentials_refused_at,breaker,failing_since FROM provider_channels WHERE workspace_id=$1 AND provider=$2`, workspace, provider).Scan(&refused, &state, &failing)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}
	return err == nil && refused == nil && state == "closed" && failing == nil, err
}
