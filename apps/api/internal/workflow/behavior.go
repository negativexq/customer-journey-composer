package workflow

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// Human-supplied synthetic expectations are metadata, never runtime instructions.
// Providers cannot return or overwrite this field in the structured-output schema.
type BehaviorChecks struct {
	Version         int                 `json:"version"`
	Confirmed       bool                `json:"confirmed"`
	EntryEvent      string              `json:"entry_event"`
	Inputs          map[string]any      `json:"inputs,omitempty"`
	EmailCount      *int                `json:"email_count,omitempty" minimum:"0" maximum:"100"`
	SMSCount        *int                `json:"sms_count,omitempty" minimum:"0" maximum:"100"`
	Outcome         string              `json:"outcome" enum:"completed,converted,condition_not_met,manually_cancelled"`
	DurationSeconds *int64              `json:"duration_seconds,omitempty" minimum:"0" maximum:"31536000"`
	Thresholds      []BehaviorThreshold `json:"thresholds,omitempty" maxItems:"4"`
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
	if c.Version != 1 || !c.Confirmed {
		return fmt.Errorf("Review and confirm the independent behavior checks")
	}
	if !eventName.MatchString(c.EntryEvent) {
		return fmt.Errorf("Choose the expected entry event")
	}
	if c.EmailCount == nil || c.SMSCount == nil || *c.EmailCount < 0 || *c.SMSCount < 0 || *c.EmailCount > 100 || *c.SMSCount > 100 {
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
	return nil
}
