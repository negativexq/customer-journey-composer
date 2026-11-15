// Package semantic holds design-time vocabulary. The execution engine never
// consults this catalog: approved workflows retain explicit technical names.
package semantic

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

type Entry struct {
	Key         string   `json:"key,omitempty"`
	Kind        string   `json:"kind" enum:"event,customer_field,event_field"`
	EventName   string   `json:"event_name,omitempty" maxLength:"80"`
	Path        string   `json:"path,omitempty" maxLength:"200"`
	Label       string   `json:"label" minLength:"1" maxLength:"160"`
	Description string   `json:"description,omitempty" maxLength:"1000"`
	DataType    string   `json:"data_type" enum:"event,string,number,boolean,mixed"`
	Aliases     []string `json:"aliases" maxItems:"12"`
	Enabled     bool     `json:"enabled"`
	Source      string   `json:"source,omitempty"`
}

func Key(kind, event, path string) string { return kind + ":" + event + ":" + path }

var eventPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,79}$`)
var pathPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*(\.[a-zA-Z_][a-zA-Z0-9_]*){1,11}$`)

func (e *Entry) Validate() error {
	if len(e.Label) == 0 || len(e.Label) > 160 || len(e.Description) > 1000 || len(e.Aliases) > 12 {
		return fmt.Errorf("Provide a label and bounded description/aliases")
	}
	for _, a := range e.Aliases {
		if len(a) == 0 || len(a) > 160 {
			return fmt.Errorf("Aliases must contain 1–160 bytes")
		}
	}
	switch e.Kind {
	case "event":
		if !eventPattern.MatchString(e.EventName) || e.Path != "" || e.DataType != "event" {
			return fmt.Errorf("Valid event name and event type required")
		}
	case "customer_field":
		if e.EventName != "" || !strings.HasPrefix(e.Path, "customer.attributes.") {
			return fmt.Errorf("Custom profile fields must use customer.attributes paths")
		}
	case "event_field":
		if !eventPattern.MatchString(e.EventName) || !strings.HasPrefix(e.Path, "event.properties.") {
			return fmt.Errorf("Event fields require an event and event.properties path")
		}
	default:
		return fmt.Errorf("Unsupported catalog kind")
	}
	if e.Kind != "event" {
		if !pathPattern.MatchString(e.Path) || len(e.Path) > 200 {
			return fmt.Errorf("Invalid field path")
		}
		for _, p := range strings.Split(e.Path, ".") {
			if p == "__proto__" || p == "constructor" || p == "prototype" {
				return fmt.Errorf("Reserved field path")
			}
		}
		if e.DataType != "string" && e.DataType != "number" && e.DataType != "boolean" && e.DataType != "mixed" {
			return fmt.Errorf("Invalid field type")
		}
	}
	e.Key = Key(e.Kind, e.EventName, e.Path)
	if e.Aliases == nil {
		e.Aliases = []string{}
	}
	return nil
}
func Builtins() []Entry {
	result := []Entry{}
	for _, p := range []string{"email", "phone", "locale", "timezone", "external_id", "customer_id", "consents.email", "consents.sms"} {
		typ := "string"
		if strings.HasPrefix(p, "consents.") {
			typ = "boolean"
		}
		result = append(result, Entry{Key: Key("customer_field", "", "customer."+p), Kind: "customer_field", Path: "customer." + p, Label: Humanize(p), DataType: typ, Enabled: true, Source: "system", Aliases: []string{}})
	}
	return result
}

// EngagementEntries are the events the delivery service sends after a journey
// message is accepted. They follow the workspace's own definitions, so a
// workspace's events stay first wherever events are listed.
func EngagementEntries() []Entry {
	result := []Entry{}
	for _, e := range []struct{ name, label, description string }{
		{"message_delivered", "Message delivered", "The provider reported that a journey message reached the inbox or phone."},
		{"message_bounced", "Message bounced", "The provider reported that a journey message could not be delivered."},
		{"message_opened", "Email opened", "The customer's mail client loaded the email's images. Some clients load them without the person reading."},
		{"message_clicked", "Message link clicked", "The customer opened a tracked link in an email or SMS."},
		{"message_unsubscribed", "Unsubscribed from email", "The customer used the email's unsubscribe link. Email consent is withdrawn."},
	} {
		result = append(result, Entry{Key: Key("event", e.name, ""), Kind: "event", EventName: e.name, Label: e.label, Description: e.description, DataType: "event", Enabled: true, Source: "system", Aliases: []string{}})
		for _, f := range []struct{ path, label string }{{"channel", "Channel"}, {"node_id", "Message step"}, {"journey_id", "Journey"}, {"execution_id", "Journey run"}, {"delivery_id", "Delivery"}} {
			path := "event.properties." + f.path
			result = append(result, Entry{Key: Key("event_field", e.name, path), Kind: "event_field", EventName: e.name, Path: path, Label: f.label, DataType: "string", Enabled: true, Source: "system", Aliases: []string{}})
		}
		if e.name == "message_clicked" {
			path := "event.properties.url"
			result = append(result, Entry{Key: Key("event_field", e.name, path), Kind: "event_field", EventName: e.name, Path: path, Label: "Link address", DataType: "string", Enabled: true, Source: "system", Aliases: []string{}})
		}
	}
	return result
}
func Humanize(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "_", " "), ".", " / ")
}
func Active(entries []Entry) []Entry {
	out := []Entry{}
	for _, e := range entries {
		if e.Enabled {
			out = append(out, e)
		}
	}
	return out
}

type Reference struct {
	Key       string `json:"key"`
	Kind      string `json:"kind"`
	EventName string `json:"event_name,omitempty"`
	Path      string `json:"path,omitempty"`
}
type Mapping struct {
	Reference  Reference `json:"reference"`
	Label      string    `json:"label"`
	Status     string    `json:"status"`
	Candidates []Entry   `json:"candidates"`
}
type Report struct {
	Version  string    `json:"version"`
	Ready    bool      `json:"ready"`
	Mappings []Mapping `json:"mappings"`
}
type Binding struct {
	Reference string `json:"reference" maxLength:"400"`
	Target    string `json:"target" maxLength:"400"`
}

var variable = regexp.MustCompile(`{{\s*([a-zA-Z_][\w.]*)\s*}}`)

// visit covers every executable reference, including fallback paths and writes.
// Event properties are scoped to the entry event for conditions/messages; waiter
// and global-exit filters are scoped to their incoming event.
func visit(w *workflow.Workflow, fn func(Reference) string) {
	entry := ""
	for _, n := range w.Nodes {
		if n.Type == "event_trigger" {
			entry = n.Config.EventName
		}
	}
	event := func(name *string) {
		if *name != "" {
			*name = fn(Reference{Key: Key("event", *name, ""), Kind: "event", EventName: *name})
		}
	}
	path := func(p *string, scope string) {
		if *p == "" {
			return
		}
		kind := "unsupported"
		if strings.HasPrefix(*p, "customer.") {
			kind = "customer_field"
			scope = ""
		} else if strings.HasPrefix(*p, "event.properties.") {
			kind = "event_field"
		} else if *p == "event.event_name" || *p == "event.event_id" || *p == "event.occurred_at" {
			return
		}
		*p = fn(Reference{Key: Key(kind, scope, *p), Kind: kind, EventName: scope, Path: *p})
	}
	var rules func(*workflow.Rule, string)
	rules = func(r *workflow.Rule, scope string) {
		if r == nil {
			return
		}
		path(&r.Path, scope)
		for i := range r.Rules {
			rules(&r.Rules[i], scope)
		}
	}
	for i := range w.Nodes {
		c := &w.Nodes[i].Config
		scope := entry
		if w.Nodes[i].Type == "event_trigger" || w.Nodes[i].Type == "wait_event" {
			scope = c.EventName
			// An audience entry's event is internal, not part of the catalog.
			if !c.IsAudience() {
				event(&c.EventName)
			}
		}
		rules(c.Filters, scope)
		rules(c.Rule, entry)
		for _, text := range []*string{&c.Subject, &c.Preheader, &c.HTML, &c.Text} {
			*text = variable.ReplaceAllStringFunc(*text, func(token string) string {
				p := variable.FindStringSubmatch(token)[1]
				path(&p, entry)
				return "{{ " + p + " }}"
			})
		}
		if len(c.Fallbacks) > 0 {
			next := map[string]string{}
			for p, v := range c.Fallbacks {
				path(&p, entry)
				next[p] = v
			}
			c.Fallbacks = next
		}
		if w.Nodes[i].Type == "update_attribute" {
			p := "customer.attributes." + c.Attribute
			path(&p, entry)
			c.Attribute = strings.TrimPrefix(p, "customer.attributes.")
		}
	}
	if w.GlobalExit != nil {
		scope := w.GlobalExit.EventName
		event(&w.GlobalExit.EventName)
		rules(w.GlobalExit.Filters, scope)
		if c := w.GlobalExit.Correlation; c != nil {
			path(&c.EntryPath, entry)
			path(&c.EventPath, scope)
		}
	}
}
func clone(w workflow.Workflow) workflow.Workflow {
	raw, _ := json.Marshal(w)
	var out workflow.Workflow
	_ = json.Unmarshal(raw, &out)
	return out
}
func References(w workflow.Workflow) []Reference {
	refs := map[string]Reference{}
	c := clone(w)
	visit(&c, func(r Reference) string {
		refs[r.Key] = r
		if r.Kind == "event" {
			return r.EventName
		}
		return r.Path
	})
	keys := []string{}
	for k := range refs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := []Reference{}
	for _, k := range keys {
		out = append(out, refs[k])
	}
	return out
}
func normal(s string) string {
	return strings.ToLower(strings.Join(strings.FieldsFunc(s, func(r rune) bool { return r == '_' || r == '.' || r == ' ' || r == '-' }), " "))
}
func Analyze(w workflow.Workflow, entries []Entry, confirmed []string) Report {
	active := Active(entries)
	byKey := map[string]Entry{}
	for _, e := range active {
		byKey[e.Key] = e
	}
	ok := map[string]bool{}
	for _, k := range confirmed {
		ok[k] = true
	}
	typeErrors := typeMismatches(w, byKey)
	report := Report{Version: "semantic.v1", Ready: true, Mappings: []Mapping{}}
	for _, r := range References(w) {
		m := Mapping{Reference: r, Label: Humanize(r.Path), Status: "missing", Candidates: []Entry{}}
		if r.Kind == "event" {
			m.Label = Humanize(r.EventName)
		}
		exact, known := byKey[r.Key]
		if known {
			m.Label = exact.Label
			m.Status = "matched"
		}
		for _, e := range active {
			if e.Kind != r.Kind || (r.Kind == "event_field" && e.EventName != r.EventName) {
				continue
			}
			names := append([]string{e.Label}, e.Aliases...)
			wanted := []string{r.EventName, r.Path}
			if parts := strings.Split(r.Path, "."); len(parts) > 0 {
				wanted = append(wanted, parts[len(parts)-1])
			}
			if known {
				wanted = append([]string{exact.Label}, exact.Aliases...)
			}
			related := false
			for _, a := range names {
				for _, b := range wanted {
					if normal(a) != "" && normal(a) == normal(b) {
						related = true
					}
				}
			}
			if related && len(m.Candidates) < 12 {
				m.Candidates = append(m.Candidates, e)
			}
			if known && e.Key != r.Key && related {
				m.Status = "ambiguous"
			}
		}
		if known && exact.DataType == "mixed" {
			m.Status = "mixed"
		}
		if typeErrors[r.Key] {
			m.Status = "type_mismatch"
		}
		if m.Status == "ambiguous" && ok[r.Key] {
			m.Status = "confirmed"
		}
		if m.Status != "matched" && m.Status != "confirmed" {
			report.Ready = false
		}
		report.Mappings = append(report.Mappings, m)
	}
	return report
}
func Resolve(w workflow.Workflow, entries []Entry, bindings []Binding) (workflow.Workflow, error) {
	active := map[string]Entry{}
	for _, e := range Active(entries) {
		active[e.Key] = e
	}
	refs := map[string]Reference{}
	for _, r := range References(w) {
		refs[r.Key] = r
	}
	targets := map[string]Entry{}
	for _, b := range bindings {
		r, exists := refs[b.Reference]
		e, found := active[b.Target]
		if !exists || !found || e.Kind != r.Kind || (e.Kind == "event_field" && e.EventName != r.EventName) || (strings.HasPrefix(r.Path, "customer.attributes.") && !strings.HasPrefix(e.Path, "customer.attributes.")) {
			return w, fmt.Errorf("Choose an enabled field or event of the same kind and event scope")
		}
		if _, duplicate := targets[b.Reference]; duplicate {
			return w, fmt.Errorf("Duplicate binding")
		}
		targets[b.Reference] = e
	}
	entryEvent := ""
	for _, node := range w.Nodes {
		if node.Type == "event_trigger" {
			entryEvent = node.Config.EventName
		}
	}
	for _, node := range w.Nodes {
		fallbackValues := map[string]string{}
		for path, value := range node.Config.Fallbacks {
			kind, scope := "event_field", entryEvent
			if strings.HasPrefix(path, "customer.") {
				kind, scope = "customer_field", ""
			}
			targetPath := path
			if target, ok := targets[Key(kind, scope, path)]; ok {
				targetPath = target.Path
			}
			if previous, exists := fallbackValues[targetPath]; exists && previous != value {
				return w, fmt.Errorf("These mappings merge different fallback values; correct the message before applying")
			}
			fallbackValues[targetPath] = value
		}
	}
	out := clone(w)
	visit(&out, func(r Reference) string {
		if e, ok := targets[r.Key]; ok {
			if r.Kind == "event" {
				return e.EventName
			}
			return e.Path
		}
		if r.Kind == "event" {
			return r.EventName
		}
		return r.Path
	})
	return out, nil
}

// Validate scalar rule/write types against definitions, without reading profiles.
func typeMismatches(w workflow.Workflow, entries map[string]Entry) map[string]bool {
	bad := map[string]bool{}
	entry := ""
	for _, n := range w.Nodes {
		if n.Type == "event_trigger" {
			entry = n.Config.EventName
		}
	}
	check := func(path, scope, operator string, value any) {
		kind := "event_field"
		if strings.HasPrefix(path, "customer.") {
			kind = "customer_field"
			scope = ""
		}
		key := Key(kind, scope, path)
		e, known := entries[key]
		if !known || operator == "exists" {
			return
		}
		expected := ""
		switch value.(type) {
		case bool:
			expected = "boolean"
		case string:
			expected = "string"
		case float64, int, int64:
			expected = "number"
		}
		if operator == "greater_than" || operator == "greater_than_or_equal" || operator == "less_than" || operator == "less_than_or_equal" || operator == "increment" {
			if expected != "number" {
				bad[key] = true
			}
			expected = "number"
		}
		if operator == "before" || operator == "after" || operator == "contains" {
			if expected != "string" {
				bad[key] = true
			}
			expected = "string"
		}
		if expected != "" && e.DataType != expected {
			bad[key] = true
		}
	}
	var rules func(*workflow.Rule, string)
	rules = func(r *workflow.Rule, scope string) {
		if r == nil {
			return
		}
		check(r.Path, scope, r.Operator, r.Value)
		for i := range r.Rules {
			rules(&r.Rules[i], scope)
		}
	}
	for _, n := range w.Nodes {
		scope := entry
		if n.Type == "event_trigger" || n.Type == "wait_event" {
			scope = n.Config.EventName
		}
		rules(n.Config.Filters, scope)
		rules(n.Config.Rule, entry)
		if n.Type == "update_attribute" && n.Config.Operation != "unset" {
			check("customer.attributes."+n.Config.Attribute, "", n.Config.Operation, n.Config.Value)
		}
	}
	if w.GlobalExit != nil {
		rules(w.GlobalExit.Filters, w.GlobalExit.EventName)
		if c := w.GlobalExit.Correlation; c != nil {
			aKey, bKey := Key("event_field", entry, c.EntryPath), Key("event_field", w.GlobalExit.EventName, c.EventPath)
			a, aOK := entries[aKey]
			b, bOK := entries[bKey]
			if aOK && a.DataType != "string" && a.DataType != "number" {
				bad[aKey] = true
			}
			if bOK && b.DataType != "string" && b.DataType != "number" {
				bad[bKey] = true
			}
			if aOK && bOK && a.DataType != b.DataType {
				bad[aKey] = true
				bad[bKey] = true
			}
		}
	}
	return bad
}
