package delivery

import (
	"strings"
	"testing"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/messaging"
)

func TestPushGoesThroughTheMockProviderAndTracksItsTap(t *testing.T) {
	if ProviderFor("push") != "mock_push" || ProviderFor("sms") != "mock_sms" || ProviderFor("email") != "smtp" {
		t.Fatal("wrong provider per channel")
	}
	m := messaging.Message{Channel: "push", To: "device-token-1", Subject: "Hi", Text: "Visit https://example.com/text", URL: "https://example.com/cart"}
	codes := Track(&m, "https://track.example")
	if len(codes) != 1 || codes[0].Kind != "click" || codes[0].URL != "https://example.com/cart" {
		t.Fatalf("codes: %+v", codes)
	}
	if !strings.HasPrefix(m.URL, "https://track.example"+TrackingPath) || len(m.Links) != 1 || m.Links[0] != m.URL {
		t.Fatalf("tap link not tracked: %+v", m)
	}
}

func TestPushNeedsConsentAndADeviceToken(t *testing.T) {
	m := messaging.Message{Channel: "push", To: "device-token-1"}
	if d := EvaluateEligibility(m, map[string]bool{"push": true}, 0, 3, "RUNNING"); !d.Eligible {
		t.Fatalf("eligible push: %+v", d)
	}
	if d := EvaluateEligibility(m, map[string]bool{"email": true, "sms": true}, 0, 3, "RUNNING"); d.Reason != "consent_denied" {
		t.Fatalf("without push consent: %+v", d)
	}
	m.To = ""
	if d := EvaluateEligibility(m, map[string]bool{"push": true}, 0, 3, "RUNNING"); d.Reason != "invalid_recipient" {
		t.Fatalf("without a token: %+v", d)
	}
	if simulatedReport("push", "device-token-1") != "delivered" || simulatedReport("push", "expired-token-1") != "bounced" {
		t.Fatal("push reports")
	}
}
