package simulation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/messaging"
	engine "github.com/negativexq/customer-journey-composer/apps/api/internal/runtime"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

func New(spec workflow.Workflow, scenario Scenario) (State, workflow.Workflow, Scenario, error) {
	var err error
	spec, err = clone(spec)
	if err != nil {
		return State{}, spec, scenario, err
	}
	scenario, err = clone(scenario)
	if err != nil {
		return State{}, spec, scenario, err
	}
	if scenario.Mode == "" {
		scenario.Mode = "full"
	}
	if scenario.FrequencyLimit == 0 {
		scenario.FrequencyLimit = 3
	}
	if scenario.FrequencyWindowSeconds == 0 {
		scenario.FrequencyWindowSeconds = 86400
	}
	if scenario.FaultChannel == "" {
		scenario.FaultChannel = "email"
	}
	if scenario.Mode == "single_node" {
		spec, err = SingleNode(spec, scenario.NodeID, scenario.Entry.EventName)
		if err != nil {
			return State{}, spec, scenario, err
		}
	}
	if issues := messaging.ValidateWorkflow(spec); len(issues) > 0 {
		return State{}, spec, scenario, fmt.Errorf("workflow is invalid: %s", issues[0].Message)
	}
	if err := ValidateScenario(scenario); err != nil {
		return State{}, spec, scenario, err
	}
	if scenario.Customer == nil {
		scenario.Customer = map[string]any{}
	}
	profile, _ := clone(scenario.Customer)
	if _, ok := profile["customer_id"]; !ok {
		profile["customer_id"] = "test_customer"
	}
	if profile["attributes"] == nil {
		profile["attributes"] = map[string]any{}
	}
	if _, ok := profile["attributes"].(map[string]any); !ok {
		return State{}, spec, scenario, fmt.Errorf("customer attributes must be an object")
	}
	if zone, ok := profile["timezone"].(string); ok && zone != "" {
		if _, err := time.LoadLocation(zone); err != nil {
			return State{}, spec, scenario, fmt.Errorf("invalid customer timezone")
		}
	}
	if len(scenario.Assertions) == 0 {
		scenario.Assertions = []Assertion{{Type: "state", Expected: "COMPLETED"}, {Type: "no_duplicate_delivery", Expected: true}}
	}
	for _, assertion := range scenario.Assertions {
		switch assertion.Type {
		case "state", "message_sent", "no_message", "message_count", "condition", "no_duplicate_delivery", "exit_reason", "enrollment_count":
		default:
			return State{}, spec, scenario, fmt.Errorf("unsupported assertion type")
		}
	}
	start := scenario.StartTime.UTC().Truncate(time.Microsecond)
	scenario.StartTime = start
	state := State{SchemaVersion: 1, Now: start, Status: "READY", Customer: profile, Context: map[string]any{}, CurrentEvent: map[string]any{}, SeenEvents: map[string]string{}, Outcomes: map[string]string{}, Queue: []Task{}, Steps: []Step{}, Trace: []Trace{}, Messages: []Message{}, Assertions: []AssertionResult{}, Result: "PENDING"}
	entry := scenario.Entry
	if entry.EventName == "" {
		entry.EventName = spec.Start().Config.EventName
	}
	state.enqueue(Task{At: start, Kind: "entry", Event: &entry})
	for i, event := range scenario.Events {
		copy := event.Event
		if copy.EventID == "" {
			copy.EventID = "event_" + strconv.Itoa(i+1)
		}
		state.enqueue(Task{At: start.Add(time.Duration(event.AfterSeconds) * time.Second), Kind: "event", Event: &copy})
	}
	for _, change := range scenario.Changes {
		copy := change
		state.enqueue(Task{At: start.Add(time.Duration(change.AfterSeconds) * time.Second), Kind: "attributes", Change: &copy})
	}
	return state, spec, scenario, nil
}
func SingleNode(original workflow.Workflow, id, entryEvent string) (workflow.Workflow, error) {
	node, exists := original.Node(id)
	if !exists {
		return workflow.Workflow{}, fmt.Errorf("select a valid node for a single-node test")
	}
	spec := workflow.Workflow{SchemaVersion: 1, Nodes: []workflow.Node{node}, Edges: []workflow.Edge{}}
	if node.Type != "event_trigger" {
		if entryEvent == "" {
			entryEvent = original.Start().Config.EventName
		}
		if entryEvent == "" {
			entryEvent = "sandbox_start"
		}
		startID := "sandbox_entry"
		if startID == node.ID {
			startID += "_0"
		}
		spec.Nodes = append(spec.Nodes, workflow.Node{ID: startID, Type: "event_trigger", Label: "Test entry", Config: workflow.Config{EventName: entryEvent}})
		spec.Edges = append(spec.Edges, workflow.Edge{ID: "sandbox_entry_edge", Source: startID, Target: node.ID, Port: "started"})
	}
	for _, port := range workflow.Ports(node.Type) {
		endID := "sandbox_exit_" + port
		if endID == node.ID {
			endID += "_0"
		}
		spec.Nodes = append(spec.Nodes, workflow.Node{ID: endID, Type: "exit", Label: "Test complete", Config: workflow.Config{Reason: "completed"}})
		spec.Edges = append(spec.Edges, workflow.Edge{ID: "sandbox_edge_" + port, Source: node.ID, Target: endID, Port: port})
	}
	return spec, nil
}
func Advance(spec workflow.Workflow, scenario Scenario, state *State, operation string, seconds int64) error {
	if seconds < 0 || seconds > 31536000 {
		return fmt.Errorf("advance must be within one year")
	}
	switch operation {
	case "pause":
		state.Paused = true
		evaluateAssertions(scenario, state)
		return nil
	case "resume":
		state.Paused = false
		return nil
	case "step":
		if err := next(spec, scenario, state); err != nil {
			return err
		}
	case "run":
		state.Paused = false
		if err := until(spec, scenario, state, state.Now); err != nil {
			return err
		}
	case "advance":
		target := state.Now.Add(time.Duration(seconds) * time.Second)
		if target.After(scenario.StartTime.Add(100 * 365 * 24 * time.Hour)) {
			return fmt.Errorf("simulation horizon exceeds 100 years")
		}
		if err := until(spec, scenario, state, target); err != nil {
			return err
		}
		if state.Now.Before(target) {
			state.Now = target
		}
	case "until_next":
		state.Paused = false
		if err := until(spec, scenario, state, state.Now); err != nil {
			return err
		}
		if len(state.Queue) > 0 {
			sortQueue(state)
			if err := until(spec, scenario, state, state.Queue[0].At); err != nil {
				return err
			}
		}
	case "complete":
		state.Paused = false
		for len(state.Queue) > 0 {
			if err := next(spec, scenario, state); err != nil {
				return err
			}
			if state.Error == "simulation_transition_limit" || state.Error == "simulation_output_limit" {
				break
			}
		}
	default:
		return fmt.Errorf("unsupported virtual clock operation")
	}
	state.Finalized = len(state.Queue) == 0
	evaluateAssertions(scenario, state)
	return nil
}
func sortQueue(s *State) {
	sort.SliceStable(s.Queue, func(i, j int) bool {
		a, b := s.Queue[i], s.Queue[j]
		if !a.At.Equal(b.At) {
			return a.At.Before(b.At)
		}
		if taskPriority(a.Kind) != taskPriority(b.Kind) {
			return taskPriority(a.Kind) < taskPriority(b.Kind)
		}
		return a.Sequence < b.Sequence
	})
}
func until(spec workflow.Workflow, scenario Scenario, state *State, target time.Time) error {
	for len(state.Queue) > 0 {
		sortQueue(state)
		if state.Queue[0].At.After(target) {
			break
		}
		if err := next(spec, scenario, state); err != nil {
			return err
		}
		if state.Error == "simulation_transition_limit" || state.Error == "simulation_output_limit" {
			break
		}
	}
	return nil
}
func next(spec workflow.Workflow, scenario Scenario, s *State) error {
	if len(s.Queue) == 0 {
		return nil
	}
	if s.Transitions >= 5000 {
		s.fail("simulation_transition_limit")
		s.Queue = []Task{}
		return nil
	}
	sortQueue(s)
	task := s.Queue[0]
	s.Queue = s.Queue[1:]
	if task.At.After(scenario.StartTime.Add(100 * 365 * 24 * time.Hour)) {
		s.fail("simulation_horizon_limit")
		s.Queue = []Task{}
		return nil
	}
	if task.At.After(s.Now) {
		s.Now = task.At
	}
	s.Transitions++
	switch task.Kind {
	case "entry", "event":
		if task.Event == nil {
			return fmt.Errorf("event task missing input")
		}
		return onEvent(spec, scenario, s, *task.Event, task.Kind == "entry")
	case "attributes":
		if task.Change == nil {
			return fmt.Errorf("attribute task missing input")
		}
		attrs, _ := s.Customer["attributes"].(map[string]any)
		for key, value := range task.Change.Attributes {
			attrs[key] = value
		}
		s.Customer["attributes"] = attrs
		if task.Change.Consents != nil {
			consents := map[string]any{}
			for key, value := range task.Change.Consents {
				consents[key] = value
			}
			s.Customer["consents"] = consents
		}
		for key, value := range task.Change.Contact {
			if key == "email" || key == "phone" {
				s.Customer[key] = value
			}
		}
		s.trace("attributes", "", "Sandbox profile updated")
	case "node", "timer":
		if task.Execution != s.Execution || !s.Active || task.NodeID != s.CurrentNodeID {
			return nil
		}
		return onNode(spec, scenario, s, task)
	case "delivery":
		return onDelivery(scenario, s, task.MessageID)
	default:
		return fmt.Errorf("unknown simulation task")
	}
	return nil
}
func onEvent(spec workflow.Workflow, scenario Scenario, s *State, event Event, initial bool) error {
	if event.EventID == "" {
		event.EventID = "entry"
	}
	if event.CustomerID == "" {
		event.CustomerID = customerID(s.Customer)
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = s.Now
	}
	event.OccurredAt = event.OccurredAt.UTC().Truncate(time.Microsecond)
	if event.Properties == nil {
		event.Properties = map[string]any{}
	}
	if event.CustomerID != customerID(s.Customer) {
		s.trace("event", "", "Ignored event for another customer")
		return nil
	}
	if err := engine.ValidateEvent(runtimeEvent(event), s.Now); err != nil {
		s.trace("event", "", "REJECTED: "+err.Error())
		if initial {
			s.Status = "NOT_ENROLLED"
		}
		return nil
	}
	raw, _ := json.Marshal(runtimeEvent(event))
	sum := sha256.Sum256(raw)
	hash := hex.EncodeToString(sum[:])
	if previous, exists := s.SeenEvents[event.EventID]; exists {
		status := "DUPLICATE"
		if previous != hash {
			status = "REJECTED conflicting event ID"
		}
		s.trace("event", "", status+": "+event.EventID)
		return nil
	}
	s.SeenEvents[event.EventID] = hash
	if event.OccurredAt.Before(s.Now.Add(-24 * time.Hour)) {
		s.trace("event", "", "LATE: "+event.EventID)
		if initial {
			s.Status = "NOT_ENROLLED"
		}
		return nil
	}
	s.trace("event", "", "ACCEPTED: "+event.EventName)
	data := engine.ContextData(s.Customer, eventData(event), s.Context)
	if s.Active && spec.GlobalExit != nil && spec.GlobalExit.EventName == event.EventName && !event.OccurredAt.Before(s.EntryOccurredAt) && (spec.GlobalExit.Filters == nil || workflow.Evaluate(*spec.GlobalExit.Filters, data)) {
		s.Status = "EXITED"
		s.ExitReason = spec.GlobalExit.ExitOutcome()
		s.Active = false
		s.Outcomes[strconv.Itoa(s.Execution)] = s.Status
		s.trace("exit", s.CurrentNodeID, "Global exit: "+s.ExitReason)
		s.cancelPending()
	}
	if s.Active {
		node, _ := spec.Node(s.CurrentNodeID)
		if node.Type == "wait_event" && node.Config.EventName == event.EventName {
			index := stepIndex(s, s.Execution, node.ID)
			if index >= 0 {
				step := &s.Steps[index]
				if step.Deadline != nil && !s.Now.After(*step.Deadline) && !event.OccurredAt.Before(step.StartedAt) && (node.Config.Filters == nil || workflow.Evaluate(*node.Config.Filters, data)) {
					decision, err := engine.Decide(node, data, s.Now, "matched")
					if err != nil {
						return err
					}
					step.State = "SUCCEEDED"
					at := s.Now
					step.CompletedAt = &at
					step.Output = decision
					s.CurrentEvent = eventData(event)
					s.Context["_event_id"] = event.EventID
					queue := []Task{}
					for _, task := range s.Queue {
						if !(task.Kind == "timer" && task.Execution == s.Execution && task.NodeID == node.ID) {
							queue = append(queue, task)
						}
					}
					s.Queue = queue
					s.trace("node", node.ID, "Event matched")
					scheduleNode(spec, s, node.ID, decision.Port)
				}
			}
		}
	}
	if !s.Active && event.EventName == spec.Start().Config.EventName {
		root := spec.Start()
		if root.Config.Filters != nil && !workflow.Evaluate(*root.Config.Filters, data) {
			s.trace("entry", root.ID, "Entry filters did not match")
			if initial {
				s.Status = "NOT_ENROLLED"
			}
			return nil
		}
		if spec.Reentry != nil {
			if spec.Reentry.Mode == "once" && s.EnrollmentCount > 0 {
				return nil
			}
			if spec.Reentry.Mode == "cooldown" && s.LastEntryAt != nil && s.Now.Before(s.LastEntryAt.Add(time.Duration(spec.Reentry.CooldownSeconds)*time.Second)) {
				return nil
			}
		}
		s.Execution++
		s.EnrollmentCount++
		at := s.Now
		s.LastEntryAt = &at
		s.EntryOccurredAt = event.OccurredAt
		s.Status = "RUNNING"
		s.ExitReason = ""
		s.Active = true
		s.CurrentNodeID = root.ID
		s.Context = map[string]any{}
		s.CurrentEvent = eventData(event)
		s.Outcomes[strconv.Itoa(s.Execution)] = s.Status
		s.enqueue(Task{At: s.Now, Kind: "node", Execution: s.Execution, NodeID: root.ID})
		s.trace("entry", root.ID, "Customer enrolled")
	} else if initial && !s.Active && s.EnrollmentCount == 0 {
		s.Status = "NOT_ENROLLED"
	}
	return nil
}
func stepIndex(s *State, execution int, node string) int {
	for i := range s.Steps {
		if s.Steps[i].Execution == execution && s.Steps[i].NodeID == node {
			return i
		}
	}
	return -1
}
func scheduleNode(spec workflow.Workflow, s *State, source, port string) {
	next := spec.Next(source, port)
	if next == "" {
		s.fail("workflow has no next node")
		return
	}
	s.CurrentNodeID = next
	s.Status = "RUNNING"
	s.Outcomes[strconv.Itoa(s.Execution)] = s.Status
	s.enqueue(Task{At: s.Now, Kind: "node", Execution: s.Execution, NodeID: next})
}
