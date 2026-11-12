// Package workflow defines the only executable node vocabulary. AI and humans
// produce the same validated specification; neither can inject executable code.
package workflow

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
)

type Rule struct {
	Logic    string `json:"logic,omitempty" enum:"and,or"`
	Rules    []Rule `json:"rules,omitempty" maxItems:"50"`
	Path     string `json:"path,omitempty" maxLength:"200"`
	Operator string `json:"operator,omitempty" enum:"equals,not_equals,greater_than,less_than,contains,exists,before,after"`
	Value    any    `json:"value,omitempty"`
}
type Config struct {
	EventName       string            `json:"event_name,omitempty" maxLength:"80"`
	Filters         *Rule             `json:"filters,omitempty"`
	DurationSeconds int64             `json:"duration_seconds,omitempty"`
	TimeoutSeconds  int64             `json:"timeout_seconds,omitempty"`
	Timezone        string            `json:"timezone,omitempty"`
	Rule            *Rule             `json:"rule,omitempty"`
	Subject         string            `json:"subject,omitempty" maxLength:"998"`
	Preheader       string            `json:"preheader,omitempty" maxLength:"300"`
	HTML            string            `json:"html,omitempty" maxLength:"200000"`
	Text            string            `json:"text,omitempty" maxLength:"20000"`
	TemplateID      string            `json:"template_id,omitempty"`
	Fallbacks       map[string]string `json:"fallbacks,omitempty"`
	Attribute       string            `json:"attribute,omitempty"`
	Operation       string            `json:"operation,omitempty" enum:"set,increment,unset"`
	Value           any               `json:"value,omitempty"`
	Reason          string            `json:"reason,omitempty"`
}
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}
type Node struct {
	ID       string   `json:"id" minLength:"1" maxLength:"80"`
	Type     string   `json:"type" enum:"event_trigger,wait,wait_event,condition,email,sms,update_attribute,exit"`
	Label    string   `json:"label" minLength:"1" maxLength:"160"`
	Position Position `json:"position"`
	Config   Config   `json:"config"`
}
type Edge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Port   string `json:"port"`
}
type EventCorrelation struct {
	EntryPath string `json:"entry_path" maxLength:"200"`
	EventPath string `json:"event_path" maxLength:"200"`
}
type ExitCriteria struct {
	Correlation *EventCorrelation `json:"correlation,omitempty"`
	Reason      string            `json:"reason,omitempty" enum:"converted,completed,condition_not_met,manually_cancelled"`
	EventName   string            `json:"event_name"`
	Filters     *Rule             `json:"filters,omitempty"`
}
type Reentry struct {
	Mode            string `json:"mode" enum:"once,after_completion,cooldown"`
	CooldownSeconds int64  `json:"cooldown_seconds,omitempty"`
}
type Workflow struct {
	AcceptanceContract string        `json:"acceptance_contract,omitempty" enum:"cart-recovery-eur.v1"`
	SchemaVersion      int           `json:"schema_version" minimum:"1" maximum:"1"`
	Nodes              []Node        `json:"nodes" maxItems:"100"`
	Edges              []Edge        `json:"edges" maxItems:"200"`
	GlobalExit         *ExitCriteria `json:"global_exit,omitempty"`
	Reentry            *Reentry      `json:"reentry,omitempty"`
}
type Issue struct {
	NodeID  string `json:"node_id,omitempty"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

var identifier = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,80}$`)
var eventName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,79}$`)
var attribute = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{0,79}$`)

func Ports(kind string) []string {
	switch kind {
	case "event_trigger":
		return []string{"started"}
	case "wait":
		return []string{"completed"}
	case "wait_event":
		return []string{"matched", "timeout"}
	case "condition":
		return []string{"true", "false"}
	case "email", "sms", "update_attribute":
		return []string{"next"}
	case "exit":
		return []string{}
	default:
		return nil
	}
}
func Validate(w Workflow) []Issue {
	issues := []Issue{}
	add := func(id, field, message string) { issues = append(issues, Issue{id, field, message}) }
	if w.SchemaVersion != 1 {
		add("", "schema_version", "Only schema version 1 is supported")
	}
	if len(w.Nodes) == 0 || len(w.Nodes) > 100 || len(w.Edges) > 200 {
		add("", "nodes", "Workflow requires 1–100 nodes and at most 200 edges")
		return issues
	}
	nodes := map[string]Node{}
	roots := []string{}
	for _, n := range w.Nodes {
		if !identifier.MatchString(n.ID) {
			add(n.ID, "id", "Use a unique alphanumeric node ID")
		}
		if _, exists := nodes[n.ID]; exists {
			add(n.ID, "id", "Duplicate node ID")
		}
		nodes[n.ID] = n
		if strings.TrimSpace(n.Label) == "" || len(n.Label) > 160 {
			add(n.ID, "label", "Node label is required (maximum 160 characters)")
		}
		if Ports(n.Type) == nil {
			add(n.ID, "type", "Unsupported node type")
		}
		if math.IsNaN(n.Position.X) || math.IsNaN(n.Position.Y) || math.IsInf(n.Position.X, 0) || math.IsInf(n.Position.Y, 0) {
			add(n.ID, "position", "Finite coordinates required")
		}
		c := n.Config
		if len(c.HTML) > 200000 || len(c.Text) > 20000 || len(c.Preheader) > 300 || len(c.Fallbacks) > 100 {
			add(n.ID, "content", "Message exceeds configured size limits")
		}
		switch n.Type {
		case "event_trigger", "wait_event":
			if n.Type == "event_trigger" {
				roots = append(roots, n.ID)
			}
			if !eventName.MatchString(c.EventName) {
				add(n.ID, "event_name", "Valid event name required")
			}
			if n.Type == "wait_event" && (c.TimeoutSeconds < 1 || c.TimeoutSeconds > 31536000) {
				add(n.ID, "timeout_seconds", "Timeout must be between 1 second and 365 days")
			}
		case "wait":
			if c.DurationSeconds < 1 || c.DurationSeconds > 31536000 {
				add(n.ID, "duration_seconds", "Delay must be between 1 second and 365 days")
			}
			if c.Timezone != "" {
				if _, err := time.LoadLocation(c.Timezone); err != nil {
					add(n.ID, "timezone", "Unknown timezone")
				}
			}
		case "condition":
			if c.Rule == nil {
				add(n.ID, "rule", "Condition required")
			}
		case "email":
			if strings.TrimSpace(c.Subject) == "" || len(c.Subject) > 998 {
				add(n.ID, "subject", "Subject required (maximum 998 bytes)")
			}
			if c.HTML == "" && c.Text == "" {
				add(n.ID, "content", "Email content required")
			}
		case "sms":
			if strings.TrimSpace(c.Text) == "" || len(c.Text) > 10000 {
				add(n.ID, "text", "SMS body required (maximum 10000 bytes)")
			}
		case "update_attribute":
			if !attribute.MatchString(c.Attribute) {
				add(n.ID, "attribute", "Valid custom attribute required")
			}
			if c.Operation != "set" && c.Operation != "increment" && c.Operation != "unset" {
				add(n.ID, "operation", "Use set, increment or unset")
			}
			if c.Operation == "increment" {
				if _, ok := number(c.Value); !ok {
					add(n.ID, "value", "Increment requires a numeric value")
				}
			}
		case "exit":
			if c.Reason == "" {
				add(n.ID, "reason", "Exit reason required")
			}
		}
		for _, r := range []*Rule{c.Rule, c.Filters} {
			if r != nil {
				if err := validateRule(*r, 0); err != nil {
					add(n.ID, "rule", err.Error())
				}
			}
		}
	}
	if len(roots) != 1 {
		add("", "nodes", "Exactly one Event Trigger is required")
	}
	edges := map[string][]Edge{}
	ports := map[string]bool{}
	edgeIDs := map[string]bool{}
	for _, e := range w.Edges {
		source, ok := nodes[e.Source]
		_, targetOK := nodes[e.Target]
		if !ok || !targetOK {
			add(e.Source, "edges", "Edge references an unknown node")
			continue
		}
		if edgeIDs[e.ID] || e.ID == "" {
			add(e.Source, "edges", "Unique edge ID required")
		}
		edgeIDs[e.ID] = true
		valid := false
		for _, p := range Ports(source.Type) {
			if p == e.Port {
				valid = true
			}
		}
		if !valid {
			add(e.Source, "edges", "Invalid output port: "+e.Port)
		}
		key := e.Source + ":" + e.Port
		if ports[key] {
			add(e.Source, "edges", "Each output port must have exactly one connection")
		}
		ports[key] = true
		if nodes[e.Target].Type == "event_trigger" {
			add(e.Target, "edges", "Trigger cannot have incoming edges")
		}
		edges[e.Source] = append(edges[e.Source], e)
	}
	for _, n := range w.Nodes {
		for _, port := range Ports(n.Type) {
			if !ports[n.ID+":"+port] {
				add(n.ID, "edges", "Connect the "+port+" output")
			}
		}
	}
	visited := map[string]int{}
	var visit func(string)
	visit = func(id string) {
		if visited[id] == 1 {
			add(id, "edges", "Cycles are not supported")
			return
		}
		if visited[id] == 2 {
			return
		}
		visited[id] = 1
		for _, e := range edges[id] {
			visit(e.Target)
		}
		visited[id] = 2
	}
	for id := range nodes {
		visit(id)
	}
	if len(roots) == 1 {
		reachable := map[string]bool{}
		queue := []string{roots[0]}
		for len(queue) > 0 {
			id := queue[0]
			queue = queue[1:]
			if reachable[id] {
				continue
			}
			reachable[id] = true
			for _, e := range edges[id] {
				queue = append(queue, e.Target)
			}
		}
		hasExit := false
		for id, n := range nodes {
			if !reachable[id] {
				add(id, "edges", "Node is unreachable")
			}
			if reachable[id] && n.Type == "exit" {
				hasExit = true
			}
		}
		if !hasExit {
			add("", "nodes", "At least one reachable Exit is required")
		}
	}
	if w.GlobalExit != nil {
		switch w.GlobalExit.Reason {
		case "", "converted", "completed", "condition_not_met", "manually_cancelled":
		default:
			add("", "global_exit.reason", "Invalid global exit outcome")
		}
		if !eventName.MatchString(w.GlobalExit.EventName) {
			add("", "global_exit", "Valid exit event name required")
		}
		if c := w.GlobalExit.Correlation; c != nil {
			if !validCorrelationPath(c.EntryPath) || !validCorrelationPath(c.EventPath) {
				add("", "global_exit.correlation", "Choose valid entry and incoming event property paths")
			}
		}
		if w.GlobalExit.Filters != nil {
			if err := validateRule(*w.GlobalExit.Filters, 0); err != nil {
				add("", "global_exit", err.Error())
			}
		}
	}
	if w.Reentry != nil {
		if w.Reentry.Mode != "once" && w.Reentry.Mode != "after_completion" && w.Reentry.Mode != "cooldown" {
			add("", "reentry", "Invalid reentry mode")
		}
		if w.Reentry.Mode == "cooldown" && w.Reentry.CooldownSeconds < 1 {
			add("", "reentry", "Positive cooldown required")
		}
	}
	return issues
}
func validateRule(r Rule, depth int) error {
	if depth > 8 {
		return fmt.Errorf("Condition nesting exceeds 8 levels")
	}
	if len(r.Rules) > 0 {
		if r.Logic != "and" && r.Logic != "or" {
			return fmt.Errorf("Rule group requires and/or")
		}
		if len(r.Rules) > 50 {
			return fmt.Errorf("Rule group exceeds 50 items")
		}
		for _, child := range r.Rules {
			if err := validateRule(child, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if r.Logic != "" {
		return fmt.Errorf("Rule group cannot be empty")
	}
	if !strings.HasPrefix(r.Path, "customer.") && !strings.HasPrefix(r.Path, "event.") && !strings.HasPrefix(r.Path, "context.") {
		return fmt.Errorf("Rule path must start with customer, event or context")
	}
	switch r.Operator {
	case "equals", "not_equals", "greater_than", "less_than", "contains", "exists", "before", "after":
	default:
		return fmt.Errorf("Unsupported condition operator")
	}
	return nil
}
func Lookup(data map[string]any, path string) (any, bool) {
	var value any = data
	for _, part := range strings.Split(path, ".") {
		m, ok := value.(map[string]any)
		if !ok {
			return nil, false
		}
		value, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return value, true
}
func number(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		x, e := n.Float64()
		return x, e == nil
	}
	return 0, false
}
func Evaluate(r Rule, data map[string]any) bool {
	if len(r.Rules) > 0 {
		result := r.Logic == "and"
		for _, child := range r.Rules {
			v := Evaluate(child, data)
			if r.Logic == "and" {
				result = result && v
			} else {
				result = result || v
			}
		}
		return result
	}
	value, exists := Lookup(data, r.Path)
	if r.Operator == "exists" {
		return exists && value != nil
	}
	if !exists {
		return false
	}
	equal := reflect.DeepEqual(value, r.Value)
	if a, ok := number(value); ok {
		if b, ok := number(r.Value); ok {
			equal = a == b
		}
	}
	switch r.Operator {
	case "equals":
		return equal
	case "not_equals":
		return !equal
	case "contains":
		if a, ok := value.(string); ok {
			b, ok := r.Value.(string)
			return ok && strings.Contains(a, b)
		}
		if a, ok := value.([]any); ok {
			for _, v := range a {
				if reflect.DeepEqual(v, r.Value) {
					return true
				}
			}
		}
	case "greater_than", "less_than":
		a, ok := number(value)
		b, ok2 := number(r.Value)
		return ok && ok2 && ((r.Operator == "greater_than" && a > b) || (r.Operator == "less_than" && a < b))
	case "before", "after":
		a, e := time.Parse(time.RFC3339, fmt.Sprint(value))
		b, e2 := time.Parse(time.RFC3339, fmt.Sprint(r.Value))
		return e == nil && e2 == nil && ((r.Operator == "before" && a.Before(b)) || (r.Operator == "after" && a.After(b)))
	}
	return false
}
func (w Workflow) Node(id string) (Node, bool) {
	for _, n := range w.Nodes {
		if n.ID == id {
			return n, true
		}
	}
	return Node{}, false
}
func (w Workflow) Start() Node {
	for _, n := range w.Nodes {
		if n.Type == "event_trigger" {
			return n
		}
	}
	return Node{}
}
func (w Workflow) Next(id, port string) string {
	for _, e := range w.Edges {
		if e.Source == id && e.Port == port {
			return e.Target
		}
	}
	return ""
}
func BoolPort(value bool) string { return strconv.FormatBool(value) }

// ExitOutcome keeps the semantics of existing immutable snapshots: a missing
// outcome is a conversion. Newly authored cancellation exits are explicit.
func (e *ExitCriteria) ExitOutcome() string {
	if e == nil || e.Reason == "" {
		return "converted"
	}
	return e.Reason
}
