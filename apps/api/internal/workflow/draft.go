package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// DecodeDraft permits incomplete graphs but rejects shapes the editor/runtime
// cannot represent. Full semantic validation remains a publication gate.
func DecodeDraft(raw []byte) (Workflow, error) {
	var w Workflow
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&w); err != nil {
		return w, fmt.Errorf("invalid typed workflow draft")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return w, fmt.Errorf("one workflow document required")
	}
	if w.SchemaVersion != 1 || w.Nodes == nil || w.Edges == nil || len(w.Nodes) > 100 || len(w.Edges) > 200 {
		return w, fmt.Errorf("draft requires schema version 1 and bounded node/edge arrays")
	}
	ids := map[string]bool{}
	for _, node := range w.Nodes {
		if !identifier.MatchString(node.ID) || ids[node.ID] || Ports(node.Type) == nil {
			return w, fmt.Errorf("unique valid node IDs and supported node types required")
		}
		ids[node.ID] = true
		for _, rule := range []*Rule{node.Config.Rule, node.Config.Filters} {
			if rule != nil {
				if err := boundedRule(*rule, 0); err != nil {
					return w, err
				}
			}
		}
	}
	return w, nil
}
func boundedRule(rule Rule, depth int) error {
	if depth > 8 || len(rule.Rules) > 50 {
		return fmt.Errorf("condition exceeds nesting or group limits")
	}
	for _, child := range rule.Rules {
		if err := boundedRule(child, depth+1); err != nil {
			return err
		}
	}
	return nil
}
