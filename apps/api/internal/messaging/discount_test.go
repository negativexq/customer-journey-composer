package messaging

import (
	"slices"
	"strings"
	"testing"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

func TestADiscountCodeIsWrittenWhereTheMessageShowsIt(t *testing.T) {
	c := workflow.Config{Text: "Use {{ offer.code }} for 15% off, {{ customer.first_name }}", DiscountCode: "SEPET15"}
	data := map[string]any{"customer": map[string]any{"first_name": "Ada"}}
	m, err := Render("sms", c, data)
	if err != nil || m.Text != "Use SEPET15 for 15% off, Ada" || len(m.Missing) != 0 {
		t.Fatal(m, err)
	}
	if _, touched := data["offer"]; touched {
		t.Fatal("the caller's data is not changed")
	}
	c.DiscountCode = ""
	if m, _ := Render("sms", c, data); !slices.Contains(m.Missing, "offer.code") {
		t.Fatal("a message showing an unset code is missing a value", m.Missing)
	}
	if got := Variables("{{ offer.code }} {{customer.attributes.first_name}} {{ offer.code }}"); !slices.Equal(got, []string{"offer.code", "customer.attributes.first_name"}) {
		t.Fatal(got)
	}
}

func TestAStepsDiscountCodeMustBeShownAndShownCodesMustBeSet(t *testing.T) {
	flow := func(c workflow.Config) workflow.Workflow {
		return workflow.Workflow{SchemaVersion: 1, Nodes: []workflow.Node{
			{ID: "start", Type: "event_trigger", Label: "Cart", Config: workflow.Config{EventName: "cart_updated"}},
			{ID: "sms", Type: "sms", Label: "Code", Config: c},
			{ID: "done", Type: "exit", Label: "Done", Config: workflow.Config{Reason: "completed"}},
		}, Edges: []workflow.Edge{{ID: "a", Source: "start", Target: "sms", Port: "started"}, {ID: "b", Source: "sms", Target: "done", Port: "next"}}}
	}
	message := func(c workflow.Config) string {
		for _, issue := range ValidateWorkflow(flow(c)) {
			if issue.Field == "discount_code" {
				return issue.Message
			}
		}
		return ""
	}
	if got := message(workflow.Config{Text: "Come back", DiscountCode: "SEPET15"}); !strings.Contains(got, "{{ offer.code }}") {
		t.Fatal("a code nobody sees", got)
	}
	if got := message(workflow.Config{Text: "Use {{ offer.code }}"}); !strings.Contains(got, "Set the discount code") {
		t.Fatal("a shown code nobody set", got)
	}
	if got := message(workflow.Config{Text: "Use {{ offer.code }}", DiscountCode: "sepet15"}); got == "" {
		t.Fatal("a code a customer cannot type")
	}
	if got := message(workflow.Config{Text: "Use {{ offer.code }}", DiscountCode: "SEPET15"}); got != "" {
		t.Fatal(got)
	}
}

func TestAnEmailShowsItsCodeInABoxEvenWhenTheBodyDoesNot(t *testing.T) {
	c := workflow.Config{Subject: "Your cart", HTML: `<p>Finish your order</p>`, Text: "Finish your order", DiscountCode: "SEPET15"}
	m, err := Render("email", c, map[string]any{})
	if err != nil || strings.Count(m.HTML, "SEPET15") != 1 || !strings.Contains(m.HTML, "dashed") || !strings.HasSuffix(m.Text, "Your discount code: SEPET15") || m.Language != "en" || len(m.Warnings) != 0 {
		t.Fatal(m.HTML, m.Text, m.Warnings, err)
	}
	// The label follows the customer's language, not a fixed one.
	turkish := map[string]any{"customer": map[string]any{"locale": "tr-TR"}}
	if m, _ := Render("email", c, turkish); m.Language != "tr" || !strings.Contains(m.HTML, "İndirim kodun") || !strings.HasSuffix(m.Text, "İndirim kodun: SEPET15") {
		t.Fatal("a Turkish reader's code is labelled in Turkish", m.Language, m.Text)
	}
	if m, _ := Render("email", c, map[string]any{"customer": map[string]any{"locale": "ru"}}); m.Language != "en" || !strings.HasSuffix(m.Text, "Your discount code: SEPET15") {
		t.Fatal("a language with no text falls back to English", m.Language, m.Text)
	}
	c.HTML = `<p>Use {{ offer.code }}</p>`
	if m, _ := Render("email", c, map[string]any{}); strings.Count(m.HTML, "SEPET15") != 1 || strings.Contains(m.HTML, "dashed") {
		t.Fatal("a body that shows the code gets no second copy", m.HTML)
	}
	flow := workflow.Workflow{SchemaVersion: 1, Nodes: []workflow.Node{
		{ID: "start", Type: "event_trigger", Label: "Cart", Config: workflow.Config{EventName: "cart_updated"}},
		{ID: "email", Type: "email", Label: "Code", Config: workflow.Config{Subject: "Hi", Text: "Come back", DiscountCode: "SEPET15"}},
		{ID: "done", Type: "exit", Label: "Done", Config: workflow.Config{Reason: "completed"}},
	}, Edges: []workflow.Edge{{ID: "a", Source: "start", Target: "email", Port: "started"}, {ID: "b", Source: "email", Target: "done", Port: "next"}}}
	if issues := ValidateWorkflow(flow); len(issues) != 0 {
		t.Fatal("an email need not write the code itself", issues)
	}
}
