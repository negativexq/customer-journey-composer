package catalog

import (
	"github.com/negativexq/customer-journey-composer/apps/api/internal/simulation"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
	"time"
)

const RecoveryEURBrief = `Create an abandoned-cart recovery journey for EUR carts worth at least 50. Wait one hour, then email customers who have not purchased and have email consent. Wait up to 24 hours for a purchase. If there is no purchase and SMS consent is present, send a personalized SMS with the code COMEBACK10 for 10% off. Wait another 48 hours, then finish as converted or without conversion. Exit immediately whenever purchase_completed arrives. Use the customer's first name and cart amount in both messages. Prevent re-entry for seven days, while allowing the email and SMS within the same execution. Enforce consent and workspace frequency limits before dispatch. Report conversion separately from provider acceptance.`

func recoveryEUR() Template {
	consent := func(channel string) *workflow.Rule {
		return &workflow.Rule{Path: "customer.consents." + channel, Operator: "equals", Value: true}
	}
	purchased := &workflow.Rule{Path: "customer.attributes.cart_purchased", Operator: "equals", Value: true}
	entry := &workflow.Rule{Logic: "and", Rules: []workflow.Rule{{Path: "event.properties.currency", Operator: "equals", Value: "EUR"}, {Logic: "or", Rules: []workflow.Rule{{Path: "event.properties.amount", Operator: "greater_than", Value: 50.0}, {Path: "event.properties.amount", Operator: "equals", Value: 50.0}}}}}
	w := workflow.Workflow{SchemaVersion: 1, GlobalExit: &workflow.ExitCriteria{EventName: "purchase_completed"}, Reentry: &workflow.Reentry{Mode: "cooldown", CooldownSeconds: 604800}, Nodes: []workflow.Node{
		node("start", "event_trigger", "Cart abandoned · €50+", 340, 0, workflow.Config{EventName: "cart_abandoned", Filters: entry}),
		node("channels", "condition", "Any channel consent?", 340, 160, workflow.Config{Rule: &workflow.Rule{Logic: "or", Rules: []workflow.Rule{*consent("email"), *consent("sms")}}}),
		node("first_wait", "wait", "Wait 1 hour", 340, 320, workflow.Config{DurationSeconds: 3600, Timezone: "UTC"}),
		node("purchase_check", "condition", "Already purchased?", 340, 480, workflow.Config{Rule: purchased}),
		node("email_consent", "condition", "Email consent?", 340, 640, workflow.Config{Rule: consent("email")}),
		node("email", "email", "Your cart is waiting", 80, 800, workflow.Config{Subject: "{{ customer.first_name }}, your cart is waiting!", Preheader: "You left something behind", HTML: `<div style="font-family:Arial,sans-serif;padding:32px;color:#202639"><p>YOUR CART</p><h1>You left something behind</h1><p>Hi {{ customer.first_name }},</p><p>You still have items worth €{{ event.properties.amount_display }} waiting in your cart. Complete your order before they're gone.</p><p><a href="https://example.com/cart">Return to Your Cart</a></p></div>`, Text: "Hi {{ customer.first_name }}, you still have items worth €{{ event.properties.amount_display }} waiting in your cart. Return to your cart: https://example.com/cart"}),
		node("purchase_24h", "wait_event", "Wait for purchase · 24h", 340, 980, workflow.Config{EventName: "purchase_completed", TimeoutSeconds: 86400}),
		node("sms_consent", "condition", "SMS consent?", 340, 1140, workflow.Config{Rule: consent("sms")}),
		node("sms", "sms", "10% off · COMEBACK10", 80, 1300, workflow.Config{Text: "Hi {{ customer.first_name }}! Your €{{ event.properties.amount_display }} cart is still waiting. Use code COMEBACK10 for 10% off your order. Shop now: https://example.com/cart"}),
		node("purchase_48h", "wait_event", "Wait for purchase · 48h", 80, 1480, workflow.Config{EventName: "purchase_completed", TimeoutSeconds: 172800}),
		node("final_check", "condition", "Final purchase check", 80, 1640, workflow.Config{Rule: purchased}),
		node("converted", "exit", "Converted", 720, 800, workflow.Config{Reason: "converted"}),
		node("not_converted", "exit", "Not converted", 640, 1480, workflow.Config{Reason: "completed"}),
		node("no_consent", "exit", "No permitted channel", 720, 320, workflow.Config{Reason: "condition_not_met"}),
	}, Edges: []workflow.Edge{
		edge("01", "start", "channels", "started"), edge("02", "channels", "first_wait", "true"), edge("03", "channels", "no_consent", "false"),
		edge("04", "first_wait", "purchase_check", "completed"), edge("05", "purchase_check", "converted", "true"), edge("06", "purchase_check", "email_consent", "false"),
		edge("07", "email_consent", "email", "true"), edge("08", "email_consent", "purchase_24h", "false"), edge("09", "email", "purchase_24h", "next"),
		edge("10", "purchase_24h", "converted", "matched"), edge("11", "purchase_24h", "sms_consent", "timeout"), edge("12", "sms_consent", "sms", "true"), edge("13", "sms_consent", "not_converted", "false"),
		edge("14", "sms", "purchase_48h", "next"), edge("15", "purchase_48h", "converted", "matched"), edge("16", "purchase_48h", "final_check", "timeout"), edge("17", "final_check", "converted", "true"), edge("18", "final_check", "not_converted", "false"),
	}}
	return Template{ID: "abandoned-cart-eur", Name: "Abandoned Cart · €50+", Description: "A one-hour email, a 24-hour SMS recovery window and a final 48-hour purchase window. Seven-day re-entry cooldown.", Tags: []string{"commerce", "eur-recovery", "consent-aware"}, Workflow: w, Brief: RecoveryEURBrief, Scenarios: RecoveryEURScenarios(), Notes: []string{"Entry requires cart_abandoned with amount >= 50 and currency EUR.", "cart_purchased is the current cart status, not lifetime purchase history.", "amount is numeric; amount_display is the store-formatted amount used in messages (for example 89.90). Missing personalization blocks rendering.", "Seven days means no repeat enrollment; the email and SMS in one execution remain eligible. Workspace frequency limits also apply before dispatch.", "Without email consent, email is skipped and the 24-hour purchase window still runs before an eligible SMS. Without either consent the journey exits.", "COMEBACK10 is sample message content. Configure a real coupon and cart URL in your store before publishing."}}
}

func RecoveryEURScenarios() []simulation.Scenario {
	start := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	makeScenario := func(name string, email, sms bool) simulation.Scenario {
		return simulation.Scenario{Name: name, StartTime: start, Customer: map[string]any{"customer_id": "recovery_test", "email": "alex@example.invalid", "phone": "+12025550100", "locale": "en", "timezone": "Europe/Paris", "attributes": map[string]any{"first_name": "Alex", "cart_purchased": false}, "consents": map[string]any{"email": email, "sms": sms}}, Entry: simulation.Event{EventName: "cart_abandoned", Properties: map[string]any{"amount": 89.9, "amount_display": "89.90", "currency": "EUR"}}, Assertions: []simulation.Assertion{{Type: "no_duplicate_delivery", Expected: true}}}
	}
	purchase := func(seconds int64) simulation.TimedEvent {
		return simulation.TimedEvent{AfterSeconds: seconds, Event: simulation.Event{EventName: "purchase_completed"}}
	}
	a := makeScenario("A · Purchase after 30 minutes", true, true)
	a.Events = []simulation.TimedEvent{purchase(1800)}
	a.Assertions = append(a.Assertions, simulation.Assertion{Type: "no_message"}, simulation.Assertion{Type: "exit_reason", Expected: "converted"})
	b := makeScenario("B · Purchase after the email", true, true)
	b.Events = []simulation.TimedEvent{purchase(18000)}
	b.Assertions = append(b.Assertions, simulation.Assertion{Type: "message_count", Channel: "email", Expected: 1}, simulation.Assertion{Type: "message_count", Channel: "sms", Expected: 0}, simulation.Assertion{Type: "exit_reason", Expected: "converted"})
	c := makeScenario("C · Email and SMS, no purchase", true, true)
	c.Assertions = append(c.Assertions, simulation.Assertion{Type: "message_count", Channel: "email", Expected: 1}, simulation.Assertion{Type: "message_count", Channel: "sms", Expected: 1}, simulation.Assertion{Type: "state", Expected: "COMPLETED"}, simulation.Assertion{Type: "exit_reason", Expected: "completed"})
	d := makeScenario("D · SMS consent denied", true, false)
	d.Assertions = append(d.Assertions, simulation.Assertion{Type: "message_count", Channel: "email", Expected: 1}, simulation.Assertion{Type: "message_count", Channel: "sms", Expected: 0}, simulation.Assertion{Type: "state", Expected: "COMPLETED"})
	e := makeScenario("E · Repeat enrollment inside seven days", true, true)
	e.Events = []simulation.TimedEvent{purchase(1800), {AfterSeconds: 86400, Event: simulation.Event{EventName: "cart_abandoned", Properties: map[string]any{"amount": 89.9, "amount_display": "89.90", "currency": "EUR"}}}}
	e.Assertions = append(e.Assertions, simulation.Assertion{Type: "enrollment_count", Expected: 1}, simulation.Assertion{Type: "no_message"})
	return []simulation.Scenario{a, b, c, d, e}
}
