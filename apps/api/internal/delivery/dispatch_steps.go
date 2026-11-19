package delivery

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/observability"
	"go.opentelemetry.io/otel/attribute"
)

// recipient is the customer row a live message is checked against, read under
// the customer's lock just before the message goes out.
type recipient struct {
	consents                []byte
	email, phone, pushToken *string
}

// liveDecision checks a claimed live message against its customer and journey
// as they are now. It returns why the message may not go out, or held when the
// journey is paused and the message waits for it instead.
func liveDecision(job Job, r recipient, state, status string) (blocked string, held bool) {
	var consent map[string]bool
	_ = json.Unmarshal(r.consents, &consent)
	switch {
	case state == "EXITED" || state == "CANCELLED" || state == "FAILED":
		return "execution_terminated", false
	case !consent[job.Message.Channel]:
		return "consent_revoked", false
	case job.Message.Channel == "email" && (r.email == nil || *r.email != job.Message.To) || job.Message.Channel == "sms" && (r.phone == nil || *r.phone != job.Message.To) || job.Message.Channel == "push" && (r.pushToken == nil || *r.pushToken != job.Message.To):
		return "recipient_changed", false
	case status != "Active" && status != "Sunset":
		return "", true
	}
	return "", false
}

// handBack returns a claimed message to the queue without using an attempt.
func handBack(ctx context.Context, tx pgx.Tx, job Job) error {
	if _, err := tx.Exec(ctx, `UPDATE delivery_jobs SET status='SCHEDULED',claim_token=NULL,lease_until=NULL,attempts=attempts-1 WHERE id=$1 AND claim_token=$2 AND status='CLAIMED'`, job.ID, job.Token); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `DELETE FROM delivery_attempts WHERE delivery_id=$1 AND attempt=$2 AND status='STARTED'`, job.ID, job.Attempt)
	return err
}

// block records why a claimed message will not go out.
func block(ctx context.Context, tx pgx.Tx, job Job, reason string, now time.Time) error {
	if _, err := tx.Exec(ctx, `UPDATE delivery_jobs SET status='BLOCKED',decision=decision||jsonb_build_object('eligible',false,'reason',$3::text),lease_until=NULL,claim_token=NULL WHERE id=$1 AND claim_token=$2 AND status='CLAIMED'`, job.ID, job.Token, reason); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE delivery_attempts SET status='CANCELLED',completed_at=$3,response=$4 WHERE delivery_id=$1 AND attempt=$2 AND status='STARTED'`, job.ID, job.Attempt, now, reason)
	return err
}

// call asks the provider to send a message that is DISPATCHING. called is
// whether the answer came from the provider, or stands in for it.
func (s *Service) call(ctx context.Context, job Job) (out Outcome, called bool) {
	provider := s.Providers[job.Provider]
	out = Outcome{Status: "FAILED", Response: "Provider is not configured"}
	if job.Counted() > 5 {
		return Outcome{Status: "FAILED", Response: "Retry limit exhausted"}, false
	}
	if fault, ok := FaultOutcome(job.Fault); ok {
		// The workspace is acting out a provider problem; nothing is called.
		return fault, true
	}
	if provider == nil {
		return out, false
	}
	request, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	providerCtx, providerSpan := observability.Start(request, "messaging.provider.send", attribute.String("provider.name", job.Provider), attribute.String("messaging.channel", job.Message.Channel))
	began := time.Now()
	out = provider.Send(providerCtx, job.Message, job.WorkspaceID+":"+job.LogicalKey)
	observability.ObserveProvider(job.Provider, job.Message.Channel, out.Status, time.Since(began))
	providerSpan.SetAttributes(attribute.String("provider.outcome", out.Status), attribute.Int("provider.response.code", out.Code))
	if out.Status != "ACCEPTED" {
		observability.Fail(providerSpan, "provider_"+strings.ToLower(out.Status))
	}
	providerSpan.End()
	return out, true
}

// settled is what a provider answer makes of a message.
type settled struct {
	out     Outcome
	verdict answer
	free    bool
	status  string
	delay   time.Duration
}

// settle turns a provider answer into the message's next status, counting
// attempts and the retry limit.
func settle(ctx context.Context, job Job, out Outcome) settled {
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
	return settled{out: out, verdict: verdict, free: free, status: status, delay: retryAfter(verdict, out, counted)}
}
