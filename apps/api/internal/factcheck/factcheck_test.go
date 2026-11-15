package factcheck

import (
	"slices"
	"strings"
	"testing"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

const brief = `Sepet terk edildiğinde 2 saat bekle, SEPET15 kodu ile %15 indirim sunan bir e-posta gönder. 1 gün sonra SMS gönder. Bağlantı olarak https://example.com/sepet’i kullan.`

func flow(nodes ...workflow.Node) workflow.Workflow {
	return workflow.Workflow{SchemaVersion: 1, Nodes: nodes}
}
func email(id, text string, code string) workflow.Node {
	return workflow.Node{ID: id, Type: "email", Config: workflow.Config{Subject: "Sepetin", Text: text, DiscountCode: code}}
}
func sms(id, text string, code string) workflow.Node {
	return workflow.Node{ID: id, Type: "sms", Config: workflow.Config{Text: text, DiscountCode: code}}
}
func messages(issues []workflow.Issue) string {
	out := []string{}
	for _, i := range issues {
		out = append(out, i.NodeID+": "+i.Message)
	}
	return strings.Join(out, "\n")
}

func TestTheBriefIsReadForLinksCodesNumbersTimesAndDiscounts(t *testing.T) {
	f := Read(brief)
	if !f.Links["https://example.com/sepet"] || len(f.Links) != 1 {
		t.Fatal("the link loses its Turkish suffix", f.Links)
	}
	if !f.Codes["SEPET15"] || len(f.Codes) != 1 {
		t.Fatal(f.Codes)
	}
	if !f.Discounts["15"] || !f.Durations[7200] || !f.Durations[86400] || !f.Numbers["15"] {
		t.Fatal("%15 next to indirim is a discount", f.Discounts, f.Durations, f.Numbers)
	}
	if got := Read("SEPET15 kodu"); len(got.Numbers) != 0 {
		t.Fatal("a code's digits are not a number", got.Numbers)
	}
	for text, want := range map[string]string{"www.shop.example/kis.": "https://www.shop.example/kis", "shop.example/kis": "https://shop.example/kis", "HTTP://Shop.Example/a/": "https://shop.example/a"} {
		if got := Read(text).Links; !got[want] {
			t.Errorf("%q: %v", text, got)
		}
	}
	if got := Read("Puanın 3.5/5 ve 1.000.000 kişi"); len(got.Links) != 0 || !got.Numbers["3.5"] || !got.Numbers["1000000"] {
		t.Fatal("a rating is not a link; a grouped number is one number", got)
	}
	if got := Read(`<p style="font-size:14px">Merhaba{{ customer.attributes.age }} &#39;</p><a href="https://example.com/sepet">Git</a>`); len(got.Numbers) != 0 || !got.Links["https://example.com/sepet"] {
		t.Fatal("markup, variables and entities state nothing; an href is a link", got)
	}
	if got := Read("SMS ve 2026 yılı, SEPET-15"); len(got.Codes) != 1 || !got.Codes["SEPET-15"] {
		t.Fatal("a code has letters and digits", got.Codes)
	}
}

func TestGeneratedTextMayStateOnlyWhatTheBriefStates(t *testing.T) {
	given := Given(brief)
	good := flow(
		email("mail", "Merhaba {{ customer.attributes.first_name }}, %15 indirimin seni bekliyor: https://example.com/sepet", "SEPET15"),
		sms("text", "{{ offer.code }} ile %15 indirim, 24 saat içinde: example.com/sepet", "SEPET15"),
	)
	if issues := Check(given, good, nil); len(issues) != 0 {
		t.Fatal("the brief's facts, and 24 saat for 1 gün, pass:\n" + messages(issues))
	}
	for name, c := range map[string]struct {
		node workflow.Node
		want string
	}{
		"another link":         {email("mail", "https://example.com/kampanya", ""), "Link https://example.com/kampanya is not in the brief"},
		"invented code":        {sms("text", "{{ offer.code }} ile gel", "SEPET20"), "Discount code SEPET20 is not in the brief"},
		"typed code":           {sms("text", "SEPET15 ile gel {{ offer.code }}", "SEPET15"), "Write the code as {{ offer.code }}"},
		"code without a field": {email("mail", "Kodun SEPET15", ""), "Set discount_code to SEPET15"},
		"other code in text":   {email("mail", "Kodun YAZ10", ""), "Code YAZ10 is not in the brief"},
		"bigger discount":      {email("mail", "%20 indirim", ""), "The number 20 is not in the brief"},
		"added time":           {sms("text", "Son 3 gün!", ""), "The number 3 is not in the brief"},
	} {
		issues := Check(given, flow(c.node), nil)
		if !strings.Contains(messages(issues), c.want) || issues[0].Field != "facts" {
			t.Errorf("%s: want %q, got:\n%s", name, c.want, messages(issues))
		}
	}
	// Only the steps asked about are checked; a person's text elsewhere stays.
	mixed := flow(email("mine", "Son 3 gün!", ""), sms("generated", "Gel: https://example.com/sepet", ""))
	if issues := Check(given, mixed, map[string]bool{"generated": true}); len(issues) != 0 {
		t.Fatal(messages(issues))
	}
}

func TestWhatTheBriefAsksAndNoMessageCarriesIsNoted(t *testing.T) {
	if notes := NotDone(brief, flow(email("mail", "%15 indirim: https://example.com/sepet", "SEPET15"))); len(notes) != 0 {
		t.Fatal(notes)
	}
	notes := NotDone(brief, flow(email("mail", "Sepetin seni bekliyor", ""), workflow.Node{ID: "wait", Type: "wait", Config: workflow.Config{Text: "https://example.com/sepet"}}))
	want := []string{
		"The brief gives the link https://example.com/sepet, but no message uses it.",
		"The brief names the code SEPET15, but no message offers it.",
		"The brief offers 15% off, but no message says so.",
	}
	if !slices.Equal(notes, want) {
		t.Fatalf("%q", notes)
	}
	if notes := NotDone("Sepet doluluğu %50'yi geçerse e-posta gönder; fiyatı 100 TL'yi geçen ürünler", flow()); len(notes) != 0 {
		t.Fatal("a percentage is a discount only beside a word for one", notes)
	}
}
