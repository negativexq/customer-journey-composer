package runtime

import (
	"fmt"
	"strings"
	"time"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/messaging"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

type Clock interface{ Now() time.Time }
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now().UTC() }

type Decision struct {
	Port       string             `json:"port,omitempty"`
	WaitUntil  *time.Time         `json:"wait_until,omitempty"`
	EventName  string             `json:"event_name,omitempty"`
	ExitReason string             `json:"exit_reason,omitempty"`
	Condition  *bool              `json:"condition,omitempty"`
	Message    *messaging.Message `json:"-"`
	Attribute  string             `json:"attribute,omitempty"`
	Operation  string             `json:"operation,omitempty"`
	Value      any                `json:"value,omitempty"`
	Variant    string             `json:"variant,omitempty"`
}

// Decide is pure: production and the virtual-clock simulator share these node
// semantics. It performs no SQL, network I/O, random choice or wall-clock reads.
func Decide(node workflow.Node, data map[string]any, now time.Time, resumePort string) (Decision, error) {
	c := node.Config
	d := Decision{}
	switch node.Type {
	case "event_trigger":
		d.Port = "started"
	case "wait":
		d.Port = "completed"
		if resumePort == "" {
			until := workflow.WaitDeadline(c, data, now)
			d.WaitUntil = &until
		}
	case "wait_event":
		if resumePort != "" {
			d.Port = resumePort
		} else {
			until := now.Add(time.Duration(c.TimeoutSeconds) * time.Second)
			d.WaitUntil = &until
			d.EventName = c.EventName
		}
	case "condition":
		if c.Rule == nil {
			return d, fmt.Errorf("condition is missing")
		}
		v := workflow.Evaluate(*c.Rule, data)
		d.Condition = &v
		d.Port = workflow.BoolPort(v)
	case "split":
		// No random choice here: the customer and the step decide the variant.
		customer, _ := data["customer"].(map[string]any)
		id, _ := customer["customer_id"].(string)
		d.Port = workflow.SplitPort(c.Percent, id+":"+node.ID)
		d.Variant = strings.ToUpper(d.Port)
	case "email", "sms":
		d.Port = "next"
		rendered, err := messaging.Render(node.Type, c, data)
		if err != nil {
			return d, err
		}
		if len(rendered.Missing) > 0 {
			return d, fmt.Errorf("missing personalization attributes: %v", rendered.Missing)
		}
		d.Message = &rendered
	case "update_attribute":
		d.Port = "next"
		d.Attribute = c.Attribute
		d.Operation = c.Operation
		d.Value = c.Value
	case "exit":
		d.ExitReason = c.Reason
		if d.ExitReason == "" {
			d.ExitReason = "completed"
		}
	default:
		return d, fmt.Errorf("unsupported node type")
	}
	return d, nil
}
func ContextData(profile map[string]any, event map[string]any, state map[string]any) map[string]any {
	copyProfile := map[string]any{}
	for key, value := range profile {
		copyProfile[key] = value
	}
	profile = copyProfile
	if attrs, ok := profile["attributes"].(map[string]any); ok {
		for k, v := range attrs {
			if _, exists := profile[k]; !exists {
				profile[k] = v
			}
		}
	}
	if latest, ok := state["_event"].(map[string]any); ok {
		event = latest
	}
	return map[string]any{"customer": profile, "event": event, "context": state}
}
