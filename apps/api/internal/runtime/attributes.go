package runtime

import (
	"encoding/json"
	"fmt"
	"math"
)

func numeric(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, !math.IsInf(v, 0) && !math.IsNaN(v)
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		n, err := v.Float64()
		return n, err == nil && !math.IsInf(n, 0) && !math.IsNaN(n)
	}
	return 0, false
}

// ApplyAttribute returns a new map; neither the profile nor node configuration
// passed by the caller is mutated. Only flat custom attributes are writable.
func ApplyAttribute(attributes map[string]any, decision Decision) (map[string]any, error) {
	next := map[string]any{}
	for key, value := range attributes {
		next[key] = value
	}
	switch decision.Operation {
	case "set":
		next[decision.Attribute] = decision.Value
	case "unset":
		delete(next, decision.Attribute)
	case "increment":
		by, ok := numeric(decision.Value)
		if !ok {
			return nil, fmt.Errorf("increment requires a finite number")
		}
		previous := 0.0
		if value, exists := next[decision.Attribute]; exists {
			var ok bool
			previous, ok = numeric(value)
			if !ok {
				return nil, fmt.Errorf("existing attribute is not numeric")
			}
		}
		value := previous + by
		if math.IsInf(value, 0) || math.IsNaN(value) {
			return nil, fmt.Errorf("increment overflow")
		}
		next[decision.Attribute] = value
	default:
		return nil, fmt.Errorf("invalid attribute operation")
	}
	return next, nil
}
