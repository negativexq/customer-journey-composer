package simulation

import (
	"fmt"
	"strconv"
	"time"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/delivery"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/messaging"
	engine "github.com/negativexq/customer-journey-composer/apps/api/internal/runtime"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

func onNode(spec workflow.Workflow, scenario Scenario, s *State, task Task) error {
	node, ok := spec.Node(task.NodeID)
	if !ok {
		s.fail("node missing from specification")
		return nil
	}
	index := stepIndex(s, s.Execution, node.ID)
	if index < 0 {
		s.Steps = append(s.Steps, Step{Execution: s.Execution, NodeID: node.ID, Label: node.Label, Type: node.Type, State: "RUNNING", StartedAt: s.Now})
		index = len(s.Steps) - 1
		s.trace("node", node.ID, "Entered "+node.Label)
	}
	data := engine.ContextData(s.Customer, s.CurrentEvent, s.Context)
	decision, err := engine.Decide(node, data, s.Now, task.Port)
	if err != nil {
		s.Steps[index].State = "FAILED"
		s.Steps[index].Error = err.Error()
		at := s.Now
		s.Steps[index].CompletedAt = &at
		if node.Type == "email" || node.Type == "sms" {
			rendered, _ := messaging.Render(node.Type, node.Config, data)
			m := Message{Dirty: true, ID: fmt.Sprintf("execution-%d/%s", s.Execution, node.ID), Execution: s.Execution, CustomerID: customerID(s.Customer), NodeID: node.ID, NodeLabel: node.Label, Channel: node.Type, Status: "FAILED", At: s.Now, Rendered: rendered, Decision: delivery.Eligibility{Eligible: false, Reason: "render_failed"}, Attempts: []Attempt{}, ProviderResponse: err.Error()}
			s.Messages = append(s.Messages, m)
		}
		s.fail(err.Error())
		return nil
	}
	s.Steps[index].Output = decision
	if decision.WaitUntil != nil {
		s.Steps[index].State = "WAITING"
		s.Steps[index].Deadline = decision.WaitUntil
		s.Status = "WAITING"
		s.Outcomes[strconv.Itoa(s.Execution)] = s.Status
		port := "completed"
		if node.Type == "wait_event" {
			port = "timeout"
		}
		s.enqueue(Task{At: *decision.WaitUntil, Kind: "timer", Execution: s.Execution, NodeID: node.ID, Port: port})
		s.trace("wait", node.ID, "Waiting until "+decision.WaitUntil.Format(time.RFC3339))
		return nil
	}
	if decision.Attribute != "" {
		attrs, _ := s.Customer["attributes"].(map[string]any)
		next, err := engine.ApplyAttribute(attrs, decision)
		if err != nil {
			s.Steps[index].State = "FAILED"
			s.Steps[index].Error = err.Error()
			s.fail(err.Error())
			return nil
		}
		s.Customer["attributes"] = next
	}
	if decision.Message != nil {
		message := *decision.Message
		reservations := scenario.PriorMessages
		if s.Now.Sub(scenario.StartTime) >= time.Duration(scenario.FrequencyWindowSeconds)*time.Second {
			reservations = 0
		}
		for _, previous := range s.Messages {
			if previous.Pending || previous.Unknown || previous.AcceptedAt != nil && !previous.AcceptedAt.Before(s.Now.Add(-time.Duration(scenario.FrequencyWindowSeconds)*time.Second)) {
				reservations++
			}
		}
		eligibility := delivery.EvaluateEligibility(message, consent(s.Customer), reservations, scenario.FrequencyLimit, s.Status)
		m := Message{Dirty: true, ID: fmt.Sprintf("execution-%d/%s", s.Execution, node.ID), Execution: s.Execution, CustomerID: customerID(s.Customer), NodeID: node.ID, NodeLabel: node.Label, Channel: node.Type, Status: "SIMULATED", Pending: eligibility.Eligible, At: s.Now, Rendered: message, Decision: eligibility, Attempts: []Attempt{}, ProviderResponse: "Sandbox only; no provider request"}
		if !eligibility.Eligible {
			m.Status = "BLOCKED"
			m.ProviderResponse = eligibility.Reason
		}
		s.MessageBytes += len(message.Subject) + len(message.HTML) + len(message.Text)
		if len(s.Messages) >= 500 || s.MessageBytes > 4<<20 {
			s.fail("simulation_output_limit")
			s.Queue = []Task{}
			return nil
		}
		s.Messages = append(s.Messages, m)
		if eligibility.Eligible {
			s.enqueue(Task{At: s.Now, Kind: "delivery", Execution: s.Execution, MessageID: m.ID})
		}
		s.trace("message", node.ID, m.Status+": "+node.Label)
	}
	at := s.Now
	s.Steps[index].State = "SUCCEEDED"
	s.Steps[index].CompletedAt = &at
	if decision.Condition != nil {
		s.trace("condition", node.ID, "Condition "+decision.Port)
	}
	if decision.ExitReason != "" {
		s.Status = "EXITED"
		if decision.ExitReason == "completed" {
			s.Status = "COMPLETED"
		}
		s.ExitReason = decision.ExitReason
		s.Active = false
		s.Outcomes[strconv.Itoa(s.Execution)] = s.Status
		s.trace("exit", node.ID, decision.ExitReason)
		return nil
	}
	scheduleNode(spec, s, node.ID, decision.Port)
	return nil
}
func onDelivery(scenario Scenario, s *State, id string) error {
	index := -1
	for i := range s.Messages {
		if s.Messages[i].ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		return fmt.Errorf("sandbox message not found")
	}
	m := &s.Messages[index]
	if !m.Pending {
		return nil
	}
	m.Dirty = true
	state := s.Outcomes[strconv.Itoa(m.Execution)]
	currentConsent := consent(s.Customer)
	to := ""
	if m.Channel == "email" {
		to, _ = s.Customer["email"].(string)
	} else {
		to, _ = s.Customer["phone"].(string)
	}
	if state == "EXITED" || state == "FAILED" || state == "CANCELLED" || !currentConsent[m.Channel] || to != m.Rendered.To {
		m.Pending = false
		m.Status = "BLOCKED"
		m.Decision.Eligible = false
		m.Decision.Reason = "consent_revoked"
		if state == "EXITED" || state == "FAILED" || state == "CANCELLED" {
			m.Status = "SKIPPED"
			m.Decision.Reason = "execution_terminated"
		}
		if to != m.Rendered.To {
			m.Decision.Reason = "recipient_changed"
		}
		m.ProviderResponse = m.Decision.Reason
		return nil
	}
	attempt := len(m.Attempts) + 1
	code := 200
	if m.Channel == scenario.FaultChannel && s.FaultIndex < len(scenario.Faults) {
		code = scenario.Faults[s.FaultIndex]
		s.FaultIndex++
	}
	result := Attempt{At: s.Now, Number: attempt, Code: code, Status: "ACCEPTED", Response: "Sandbox provider accepted; no external message sent"}
	switch {
	case code == 408:
		result.Status = "UNKNOWN"
		result.Response = "Sandbox acceptance unknown; no automatic retry"
		m.Pending = false
		m.Unknown = true
		m.Status = "FAILED"
	case code == 429 || code >= 500:
		result.Status = "RETRY"
		result.Response = "Injected transient provider failure"
		if attempt >= 5 {
			result.Status = "FAILED"
			result.Response = "Retry limit exhausted"
			m.Pending = false
			m.Status = "FAILED"
		} else {
			s.enqueue(Task{At: s.Now.Add(time.Duration(delivery.RetryDelay(attempt)) * time.Second), Kind: "delivery", Execution: m.Execution, MessageID: m.ID})
		}
	case code >= 400:
		result.Status = "FAILED"
		result.Response = "Injected permanent provider failure"
		m.Pending = false
		m.Status = "FAILED"
	default:
		m.Pending = false
		m.Status = "SIMULATED"
		at := s.Now
		m.AcceptedAt = &at
	}
	m.Attempts = append(m.Attempts, result)
	m.ProviderResponse = result.Response
	s.trace("provider", m.NodeID, "Sandbox "+result.Status)
	return nil
}
