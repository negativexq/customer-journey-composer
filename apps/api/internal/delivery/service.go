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

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/messaging"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/observability"
	engine "github.com/negativexq/customer-journey-composer/apps/api/internal/runtime"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
	"go.opentelemetry.io/otel/attribute"
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
	ID, WorkspaceID, CustomerID, ExecutionID, Token, Provider, LogicalKey, Mode, TraceParent string
	Attempt                                                                                  int
	// Free counts earlier calls that cost no attempt, such as a 429.
	Free    int
	Message messaging.Message
	// BreakerProbe marks a call that tests a failing provider; TestedLock is
	// the credential lock a call tests, when the channel's key was refused.
	BreakerProbe bool
	TestedLock   *time.Time
	// Fault is the provider problem the workspace is simulating, if any.
	Fault string
}

// Counted is how many of the message's attempts this call has used.
func (j Job) Counted() int { return j.Attempt - j.Free }

var e164 = regexp.MustCompile(`^\+[1-9][0-9]{6,14}$`)

func New(pool *pgxpool.Pool, clock engine.Clock) *Service {
	if clock == nil {
		clock = engine.RealClock{}
	}
	return &Service{Pool: pool, Clock: clock, Providers: map[string]Provider{"smtp": SMTPFromEnv(), "mock_sms": &Mock{}, "mock_email": &Mock{}, "mock_push": &Mock{}}, Log: slog.Default()}
}
func (s *Service) provider(channel string) (string, Provider) {
	name := ProviderFor(channel)
	return name, s.Providers[name]
}

// ProviderFor is the provider each channel sends through. SMS and push are
// simulated: nothing reaches a real phone or device.
func ProviderFor(channel string) string {
	switch channel {
	case "sms":
		return "mock_sms"
	case "push":
		return "mock_push"
	}
	return "smtp"
}

func validRecipient(channel, to string) bool {
	switch channel {
	case "sms":
		return e164.MatchString(to)
	case "push":
		return len(to) >= 8 && len(to) <= 512 && !strings.ContainsAny(to, " \t\r\n")
	}
	a, err := mail.ParseAddress(to)
	return err == nil && a.Address == to
}

// Each action commits before another customer is locked. A bounded drain keeps
// the public scheduler work budget without holding an entire batch of customers.
const scheduleDrainBudget = 32

// schedulePage is how many pending actions one discovery query returns.
const schedulePage = 32

// PendingPageQuery finds the next pending actions in order. Discovery reads only
// the ordered partial index, once per page: finding each action again from the
// head of the queue repeated the walk for every action and, with statistics
// taken while the queue was empty, made the planner scan and sort the whole
// outbox table each time.
const PendingPageQuery = `SELECT o.id::text,o.created_at FROM action_outbox o WHERE o.state='PENDING' ORDER BY o.created_at,o.id LIMIT $1`

// PendingAfterQuery continues a drain after the last action of the previous
// page. Order by the table's columns: the bare id would name the text output
// column and sort outside the index. The cursor is local to one drain, so locked/skipped or newly backdated
// actions are reconsidered from the beginning on the next drain.
const PendingAfterQuery = `SELECT o.id::text,o.created_at FROM action_outbox o WHERE o.state='PENDING' AND (o.created_at,o.id)>($2,$3::uuid) ORDER BY o.created_at,o.id LIMIT $1`

// ScheduleItemQuery locks one discovered action's customer by primary key. A
// customer another transaction holds is skipped, not waited for.
const ScheduleItemQuery = `SELECT o.id::text,o.workspace_id::text,o.execution_id::text,o.node_id,o.channel,o.payload,p.customer_id,p.state,p.email,p.phone,p.push_token,p.consents,p.frequency_limit,p.frequency_window_seconds FROM action_outbox o JOIN LATERAL (
   SELECT e.customer_id,e.state,c.email,c.phone,c.push_token,c.consents,COALESCE(s.frequency_limit,3) AS frequency_limit,COALESCE(s.frequency_window_seconds,86400) AS frequency_window_seconds
   FROM journey_executions e JOIN customers c ON c.workspace_id=e.workspace_id AND c.customer_id=e.customer_id
   LEFT JOIN workspace_settings s ON s.workspace_id=o.workspace_id
   WHERE e.id=o.execution_id AND (SELECT j.status IN ('Active','Sunset') FROM journeys j WHERE j.id=e.journey_id)
   FOR UPDATE OF c SKIP LOCKED
 ) p ON true
 WHERE o.id=$1 AND o.state='PENDING'`

type scheduleCursor struct {
	createdAt time.Time
	id        string
}

// pendingPage lists up to limit pending actions, after the cursor when given.
func (s *Service) pendingPage(ctx context.Context, after *scheduleCursor, limit int) ([]scheduleCursor, error) {
	query, args := PendingPageQuery, []any{limit}
	if after != nil {
		query, args = PendingAfterQuery, []any{limit, after.createdAt, after.id}
	}
	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (scheduleCursor, error) {
		var c scheduleCursor
		err := row.Scan(&c.id, &c.createdAt)
		return c, err
	})
}

func (s *Service) Schedule(ctx context.Context) (int, error) {
	total := 0
	var after *scheduleCursor
	for total < scheduleDrainBudget {
		page, err := s.pendingPage(ctx, after, schedulePage)
		if err != nil {
			return total, err
		}
		for _, item := range page {
			n, err := s.scheduleItem(ctx, item.id)
			total += n
			if err != nil {
				return total, err
			}
		}
		if len(page) < schedulePage {
			return total, nil
		}
		after = &page[len(page)-1]
	}
	return total, nil
}

// scheduleOne schedules the first pending action whose customer is free.
func (s *Service) scheduleOne(ctx context.Context) (int, error) {
	page, err := s.pendingPage(ctx, nil, schedulePage)
	if err != nil {
		return 0, err
	}
	for _, item := range page {
		if n, err := s.scheduleItem(ctx, item.id); n > 0 || err != nil {
			return n, err
		}
	}
	return 0, nil
}

// scheduleItem turns one pending action into a delivery decision in its own
// transaction. It returns 0 when the action is gone or its customer is busy.
func (s *Service) scheduleItem(ctx context.Context, id string) (int, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	now := s.Clock.Now()
	type pending struct {
		id, workspace, execution, node, channel, customer, state string
		raw, consents                                            []byte
		email, phone, pushToken                                  *string
		limit, window                                            int
	}
	var p pending
	err = tx.QueryRow(ctx, ScheduleItemQuery, id).Scan(&p.id, &p.workspace, &p.execution, &p.node, &p.channel, &p.raw, &p.customer, &p.state, &p.email, &p.phone, &p.pushToken, &p.consents, &p.limit, &p.window)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	items := []pending{p}
	for _, p := range items {
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
		if p.channel == "push" && p.pushToken != nil {
			message.To = *p.pushToken
		}
		// This is a new READ COMMITTED statement after acquiring the customer.
		// Keep the journey -> outbox lock order and re-read execution state here;
		// the initial claim's snapshot can predate a concurrent global exit.
		var lifecycle string
		var stillPending bool
		var reservations int
		countReservations := renderError == "" && EvaluateEligibility(message, consent, 0, p.limit, "RUNNING").Eligible
		err = tx.QueryRow(ctx, `WITH lifecycle AS MATERIALIZED (
          SELECT e.state,j.status FROM journeys j JOIN journey_executions e ON e.journey_id=j.id WHERE e.id=$1 FOR SHARE OF j
        ), pending AS MATERIALIZED (
          SELECT a.id FROM action_outbox a CROSS JOIN lifecycle l WHERE a.id=$2 AND a.state='PENDING' AND l.status IN ('Active','Sunset') FOR UPDATE OF a
        ) SELECT l.state,l.status,EXISTS(SELECT 1 FROM pending),
          CASE WHEN $6::boolean AND EXISTS(SELECT 1 FROM pending) THEN (
            SELECT count(*) FROM (SELECT 1 FROM (
              (SELECT 1 FROM delivery_jobs WHERE workspace_id=$3 AND customer_id=$4 AND mode='LIVE' AND status IN ('SCHEDULED','CLAIMED','DISPATCHING','UNKNOWN') LIMIT $7)
              UNION ALL
              (SELECT 1 FROM delivery_jobs WHERE workspace_id=$3 AND customer_id=$4 AND mode='LIVE' AND status='ACCEPTED' AND accepted_at>=$5 LIMIT $7)
            ) slots LIMIT $7) reservations
          ) ELSE 0 END FROM lifecycle l`, p.execution, p.id, p.workspace, p.customer, now.Add(-time.Duration(p.window)*time.Second), countReservations, p.limit).Scan(&p.state, &lifecycle, &stillPending, &reservations)
		if err != nil {
			return 0, err
		}
		if !stillPending || (lifecycle != "Active" && lifecycle != "Sunset") {
			continue
		}
		decision := EvaluateEligibility(message, consent, reservations, p.limit, p.state)
		status := "SCHEDULED"
		if !decision.Eligible {
			status = "BLOCKED"
			if decision.Reason == "execution_terminated" {
				status = "SKIPPED"
			}
			if decision.Reason == "missing_personalization" {
				status = "FAILED"
			}
		}
		if renderError != "" {
			decision.Eligible = false
			decision.Reason = renderError
			status = "FAILED"
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
		// Only a message that will be sent gets tracked links, an open pixel and
		// an unsubscribe link; the stored copy is exactly what goes out.
		var codes []TrackingCode
		if status == "SCHEDULED" {
			codes = Track(&message, TrackingBase())
		}
		body, _ := json.Marshal(message)
		eligibility, _ := json.Marshal(decision)
		if _, err := tx.Exec(ctx, `WITH delivery AS (INSERT INTO delivery_jobs(id,workspace_id,execution_id,outbox_id,node_id,customer_id,channel,provider,idempotent_provider,logical_key,message,decision,status,due_at,created_at) VALUES($14,$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$13) ON CONFLICT DO NOTHING RETURNING id), codes AS (INSERT INTO tracking_codes(code,workspace_id,delivery_id,kind,url) SELECT c.code,$1,d.id,c.kind,c.url FROM delivery d CROSS JOIN jsonb_to_recordset($15::jsonb) c(code text,kind text,url text)) UPDATE action_outbox SET state='PROCESSED',payload='{}' WHERE id=$3`, p.workspace, p.execution, p.id, p.node, p.customer, p.channel, name, idempotent, p.execution+":"+p.node, body, eligibility, status, now, uuid.NewString(), codesJSON(codes)); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return len(items), nil
}
func (s *Service) Recover(ctx context.Context) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	rows, err := tx.Query(ctx, `UPDATE delivery_jobs SET status=CASE WHEN status='CLAIMED' OR idempotent_provider THEN 'SCHEDULED' ELSE 'UNKNOWN' END,claim_token=NULL,lease_until=NULL,last_error='Delivery lease expired; previous provider outcome may be unknown' WHERE id IN (SELECT id FROM delivery_jobs WHERE status IN ('CLAIMED','DISPATCHING') AND lease_until<=$1 ORDER BY lease_until,id LIMIT 100 FOR UPDATE SKIP LOCKED) RETURNING id::text,attempts,status`, s.Clock.Now())
	if err != nil {
		return err
	}
	type lost struct {
		status  string
		id      string
		attempt int
	}
	items := []lost{}
	for rows.Next() {
		var p lost
		if err := rows.Scan(&p.id, &p.attempt, &p.status); err != nil {
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
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	for _, item := range items {
		result := "scheduled"
		if item.status == "UNKNOWN" {
			result = "unknown"
		}
		observability.ObserveLeaseRecovery("delivery", result, 1)
	}
	return nil
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
	// A held channel's messages are left where they are, so others go first.
	held, err := HeldChannels(ctx, tx, now)
	if err != nil {
		return nil, err
	}
	err = tx.QueryRow(ctx, ClaimLive, now, held).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, claimTest, now, held).Scan(&id)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var job Job
	var workspace, providerName string
	if err := tx.QueryRow(ctx, `SELECT workspace_id::text,provider FROM delivery_jobs WHERE id=$1`, id).Scan(&workspace, &providerName); err != nil {
		return nil, err
	}
	job.BreakerProbe, job.TestedLock, job.Fault, err = admit(ctx, tx, workspace, providerName, now)
	if errors.Is(err, ErrGated) {
		// The pause a full rate window wrote is kept; the message is untouched.
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return nil, ErrGated
	}
	if err != nil {
		return nil, err
	}
	var raw []byte
	err = tx.QueryRow(ctx, `UPDATE delivery_jobs SET status='CLAIMED',claim_token=gen_random_uuid(),lease_until=$2::timestamptz+interval '30 seconds',attempts=attempts+1 WHERE id=$1 AND status='SCHEDULED' RETURNING id::text,workspace_id::text,COALESCE(customer_id,''),COALESCE(execution_id::text,''),claim_token::text,provider,logical_key,mode,attempts,free_attempts,message,COALESCE((SELECT trace_parent FROM journey_executions e WHERE e.id=delivery_jobs.execution_id),'')`, id, now).Scan(&job.ID, &job.WorkspaceID, &job.CustomerID, &job.ExecutionID, &job.Token, &job.Provider, &job.LogicalKey, &job.Mode, &job.Attempt, &job.Free, &raw, &job.TraceParent)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &job.Message); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO delivery_attempts(workspace_id,delivery_id,attempt,status,started_at,provider,gate) VALUES($1,$2,$3,'STARTED',$4,$5,$6)`, job.WorkspaceID, job.ID, job.Attempt, now, job.Provider, gateLabel(job.BreakerProbe, job.TestedLock != nil)); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &job, nil
}
func (s *Service) Dispatch(ctx context.Context, job Job) (err error) {
	ctx, span := observability.Start(observability.Parent(ctx, job.TraceParent), "delivery.dispatch", attribute.String("delivery.id", job.ID), attribute.Int("delivery.attempt", job.Attempt))
	ctx, finish := observability.TrackOperation(ctx, "delivery_dispatch")
	defer func() {
		finish(err)
		if err != nil && !errors.Is(err, context.Canceled) {
			s.Log.ErrorContext(ctx, "delivery dispatch failed", "delivery_id", job.ID, "error_class", observability.ErrorClass(err))
		}
		span.End()
	}()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	now := s.Clock.Now()
	blocked := ""
	if job.Mode == "LIVE" {
		var consents []byte
		var email, phone, pushToken *string
		if err := tx.QueryRow(ctx, `SELECT consents,email,phone,push_token FROM customers WHERE workspace_id=$1 AND customer_id=$2 FOR UPDATE`, job.WorkspaceID, job.CustomerID).Scan(&consents, &email, &phone, &pushToken); err != nil {
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
		} else if job.Message.Channel == "email" && (email == nil || *email != job.Message.To) || job.Message.Channel == "sms" && (phone == nil || *phone != job.Message.To) || job.Message.Channel == "push" && (pushToken == nil || *pushToken != job.Message.To) {
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
	// called is whether the answer came from the provider, or stands in for it.
	called := false
	if job.Counted() > 5 {
		out = Outcome{Status: "FAILED", Response: "Retry limit exhausted"}
	} else if fault, ok := FaultOutcome(job.Fault); ok {
		// The workspace is acting out a provider problem; nothing is called.
		out, called = fault, true
	} else if provider != nil {
		called = true
		request, cancel := context.WithTimeout(ctx, 10*time.Second)
		providerCtx, providerSpan := observability.Start(request, "messaging.provider.send", attribute.String("provider.name", job.Provider), attribute.String("messaging.channel", job.Message.Channel))
		began := time.Now()
		out = provider.Send(providerCtx, job.Message, job.WorkspaceID+":"+job.LogicalKey)
		observability.ObserveProvider(job.Provider, job.Message.Channel, out.Status, time.Since(began))
		providerSpan.SetAttributes(attribute.String("provider.outcome", out.Status), attribute.Int("provider.response.code", out.Code))
		if out.Status != "ACCEPTED" {
			observability.Fail(providerSpan, "provider_"+strings.ToLower(out.Status))
		}
		providerSpan.End()
		cancel()
	}
	switch out.Status {
	case "RETRY":
		observability.SetOutcome(ctx, "retry")
	case "FAILED":
		observability.SetOutcome(ctx, "failed")
	case "UNKNOWN":
		observability.SetOutcome(ctx, "unknown")
	}
	verdict := classify(out)
	free := job.free(verdict)
	counted := job.Counted()
	if free {
		counted--
	}
	status := out.Status
	if status == "RETRY" {
		if counted >= 5 {
			out.Status = "FAILED"
			observability.SetOutcome(ctx, "failed")
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
	delay := retryAfter(verdict, out, counted)
	tag, err = tx2.Exec(completed, `UPDATE delivery_jobs SET status=$3,due_at=$4,accepted_at=CASE WHEN $3='ACCEPTED' THEN $5::timestamptz ELSE NULL END,lease_until=NULL,claim_token=NULL,last_error=CASE WHEN $3='ACCEPTED' THEN NULL ELSE $6 END,free_attempts=free_attempts+$7 WHERE id=$1 AND claim_token=$2 AND status='DISPATCHING'`, job.ID, job.Token, status, finished.Add(delay), finished, out.Response, map[bool]int{true: 1}[free])
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	var signal *string
	if out.Signal != "" {
		signal = &out.Signal
	}
	if _, err := tx2.Exec(completed, `UPDATE delivery_attempts SET status=$3,provider_code=$4,provider_id=$5,response=$6,completed_at=$7,signal=$8 WHERE delivery_id=$1 AND attempt=$2 AND status='STARTED'`, job.ID, job.Attempt, out.Status, out.Code, out.ProviderID, out.Response, finished, signal); err != nil {
		return err
	}
	if called {
		if err := observe(completed, tx2, job, job.Message.Channel, verdict, out, finished); err != nil {
			return err
		}
	}
	if err := tx2.Commit(completed); err != nil {
		return err
	}
	// The mock providers report delivery the way a real one would by webhook.
	// The send is already recorded, so a failed report is logged, not retried.
	if _, mock := provider.(*Mock); mock && status == "ACCEPTED" {
		kind := simulatedReport(job.Message.Channel, job.Message.To)
		if _, err := RecordEngagement(completed, s.Pool, job.WorkspaceID, job.ID, kind, "simulated", nil, finished, finished); err != nil {
			s.Log.WarnContext(ctx, "simulated delivery report failed", "delivery_id", job.ID, "error_class", observability.ErrorClass(err))
		}
	}
	return nil
}
func (s *Service) Run(ctx context.Context) error {
	wake := make(chan struct{}, 4)
	var group sync.WaitGroup
	// Scheduling and lease recovery do not wait for provider network calls.
	group.Go(func() {
		for ctx.Err() == nil {
			request, cancel := context.WithTimeout(ctx, 15*time.Second)
			n, err := s.Schedule(request)
			cancel()
			if err != nil {
				if ctx.Err() == nil {
					s.Log.ErrorContext(ctx, "delivery scheduling failed")
				}
				if !deliveryPause(ctx, nil, time.Second) {
					return
				}
				continue
			}
			if n > 0 {
				for range 4 {
					select {
					case wake <- struct{}{}:
					default:
					}
				}
			}
			if n < scheduleDrainBudget && !deliveryPause(ctx, nil, 250*time.Millisecond) {
				return
			}
		}
	})
	group.Go(func() {
		for ctx.Err() == nil {
			request, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := s.Recover(request)
			cancel()
			if err != nil && ctx.Err() == nil {
				s.Log.ErrorContext(ctx, "delivery lease recovery failed")
			}
			if !deliveryPause(ctx, nil, time.Second) {
				return
			}
		}
	})
	for range 4 {
		group.Go(func() {
			for ctx.Err() == nil {
				request, cancel := context.WithTimeout(ctx, 15*time.Second)
				job, err := s.Claim(request)
				cancel()
				if errors.Is(err, ErrGated) {
					// Another channel's messages may be waiting behind this one.
					continue
				}
				if err != nil {
					if ctx.Err() == nil {
						s.Log.ErrorContext(ctx, "delivery claim failed")
					}
					if !deliveryPause(ctx, nil, time.Second) {
						return
					}
					continue
				}
				if job == nil {
					// New work does not cut a wait on held channels short: it would
					// only queue behind them and send the dispatcher back to skip them.
					request, cancel := context.WithTimeout(ctx, 5*time.Second)
					held, err := s.heldWait(request)
					cancel()
					if err != nil && ctx.Err() == nil {
						s.Log.ErrorContext(ctx, "delivery held channel check failed")
					}
					if held > 0 {
						if !deliveryPause(ctx, nil, held) {
							return
						}
						continue
					}
					if !deliveryPause(ctx, wake, 250*time.Millisecond) {
						return
					}
					continue
				}
				_ = s.Dispatch(ctx, *job)
			}
		})
	}
	group.Wait()
	return nil
}
func deliveryPause(ctx context.Context, wake <-chan struct{}, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-wake:
		return true
	case <-timer.C:
		return true
	}
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
