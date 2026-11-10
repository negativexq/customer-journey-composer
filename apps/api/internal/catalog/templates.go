package catalog

import (
	"github.com/negativexq/customer-journey-composer/apps/api/internal/simulation"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

type Template struct {
	Brief       string                `json:"brief,omitempty"`
	Notes       []string              `json:"notes,omitempty"`
	Scenarios   []simulation.Scenario `json:"scenarios,omitempty"`
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Tags        []string              `json:"tags"`
	Workflow    workflow.Workflow     `json:"workflow"`
}

func edge(id, source, target, port string) workflow.Edge {
	return workflow.Edge{ID: id, Source: source, Target: target, Port: port}
}
func node(id, kind, label string, x, y float64, config workflow.Config) workflow.Node {
	return workflow.Node{ID: id, Type: kind, Label: label, Position: workflow.Position{X: x, Y: y}, Config: config}
}
func Builtins() []Template {
	fallback := map[string]string{"customer.first_name": "there"}
	cart := workflow.Workflow{SchemaVersion: 1, GlobalExit: &workflow.ExitCriteria{EventName: "purchase_completed"}, Reentry: &workflow.Reentry{Mode: "after_completion"}, Nodes: []workflow.Node{
		node("start", "event_trigger", "Cart updated", 300, 0, workflow.Config{EventName: "cart_updated"}),
		node("delay", "wait", "Wait 2 hours", 300, 160, workflow.Config{DurationSeconds: 7200, Timezone: "UTC"}),
		node("check", "condition", "Has purchased?", 300, 320, workflow.Config{Rule: &workflow.Rule{Path: "customer.attributes.purchased", Operator: "equals", Value: true}}),
		node("email", "email", "Cart reminder email", 80, 500, workflow.Config{Subject: "Your cart is waiting, {{ customer.first_name }}", Preheader: "Pick up where you left off", HTML: "<div style=\"font-family:Arial;padding:32px;background:#f4f7ef\"><h1>Hello {{ customer.first_name }},</h1><p>Your next favourite thing is still in your cart.</p><p><a href=\"https://example.invalid/cart\">Return to your cart</a></p><p>Take your time. We saved your place.</p></div>", Text: "Hello {{ customer.first_name }}, your cart is waiting: https://example.invalid/cart", Fallbacks: fallback}),
		node("await", "wait_event", "Wait for purchase · 24h", 80, 680, workflow.Config{EventName: "purchase_completed", TimeoutSeconds: 86400}),
		node("sms", "sms", "Final cart reminder", 80, 860, workflow.Config{Text: "Hello {{ customer.first_name }}, your cart is still waiting. https://example.invalid/cart", Fallbacks: fallback}),
		node("done", "exit", "Journey complete", 80, 1040, workflow.Config{Reason: "completed"}),
		node("converted", "exit", "Purchase complete", 540, 680, workflow.Config{Reason: "converted"}),
	}, Edges: []workflow.Edge{edge("a", "start", "delay", "started"), edge("b", "delay", "check", "completed"), edge("c", "check", "converted", "true"), edge("d", "check", "email", "false"), edge("e", "email", "await", "next"), edge("f", "await", "converted", "matched"), edge("g", "await", "sms", "timeout"), edge("h", "sms", "done", "next")}}
	onboarding := workflow.Workflow{SchemaVersion: 1, GlobalExit: &workflow.ExitCriteria{EventName: "purchase_completed"}, Reentry: &workflow.Reentry{Mode: "once"}, Nodes: []workflow.Node{
		node("start", "event_trigger", "User registered", 300, 0, workflow.Config{EventName: "user_registered"}),
		node("welcome", "email", "Welcome email", 300, 160, workflow.Config{Subject: "Welcome, {{ customer.first_name }}", Preheader: "Your next chapter starts here", HTML: "<div style=\"font-family:Arial;padding:32px\"><h1>Welcome, {{ customer.first_name }}.</h1><p>We are glad you are here. Get started at your own pace.</p><a href=\"https://example.invalid/start\">Explore your account</a></div>", Text: "Welcome {{ customer.first_name }}. Get started: https://example.invalid/start", Fallbacks: fallback}),
		node("delay", "wait", "Wait 48 hours", 300, 320, workflow.Config{DurationSeconds: 172800}),
		node("active", "condition", "Account activated?", 300, 480, workflow.Config{Rule: &workflow.Rule{Path: "customer.attributes.activated", Operator: "equals", Value: true}}),
		node("reminder", "sms", "Activation reminder", 80, 660, workflow.Config{Text: "Hello {{ customer.first_name }}, ready to get started? https://example.invalid/start", Fallbacks: fallback}),
		node("done", "exit", "Onboarding complete", 80, 840, workflow.Config{Reason: "completed"}),
		node("converted", "exit", "Activated", 540, 660, workflow.Config{Reason: "converted"}),
	}, Edges: []workflow.Edge{edge("a", "start", "welcome", "started"), edge("b", "welcome", "delay", "next"), edge("c", "delay", "active", "completed"), edge("d", "active", "converted", "true"), edge("e", "active", "reminder", "false"), edge("f", "reminder", "done", "next")}}
	return []Template{{ID: "abandoned-cart", Name: "Abandoned Cart Recovery", Description: "Wait, check purchase and recover the cart with email/SMS. A purchase exits immediately.", Tags: []string{"commerce", "cart", "recovery"}, Workflow: cart}, {ID: "customer-onboarding", Name: "Customer Onboarding", Description: "Welcome new customers, wait 48 hours and remind inactive accounts.", Tags: []string{"onboarding", "activation"}, Workflow: onboarding}, recoveryEUR()}
}
func Find(id string) (Template, bool) {
	for _, t := range Builtins() {
		if t.ID == id {
			return t, true
		}
	}
	return Template{}, false
}
