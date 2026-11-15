package workflow

import (
	"fmt"
	"strings"
	"testing"
)

func valid() Workflow {
	return Workflow{SchemaVersion: 1, Nodes: []Node{{ID: "start", Type: "event_trigger", Label: "Cart updated", Config: Config{EventName: "cart_updated"}}, {ID: "exit", Type: "exit", Label: "Complete", Config: Config{Reason: "completed"}}}, Edges: []Edge{{ID: "edge", Source: "start", Target: "exit", Port: "started"}}}
}
func TestGraphValidation(t *testing.T) {
	if issues := Validate(valid()); len(issues) != 0 {
		t.Fatal(issues)
	}
	for _, tc := range []struct {
		name    string
		change  func(*Workflow)
		message string
	}{
		{"missing trigger", func(w *Workflow) { w.Nodes[0].Type = "exit" }, "Exactly one"},
		{"missing port", func(w *Workflow) { w.Edges = nil }, "Connect"},
		{"cycle", func(w *Workflow) {
			w.Nodes[1].Type = "wait"
			w.Nodes[1].Config.DurationSeconds = 1
			w.Edges = append(w.Edges, Edge{ID: "back", Source: "exit", Target: "start", Port: "completed"})
		}, "Cycles"},
		{"unknown node", func(w *Workflow) { w.Edges[0].Target = "missing" }, "unknown node"},
		{"unreachable", func(w *Workflow) {
			w.Nodes = append(w.Nodes, Node{ID: "extra", Type: "exit", Label: "Extra", Config: Config{Reason: "completed"}})
		}, "unreachable"},
		{"invalid email", func(w *Workflow) { w.Nodes[1].Type = "email" }, "Subject required"},
		{"split without a share", func(w *Workflow) { w.Nodes[1].Type = "split" }, "Variant A share"},
		{"webhook without an address", func(w *Workflow) { w.Nodes[1].Type = "webhook" }, "http or https address"},
		{"webhook with credentials in the address", func(w *Workflow) {
			w.Nodes[1].Type = "webhook"
			w.Nodes[1].Config.URL = "https://user:secret@example.invalid/hook"
		}, "without credentials"},
		{"webhook field from an unknown source", func(w *Workflow) {
			w.Nodes[1].Type = "webhook"
			w.Nodes[1].Config.URL = "https://example.invalid/hook"
			w.Nodes[1].Config.Fields = map[string]string{"x": "secrets.token"}
		}, "customer, event or context"},
		{"multi-way with one branch", func(w *Workflow) {
			w.Nodes[1].Type = "switch"
			w.Nodes[1].Config.Branches = []Branch{{Label: "Only", Rule: &Rule{Path: "customer.attributes.x", Operator: "exists"}}}
		}, "2 to 5 branches"},
		{"multi-way branch without a condition", func(w *Workflow) {
			w.Nodes[1].Type = "switch"
			w.Nodes[1].Config.Branches = []Branch{{Label: "A"}, {Label: "B", Rule: &Rule{Path: "customer.attributes.x", Operator: "exists"}}}
		}, "needs a condition"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := valid()
			tc.change(&w)
			found := false
			for _, issue := range Validate(w) {
				found = found || strings.Contains(issue.Message, tc.message)
			}
			if !found {
				t.Fatalf("missing %s in %v", tc.message, Validate(w))
			}
		})
	}
}
func TestConditions(t *testing.T) {
	data := map[string]any{"customer": map[string]any{"amount": 42.0, "name": "Ada", "date": "2026-11-10T00:00:00Z"}}
	for _, tc := range []struct {
		rule Rule
		want bool
	}{
		{Rule{Path: "customer.amount", Operator: "equals", Value: 42}, true}, {Rule{Path: "customer.amount", Operator: "greater_than", Value: 40}, true}, {Rule{Path: "customer.missing", Operator: "exists"}, false}, {Rule{Path: "customer.name", Operator: "contains", Value: "Ad"}, true}, {Rule{Path: "customer.date", Operator: "before", Value: "2026-11-11T00:00:00Z"}, true}, {Rule{Logic: "and", Rules: []Rule{{Path: "customer.name", Operator: "equals", Value: "Ada"}, {Path: "customer.amount", Operator: "less_than", Value: 2}}}, false},
	} {
		if got := Evaluate(tc.rule, data); got != tc.want {
			t.Fatalf("%+v: got %v", tc.rule, got)
		}
	}
}

func TestDraftShapeAllowsIncompleteGraphsButRejectsUnsafeShapes(t *testing.T) {
	for _, raw := range []string{`null`, `{"schema_version":1,"nodes":null,"edges":[]}`, `{"schema_version":1,"nodes":[],"edges":[],"execute_code":"no"}`, `{"schema_version":1,"nodes":[{"id":"bad","type":"execute_code","label":"Bad","config":{},"position":{"x":0,"y":0}}],"edges":[]}`} {
		if _, err := DecodeDraft([]byte(raw)); err == nil {
			t.Fatalf("invalid draft accepted: %s", raw)
		}
	}
	if _, err := DecodeDraft([]byte(`{"schema_version":1,"nodes":[],"edges":[]}`)); err != nil {
		t.Fatal(err)
	}
}

func TestInclusiveThresholdsHaveExactBoundaries(t *testing.T) {
	for _, tc := range []struct {
		op    string
		value float64
		want  bool
	}{
		{"greater_than_or_equal", 49.995, false}, {"greater_than_or_equal", 50, true}, {"greater_than_or_equal", 50.01, true},
		{"less_than_or_equal", 49.995, true}, {"less_than_or_equal", 50, true}, {"less_than_or_equal", 50.01, false},
		{"greater_than", 50, false}, {"less_than", 50, false},
	} {
		t.Run(tc.op+fmt.Sprint(tc.value), func(t *testing.T) {
			r := Rule{Path: "event.properties.amount", Operator: tc.op, Value: 50}
			w := valid()
			w.Nodes[0].Config.Filters = &r
			if len(Validate(w)) != 0 {
				t.Fatal("operator rejected")
			}
			if got := Evaluate(r, map[string]any{"event": map[string]any{"properties": map[string]any{"amount": tc.value}}}); got != tc.want {
				t.Fatal(got, tc.want)
			}
		})
	}
	if Evaluate(Rule{Path: "event.properties.amount", Operator: "greater_than_or_equal", Value: 50}, map[string]any{"event": map[string]any{"properties": map[string]any{"amount": "50"}}}) {
		t.Fatal("numeric strings were coerced")
	}
}
