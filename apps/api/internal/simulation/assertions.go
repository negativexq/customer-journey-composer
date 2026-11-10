package simulation

import (
	"encoding/json"
	"fmt"
	"reflect"
)

func expectedNumber(value any) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	}
	return 0
}
func evaluateAssertions(scenario Scenario, s *State) {
	results := []AssertionResult{}
	all := true
	for _, assertion := range scenario.Assertions {
		result := AssertionResult{Assertion: assertion, Description: assertion.Type}
		switch assertion.Type {
		case "state":
			result.Actual = s.Status
			result.Pass = s.Status == fmt.Sprint(assertion.Expected)
		case "exit_reason":
			result.Actual = s.ExitReason
			result.Pass = s.ExitReason == fmt.Sprint(assertion.Expected)
		case "enrollment_count":
			result.Actual = s.EnrollmentCount
			result.Pass = s.EnrollmentCount == expectedNumber(assertion.Expected)
		case "message_count", "message_sent", "no_message":
			count := 0
			for _, message := range s.Messages {
				if (assertion.Channel == "" || message.Channel == assertion.Channel) && (assertion.NodeID == "" || message.NodeID == assertion.NodeID) && message.AcceptedAt != nil {
					count++
				}
			}
			result.Actual = count
			if assertion.Type == "message_count" {
				result.Pass = count == expectedNumber(assertion.Expected)
			} else if assertion.Type == "no_message" {
				result.Pass = count == 0
			} else {
				result.Pass = count > 0
				if expected, ok := assertion.Expected.(bool); ok && !expected {
					result.Pass = count == 0
				}
			}
		case "condition":
			values := []bool{}
			result.Pass = true
			for _, step := range s.Steps {
				if step.NodeID == assertion.NodeID && step.Output.Condition != nil {
					values = append(values, *step.Output.Condition)
					result.Pass = result.Pass && reflect.DeepEqual(*step.Output.Condition, assertion.Expected)
				}
			}
			result.Actual = values
			result.Pass = result.Pass && len(values) > 0
		case "no_duplicate_delivery":
			seen := map[string]bool{}
			duplicates := 0
			for _, message := range s.Messages {
				if seen[message.ID] {
					duplicates++
				}
				seen[message.ID] = true
			}
			result.Actual = duplicates
			result.Pass = duplicates == 0
		default:
			result.Description = "Unsupported assertion"
			result.Pass = false
		}
		results = append(results, result)
		all = all && result.Pass
	}
	s.Assertions = results
	s.Result = "PENDING"
	if s.Finalized {
		if all {
			s.Result = "PASS"
		} else {
			s.Result = "FAIL"
		}
	}
}
