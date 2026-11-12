package simulation

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

func cart() workflow.Workflow {
	return workflow.Workflow{SchemaVersion: 1, GlobalExit: &workflow.ExitCriteria{EventName: "purchase_completed"}, Nodes: []workflow.Node{
		{ID: "start", Type: "event_trigger", Label: "Cart updated", Config: workflow.Config{EventName: "cart_updated"}},
		{ID: "delay", Type: "wait", Label: "Wait 2 hours", Config: workflow.Config{DurationSeconds: 7200}},
		{ID: "check", Type: "condition", Label: "Purchased?", Config: workflow.Config{Rule: &workflow.Rule{Path: "customer.attributes.purchased", Operator: "equals", Value: true}}},
		{ID: "email", Type: "email", Label: "Cart reminder", Config: workflow.Config{Subject: "Your cart", Text: "Hello {{ customer.first_name }}"}},
		{ID: "await", Type: "wait_event", Label: "Wait for purchase", Config: workflow.Config{EventName: "purchase_completed", TimeoutSeconds: 86400}},
		{ID: "sms", Type: "sms", Label: "SMS reminder", Config: workflow.Config{Text: "Hello {{ customer.first_name }}, your cart is waiting."}},
		{ID: "done", Type: "exit", Label: "Complete", Config: workflow.Config{Reason: "completed"}},
		{ID: "converted", Type: "exit", Label: "Converted", Config: workflow.Config{Reason: "converted"}},
	}, Edges: []workflow.Edge{{ID: "a", Source: "start", Target: "delay", Port: "started"}, {ID: "b", Source: "delay", Target: "check", Port: "completed"}, {ID: "c", Source: "check", Target: "converted", Port: "true"}, {ID: "d", Source: "check", Target: "email", Port: "false"}, {ID: "e", Source: "email", Target: "await", Port: "next"}, {ID: "f", Source: "await", Target: "converted", Port: "matched"}, {ID: "g", Source: "await", Target: "sms", Port: "timeout"}, {ID: "h", Source: "sms", Target: "done", Port: "next"}}}
}
func scenario() Scenario {
	return Scenario{Name: "Cart recovery", StartTime: time.Date(2026, 11, 10, 10, 0, 0, 0, time.UTC), Customer: map[string]any{"customer_id": "test_customer", "email": "test@example.invalid", "phone": "+905551234567", "timezone": "Europe/Istanbul", "attributes": map[string]any{"first_name": "Ada", "purchased": false}, "consents": map[string]any{"email": true, "sms": true}}, Entry: Event{EventID: "cart_event", EventName: "cart_updated", Properties: map[string]any{"amount": 2450.0}}, Assertions: []Assertion{{Type: "state", Expected: "COMPLETED"}, {Type: "message_sent", Channel: "email"}, {Type: "message_sent", Channel: "sms"}, {Type: "condition", NodeID: "check", Expected: false}, {Type: "no_duplicate_delivery", Expected: true}}}
}
func complete(t *testing.T, w workflow.Workflow, input Scenario) State {
	t.Helper()
	state, spec, s, err := New(w, input)
	if err != nil {
		t.Fatal(err)
	}
	if err := Advance(spec, s, &state, "complete", 0); err != nil {
		t.Fatal(err)
	}
	return state
}
func TestAbandonedCartClockAndReplay(t *testing.T) {
	w := cart()
	input := scenario()
	first := complete(t, w, input)
	second := complete(t, w, input)
	if first.Result != "PASS" || len(first.Messages) != 2 || first.Now.Sub(input.StartTime) != 26*time.Hour {
		t.Fatal(first.Result, first.Status, first.Now, first.Messages)
	}
	if first.Messages[0].At.Sub(input.StartTime) != 2*time.Hour || first.Messages[1].At.Sub(input.StartTime) != 26*time.Hour {
		t.Fatal("wrong simulated send times")
	}
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(second)
	if string(a) != string(b) {
		t.Fatal("replay is nondeterministic")
	}
	if input.Customer["attributes"].(map[string]any)["first_name"] != "Ada" || len(w.Nodes) != 8 {
		t.Fatal("simulator mutated caller inputs")
	}
}
func TestEarlyPurchaseAndExactDeadlinePreventMessages(t *testing.T) {
	for _, after := range []int64{3600, 7200} {
		input := scenario()
		input.Events = []TimedEvent{{AfterSeconds: after, Event: Event{EventID: "purchase", EventName: "purchase_completed"}}}
		input.Assertions = []Assertion{{Type: "state", Expected: "EXITED"}, {Type: "exit_reason", Expected: "converted"}, {Type: "no_message", Channel: "email"}, {Type: "no_message", Channel: "sms"}}
		state := complete(t, cart(), input)
		if state.Result != "PASS" || len(state.Messages) != 0 {
			t.Fatalf("deadline %d: %+v", after, state)
		}
	}
}
func TestProviderFaultRetryAndCapAreIsolated(t *testing.T) {
	input := scenario()
	input.Faults = []int{429, 500, 200}
	state := complete(t, cart(), input)
	if state.Result != "PASS" || len(state.Messages[0].Attempts) != 3 || state.Messages[0].Attempts[1].At.Sub(state.Messages[0].At) != 2*time.Second || state.Messages[0].Attempts[2].At.Sub(state.Messages[0].At) != 6*time.Second {
		t.Fatal(state.Messages)
	}
	input = scenario()
	input.PriorMessages = 3
	input.Assertions = []Assertion{{Type: "no_message", Channel: "email"}, {Type: "message_sent", Channel: "sms"}, {Type: "state", Expected: "COMPLETED"}}
	state = complete(t, cart(), input)
	if state.Result != "PASS" || state.Messages[0].Status != "BLOCKED" || state.Messages[0].Decision.Reason != "frequency_cap" {
		t.Fatal(state.Result, state.Messages)
	}
}
func TestDuplicateEventsAndPauseStep(t *testing.T) {
	input := scenario()
	duplicate := input.Entry
	duplicate.OccurredAt = input.StartTime
	input.Events = []TimedEvent{{AfterSeconds: 0, Event: duplicate}}
	input.Assertions = append(input.Assertions, Assertion{Type: "enrollment_count", Expected: 1})
	state, spec, s, err := New(cart(), input)
	if err != nil {
		t.Fatal(err)
	}
	if err := Advance(spec, s, &state, "run", 0); err != nil {
		t.Fatal(err)
	}
	if state.Status != "WAITING" {
		t.Fatal(state.Status)
	}
	Advance(spec, s, &state, "pause", 0)
	if !state.Paused {
		t.Fatal("pause missing")
	}
	Advance(spec, s, &state, "advance", 3600)
	if state.Now.Sub(input.StartTime) != time.Hour || len(state.Messages) != 0 {
		t.Fatal("clock advance crossed a future timer")
	}
	Advance(spec, s, &state, "complete", 0)
	if state.Result != "PASS" || state.EnrollmentCount != 1 {
		t.Fatal(state.Result, state.EnrollmentCount)
	}
}
func TestSingleNodeAndAttributeUpdatesDoNotMutateSource(t *testing.T) {
	input := scenario()
	input.Mode = "single_node"
	input.NodeID = "email"
	input.Assertions = []Assertion{{Type: "message_sent", Channel: "email"}, {Type: "no_message", Channel: "sms"}}
	state := complete(t, cart(), input)
	if state.Result != "PASS" || len(state.Messages) != 1 {
		t.Fatal(state.Result, state.Messages)
	}
	w := cart()
	w.Nodes[2].Type = "update_attribute"
	w.Nodes[2].Config = workflow.Config{Attribute: "first_name", Operation: "set", Value: "Grace"}
	w.Edges = []workflow.Edge{{ID: "a", Source: "start", Target: "check", Port: "started"}, {ID: "b", Source: "check", Target: "email", Port: "next"}, {ID: "c", Source: "email", Target: "done", Port: "next"}}
	w.Nodes = []workflow.Node{w.Nodes[0], w.Nodes[2], w.Nodes[3], w.Nodes[6]}
	input = scenario()
	input.Assertions = []Assertion{{Type: "message_sent", Channel: "email"}}
	state = complete(t, w, input)
	if state.Messages[0].Rendered.Text != "Hello Grace" || input.Customer["attributes"].(map[string]any)["first_name"] != "Ada" {
		t.Fatal("attribute update isolation failed", state.Messages)
	}
}
func TestThousandSandboxCustomersLeaveInputsUnchanged(t *testing.T) {
	input := scenario()
	before, _ := json.Marshal(input)
	for range 1000 {
		state := complete(t, cart(), input)
		if state.Result != "PASS" {
			t.Fatal(state.Result)
		}
	}
	after, _ := json.Marshal(input)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("batch simulation mutated shared profile/scenario")
	}
}

func TestExplicitGlobalCancellationDoesNotConvert(t *testing.T) {
	for _, reason := range []string{"completed", "condition_not_met", "manually_cancelled", "converted", ""} {
		t.Run(reason, func(t *testing.T) {
			w := cart()
			w.GlobalExit = &workflow.ExitCriteria{EventName: "order_refunded", Reason: reason}
			s := scenario()
			s.Events = []TimedEvent{{AfterSeconds: 1800, Event: Event{EventID: "refund", EventName: "order_refunded"}}}
			s.Assertions = []Assertion{{Type: "no_message"}}
			state := complete(t, w, s)
			want := reason
			if want == "" {
				want = "converted"
			}
			if state.ExitReason != want || state.Status != "EXITED" || len(state.Messages) != 0 {
				t.Fatalf("outcome=%s state=%s", state.ExitReason, state.Status)
			}
		})
	}
}

func TestGlobalExitUsesOriginalEntryIdentityAcrossPersistence(t *testing.T) {
	for _, tc := range []struct {
		name, entry, paid string
		want              string
		messages          int
	}{
		{"matching", "A", "A", "converted", 0}, {"other_cart", "A", "B", "completed", 1},
		{"missing_incoming", "A", "", "completed", 1}, {"missing_entry", "", "A", "completed", 1},
		{"both_missing", "", "", "completed", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := cart()
			w.Nodes = []workflow.Node{w.Nodes[0], w.Nodes[1], w.Nodes[3], w.Nodes[6]}
			w.Edges = []workflow.Edge{{ID: "a", Source: "start", Target: "delay", Port: "started"}, {ID: "b", Source: "delay", Target: "email", Port: "completed"}, {ID: "c", Source: "email", Target: "done", Port: "next"}}
			w.GlobalExit.Correlation = &workflow.EventCorrelation{EntryPath: "event.properties.cart_id", EventPath: "event.properties.cart_id"}
			input := scenario()
			input.Entry.Properties = map[string]any{}
			if tc.entry != "" {
				input.Entry.Properties["cart_id"] = tc.entry
			}
			paid := map[string]any{}
			if tc.paid != "" {
				paid["cart_id"] = tc.paid
			}
			input.Events = []TimedEvent{{AfterSeconds: 3600, Event: Event{EventID: "paid", EventName: "purchase_completed", Properties: paid}}}
			input.Assertions = []Assertion{{Type: "exit_reason", Expected: tc.want}, {Type: "message_count", Channel: "email", Expected: tc.messages}}
			state, spec, in, err := New(w, input)
			if err != nil {
				t.Fatal(err)
			}
			if err = Advance(spec, in, &state, "advance", 1800); err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(state)
			state = State{}
			if err = json.Unmarshal(raw, &state); err != nil {
				t.Fatal(err)
			}
			// Current/resume events cannot replace the immutable enrollment identity.
			state.CurrentEvent = map[string]any{"properties": map[string]any{"cart_id": "B"}}
			if err = Advance(spec, in, &state, "complete", 0); err != nil {
				t.Fatal(err)
			}
			if state.Result != "PASS" {
				t.Fatal(state.ExitReason, state.Assertions)
			}
		})
	}
}

func TestCorrelatedGoalAlsoScopesWaitForThatEvent(t *testing.T) {
	for _, id := range []string{"A", "B"} {
		t.Run(id, func(t *testing.T) {
			w := cart()
			w.GlobalExit.Correlation = &workflow.EventCorrelation{EntryPath: "event.properties.cart_id", EventPath: "event.properties.cart_id"}
			input := scenario()
			input.Entry.Properties = map[string]any{"cart_id": "A"}
			input.Events = []TimedEvent{{AfterSeconds: 10800, Event: Event{EventID: "purchase", EventName: "purchase_completed", Properties: map[string]any{"cart_id": id}}}}
			reason, sms := "completed", 1
			if id == "A" {
				reason, sms = "converted", 0
			}
			input.Assertions = []Assertion{{Type: "exit_reason", Expected: reason}, {Type: "message_count", Channel: "email", Expected: 1}, {Type: "message_count", Channel: "sms", Expected: sms}}
			state := complete(t, w, input)
			if state.Result != "PASS" {
				t.Fatal(state.Assertions)
			}
		})
	}
}
