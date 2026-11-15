package delivery

import (
	"regexp"
	"strings"
	"testing"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/messaging"
)

func TestTrackRewritesEveryEmailLinkThroughItsOwnCode(t *testing.T) {
	m := messaging.Message{Channel: "email", HTML: `<p><a href="https://shop.example/cart?a=1&amp;b=2" rel="nofollow">Cart</a> <a href="https://shop.example/cart?a=1&amp;b=2">Again</a> <a href="mailto:help@example.com">Help</a></p>`, Text: "Open https://shop.example/cart?a=1&b=2."}
	codes := Track(&m, "https://go.example")
	clicks := map[string]string{}
	kinds := map[string]int{}
	for _, c := range codes {
		kinds[c.Kind]++
		if c.Kind == "click" {
			clicks[c.URL] = c.Code
		}
		if !regexp.MustCompile(`^[A-Za-z0-9]{12}$`).MatchString(c.Code) {
			t.Fatalf("code %q has the wrong shape", c.Code)
		}
	}
	if kinds["click"] != 1 || kinds["open"] != 1 || kinds["unsubscribe"] != 1 {
		t.Fatalf("codes = %+v", codes)
	}
	code := clicks["https://shop.example/cart?a=1&b=2"]
	if code == "" || strings.Count(m.HTML, "https://go.example/api/t/"+code) != 2 {
		t.Fatalf("both links to the same address share one code: %s", m.HTML)
	}
	if strings.Contains(m.HTML, "shop.example") || !strings.Contains(m.HTML, `href="mailto:help@example.com"`) {
		t.Fatalf("only http(s) links are tracked: %s", m.HTML)
	}
	if !strings.Contains(m.Text, "https://go.example/api/t/"+code+".") {
		t.Fatalf("the text part keeps its punctuation after the link: %q", m.Text)
	}
	if m.Unsubscribe == "" || !strings.Contains(m.HTML, m.Unsubscribe) || !strings.HasSuffix(m.Text, "Unsubscribe: "+m.Unsubscribe) {
		t.Fatalf("unsubscribe link missing: %q / %q", m.HTML, m.Text)
	}
}

func TestTrackShortensSMSLinksAndRecountsTheMessage(t *testing.T) {
	long := "https://shop.example/" + strings.Repeat("x", 120)
	m := messaging.Message{Channel: "sms", Text: "Your cart: " + long}
	m.Encoding, m.Units, m.Segments = messaging.Segments(m.Text)
	before := m.Units
	codes := Track(&m, "https://go.example")
	if len(codes) != 1 || codes[0].URL != long || m.Unsubscribe != "" {
		t.Fatalf("codes = %+v", codes)
	}
	if m.Text != "Your cart: https://go.example/api/t/"+codes[0].Code || m.Units >= before {
		t.Fatalf("text %q units %d (was %d)", m.Text, m.Units, before)
	}
}

func TestTrackingCodesAreNotRepeated(t *testing.T) {
	seen := map[string]bool{}
	for range 2000 {
		code := NewTrackingCode()
		if seen[code] {
			t.Fatalf("repeated code %s", code)
		}
		seen[code] = true
	}
}

func TestTrackedEmailsCarryOneClickUnsubscribeHeaders(t *testing.T) {
	m := messaging.Message{Channel: "email", To: "reader@example.invalid", Subject: "Hi", HTML: "<p>Hi</p>", Text: "Hi"}
	Track(&m, "https://go.example")
	raw, err := emailBytes(m, "from@example.invalid", "key")
	if err != nil {
		t.Fatal(err)
	}
	head := string(raw[:strings.Index(string(raw), "\r\n\r\n")])
	if !strings.Contains(head, "List-Unsubscribe: <"+m.Unsubscribe+">\r\n") || !strings.Contains(head, "List-Unsubscribe-Post: List-Unsubscribe=One-Click") {
		t.Fatalf("headers: %s", head)
	}
	plain := messaging.Message{Channel: "email", To: "reader@example.invalid", Subject: "Hi", Text: "Hi"}
	raw, _ = emailBytes(plain, "from@example.invalid", "key")
	if strings.Contains(string(raw), "List-Unsubscribe") {
		t.Fatal("an untracked message has no unsubscribe header")
	}
}

func TestMockProvidersReportBouncesOnlyForReservedRecipients(t *testing.T) {
	for _, c := range []struct{ channel, to, want string }{
		{"email", "reader@example.invalid", "delivered"},
		{"email", "bounce@example.invalid", "bounced"},
		{"email", "reader+bounce@example.invalid", "bounced"},
		{"sms", "+905551234567", "delivered"},
		{"sms", "+905551230000", "bounced"},
	} {
		if got := simulatedReport(c.channel, c.to); got != c.want {
			t.Errorf("%s %s: %s", c.channel, c.to, got)
		}
	}
}
