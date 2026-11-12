package workflow

import (
	"math"
	"strings"
)

// Correlation compares identities, not expressions or templates. Absent
// configuration preserves the customer-wide behavior of existing versions.
func validCorrelationPath(path string) bool {
	if len(path) > 200 || !strings.HasPrefix(path, "event.properties.") {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(path, "event.properties."), ".")
	if len(parts) > 8 {
		return false
	}
	for _, p := range parts {
		if !attribute.MatchString(p) || p == "__proto__" || p == "constructor" || p == "prototype" {
			return false
		}
	}
	return true
}

// MatchesEntry fails closed for absent/null/empty/non-scalar IDs. Entry and
// incoming payloads are separate so a resumed wait cannot replace the entry ID.
func (x *ExitCriteria) MatchesEntry(entry, incoming map[string]any) bool {
	if x.Correlation == nil {
		return true
	}
	c := x.Correlation
	if !validCorrelationPath(c.EntryPath) || !validCorrelationPath(c.EventPath) {
		return false
	}
	a, ok := Lookup(entry, strings.TrimPrefix(c.EntryPath, "event.properties."))
	b, ok2 := Lookup(incoming, strings.TrimPrefix(c.EventPath, "event.properties."))
	if !ok || !ok2 {
		return false
	}
	if text, ok := a.(string); ok {
		other, ok := b.(string)
		return ok && strings.TrimSpace(text) != "" && text == other
	}
	n, ok := number(a)
	m, ok2 := number(b)
	return ok && ok2 && !math.IsNaN(n) && !math.IsInf(n, 0) && !math.IsNaN(m) && !math.IsInf(m, 0) && n == m
}
