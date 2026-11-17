package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// Human-supplied synthetic expectations are metadata, never runtime instructions.
// Providers cannot return or overwrite this field in the structured-output schema.
type BehaviorChecks struct {
	Version    int            `json:"version" minimum:"1" maximum:"2"`
	Confirmed  bool           `json:"confirmed"`
	EntryEvent string         `json:"entry_event"`
	Inputs     map[string]any `json:"inputs,omitempty"`
	EmailCount *int           `json:"email_count,omitempty" minimum:"0" maximum:"100"`
	SMSCount   *int           `json:"sms_count,omitempty" minimum:"0" maximum:"100"`
	// PushCount is checked only when set, so checks written before push existed keep passing.
	PushCount          *int                `json:"push_count,omitempty" minimum:"0" maximum:"100"`
	Outcome            string              `json:"outcome" enum:"completed,converted,condition_not_met,manually_cancelled"`
	DurationSeconds    *int64              `json:"duration_seconds,omitempty" minimum:"0" maximum:"31536000"`
	Thresholds         []BehaviorThreshold `json:"thresholds,omitempty" maxItems:"4"`
	CaseName           string              `json:"case_name,omitempty" maxLength:"160"`
	Conditions         []Rule              `json:"conditions,omitempty" maxItems:"8"`
	Consents           map[string]bool     `json:"consents,omitempty"`
	Events             []BehaviorEvent     `json:"events,omitempty" maxItems:"8"`
	ExpectedAttributes map[string]any      `json:"expected_attributes,omitempty"`
	IneligibleOutcome  string              `json:"ineligible_outcome,omitempty" enum:"completed,condition_not_met,manually_cancelled"`
	Alternatives       []BehaviorCase      `json:"alternatives,omitempty" maxItems:"7"`
}
type BehaviorThreshold struct {
	Path     string  `json:"path" maxLength:"200"`
	Operator string  `json:"operator" enum:"greater_than,greater_than_or_equal,less_than,less_than_or_equal"`
	Value    float64 `json:"value" minimum:"-1000000000000" maximum:"1000000000000"`
}

func BehaviorInputPath(path string) bool {
	if strings.HasPrefix(path, "customer.attributes.") {
		path = "event.properties." + strings.TrimPrefix(path, "customer.attributes.")
	}
	return validCorrelationPath(path)
}
func ValidateBehaviorChecks(c *BehaviorChecks) error {
	if c == nil {
		return fmt.Errorf("Define independent behavior checks before generating a general journey")
	}
	if (c.Version != 1 && c.Version != 2) || !c.Confirmed {
		return fmt.Errorf("Review and confirm the independent behavior checks")
	}
	if !eventName.MatchString(c.EntryEvent) {
		return fmt.Errorf("Choose the expected entry event")
	}
	if c.EmailCount == nil || c.SMSCount == nil || *c.EmailCount < 0 || *c.SMSCount < 0 || *c.EmailCount > 100 || *c.SMSCount > 100 || !optionalCount(c.PushCount) {
		return fmt.Errorf("Expected channel counts must be between 0 and 100")
	}
	switch c.Outcome {
	case "completed", "converted", "condition_not_met", "manually_cancelled":
	default:
		return fmt.Errorf("Choose the expected outcome")
	}
	if c.DurationSeconds != nil && (*c.DurationSeconds < 0 || *c.DurationSeconds > 31536000) {
		return fmt.Errorf("Expected duration is out of range")
	}
	if len(c.Inputs) > 50 || len(c.Thresholds) > 4 {
		return fmt.Errorf("Behavior checks exceed the input/threshold limit")
	}
	raw, err := json.Marshal(c)
	if err != nil || len(raw) > 16384 {
		return fmt.Errorf("Behavior checks exceed 16 KiB")
	}
	for path, v := range c.Inputs {
		if !BehaviorInputPath(path) {
			return fmt.Errorf("Invalid synthetic input path")
		}
		switch x := v.(type) {
		case nil, bool:
		case string:
			if len(x) > 512 {
				return fmt.Errorf("Synthetic text exceeds 512 bytes")
			}
		case float64:
			if math.IsNaN(x) || math.IsInf(x, 0) {
				return fmt.Errorf("Finite synthetic values required")
			}
		case int, int64, json.Number:
		default:
			return fmt.Errorf("Synthetic inputs must be scalar values")
		}
	}
	for _, t := range c.Thresholds {
		if !BehaviorInputPath(t.Path) || math.IsNaN(t.Value) || math.IsInf(t.Value, 0) || math.Abs(t.Value) > 1e12 {
			return fmt.Errorf("Invalid eligibility threshold")
		}
		switch t.Operator {
		case "greater_than", "greater_than_or_equal", "less_than", "less_than_or_equal":
		default:
			return fmt.Errorf("Use an exact numeric threshold operator")
		}
		if !Evaluate(Rule{Path: "event.value", Operator: t.Operator, Value: t.Value}, map[string]any{"event": map[string]any{"value": c.Inputs[t.Path]}}) {
			return fmt.Errorf("The synthetic example must satisfy every reviewed eligibility threshold")
		}

	}
	if c.Version == 1 {
		if c.CaseName != "" || len(c.Conditions)+len(c.Events)+len(c.Consents)+len(c.Alternatives)+len(c.ExpectedAttributes) > 0 || c.IneligibleOutcome != "" {
			return fmt.Errorf("Scoped clauses require behavior contract version 2")
		}
		return nil
	}
	if (len(c.Thresholds) > 0 || c.IneligibleOutcome != "") && c.IneligibleOutcome != "completed" && c.IneligibleOutcome != "condition_not_met" && c.IneligibleOutcome != "manually_cancelled" {
		return fmt.Errorf("Specify the outcome for ineligible inputs")
	}
	if len(c.Alternatives) > 7 {
		return fmt.Errorf("Behavior contract supports at most eight scoped clauses")
	}
	clauses := c.Cases()
	for i, clause := range clauses {
		if err := validateBehaviorCase(c, clause); err != nil {
			if clause.Name != "" {
				return fmt.Errorf("Situation %q: %w", clause.Name, err)
			}
			return fmt.Errorf("Clause %d: %w", i+1, err)
		}
		for j := 0; j < i; j++ {
			a, _ := json.Marshal(struct {
				Data   map[string]any
				Events []BehaviorEvent `json:"events,omitempty"`
			}{BehaviorCaseData(c, clause), clause.Events})
			b, _ := json.Marshal(struct {
				Data   map[string]any
				Events []BehaviorEvent `json:"events,omitempty"`
			}{BehaviorCaseData(c, clauses[j]), clauses[j].Events})
			if bytes.Equal(a, b) && behaviorExpectationsConflict(clause, clauses[j]) {
				return fmt.Errorf("Clauses %d and %d give contradictory outcomes for the same initial state and event sequence", j+1, i+1)
			}
		}
	}
	return nil
}

// A clause is a scoped obligation, not a global message count. Properties and
// example inputs remain private; conditions and event timing are reviewed rules.
type BehaviorCase struct {
	Name               string          `json:"name" maxLength:"160"`
	Conditions         []Rule          `json:"conditions,omitempty" maxItems:"8"`
	Inputs             map[string]any  `json:"inputs,omitempty"`
	Consents           map[string]bool `json:"consents,omitempty"`
	Events             []BehaviorEvent `json:"events,omitempty" maxItems:"8"`
	EmailCount         *int            `json:"email_count,omitempty" minimum:"0" maximum:"100"`
	SMSCount           *int            `json:"sms_count,omitempty" minimum:"0" maximum:"100"`
	PushCount          *int            `json:"push_count,omitempty" minimum:"0" maximum:"100"`
	Outcome            string          `json:"outcome" enum:"completed,converted,condition_not_met,manually_cancelled"`
	DurationSeconds    *int64          `json:"duration_seconds,omitempty" minimum:"0" maximum:"31536000"`
	ExpectedAttributes map[string]any  `json:"expected_attributes,omitempty"`
}
type BehaviorBinding struct {
	EntryPath string `json:"entry_path" maxLength:"200"`
	EventPath string `json:"event_path" maxLength:"200"`
	Operator  string `json:"operator" enum:"equals,not_equals"`
}
type BehaviorEvent struct {
	EntryBinding *BehaviorBinding `json:"entry_binding,omitempty"`
	EventName    string           `json:"event_name"`
	AfterSeconds int64            `json:"after_seconds" minimum:"0" maximum:"31536000"`
	Properties   map[string]any   `json:"properties,omitempty"`
}

func (c *BehaviorChecks) Cases() []BehaviorCase {
	return append([]BehaviorCase{{Name: c.CaseName, Conditions: c.Conditions, Inputs: c.Inputs, Consents: c.Consents, Events: c.Events, EmailCount: c.EmailCount, SMSCount: c.SMSCount, PushCount: c.PushCount, Outcome: c.Outcome, DurationSeconds: c.DurationSeconds, ExpectedAttributes: c.ExpectedAttributes}}, c.Alternatives...)
}
func BehaviorCaseData(c *BehaviorChecks, clause BehaviorCase) map[string]any {
	data := map[string]any{"customer": map[string]any{"customer_id": "behavior_check", "email": "preview@example.invalid", "phone": "+15005550006", "push_token": "behavior-check-device", "attributes": map[string]any{"first_name": "Sample"}, "consents": map[string]any{"email": true, "sms": true, "push": true}}, "event": map[string]any{"properties": map[string]any{}}}
	set := func(path string, v any) {
		parts := strings.Split(path, ".")
		m := data
		for _, part := range parts[:len(parts)-1] {
			next, ok := m[part].(map[string]any)
			if !ok {
				next = map[string]any{}
				m[part] = next
			}
			m = next
		}
		m[parts[len(parts)-1]] = v
	}
	for p, v := range c.Inputs {
		set(p, v)
	}
	for p, v := range clause.Inputs {
		set(p, v)
	}
	for p, v := range c.Consents {
		set("customer.consents."+p, v)
	}
	for p, v := range clause.Consents {
		set("customer.consents."+p, v)
	}
	return data
}
func validBehaviorScalar(v any) bool {
	switch x := v.(type) {
	case nil, bool:
		return true
	case string:
		return len(x) <= 512
	case float64:
		return !math.IsNaN(x) && !math.IsInf(x, 0)
	case int, int64:
		return true
	case json.Number:
		n, e := x.Float64()
		return e == nil && !math.IsNaN(n) && !math.IsInf(n, 0)
	}
	return false
}
func validateBehaviorCase(c *BehaviorChecks, clause BehaviorCase) error {
	if strings.TrimSpace(clause.Name) == "" || len(clause.Name) > 160 {
		return fmt.Errorf("Name the situation this clause describes")
	}
	if clause.EmailCount == nil || clause.SMSCount == nil || *clause.EmailCount < 0 || *clause.EmailCount > 100 || *clause.SMSCount < 0 || *clause.SMSCount > 100 || !optionalCount(clause.PushCount) {
		return fmt.Errorf("Specify channel counts for this clause")
	}
	switch clause.Outcome {
	case "completed", "converted", "condition_not_met", "manually_cancelled":
	default:
		return fmt.Errorf("Specify this clause's outcome")
	}
	if clause.DurationSeconds != nil && (*clause.DurationSeconds < 0 || *clause.DurationSeconds > 31536000) {
		return fmt.Errorf("Clause duration out of range")
	}
	if conflictingBehaviorPaths(clause.Inputs) || conflictingBehaviorPaths(clause.ExpectedAttributes) {
		return fmt.Errorf("Private example paths conflict")
	}
	if len(clause.Conditions) > 8 || len(clause.Events) > 8 || len(clause.Inputs) > 50 || len(clause.ExpectedAttributes) > 50 || len(clause.Consents) > len(MessageChannels) {
		return fmt.Errorf("Clause exceeds bounded limits")
	}
	for p, v := range clause.Inputs {
		if !BehaviorInputPath(p) || !validBehaviorScalar(v) {
			return fmt.Errorf("Invalid private example input")
		}
	}
	for p, v := range clause.ExpectedAttributes {
		if !BehaviorInputPath("customer.attributes."+p) || !validBehaviorScalar(v) {
			return fmt.Errorf("Invalid expected attribute")
		}
	}
	for p := range clause.Consents {
		if !IsMessage(p) {
			return fmt.Errorf("Unsupported channel consent")
		}
	}
	data := BehaviorCaseData(c, clause)
	for _, r := range clause.Conditions {
		if len(r.Rules) > 0 || r.Logic != "" || (!BehaviorInputPath(r.Path) && r.Path != "customer.consents.email" && r.Path != "customer.consents.sms" && r.Path != "customer.consents.push") || !validBehaviorScalar(r.Value) {
			return fmt.Errorf("Scope conditions must be bounded scalar predicates")
		}
		if err := validateRule(r, 0); err != nil {
			return err
		}
		if !Evaluate(r, data) {
			return fmt.Errorf("Private example contradicts the clause's reviewed conditions")
		}
	}
	var last int64
	for _, e := range clause.Events {
		if conflictingBehaviorPaths(e.Properties) {
			return fmt.Errorf("Private event property paths conflict")
		}
		if !eventName.MatchString(e.EventName) || e.AfterSeconds < last || e.AfterSeconds > 31536000 || len(e.Properties) > 50 {
			return fmt.Errorf("Invalid or unordered event sequence")
		}
		last = e.AfterSeconds
		for p, v := range e.Properties {
			if !BehaviorInputPath("event.properties."+p) || !validBehaviorScalar(v) {
				return fmt.Errorf("Invalid private event property")
			}
		}
		if binding := e.EntryBinding; binding != nil {
			if !validCorrelationPath(binding.EntryPath) || !validCorrelationPath(binding.EventPath) || (binding.Operator != "equals" && binding.Operator != "not_equals") {
				return fmt.Errorf("Invalid event identity relationship")
			}
			entry := data["event"].(map[string]any)["properties"].(map[string]any)
			incoming := e.Payload()
			validEntry := (&ExitCriteria{Correlation: &EventCorrelation{EntryPath: binding.EntryPath, EventPath: binding.EntryPath}}).MatchesEntry(entry, entry)
			validEvent := (&ExitCriteria{Correlation: &EventCorrelation{EntryPath: binding.EventPath, EventPath: binding.EventPath}}).MatchesEntry(incoming, incoming)
			matches := (&ExitCriteria{Correlation: &EventCorrelation{EntryPath: binding.EntryPath, EventPath: binding.EventPath}}).MatchesEntry(entry, incoming)
			if !validEntry || !validEvent || matches != (binding.Operator == "equals") {
				relation := "the same"
				if binding.Operator == "not_equals" {
					relation = "a different"
				}
				return fmt.Errorf("the example %s event needs %s %s as the entry; set it under Private event example", e.EventName, relation, strings.TrimPrefix(binding.EventPath, "event.properties."))
			}
		}
	}
	return nil
}

func behaviorExpectationsConflict(a, b BehaviorCase) bool {
	if *a.EmailCount != *b.EmailCount || *a.SMSCount != *b.SMSCount || a.Outcome != b.Outcome {
		return true
	}
	if a.PushCount != nil && b.PushCount != nil && *a.PushCount != *b.PushCount {
		return true
	}
	if a.DurationSeconds != nil && b.DurationSeconds != nil && *a.DurationSeconds != *b.DurationSeconds {
		return true
	}
	for p, v := range a.ExpectedAttributes {
		if other, ok := b.ExpectedAttributes[p]; ok {
			x, _ := json.Marshal(v)
			y, _ := json.Marshal(other)
			if !bytes.Equal(x, y) {
				return true
			}
		}
	}
	return false
}

// optionalCount accepts a missing count or one between 0 and 100.
func optionalCount(n *int) bool { return n == nil || (*n >= 0 && *n <= 100) }

func (e BehaviorEvent) Payload() map[string]any {
	data := map[string]any{}
	for path, value := range e.Properties {
		parts := strings.Split(path, ".")
		m := data
		for _, part := range parts[:len(parts)-1] {
			next, ok := m[part].(map[string]any)
			if !ok {
				next = map[string]any{}
				m[part] = next
			}
			m = next
		}
		m[parts[len(parts)-1]] = value
	}
	return data
}

func conflictingBehaviorPaths(values map[string]any) bool {
	for a := range values {
		for b := range values {
			if a != b && strings.HasPrefix(a, b+".") {
				return true
			}
		}
	}
	return false
}
