package semantic

import (
	"encoding/json"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
	"strings"
	"testing"
)

func event(name, label string) Entry {
	e := Entry{Kind: "event", EventName: name, Label: label, DataType: "event", Aliases: []string{}, Enabled: true}
	_ = e.Validate()
	return e
}
func field(path, label string) Entry {
	e := Entry{Kind: "customer_field", Path: path, Label: label, DataType: "boolean", Aliases: []string{}, Enabled: true}
	_ = e.Validate()
	return e
}
func specimen() workflow.Workflow {
	return workflow.Workflow{SchemaVersion: 1, Nodes: []workflow.Node{{ID: "start", Type: "event_trigger", Config: workflow.Config{EventName: "signed_up"}}, {ID: "c", Type: "condition", Config: workflow.Config{Rule: &workflow.Rule{Path: "customer.attributes.active", Operator: "equals", Value: true}}}, {ID: "email", Type: "email", Config: workflow.Config{Subject: "Hi {{ customer.attributes.first_name }}", Text: "{{ customer.attributes.active }}", Fallbacks: map[string]string{"customer.attributes.first_name": "friend"}}}}, GlobalExit: &workflow.ExitCriteria{EventName: "activated", Filters: &workflow.Rule{Path: "event.properties.account_id", Operator: "exists"}}}
}
func TestGroundingMissingAmbiguousDisabledAndExplicitChoice(t *testing.T) {
	w := specimen()
	entries := append(Builtins(), event("signed_up", "Registration"), event("activated", "Activation"), field("customer.attributes.active", "Active"), field("customer.attributes.first_name", "First name"))
	r := Analyze(w, entries, nil)
	if r.Ready {
		t.Fatal("unknown global-exit filter accepted")
	}
	scoped := Entry{Kind: "event_field", EventName: "activated", Path: "event.properties.account_id", Label: "Account", DataType: "string", Enabled: true}
	_ = scoped.Validate()
	entries = append(entries, scoped)
	if !Analyze(w, entries, nil).Ready {
		t.Fatal("known references blocked")
	}
	other := field("customer.attributes.enabled", "Enabled")
	other.Aliases = []string{"Active"}
	entries = append(entries, other)
	if Analyze(w, entries, nil).Ready {
		t.Fatal("shared alias silently picked")
	}
	if !Analyze(w, entries, []string{Key("customer_field", "", "customer.attributes.active")}).Ready {
		t.Fatal("explicit choice not respected")
	}
	entries[8].Enabled = false // first custom event, after eight built-ins
	if Analyze(w, entries, []string{Key("event", "signed_up", "")}).Ready {
		t.Fatal("confirmation bypassed disabled event")
	}
}
func TestBindingsRewriteRulesVariablesFallbacksAndNeverMutateSource(t *testing.T) {
	w := specimen()
	before, _ := json.Marshal(w)
	entries := []Entry{event("registered", "Registration"), field("customer.attributes.activated", "Active"), field("customer.attributes.given_name", "First name")}
	out, err := Resolve(w, entries, []Binding{{Key("event", "signed_up", ""), entries[0].Key}, {Key("customer_field", "", "customer.attributes.active"), entries[1].Key}, {Key("customer_field", "", "customer.attributes.first_name"), entries[2].Key}})
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(w)
	if string(before) != string(after) {
		t.Fatal("source mutated")
	}
	if out.Nodes[0].Config.EventName != "registered" || out.Nodes[1].Config.Rule.Path != "customer.attributes.activated" || !strings.Contains(out.Nodes[2].Config.Subject, "customer.attributes.given_name") || out.Nodes[2].Config.Fallbacks["customer.attributes.given_name"] != "friend" {
		t.Fatal("incomplete rewrite")
	}
	if _, err = Resolve(w, entries, []Binding{{Key("event", "signed_up", ""), entries[1].Key}}); err == nil {
		t.Fatal("cross-kind mapping accepted")
	}
}
func TestCatalogRejectsUnsafePathsAndReferenceCoverage(t *testing.T) {
	for _, path := range []string{"customer.attributes.__proto__.x", "customer.attributes.constructor.x", "customer.email", "customer.attributes..x"} {
		e := field(path, "Unsafe")
		if e.Validate() == nil {
			t.Fatal(path)
		}
	}
	w := specimen()
	w.Nodes = append(w.Nodes, workflow.Node{ID: "update", Type: "update_attribute", Config: workflow.Config{Attribute: "score"}}, workflow.Node{ID: "wait", Type: "wait_event", Config: workflow.Config{EventName: "paid", Filters: &workflow.Rule{Path: "event.properties.total", Operator: "exists"}}})
	refs := References(w)
	keys := map[string]bool{}
	for _, r := range refs {
		keys[r.Key] = true
	}
	for _, key := range []string{Key("customer_field", "", "customer.attributes.score"), Key("event_field", "paid", "event.properties.total"), Key("event_field", "activated", "event.properties.account_id")} {
		if !keys[key] {
			t.Fatal("missing reference", key)
		}
	}
	wrongScope := Entry{Kind: "event_field", EventName: "signed_up", Path: "event.properties.total", Label: "Total", DataType: "number", Enabled: true}
	_ = wrongScope.Validate()
	if _, err := Resolve(w, []Entry{wrongScope}, []Binding{{Key("event_field", "paid", "event.properties.total"), wrongScope.Key}}); err == nil {
		t.Fatal("cross-event mapping accepted")
	}
}
func TestKnownFieldWithWrongScalarTypeCannotBeConfirmed(t *testing.T) {
	w := workflow.Workflow{Nodes: []workflow.Node{{Type: "condition", Config: workflow.Config{Rule: &workflow.Rule{Path: "customer.attributes.active", Operator: "equals", Value: "true"}}}}}
	e := field("customer.attributes.active", "Active")
	report := Analyze(w, []Entry{e}, []string{e.Key})
	if report.Ready || report.Mappings[0].Status != "type_mismatch" {
		t.Fatal("confirmation bypassed incompatible type")
	}
}
func TestConflictingFallbackMappingsAreRejectedInsteadOfChoosingRandomly(t *testing.T) {
	w := specimen()
	w.Nodes[2].Config.Text = "Hi {{ customer.attributes.name }}"
	w.Nodes[2].Config.Fallbacks["customer.attributes.name"] = "customer"
	e := field("customer.attributes.given_name", "First name")
	_, err := Resolve(w, []Entry{e}, []Binding{{Key("customer_field", "", "customer.attributes.first_name"), e.Key}, {Key("customer_field", "", "customer.attributes.name"), e.Key}})
	if err == nil {
		t.Fatal("conflicting fallbacks merged nondeterministically")
	}
}

func TestCorrelationReferencesKeepBothEventScopes(t *testing.T) {
	w := workflow.Workflow{Nodes: []workflow.Node{{Type: "event_trigger", Config: workflow.Config{EventName: "cart_started"}}}, GlobalExit: &workflow.ExitCriteria{EventName: "paid", Correlation: &workflow.EventCorrelation{EntryPath: "event.properties.cart_id", EventPath: "event.properties.order_id"}}}
	refs := References(w)
	keys := map[string]bool{}
	for _, r := range refs {
		keys[r.Key] = true
	}
	for _, key := range []string{Key("event_field", "cart_started", "event.properties.cart_id"), Key("event_field", "paid", "event.properties.order_id")} {
		if !keys[key] {
			t.Fatal("missing scoped identifier", key)
		}
	}
	target := Entry{Kind: "event_field", EventName: "paid", Path: "event.properties.purchased_cart", Label: "Paid cart", DataType: "string", Enabled: true}
	_ = target.Validate()
	resolved, err := Resolve(w, []Entry{target}, []Binding{{Key("event_field", "paid", "event.properties.order_id"), target.Key}})
	if err != nil || resolved.GlobalExit.Correlation.EventPath != target.Path || w.GlobalExit.Correlation.EventPath != "event.properties.order_id" {
		t.Fatal(resolved, err)
	}
}

func TestCorrelationCannotConfirmIncompatibleIdentifierTypes(t *testing.T) {
	w := workflow.Workflow{Nodes: []workflow.Node{{Type: "event_trigger", Config: workflow.Config{EventName: "cart"}}}, GlobalExit: &workflow.ExitCriteria{EventName: "paid", Correlation: &workflow.EventCorrelation{EntryPath: "event.properties.id", EventPath: "event.properties.id"}}}
	a := Entry{Kind: "event_field", EventName: "cart", Path: "event.properties.id", Label: "Entry cart", DataType: "string", Enabled: true}
	_ = a.Validate()
	b := Entry{Kind: "event_field", EventName: "paid", Path: "event.properties.id", Label: "Paid cart", DataType: "number", Enabled: true}
	_ = b.Validate()
	entries := []Entry{event("cart", "Cart"), event("paid", "Paid"), a, b}
	if Analyze(w, entries, []string{a.Key, b.Key}).Ready {
		t.Fatal("incompatible identity types were confirmed")
	}
	b.DataType = "string"
	entries[3] = b
	if !Analyze(w, entries, nil).Ready {
		t.Fatal("matching catalog identities were rejected")
	}
}
