// Package factcheck compares generated message text with the brief it was
// written from, without a model. It reads only what it can read for certain:
// links, discount codes, numbers and lengths of time. Generated text that
// states one the brief does not is refused, and a link, code or discount the
// brief asks for that no message carries is noted for the person reviewing.
// Reading less than a person does is the point: it cannot prove a message
// right, only catch these disagreements. People editing a message may write
// what they like; only generated text is checked.
package factcheck

import (
	"fmt"
	"html"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

var (
	variable = regexp.MustCompile(`\{\{[^{}]*\}\}`)
	tag      = regexp.MustCompile(`<[^>]*>`)
	href     = regexp.MustCompile(`(?i)\bhref\s*=\s*["']([^"']+)["']`)
	// A link has a scheme, starts with www., or is a domain followed by a
	// path. A domain ends in letters, so "3.5/5" is not a link.
	link = regexp.MustCompile(`(?i)\b(?:https?://[^\s<>"']+|www\.[^\s<>"']+|[a-z0-9][a-z0-9-]*(?:\.[a-z0-9-]+)*\.[a-z]{2,}/[^\s<>"']*)`)
	// A code is what a customer types at checkout: capitals and digits, with
	// at least one of each, so "SMS" or "2026" is not one.
	code     = regexp.MustCompile(`\b[A-Z0-9][A-Z0-9-]{2,29}\b`)
	number   = regexp.MustCompile(`\d+(?:[.,]\d+)*`)
	duration = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*(dakika|dk|saat|gün|gun|hafta|ay|minutes?|mins?|hours?|hrs?|days?|weeks?|months?)\b`)
	percent  = regexp.MustCompile(`(?i)%\s*(\d+(?:[.,]\d+)?)|(\d+(?:[.,]\d+)?)\s*%|yüzde\s*(\d+(?:[.,]\d+)?)|(\d+(?:[.,]\d+)?)\s*percent`)
	discount = regexp.MustCompile(`(?i)indirim|discount|\boff\b|rabatt|réduction|descuento`)
)

var unitSeconds = map[string]int64{"dakika": 60, "dk": 60, "minute": 60, "minutes": 60, "min": 60, "mins": 60, "saat": 3600, "hour": 3600, "hours": 3600, "hr": 3600, "hrs": 3600, "gün": 86400, "gun": 86400, "day": 86400, "days": 86400, "hafta": 604800, "week": 604800, "weeks": 604800, "ay": 2592000, "month": 2592000, "months": 2592000}

// Facts are what a text states that this package can read.
type Facts struct {
	Links     map[string]bool
	Codes     map[string]bool
	Numbers   map[string]bool
	Durations map[int64]bool
	// Discounts are percentages written next to a word for a discount.
	Discounts map[string]bool
}

// plain drops variables and markup and decodes entities, so "&#39;" is not
// read as the number 39.
func plain(text string) string {
	return html.UnescapeString(tag.ReplaceAllString(variable.ReplaceAllString(text, " "), " "))
}

func newFacts() Facts {
	return Facts{Links: map[string]bool{}, Codes: map[string]bool{}, Numbers: map[string]bool{}, Durations: map[int64]bool{}, Discounts: map[string]bool{}}
}

// NormalLink gives a link the form it is compared in: a scheme, a lowercase
// host, and no trailing slash or sentence punctuation.
func NormalLink(raw string) string {
	// A Turkish suffix after an apostrophe is not part of the link.
	raw, _, _ = strings.Cut(strings.TrimSpace(raw), "’")
	raw = strings.TrimRight(raw, ".,;:!?)]}'\"")
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	scheme, rest, _ := strings.Cut(raw, "://")
	host, path, _ := strings.Cut(rest, "/")
	out := strings.ToLower(scheme) + "://" + strings.ToLower(host)
	if path = strings.TrimRight(path, "/"); path != "" {
		out += "/" + path
	}
	// http and https name the same page for this comparison.
	return strings.Replace(out, "http://", "https://", 1)
}

func normalNumber(raw string) string {
	raw = strings.ReplaceAll(raw, ",", ".")
	if strings.Count(raw, ".") > 1 {
		// 1.000.000 is a grouped integer.
		raw = strings.ReplaceAll(raw, ".", "")
	}
	if f, err := strconv.ParseFloat(raw, 64); err == nil {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return raw
}

// Read collects the facts of one text. Variables are left out, because their
// values come from the customer, not from the writer.
func Read(text string) Facts {
	f := newFacts()
	for _, m := range href.FindAllStringSubmatch(text, -1) {
		if strings.HasPrefix(strings.ToLower(m[1]), "http") {
			f.Links[NormalLink(m[1])] = true
		}
	}
	text = plain(text)
	for _, l := range link.FindAllString(text, -1) {
		f.Links[NormalLink(l)] = true
	}
	text = link.ReplaceAllString(text, " ")
	for _, c := range code.FindAllString(text, -1) {
		if strings.ContainsAny(c, "0123456789") && strings.ContainsAny(c, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
			f.Codes[c] = true
		}
	}
	text = code.ReplaceAllStringFunc(text, func(c string) string {
		if f.Codes[c] {
			return " "
		}
		return c
	})
	for _, m := range duration.FindAllStringSubmatch(text, -1) {
		if seconds, ok := unitSeconds[strings.ToLower(m[2])]; ok {
			if v, err := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", "."), 64); err == nil {
				f.Durations[int64(v*float64(seconds))] = true
			}
		}
	}
	for _, loc := range percent.FindAllStringSubmatchIndex(text, -1) {
		value := ""
		for g := 2; g < len(loc); g += 2 {
			if loc[g] >= 0 {
				value = text[loc[g]:loc[g+1]]
			}
		}
		// A percentage is a discount when a word for one is in its clause.
		start, end := max(0, loc[0]-40), min(len(text), loc[1]+40)
		if discount.MatchString(text[start:end]) {
			f.Discounts[normalNumber(value)] = true
		}
	}
	for _, n := range number.FindAllString(text, -1) {
		f.Numbers[normalNumber(n)] = true
	}
	return f
}

func (f Facts) add(other Facts) {
	for k := range other.Links {
		f.Links[k] = true
	}
	for k := range other.Codes {
		f.Codes[k] = true
	}
	for k := range other.Numbers {
		f.Numbers[k] = true
	}
	for k := range other.Durations {
		f.Durations[k] = true
	}
	for k := range other.Discounts {
		f.Discounts[k] = true
	}
}

// Given collects what the brief and any approved text may state.
func Given(texts ...string) Facts {
	f := newFacts()
	for _, t := range texts {
		f.add(Read(t))
	}
	return f
}

func messageText(n workflow.Node) string {
	c := n.Config
	return strings.Join([]string{c.Subject, c.Preheader, c.HTML, c.Text}, "\n")
}

func sorted(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Check refuses generated text in the given message steps that states a
// link, code or number the given facts do not contain. A number is also
// accepted as a length of time the brief gives in another unit ("24 saat"
// for "1 gün"). A step's discount code must be given too, and its text shows
// it as {{ offer.code }} instead of typing it out.
func Check(given Facts, w workflow.Workflow, steps map[string]bool) []workflow.Issue {
	issues := []workflow.Issue{}
	for _, n := range w.Nodes {
		if (n.Type != "email" && n.Type != "sms") || (steps != nil && !steps[n.ID]) {
			continue
		}
		issue := func(message string) {
			issues = append(issues, workflow.Issue{NodeID: n.ID, Field: "facts", Message: message})
		}
		if c := n.Config.DiscountCode; c != "" && !given.Codes[c] {
			issue(fmt.Sprintf("Discount code %s is not in the brief; use only a code the brief names", c))
		}
		stated := Read(messageText(n))
		for _, l := range sorted(stated.Links) {
			if !given.Links[l] {
				issue(fmt.Sprintf("Link %s is not in the brief; use only links the brief gives", l))
			}
		}
		for _, c := range sorted(stated.Codes) {
			switch {
			case c == n.Config.DiscountCode:
				issue(fmt.Sprintf("Write the code as {{ offer.code }} instead of typing %s", c))
			case given.Codes[c]:
				issue(fmt.Sprintf("Set discount_code to %s and write {{ offer.code }} instead of typing it", c))
			case !given.Codes[c]:
				issue(fmt.Sprintf("Code %s is not in the brief", c))
			}
		}
		durations := map[string]bool{}
		for _, m := range duration.FindAllStringSubmatch(link.ReplaceAllString(plain(messageText(n)), " "), -1) {
			if seconds, ok := unitSeconds[strings.ToLower(m[2])]; ok {
				if v, err := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", "."), 64); err == nil && given.Durations[int64(v*float64(seconds))] {
					durations[normalNumber(m[1])] = true
				}
			}
		}
		for _, number := range sorted(stated.Numbers) {
			if !given.Numbers[number] && !durations[number] {
				issue(fmt.Sprintf("The number %s is not in the brief; do not add figures, times or amounts the brief does not state", number))
			}
		}
	}
	return issues
}

// NotDone lists what the brief asks a message to carry that no message step
// does: a link, a code or a discount percentage. These are notes, not
// refusals, because only the person knows whether the outcome is acceptable.
func NotDone(brief string, w workflow.Workflow) []string {
	asked := Read(brief)
	carried := newFacts()
	for _, n := range w.Nodes {
		if n.Type != "email" && n.Type != "sms" {
			continue
		}
		carried.add(Read(messageText(n)))
		if n.Config.DiscountCode != "" {
			carried.Codes[n.Config.DiscountCode] = true
		}
	}
	notes := []string{}
	for _, l := range sorted(asked.Links) {
		if !carried.Links[l] {
			notes = append(notes, fmt.Sprintf("The brief gives the link %s, but no message uses it.", l))
		}
	}
	for _, c := range sorted(asked.Codes) {
		if !carried.Codes[c] {
			notes = append(notes, fmt.Sprintf("The brief names the code %s, but no message offers it.", c))
		}
	}
	for _, d := range sorted(asked.Discounts) {
		if !carried.Discounts[d] && !carried.Numbers[d] {
			notes = append(notes, fmt.Sprintf("The brief offers %s%% off, but no message says so.", d))
		}
	}
	return notes
}
