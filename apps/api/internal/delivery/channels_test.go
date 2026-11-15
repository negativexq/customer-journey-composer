package delivery

import (
	"context"
	"errors"
	"net/textproto"
	"testing"
	"time"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/messaging"
)

func TestAnswersAboutTheChannelCostTheMessageNoAttempt(t *testing.T) {
	m := &Mock{Codes: []int{429, 401, 503, 400}, RetryAfter: 7 * time.Second}
	message := messaging.Message{Channel: "sms", To: "+905551234567", Text: "Hi"}
	want := []answer{answerThrottled, answerCredentials, answerFailed, answerRejected}
	for i, key := range []string{"a", "b", "c", "d"} {
		out := m.Send(context.Background(), message, key)
		if got := classify(out); got != want[i] {
			t.Fatalf("call %d: %+v classified %d", i, out, got)
		}
		if i == 0 && out.RetryAfter != 7*time.Second {
			t.Fatal("a 429 carries its Retry-After", out)
		}
	}
	job := Job{Attempt: 3}
	if !job.free(answerThrottled) || !job.free(answerCredentials) || job.free(answerFailed) || job.free(answerRejected) {
		t.Fatal("only 429 and 401 are free outside a probe")
	}
	job.BreakerProbe = true
	if !job.free(answerFailed) {
		t.Fatal("a failed probe says the provider is down, not the message")
	}
	job.Free = maxFreeAttempts
	if job.free(answerThrottled) {
		t.Fatal("free calls are bounded so a stuck channel ends")
	}
	if (Job{Attempt: 4, Free: 3}).Counted() != 1 {
		t.Fatal("counted attempts leave out free ones")
	}
}

func TestRetryWaitsFollowTheProviderAndSpreadOut(t *testing.T) {
	if got := retryAfter(answerThrottled, Outcome{RetryAfter: 5 * time.Second}, 1); got != 5*time.Second {
		t.Fatal("Retry-After is honoured", got)
	}
	if got := retryAfter(answerThrottled, Outcome{}, 1); got != minThrottle {
		t.Fatal("a 429 without Retry-After waits a second", got)
	}
	if got := retryAfter(answerThrottled, Outcome{RetryAfter: time.Hour}, 1); got != maxThrottle {
		t.Fatal("a long Retry-After is capped", got)
	}
	if got := retryAfter(answerCredentials, Outcome{}, 1); got != credentialProbeEvery {
		t.Fatal("a refused key waits for the next probe", got)
	}
	seen := map[time.Duration]bool{}
	for range 200 {
		got := retryAfter(answerFailed, Outcome{}, 3)
		if got < 8*time.Second || got > 12*time.Second {
			t.Fatal("backoff with up to half again of jitter", got)
		}
		seen[got] = true
	}
	if len(seen) < 10 {
		t.Fatal("jitter spreads retries", len(seen))
	}
}

func TestSimulatedFaultsAnswerLikeAProvider(t *testing.T) {
	for fault, want := range map[string]answer{"throttle": answerThrottled, "outage": answerFailed, "credentials": answerCredentials} {
		out, ok := FaultOutcome(fault)
		if !ok || classify(out) != want {
			t.Fatal(fault, out)
		}
	}
	if _, ok := FaultOutcome(""); ok {
		t.Fatal("no fault, no simulated answer")
	}
}

func TestSMTPLoginRefusalIsACredentialSignal(t *testing.T) {
	out := smtpError(&textproto.Error{Code: 535, Msg: "authentication failed"}, false)
	if out.Signal != "credentials" || classify(out) != answerCredentials {
		t.Fatal(out)
	}
	if out := smtpError(&textproto.Error{Code: 550, Msg: "no such user"}, false); classify(out) != answerRejected {
		t.Fatal("a refused recipient is about the message", out)
	}
	if out := smtpError(errors.New("connection reset"), false); classify(out) != answerFailed {
		t.Fatal(out)
	}
}
