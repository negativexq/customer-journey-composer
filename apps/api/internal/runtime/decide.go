package runtime

import (
	"fmt"
	"time"

	"github.com/ofk/customer-journey-composer/apps/api/internal/workflow"
)

type Clock interface{ Now() time.Time }
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now().UTC() }

type Decision struct {
	Port       string           `json:"port,omitempty"`
	WaitUntil  *time.Time       `json:"wait_until,omitempty"`
	EventName  string           `json:"event_name,omitempty"`
	ExitReason string           `json:"exit_reason,omitempty"`
	Condition  *bool            `json:"condition,omitempty"`
	Message    *workflow.Config `json:"-"`
	Attribute  string           `json:"attribute,omitempty"`
	Operation  string           `json:"operation,omitempty"`
	Value      any              `json:"value,omitempty"`
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
			until := now.Add(time.Duration(c.DurationSeconds) * time.Second)
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
	case "email", "sms":
		d.Port = "next"
		d.Message = &c
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
func dataContext(profile map[string]any, event map[string]any, state map[string]any) map[string]any {
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
