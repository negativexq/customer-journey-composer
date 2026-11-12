// Package simulation is a deterministic, no-I/O sandbox. It has no database or
// provider handle. Persistence lives in HTTP orchestration, outside this engine.
package simulation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/delivery"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/messaging"
	engine "github.com/negativexq/customer-journey-composer/apps/api/internal/runtime"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

type SandboxEvent struct {
	EventID    string         `json:"event_id,omitempty" maxLength:"128"`
	EventName  string         `json:"event_name" pattern:"^[a-z][a-z0-9_]{0,79}$"`
	CustomerID string         `json:"customer_id,omitempty" maxLength:"128"`
	OccurredAt time.Time      `json:"occurred_at,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
}
type TimedEvent struct {
	AfterSeconds int64 `json:"after_seconds" minimum:"0" maximum:"31536000"`
	Event        Event `json:"event"`
}
type SandboxChange struct {
	AfterSeconds int64             `json:"after_seconds" minimum:"0" maximum:"31536000"`
	Attributes   map[string]any    `json:"attributes,omitempty"`
	Consents     map[string]bool   `json:"consents,omitempty"`
	Contact      map[string]string `json:"contact,omitempty"`
}
type SandboxAssertion struct {
	Type     string `json:"type" enum:"state,message_sent,no_message,message_count,condition,no_duplicate_delivery,exit_reason,enrollment_count"`
	Channel  string `json:"channel,omitempty" enum:"email,sms"`
	NodeID   string `json:"node_id,omitempty"`
	Expected any    `json:"expected,omitempty"`
}
type SandboxScenario struct {
	Name                   string         `json:"name" minLength:"1" maxLength:"160"`
	Mode                   string         `json:"mode,omitempty" enum:"full,single_node"`
	NodeID                 string         `json:"node_id,omitempty"`
	StartTime              time.Time      `json:"start_time,omitempty"`
	CustomerID             string         `json:"customer_id,omitempty" maxLength:"128"`
	Customer               map[string]any `json:"customer,omitempty"`
	Entry                  Event          `json:"entry_event"`
	Events                 []TimedEvent   `json:"events,omitempty" maxItems:"100"`
	Changes                []Change       `json:"attribute_changes,omitempty" maxItems:"100"`
	FaultChannel           string         `json:"fault_channel,omitempty" enum:"email,sms"`
	Faults                 []int          `json:"provider_faults,omitempty" maxItems:"20"`
	PriorMessages          int            `json:"prior_messages,omitempty" minimum:"0" maximum:"1000"`
	FrequencyLimit         int            `json:"frequency_limit,omitempty" minimum:"1" maximum:"1000"`
	FrequencyWindowSeconds int            `json:"frequency_window_seconds,omitempty" minimum:"60" maximum:"2592000"`
	Assertions             []Assertion    `json:"assertions,omitempty" maxItems:"30"`
}
type SandboxTask struct {
	At        time.Time `json:"at"`
	Sequence  int       `json:"sequence"`
	Kind      string    `json:"kind"`
	Execution int       `json:"execution"`
	NodeID    string    `json:"node_id,omitempty"`
	Port      string    `json:"port,omitempty"`
	Event     *Event    `json:"event,omitempty"`
	Change    *Change   `json:"change,omitempty"`
	MessageID string    `json:"message_id,omitempty"`
}
type SandboxStep struct {
	Execution   int             `json:"execution"`
	NodeID      string          `json:"node_id"`
	Label       string          `json:"label"`
	Type        string          `json:"type"`
	State       string          `json:"state"`
	StartedAt   time.Time       `json:"started_at"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
	Deadline    *time.Time      `json:"deadline,omitempty"`
	Output      engine.Decision `json:"output"`
	Error       string          `json:"error,omitempty"`
}
type SandboxTrace struct {
	At        time.Time `json:"at"`
	Kind      string    `json:"kind"`
	NodeID    string    `json:"node_id,omitempty"`
	Message   string    `json:"message"`
	Execution int       `json:"execution"`
}
type SandboxAttempt struct {
	At       time.Time `json:"at"`
	Number   int       `json:"attempt"`
	Code     int       `json:"provider_code"`
	Status   string    `json:"status"`
	Response string    `json:"response"`
}
type SandboxMessage struct {
	Dirty            bool                 `json:"-"`
	PayloadPersisted bool                 `json:"payload_persisted"`
	ID               string               `json:"id"`
	Execution        int                  `json:"execution"`
	CustomerID       string               `json:"customer_id"`
	NodeID           string               `json:"node_id"`
	NodeLabel        string               `json:"node_label"`
	Channel          string               `json:"channel"`
	Status           string               `json:"status"`
	Pending          bool                 `json:"pending"`
	Unknown          bool                 `json:"unknown"`
	AcceptedAt       *time.Time           `json:"accepted_at,omitempty"`
	At               time.Time            `json:"scheduled_at"`
	Rendered         messaging.Message    `json:"rendered"`
	Decision         delivery.Eligibility `json:"decision"`
	Attempts         []Attempt            `json:"attempt_history"`
	ProviderResponse string               `json:"provider_response"`
}
type AssertionResult struct {
	Assertion   Assertion `json:"assertion"`
	Pass        bool      `json:"pass"`
	Actual      any       `json:"actual"`
	Description string    `json:"description"`
}
type SandboxState struct {
	Outcomes        map[string]string `json:"execution_outcomes"`
	SchemaVersion   int               `json:"schema_version"`
	Now             time.Time         `json:"now"`
	Status          string            `json:"status"`
	ExitReason      string            `json:"exit_reason,omitempty"`
	CurrentNodeID   string            `json:"current_node_id,omitempty"`
	Active          bool              `json:"active"`
	Paused          bool              `json:"paused"`
	Finalized       bool              `json:"finalized"`
	Execution       int               `json:"execution"`
	EnrollmentCount int               `json:"enrollment_count"`
	LastEntryAt     *time.Time        `json:"last_entry_at,omitempty"`
	EntryProperties map[string]any    `json:"entry_properties,omitempty"`
	EntryOccurredAt time.Time         `json:"entry_occurred_at"`
	Customer        map[string]any    `json:"customer"`
	Context         map[string]any    `json:"context"`
	CurrentEvent    map[string]any    `json:"current_event"`
	SeenEvents      map[string]string `json:"seen_events"`
	Queue           []Task            `json:"queue"`
	Sequence        int               `json:"sequence"`
	Transitions     int               `json:"transitions"`
	FaultIndex      int               `json:"fault_index"`
	MessageBytes    int               `json:"message_bytes"`
	Steps           []Step            `json:"steps"`
	Trace           []Trace           `json:"trace"`
	Messages        []Message         `json:"messages"`
	Assertions      []AssertionResult `json:"assertions"`
	Result          string            `json:"result"`
	Error           string            `json:"error,omitempty"`
}

func (s *State) enqueue(task Task) {
	s.Sequence++
	task.Sequence = s.Sequence
	s.Queue = append(s.Queue, task)
}
func (s *State) trace(kind, node, message string) {
	s.Trace = append(s.Trace, Trace{s.Now, kind, node, message, s.Execution})
}
func clone[T any](value T) (T, error) {
	var out T
	data, err := json.Marshal(value)
	if err != nil {
		return out, err
	}
	err = json.Unmarshal(data, &out)
	return out, err
}
func Hash(w workflow.Workflow) string {
	raw, _ := json.Marshal(w)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func eventData(event Event) map[string]any {
	return map[string]any{"event_id": event.EventID, "event_name": event.EventName, "occurred_at": event.OccurredAt.Format(time.RFC3339Nano), "properties": event.Properties}
}
func runtimeEvent(event Event) engine.Event {
	return engine.Event{EventID: event.EventID, CustomerID: event.CustomerID, EventName: event.EventName, OccurredAt: event.OccurredAt, Properties: event.Properties}
}
func consent(customer map[string]any) map[string]bool {
	result := map[string]bool{}
	if values, ok := customer["consents"].(map[string]any); ok {
		for key, value := range values {
			result[key], _ = value.(bool)
		}
	}
	return result
}
func customerID(customer map[string]any) string {
	value, _ := customer["customer_id"].(string)
	if value == "" {
		return "test_customer"
	}
	return value
}
func taskPriority(kind string) int {
	switch kind {
	case "attributes":
		return -2
	case "entry":
		return -1
	case "event":
		return 0
	case "timer":
		return 1
	case "node":
		return 2
	default:
		return 3
	}
}
func (s *State) fail(message string) {
	s.Active = false
	s.Status = "FAILED"
	s.Outcomes[fmt.Sprint(s.Execution)] = s.Status
	s.ExitReason = "simulation_failed"
	s.Error = message
	s.trace("error", s.CurrentNodeID, message)
	s.cancelPending()
}
func (s *State) cancelPending() {
	for i := range s.Steps {
		step := &s.Steps[i]
		if step.Execution == s.Execution && (step.State == "WAITING" || step.State == "RUNNING") {
			step.State = "SKIPPED"
			at := s.Now
			step.CompletedAt = &at
		}
	}
	for i := range s.Messages {
		message := &s.Messages[i]
		if message.Execution == s.Execution && message.Pending {
			message.Pending = false
			message.Dirty = true
			message.Status = "SKIPPED"
			message.ProviderResponse = "Cancelled by execution exit"
			message.Decision.Eligible = false
			message.Decision.Reason = "execution_terminated"
		}
	}
	queue := []Task{}
	for _, task := range s.Queue {
		if task.Kind == "event" || task.Kind == "attributes" || task.Kind == "entry" || task.Execution != s.Execution {
			queue = append(queue, task)
		}
	}
	s.Queue = queue
}
func ValidateScenario(s Scenario) error {
	if s.Name == "" || len(s.Name) > 160 {
		return fmt.Errorf("scenario name required (maximum 160 bytes)")
	}
	if s.StartTime.IsZero() || s.StartTime.Year() < 1970 || s.StartTime.Year() > 2200 {
		return fmt.Errorf("start time must be between 1970 and 2200")
	}
	if len(s.Events) > 100 || len(s.Changes) > 100 || len(s.Assertions) > 30 || len(s.Faults) > 20 {
		return fmt.Errorf("scenario exceeds item limits")
	}
	for _, event := range s.Events {
		if event.AfterSeconds < 0 || event.AfterSeconds > 31536000 {
			return fmt.Errorf("event time is outside one year")
		}
	}
	for _, change := range s.Changes {
		if change.AfterSeconds < 0 || change.AfterSeconds > 31536000 {
			return fmt.Errorf("attribute change time is outside one year")
		}
	}
	if s.FrequencyLimit < 1 || s.FrequencyLimit > 1000 || s.FrequencyWindowSeconds < 60 || s.FrequencyWindowSeconds > 2592000 || s.PriorMessages < 0 || s.PriorMessages > 1000 {
		return fmt.Errorf("invalid sandbox frequency policy")
	}
	if s.Mode != "" && s.Mode != "full" && s.Mode != "single_node" {
		return fmt.Errorf("invalid test mode")
	}
	return nil
}

type Event = SandboxEvent
type Message = SandboxMessage
type State = SandboxState
type Step = SandboxStep
type Trace = SandboxTrace
type Attempt = SandboxAttempt
type Assertion = SandboxAssertion
type Scenario = SandboxScenario
type Change = SandboxChange
type Task = SandboxTask
