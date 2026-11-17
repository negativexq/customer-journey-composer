package workflow

import (
	"strings"
	"testing"
)

func pushFlow(c Config) Workflow {
	return Workflow{SchemaVersion: 1, Nodes: []Node{
		{ID: "start", Type: "event_trigger", Label: "Start", Config: Config{EventName: "cart_updated"}},
		{ID: "push", Type: "push", Label: "Push", Config: c},
		{ID: "done", Type: "exit", Label: "Done", Config: Config{Reason: "completed"}},
	}, Edges: []Edge{{ID: "a", Source: "start", Target: "push", Port: "started"}, {ID: "b", Source: "push", Target: "done", Port: "next"}}}
}

func fieldIssue(w Workflow, field string) string {
	for _, issue := range Validate(w) {
		if issue.NodeID == "push" && issue.Field == field {
			return issue.Message
		}
	}
	return ""
}

func TestAPushStepNeedsAOneLineTitleABodyAndAWebLink(t *testing.T) {
	if issues := Validate(pushFlow(Config{Subject: "Your cart", Text: "Still waiting", URL: "https://example.com/cart"})); len(issues) != 0 {
		t.Fatalf("a complete push step: %v", issues)
	}
	for _, c := range []struct {
		config Config
		field  string
	}{
		{Config{Text: "Body"}, "subject"},
		{Config{Subject: "Two\nlines", Text: "Body"}, "subject"},
		{Config{Subject: strings.Repeat("a", PushTitleLimit+1), Text: "Body"}, "subject"},
		{Config{Subject: "Title"}, "text"},
		{Config{Subject: "Title", Text: strings.Repeat("a", PushBodyLimit+1)}, "text"},
		{Config{Subject: "Title", Text: "Body", URL: "javascript:alert(1)"}, "url"},
		{Config{Subject: "Title", Text: "Body", URL: "https://user:pw@example.com"}, "url"},
	} {
		if fieldIssue(pushFlow(c.config), c.field) == "" {
			t.Errorf("%+v: expected a %s issue", c.config, c.field)
		}
	}
}

func TestAWaitCanFollowAPushTapButNotAnOpen(t *testing.T) {
	flow := func(event string) Workflow {
		w := pushFlow(Config{Subject: "Title", Text: "Body"})
		w.Nodes = append(w.Nodes, Node{ID: "wait", Type: "wait_event", Label: "Wait", Config: Config{EventName: event, MessageNode: "push", TimeoutSeconds: 3600}}, Node{ID: "yes", Type: "exit", Label: "Yes", Config: Config{Reason: "converted"}})
		w.Edges = []Edge{{ID: "a", Source: "start", Target: "push", Port: "started"}, {ID: "b", Source: "push", Target: "wait", Port: "next"}, {ID: "c", Source: "wait", Target: "yes", Port: "matched"}, {ID: "d", Source: "wait", Target: "done", Port: "timeout"}}
		return w
	}
	if got := messageIssue(flow("message_clicked")); got != "" {
		t.Errorf("a tap: %q", got)
	}
	if got := messageIssue(flow("message_opened")); got != "A push notification has no opened event" {
		t.Errorf("an open: %q", got)
	}
}

func TestEveryChannelHasItsContactField(t *testing.T) {
	for channel, want := range map[string]string{"email": "email", "sms": "phone", "push": "push_token"} {
		if got := ContactField(channel); got != want {
			t.Errorf("%s: %s", channel, got)
		}
	}
}
