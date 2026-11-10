package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/mail"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ofk/customer-journey-composer/apps/api/internal/messaging"
	engine "github.com/ofk/customer-journey-composer/apps/api/internal/runtime"
	"github.com/ofk/customer-journey-composer/apps/api/internal/workflow"
)

type Service struct {
	Pool      *pgxpool.Pool
	Clock     engine.Clock
	Providers map[string]Provider
	Log       *slog.Logger
}
type Eligibility struct {
	Eligible     bool   `json:"eligible"`
	Consent      bool   `json:"consent"`
	FrequencyCap bool   `json:"frequency_cap"`
	Reason       string `json:"reason"`
}
type Job struct {
	ID, WorkspaceID, CustomerID, ExecutionID, Token, Provider, LogicalKey, Mode string
	Attempt                                                                     int
	Message                                                                     messaging.Message
}

var e164 = regexp.MustCompile(`^\+[1-9][0-9]{6,14}$`)

func New(pool *pgxpool.Pool, clock engine.Clock) *Service {
	if clock == nil {
		clock = engine.RealClock{}
	}
	return &Service{Pool: pool, Clock: clock, Providers: map[string]Provider{"smtp": SMTPFromEnv(), "mock_sms": &Mock{}, "mock_email": &Mock{}}, Log: slog.Default()}
}
func (s *Service) provider(channel string) (string, Provider) {
	name := "smtp"
	if channel == "sms" {
		name = "mock_sms"
	}
	return name, s.Providers[name]
}
func validRecipient(channel, to string) bool {
	if channel == "sms" {
		return e164.MatchString(to)
	}
	a, err := mail.ParseAddress(to)
	return err == nil && a.Address == to
}
func (s *Service) Schedule(ctx context.Context) (int, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	now := s.Clock.Now()
	rows, err := tx.Query(ctx, `SELECT o.id::text,o.workspace_id::text,o.execution_id::text,o.node_id,o.channel,o.payload,e.customer_id,e.state,c.email,c.phone,c.consents,COALESCE(p.frequency_limit,3),COALESCE(p.frequency_window_seconds,86400) FROM action_outbox o JOIN journey_executions e ON e.id=o.execution_id JOIN customers c ON c.workspace_id=e.workspace_id AND c.customer_id=e.customer_id JOIN journeys j ON j.id=e.journey_id LEFT JOIN workspace_settings p ON p.workspace_id=o.workspace_id WHERE o.state='PENDING' AND j.status IN ('Active','Sunset') ORDER BY o.created_at,o.id LIMIT 32 FOR UPDATE OF c SKIP LOCKED`)
	if err != nil {
		return 0, err
	}
	type pending struct {
		id, workspace, execution, node, channel, customer, state string
		raw, consents                                            []byte
		email, phone                                             *string
		limit, window                                            int
	}
	items := []pending{}
	for rows.Next() {
		var p pending
		if err := rows.Scan(&p.id, &p.workspace, &p.execution, &p.node, &p.channel, &p.raw, &p.customer, &p.state, &p.email, &p.phone, &p.consents, &p.limit, &p.window); err != nil {
			rows.Close()
			return 0, err
		}
		items = append(items, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	for _, p := range items {
		var lifecycle string
		if err := tx.QueryRow(ctx, `SELECT e.state,j.status FROM journeys j JOIN journey_executions e ON e.journey_id=j.id WHERE e.id=$1 FOR SHARE OF j`, p.execution).Scan(&p.state, &lifecycle); err != nil {
			return 0, err
		}
		if lifecycle != "Active" && lifecycle != "Sunset" {
			continue
		}
		var stillPending bool
		err = tx.QueryRow(ctx, `SELECT true FROM action_outbox WHERE id=$1 AND state='PENDING' FOR UPDATE`, p.id).Scan(&stillPending)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return 0, err
		}
		var message messaging.Message
		renderError := ""
		if err := json.Unmarshal(p.raw, &message); err != nil {
			renderError = "invalid_outbox_payload"
		}
		// Compatibility for pending outbox entries created by the phase-5 worker.
		if message.Channel == "" {
			var old struct {
				Config workflow.Config `json:"config"`
				Data   map[string]any  `json:"data"`
			}
			if err := json.Unmarshal(p.raw, &old); err != nil {
				renderError = "invalid_outbox_payload"
			} else {
				message, err = messaging.Render(p.channel, old.Config, old.Data)
				if err != nil {
					renderError = "render_failed"
				}
			}
		}
		var consent map[string]bool
		_ = json.Unmarshal(p.consents, &consent)
		message.To = ""
		if p.channel == "email" && p.email != nil {
			message.To = *p.email
		}
		if p.channel == "sms" && p.phone != nil {
			message.To = *p.phone
		}
		decision := Eligibility{Eligible: true, Consent: consent[p.channel], FrequencyCap: true, Reason: "eligible"}
		status := "SCHEDULED"
		switch {
		case p.state == "EXITED" || p.state == "CANCELLED" || p.state == "FAILED":
			decision.Eligible = false
			decision.Reason = "execution_terminated"
			status = "SKIPPED"
		case renderError != "":
			decision.Eligible = false
			decision.Reason = renderError
			status = "FAILED"
		case len(message.Missing) > 0:
			decision.Eligible = false
			decision.Reason = "missing_personalization"
			status = "FAILED"
		case !decision.Consent:
			decision.Eligible = false
			decision.Reason = "consent_denied"
			status = "BLOCKED"
		case !validRecipient(p.channel, message.To):
			decision.Eligible = false
			decision.Reason = "invalid_recipient"
			status = "BLOCKED"
		}
		if decision.Eligible {
			var reservations int
			err := tx.QueryRow(ctx, `SELECT count(*) FROM (SELECT 1 FROM ((SELECT 1 FROM delivery_jobs WHERE workspace_id=$1 AND customer_id=$2 AND mode='LIVE' AND status IN ('SCHEDULED','CLAIMED','DISPATCHING','UNKNOWN') LIMIT $4) UNION ALL (SELECT 1 FROM delivery_jobs WHERE workspace_id=$1 AND customer_id=$2 AND mode='LIVE' AND status='ACCEPTED' AND accepted_at>=$3 LIMIT $4)) slots LIMIT $4) reservations`, p.workspace, p.customer, now.Add(-time.Duration(p.window)*time.Second), p.limit).Scan(&reservations)
			if err != nil {
				return 0, err
			}
			if reservations >= p.limit {
				decision.Eligible = false
				decision.FrequencyCap = false
				decision.Reason = "frequency_cap"
				status = "BLOCKED"
			}
		}
		name, provider := s.provider(p.channel)
		idempotent := false
		if provider != nil {
			idempotent = provider.Idempotent()
		} else {
			decision.Eligible = false
			decision.Reason = "provider_unconfigured"
			status = "FAILED"
		}
		body, _ := json.Marshal(message)
		eligibility, _ := json.Marshal(decision)
		if _, err := tx.Exec(ctx, `INSERT INTO delivery_jobs(workspace_id,execution_id,outbox_id,node_id,customer_id,channel,provider,idempotent_provider,logical_key,message,decision,status,due_at,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$13) ON CONFLICT DO NOTHING`, p.workspace, p.execution, p.id, p.node, p.customer, p.channel, name, idempotent, p.execution+":"+p.node, body, eligibility, status, now); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, `UPDATE action_outbox SET state='PROCESSED',payload='{}' WHERE id=$1`, p.id); err != nil {
			return 0, err
		}
	}
	return len(items), tx.Commit(ctx)
}
func (s *Service) Recover(ctx context.Context) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	rows, err := tx.Query(ctx, `UPDATE delivery_jobs SET status=CASE WHEN status='CLAIMED' OR idempotent_provider THEN 'SCHEDULED' ELSE 'UNKNOWN' END,claim_token=NULL,lease_until=NULL,last_error='Delivery lease expired; previous provider outcome may be unknown' WHERE id IN (SELECT id FROM delivery_jobs WHERE status IN ('CLAIMED','DISPATCHING') AND lease_until<=$1 ORDER BY lease_until,id LIMIT 100 FOR UPDATE SKIP LOCKED) RETURNING id::text,attempts`, s.Clock.Now())
	if err != nil {
		return err
	}
	type lost struct {
		id      string
		attempt int
	}
	items := []lost{}
	for rows.Next() {
		var p lost
		if err := rows.Scan(&p.id, &p.attempt); err != nil {
			rows.Close()
			return err
		}
		items = append(items, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, item := range items {
		if _, err := tx.Exec(ctx, `UPDATE delivery_attempts SET status='UNKNOWN',response='Worker lease expired before outcome was persisted',completed_at=$3 WHERE delivery_id=$1 AND attempt=$2 AND status='STARTED'`, item.id, item.attempt, s.Clock.Now()); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func (s *Service) Claim(ctx context.Context) (*Job, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	now := s.Clock.Now()
	var id string
	// Lock the customer before the delivery row, matching runtime/global-exit order.
	err = tx.QueryRow(ctx, `SELECT d.id::text FROM delivery_jobs d JOIN customers c ON c.workspace_id=d.workspace_id AND c.customer_id=d.customer_id JOIN journey_executions e ON e.id=d.execution_id JOIN journeys j ON j.id=e.journey_id WHERE d.status='SCHEDULED' AND d.mode='LIVE' AND d.due_at<=$1 AND j.status IN ('Active','Sunset') ORDER BY d.due_at,d.id LIMIT 1 FOR UPDATE OF c SKIP LOCKED`, now).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `SELECT id::text FROM delivery_jobs WHERE status='SCHEDULED' AND mode='TEST' AND due_at<=$1 ORDER BY due_at,id LIMIT 1 FOR UPDATE SKIP LOCKED`, now).Scan(&id)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var job Job
	var raw []byte
	err = tx.QueryRow(ctx, `UPDATE delivery_jobs SET status='CLAIMED',claim_token=gen_random_uuid(),lease_until=$2::timestamptz+interval '30 seconds',attempts=attempts+1 WHERE id=$1 AND status='SCHEDULED' RETURNING id::text,workspace_id::text,COALESCE(customer_id,''),COALESCE(execution_id::text,''),claim_token::text,provider,logical_key,mode,attempts,message`, id, now).Scan(&job.ID, &job.WorkspaceID, &job.CustomerID, &job.ExecutionID, &job.Token, &job.Provider, &job.LogicalKey, &job.Mode, &job.Attempt, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &job.Message); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO delivery_attempts(workspace_id,delivery_id,attempt,status,started_at) VALUES($1,$2,$3,'STARTED',$4)`, job.WorkspaceID, job.ID, job.Attempt, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &job, nil
}
func (s *Service) Dispatch(ctx context.Context, job Job) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	now := s.Clock.Now()
	blocked := ""
	if job.Mode == "LIVE" {
		var consents []byte
		var email, phone *string
		if err := tx.QueryRow(ctx, `SELECT consents,email,phone FROM customers WHERE workspace_id=$1 AND customer_id=$2 FOR UPDATE`, job.WorkspaceID, job.CustomerID).Scan(&consents, &email, &phone); err != nil {
			return err
		}
		var state, status string
		if err := tx.QueryRow(ctx, `SELECT e.state,j.status FROM journey_executions e JOIN journeys j ON j.id=e.journey_id WHERE e.id=$1 FOR SHARE OF j`, job.ExecutionID).Scan(&state, &status); err != nil {
			return err
		}
		var consent map[string]bool
		_ = json.Unmarshal(consents, &consent)
		if state == "EXITED" || state == "CANCELLED" || state == "FAILED" {
			blocked = "execution_terminated"
		} else if !consent[job.Message.Channel] {
			blocked = "consent_revoked"
		} else if job.Message.Channel == "email" && (email == nil || *email != job.Message.To) || job.Message.Channel == "sms" && (phone == nil || *phone != job.Message.To) {
			blocked = "recipient_changed"
		} else if status != "Active" && status != "Sunset" {
			if _, err := tx.Exec(ctx, `UPDATE delivery_jobs SET status='SCHEDULED',claim_token=NULL,lease_until=NULL,attempts=attempts-1 WHERE id=$1 AND claim_token=$2 AND status='CLAIMED'`, job.ID, job.Token); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `DELETE FROM delivery_attempts WHERE delivery_id=$1 AND attempt=$2 AND status='STARTED'`, job.ID, job.Attempt); err != nil {
				return err
			}
			return tx.Commit(ctx)
		}
	}
	if job.Mode == "TEST" {
		var raw []byte
		err := tx.QueryRow(ctx, `SELECT test_allowlist FROM workspace_settings WHERE workspace_id=$1`, job.WorkspaceID).Scan(&raw)
		var allowed []string
		if err == nil {
			_ = json.Unmarshal(raw, &allowed)
		}
		matched := false
		for _, recipient := range allowed {
			matched = matched || strings.EqualFold(recipient, job.Message.To)
		}
		if !matched {
			blocked = "test_recipient_not_allowlisted"
		}
	}
	if blocked != "" {
		if _, err := tx.Exec(ctx, `UPDATE delivery_jobs SET status='BLOCKED',decision=decision||jsonb_build_object('eligible',false,'reason',$3::text),lease_until=NULL,claim_token=NULL WHERE id=$1 AND claim_token=$2 AND status='CLAIMED'`, job.ID, job.Token, blocked); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE delivery_attempts SET status='CANCELLED',completed_at=$3,response=$4 WHERE delivery_id=$1 AND attempt=$2 AND status='STARTED'`, job.ID, job.Attempt, now, blocked); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	tag, err := tx.Exec(ctx, `UPDATE delivery_jobs SET status='DISPATCHING' WHERE id=$1 AND claim_token=$2 AND status='CLAIMED' AND lease_until>$3`, job.ID, job.Token, now)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	provider := s.Providers[job.Provider]
	out := Outcome{Status: "FAILED", Response: "Provider is not configured"}
	if job.Attempt > 5 {
		out = Outcome{Status: "FAILED", Response: "Retry limit exhausted"}
	} else if provider != nil {
		request, cancel := context.WithTimeout(ctx, 10*time.Second)
		out = provider.Send(request, job.Message, job.WorkspaceID+":"+job.LogicalKey)
		cancel()
	}
	status := out.Status
	if status == "RETRY" {
		if job.Attempt >= 5 {
			out.Status = "FAILED"
			out.Response += "; retry limit exhausted"
			status = "FAILED"
		} else {
			status = "SCHEDULED"
		}
	}
	if status != "SCHEDULED" && status != "ACCEPTED" && status != "FAILED" && status != "UNKNOWN" {
		status = "FAILED"
	}
	completed, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx2, err := s.Pool.Begin(completed)
	if err != nil {
		return err
	}
	defer func() { _ = tx2.Rollback(context.Background()) }()
	finished := s.Clock.Now()
	delay := time.Duration(1<<min(job.Attempt, 8)) * time.Second
	tag, err = tx2.Exec(completed, `UPDATE delivery_jobs SET status=$3,due_at=$4,accepted_at=CASE WHEN $3='ACCEPTED' THEN $5::timestamptz ELSE NULL END,lease_until=NULL,claim_token=NULL,last_error=CASE WHEN $3='ACCEPTED' THEN NULL ELSE $6 END WHERE id=$1 AND claim_token=$2 AND status='DISPATCHING'`, job.ID, job.Token, status, finished.Add(delay), finished, out.Response)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	if _, err := tx2.Exec(completed, `UPDATE delivery_attempts SET status=$3,provider_code=$4,provider_id=$5,response=$6,completed_at=$7 WHERE delivery_id=$1 AND attempt=$2 AND status='STARTED'`, job.ID, job.Attempt, out.Status, out.Code, out.ProviderID, out.Response, finished); err != nil {
		return err
	}
	return tx2.Commit(completed)
}
func (s *Service) Run(ctx context.Context) error {
	for ctx.Err() == nil {
		count, err := s.Schedule(ctx)
		if err != nil {
			s.Log.Error("delivery scheduling failed")
		}
		if err := s.Recover(ctx); err != nil {
			s.Log.Error("delivery lease recovery failed")
		}
		var group sync.WaitGroup
		claimed := 0
		for range 4 {
			job, err := s.Claim(ctx)
			if err != nil {
				s.Log.Error("delivery claim failed")
				break
			}
			if job == nil {
				break
			}
			claimed++
			group.Go(func() {
				if err := s.Dispatch(ctx, *job); err != nil && ctx.Err() == nil {
					s.Log.Error("delivery dispatch failed", "delivery_id", job.ID)
				}
			})
		}
		group.Wait()
		if count == 0 && claimed == 0 {
			timer := time.NewTimer(250 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
			case <-timer.C:
			}
		}
	}
	return nil
}
func NormalizeAllowlist(addresses []string) []string {
	out := []string{}
	for _, address := range addresses {
		address = strings.ToLower(strings.TrimSpace(address))
		if address != "" {
			out = append(out, address)
		}
	}
	return out
}
