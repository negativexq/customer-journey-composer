package messaging

import (
	"strings"
	"testing"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

func TestDanglingLabel(t *testing.T) {
	for _, c := range []struct {
		text string
		end  bool
		want string
	}{
		// The live birthday email: no link and no code was given.
		{"Doğum günün kutlu olsun!\n\nMağazaya dönmek için:\n\nHediye kodun:", false, "Mağazaya dönmek için:"},
		{"Hediye kodun:", false, "Hediye kodun:"},
		{"Öne çıkanlar:\n\n- Ceket\n- Bot", false, ""},
		{"Mağazaya dönmek için:\nhttps://example.com/magaza", false, ""},
		{"Kodun: {{ offer.code }}", false, ""},
		// The email's code box follows the text.
		{"Sana özel kodun:", true, ""},
		{"Merhaba Deniz,\nSepetin seni bekliyor.", false, ""},
	} {
		if got := danglingLabel(c.text, c.end); got != c.want {
			t.Errorf("%q: got %q, want %q", c.text, got, c.want)
		}
	}
}

func TestEmptyLabelIsAMessageIssue(t *testing.T) {
	node := func(kind string, c workflow.Config) workflow.Workflow {
		c.EventName = "birthday"
		return workflow.Workflow{SchemaVersion: 1, Nodes: []workflow.Node{
			{ID: "start", Type: "event_trigger", Label: "Birthday", Config: workflow.Config{EventName: "birthday"}},
			{ID: "send", Type: kind, Label: "Send", Config: c},
			{ID: "done", Type: "exit", Label: "Done", Config: workflow.Config{Reason: "completed"}},
		}, Edges: []workflow.Edge{
			{ID: "a", Source: "start", Target: "send", Port: "started"},
			{ID: "b", Source: "send", Target: "done", Port: "next"},
		}}
	}
	labels := func(w workflow.Workflow) []string {
		var found []string
		for _, issue := range ValidateWorkflow(w) {
			if strings.Contains(issue.Message, "introduces nothing") {
				found = append(found, issue.Field+": "+issue.Message)
			}
		}
		return found
	}
	email := workflow.Config{Subject: "Doğum günün kutlu olsun", HTML: "<p>Doğum günün kutlu olsun!</p><p>Hediye kodun:</p>", Text: "Doğum günün kutlu olsun!\n\nHediye kodun:"}
	if got := labels(node("email", email)); len(got) != 2 || !strings.HasPrefix(got[0], `text: "Hediye kodun:"`) || !strings.HasPrefix(got[1], "html: ") {
		t.Fatalf("email without a code: %v", got)
	}
	email.DiscountCode = "BDAY20"
	if got := labels(node("email", email)); len(got) != 0 {
		t.Fatalf("the code box completes the label: %v", got)
	}
	if got := labels(node("sms", workflow.Config{Text: "Hediye kodun:", DiscountCode: "BDAY20"})); len(got) != 1 {
		t.Fatalf("an SMS has no code box: %v", got)
	}
}
