package messaging

import (
	"testing"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

func TestAPushRendersItsTitleBodyAndTapLinkForTheDevice(t *testing.T) {
	data := map[string]any{"customer": map[string]any{"push_token": "device-token-1", "attributes": map[string]any{"first_name": "Ada"}}, "event": map[string]any{"properties": map[string]any{"cart_id": "c1"}}}
	m, err := Render("push", workflow.Config{Subject: "{{ customer.attributes.first_name }}, your cart", Text: "Use {{ offer.code }} today", URL: "https://example.com/cart/{{ event.properties.cart_id }}", DiscountCode: "SAVE10", HTML: "<b>ignored</b>"}, data)
	if err != nil {
		t.Fatal(err)
	}
	if m.To != "device-token-1" || m.Subject != "Ada, your cart" || m.Text != "Use SAVE10 today" || m.URL != "https://example.com/cart/c1" || m.HTML != "" {
		t.Fatalf("%+v", m)
	}
	if len(m.Links) != 1 || m.Links[0] != m.URL {
		t.Fatalf("links: %v", m.Links)
	}
}

func TestAPushDropsAnUnsafeTapLinkAndNeedsItsTitle(t *testing.T) {
	m, err := Render("push", workflow.Config{Subject: "Hi", Text: "Body", URL: "javascript:alert(1)"}, map[string]any{})
	if err != nil || m.URL != "" || len(m.Warnings) != 1 {
		t.Fatalf("%v %+v", err, m)
	}
	if _, err := Render("push", workflow.Config{Text: "Body"}, map[string]any{}); err == nil {
		t.Fatal("a push without a title rendered")
	}
}

func TestAPushThatOffersACodeMustShowIt(t *testing.T) {
	w := workflow.Workflow{SchemaVersion: 1, Nodes: []workflow.Node{
		{ID: "start", Type: "event_trigger", Label: "Start", Config: workflow.Config{EventName: "cart_updated"}},
		{ID: "push", Type: "push", Label: "Push", Config: workflow.Config{Subject: "Offer", Text: "Come back", DiscountCode: "SAVE10"}},
		{ID: "done", Type: "exit", Label: "Done", Config: workflow.Config{Reason: "completed"}},
	}, Edges: []workflow.Edge{{ID: "a", Source: "start", Target: "push", Port: "started"}, {ID: "b", Source: "push", Target: "done", Port: "next"}}}
	found := false
	for _, issue := range ValidateWorkflow(w) {
		found = found || issue.Field == "discount_code"
	}
	if !found {
		t.Fatal("a push that hides its code passed validation")
	}
}
