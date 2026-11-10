package messaging

import (
	"github.com/ofk/customer-journey-composer/apps/api/internal/workflow"
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
