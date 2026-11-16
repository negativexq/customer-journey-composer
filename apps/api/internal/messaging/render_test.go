package messaging

import (
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
	"strings"
	"testing"
)

func TestPersonalizationIsEscapedAndSanitized(t *testing.T) {
	c := workflow.Config{Subject: "Hello {{ customer.first_name }}", HTML: `<p>{{ customer.first_name }}</p><script>alert(1)</script><a href="javascript:alert(1)">Bad</a><a href="https://example.com/cart">Cart</a>`, Text: "Hello {{ customer.first_name }}"}
	data := map[string]any{"customer": map[string]any{"first_name": `<img src=x onerror=alert(1)>`}}
	m, err := Render("email", c, data)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(m.HTML, "<script") || strings.Contains(m.HTML, "javascript:") || strings.Contains(m.HTML, "<img") {
		t.Fatal("unsafe rendered HTML", m.HTML)
	}
	if !strings.Contains(m.HTML, "&lt;img") || len(m.Links) != 1 {
		t.Fatal(m)
	}
	c.Text = "Hello {{ customer.missing }}"
	c.HTML = ""
	c.Subject = "Hello"
	m, err = Render("email", c, data)
	if err != nil || len(m.Missing) != 1 {
		t.Fatal(m, err)
	}
	c.Fallbacks = map[string]string{"customer.missing": "friend"}
	m, err = Render("email", c, data)
	if err != nil || len(m.Missing) != 0 || m.Text != "Hello friend" {
		t.Fatal(m, err)
	}
	c.Subject = "Header\r\nBcc: injected"
	if _, err := Render("email", c, data); err == nil {
		t.Fatal("header injection must fail")
	}
	c.Subject = "Hello"
	c.Text = "{{ customer.name | exec }}"
	if _, err := Render("email", c, data); err == nil {
		t.Fatal("arbitrary expressions must fail")
	}
}
func TestEmailKeepsSafeInlineStyles(t *testing.T) {
	c := workflow.Config{Subject: "Hi", HTML: `<table role="presentation" cellpadding="0" cellspacing="0" style="width:100%;max-width:560px;background-color:#ffffff;border-radius:12px"><tr><td style="padding:24px 28px;font-family:Arial, sans-serif;color:#1f2430"><h1 style="margin:0;font-size:28px;font-weight:700;letter-spacing:-0.5px">Made for you</h1><a href="https://example.com/cart" style="display:inline-block;padding:12px 22px;background-color:#1f2430;color:#ffffff;text-decoration:none;border-radius:8px">Return to cart</a></td></tr></table>`}
	m, err := Render("email", c, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	for _, kept := range []string{`role="presentation"`, `cellpadding="0"`, "max-width: 560px", "background-color: #ffffff", "border-radius: 12px", "font-size: 28px", "display: inline-block", "text-decoration: none", "letter-spacing: -0.5px"} {
		if !strings.Contains(m.HTML, kept) {
			t.Fatalf("missing %q in %s", kept, m.HTML)
		}
	}
	if len(m.Warnings) != 0 {
		t.Fatal("safe styles must not warn", m.Warnings)
	}
}

func TestEmailDropsUnsafeInlineStyles(t *testing.T) {
	c := workflow.Config{Subject: "Hi", HTML: `<p style="background-image:url(https://tracker.example/x.png);background:url(https://tracker.example/y.png);position:fixed;z-index:9;color:expression(alert(1));font-size:16px">Hi {{ customer.colour }}</p><p style="color:{{ customer.colour }}">Tinted</p>`}
	data := map[string]any{"customer": map[string]any{"colour": `red;background-image:url(https://tracker.example/z.png)`}}
	m, err := Render("email", c, data)
	if err != nil {
		t.Fatal(err)
	}
	for _, dropped := range []string{"tracker.example/x", "tracker.example/y", "position", "z-index", "expression"} {
		if strings.Contains(m.HTML, dropped) {
			t.Fatalf("unsafe %q kept in %s", dropped, m.HTML)
		}
	}
	// A personalized value cannot smuggle a second property into a style.
	if !strings.Contains(m.HTML, `<p style="color: red">Tinted</p>`) {
		t.Fatal("personalized style not reduced to its valid part", m.HTML)
	}
	if !strings.Contains(m.HTML, "font-size: 16px") {
		t.Fatal("safe style dropped next to unsafe ones", m.HTML)
	}
	if len(m.Warnings) == 0 {
		t.Fatal("removing styles must warn")
	}
}

func TestSMSSegmentAccounting(t *testing.T) {
	for _, tc := range []struct {
		text, encoding  string
		units, segments int
	}{{"", "GSM-7", 0, 0}, {strings.Repeat("a", 160), "GSM-7", 160, 1}, {strings.Repeat("a", 161), "GSM-7", 161, 2}, {strings.Repeat("^", 81), "GSM-7", 162, 2}, {strings.Repeat("ş", 70), "Unicode", 70, 1}, {strings.Repeat("ş", 71), "Unicode", 71, 2}, {"😀", "Unicode", 2, 1}} {
		encoding, units, segments := Segments(tc.text)
		if encoding != tc.encoding || units != tc.units || segments != tc.segments {
			t.Fatalf("want %s/%d/%d got %s/%d/%d", tc.encoding, tc.units, tc.segments, encoding, units, segments)
		}
	}
}
