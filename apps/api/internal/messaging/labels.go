package messaging

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

var (
	blockEnd = regexp.MustCompile(`(?i)<br\s*/?>|</(p|div|li|h[1-6]|tr|td|table|ul|ol)>`)
	tag      = regexp.MustCompile(`<[^>]*>`)
)

// danglingLabel finds a line like "Your gift code:" that announces something
// the message never gives: nothing follows it, or only another such line. A
// model told not to invent a code or link can leave the label behind. end says
// something is shown after the text, such as the email's discount code box.
func danglingLabel(text string, end bool) string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	label := func(line string) bool { return strings.HasSuffix(line, ":") || strings.HasSuffix(line, "：") }
	for i, line := range lines {
		if !label(line) {
			continue
		}
		if i+1 < len(lines) && !label(lines[i+1]) || i+1 == len(lines) && end {
			continue
		}
		if runes := []rune(line); len(runes) > 60 {
			line = string(runes[:57]) + "..."
		}
		return line
	}
	return ""
}

func htmlLines(content string) string {
	return html.UnescapeString(tag.ReplaceAllString(blockEnd.ReplaceAllString(content, "\n"), ""))
}

func labelIssue(line string) string {
	return fmt.Sprintf("%q introduces nothing; add what it announces or remove the line", line)
}

// LabelIssues reports a message line that announces a value it never shows.
func LabelIssues(node workflow.Node) []workflow.Issue {
	if node.Type != "email" && node.Type != "sms" {
		return nil
	}
	// The email's code box comes after its text and HTML.
	box := node.Type == "email" && ValidDiscountCode(node.Config.DiscountCode)
	var issues []workflow.Issue
	for _, part := range [][2]string{{"text", node.Config.Text}, {"html", htmlLines(node.Config.HTML)}} {
		if line := danglingLabel(part[1], box); line != "" {
			issues = append(issues, workflow.Issue{NodeID: node.ID, Field: part[0], Message: labelIssue(line)})
		}
	}
	return issues
}
