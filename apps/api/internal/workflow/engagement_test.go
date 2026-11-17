package workflow

import (
	"strings"
	"testing"
)

func engagementFlow(event, message string, before bool) Workflow {
	w := Workflow{SchemaVersion: 1, Nodes: []Node{
		{ID: "start", Type: "event_trigger", Label: "Start", Config: Config{EventName: "cart_updated"}},
		{ID: "email", Type: "email", Label: "Email", Config: Config{Subject: "Hi", Text: "Hi"}},
		{ID: "sms", Type: "sms", Label: "SMS", Config: Config{Text: "Hi"}},
		{ID: "wait", Type: "wait_event", Label: "Wait", Config: Config{EventName: event, MessageNode: message, TimeoutSeconds: 3600}},
		{ID: "yes", Type: "exit", Label: "Yes", Config: Config{Reason: "converted"}},
		{ID: "no", Type: "exit", Label: "No", Config: Config{Reason: "completed"}},
	}}
	if before {
		w.Edges = []Edge{{ID: "a", Source: "start", Target: "email", Port: "started"}, {ID: "b", Source: "email", Target: "sms", Port: "next"}, {ID: "c", Source: "sms", Target: "wait", Port: "next"}}
	} else {
		w.Edges = []Edge{{ID: "a", Source: "start", Target: "wait", Port: "started"}, {ID: "b", Source: "wait", Target: "email", Port: "matched"}, {ID: "c", Source: "email", Target: "sms", Port: "next"}, {ID: "c2", Source: "sms", Target: "yes", Port: "next"}}
		w.Edges = append(w.Edges, Edge{ID: "d", Source: "wait", Target: "no", Port: "timeout"})
		return w
	}
	w.Edges = append(w.Edges, Edge{ID: "d", Source: "wait", Target: "yes", Port: "matched"}, Edge{ID: "e", Source: "wait", Target: "no", Port: "timeout"})
	return w
}

func messageIssue(w Workflow) string {
	for _, issue := range Validate(w) {
		if issue.Field == "message_node" {
			return issue.Message
		}
	}
	return ""
}

func TestAStepCanWaitForEngagementWithAnEarlierMessage(t *testing.T) {
	for _, c := range []struct{ event, message, want string }{
		{"message_clicked", "email", ""},
		{"message_clicked", "sms", ""},
		{"message_opened", "email", ""},
		{"message_opened", "sms", "An SMS has no opened event"},
		{"message_unsubscribed", "sms", "An SMS has no unsubscribed event"},
		{"purchase_completed", "email", "Wait for a message event"},
		{"message_clicked", "start", "Choose an email, SMS or push step"},
		{"message_clicked", "missing", "Choose an email, SMS or push step"},
	} {
		got := messageIssue(engagementFlow(c.event, c.message, true))
		if c.want == "" && got != "" || c.want != "" && !strings.HasPrefix(got, c.want) {
			t.Errorf("%s on %s: got %q, want %q", c.event, c.message, got, c.want)
		}
	}
	if got := messageIssue(engagementFlow("message_clicked", "email", false)); got != "The message step must come before this step" {
		t.Errorf("a message after the wait: %q", got)
	}
}

func TestEngagementMatchesOnlyItsOwnMessageAndRun(t *testing.T) {
	c := Config{MessageNode: "email"}
	cases := []struct {
		execution  string
		properties map[string]any
		want       bool
	}{
		{"run-1", map[string]any{"node_id": "email", "execution_id": "run-1"}, true},
		{"run-1", map[string]any{"node_id": "email", "execution_id": "run-2"}, false},
		{"run-1", map[string]any{"node_id": "other", "execution_id": "run-1"}, false},
		{"", map[string]any{"node_id": "email"}, true},
		{"run-1", map[string]any{}, false},
	}
	for _, x := range cases {
		if got := c.MatchesMessage(x.execution, x.properties); got != x.want {
			t.Errorf("%s %v: got %v", x.execution, x.properties, got)
		}
	}
	if !(Config{}).MatchesMessage("run-1", nil) {
		t.Error("an unscoped wait matches every event of its name")
	}
}
