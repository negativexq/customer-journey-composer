package messaging

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/microcosm-cc/bluemonday"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
	htmlparse "golang.org/x/net/html"
)

type Message struct {
	Channel    string            `json:"channel"`
	To         string            `json:"to"`
	Subject    string            `json:"subject"`
	Preheader  string            `json:"preheader"`
	HTML       string            `json:"html"`
	Text       string            `json:"text"`
	Used       map[string]string `json:"personalization_values"`
	Missing    []string          `json:"missing_attributes"`
	Links      []string          `json:"links"`
	Warnings   []string          `json:"warnings"`
	Encoding   string            `json:"encoding"`
	Units      int               `json:"units"`
	Segments   int               `json:"segments"`
	TemplateID string            `json:"template_id,omitempty"`
}

var variables = regexp.MustCompile(`\{\{\s*((?:customer|event|context)\.[a-zA-Z0-9_.]+)\s*\}\}`)
var policy = bluemonday.UGCPolicy()

func Render(channel string, c workflow.Config, data map[string]any) (Message, error) {
	m := Message{Channel: channel, Used: map[string]string{}, Missing: []string{}, Links: []string{}, Warnings: []string{}, TemplateID: c.TemplateID}
	render := func(content string, escape bool) string {
		return variables.ReplaceAllStringFunc(content, func(token string) string {
			path := variables.FindStringSubmatch(token)[1]
			value, ok := workflow.Lookup(data, path)
			text := ""
			if ok && value != nil {
				switch v := value.(type) {
				case string:
					text = v
				case float64:
					text = strconv.FormatFloat(v, 'f', -1, 64)
				case int:
					text = strconv.Itoa(v)
				case bool:
					text = strconv.FormatBool(v)
				case json.Number:
					text = v.String()
				default:
					ok = false
				}
			} else {
				ok = false
			}
			if !ok {
				if fallback, exists := c.Fallbacks[path]; exists {
					text = fallback
				} else {
					m.Missing = append(m.Missing, path)
				}
			}
			m.Used[path] = text
			if escape {
				return html.EscapeString(text)
			}
			return text
		})
	}
	if channel != "email" && channel != "sms" {
		return m, fmt.Errorf("unsupported channel")
	}
	for _, content := range []string{c.Subject, c.Preheader, c.HTML, c.Text} {
		if len(variables.FindAllStringIndex(content, 501)) > 500 {
			return m, fmt.Errorf("template exceeds 500 variable references")
		}
		rest := variables.ReplaceAllString(content, "")
		if strings.Contains(rest, "{{") || strings.Contains(rest, "}}") {
			return m, fmt.Errorf("unsupported personalization expression; only dotted variable paths are allowed")
		}
	}
	m.Subject = render(c.Subject, false)
	m.Preheader = render(c.Preheader, false)
	m.Text = render(c.Text, false)
	rawHTML := render(c.HTML, true)
	m.HTML = policy.Sanitize(rawHTML)
	if strings.ContainsAny(m.Subject+m.Preheader, "\r\n") {
		return m, fmt.Errorf("email header must not contain line breaks")
	}
	if channel == "email" {
		if v, ok := workflow.Lookup(data, "customer.email"); ok {
			m.To, _ = v.(string)
		}
		if m.Text == "" {
			m.Text = html.UnescapeString(bluemonday.StrictPolicy().Sanitize(m.HTML))
		}
	} else {
		if v, ok := workflow.Lookup(data, "customer.phone"); ok {
			m.To, _ = v.(string)
		}
		m.Encoding, m.Units, m.Segments = Segments(m.Text)
	}
	if m.HTML != rawHTML {
		m.Warnings = append(m.Warnings, "HTML was sanitized; unsupported markup or attributes were removed")
	}
	tokenizer := htmlparse.NewTokenizer(strings.NewReader(m.HTML))
	for {
		tt := tokenizer.Next()
		if tt == htmlparse.ErrorToken {
			if tokenizer.Err() != io.EOF {
				return m, fmt.Errorf("invalid rendered HTML")
			}
			break
		}
		if tt == htmlparse.StartTagToken || tt == htmlparse.SelfClosingTagToken {
			token := tokenizer.Token()
			for _, attr := range token.Attr {
				if attr.Key == "href" {
					u, err := url.Parse(attr.Val)
					if err != nil || u.Scheme != "https" && u.Scheme != "http" && u.Scheme != "mailto" {
						m.Warnings = append(m.Warnings, "An unsupported link was removed")
						continue
					}
					m.Links = append(m.Links, attr.Val)
				}
			}
		}
	}
	if len(m.Missing) == 0 {
		if channel == "email" && (strings.TrimSpace(m.Subject) == "" || strings.TrimSpace(m.Text) == "" && strings.TrimSpace(m.HTML) == "") {
			return m, fmt.Errorf("email subject and content are required")
		}
		if channel == "sms" && strings.TrimSpace(m.Text) == "" {
			return m, fmt.Errorf("SMS content is required")
		}
	}
	slices.Sort(m.Missing)
	m.Missing = slices.Compact(m.Missing)
	return m, nil
}

const gsmBasic = "@£$¥èéùìòÇ\nØø\rÅåΔ_ΦΓΛΩΠΨΣΘΞ\x1bÆæßÉ !\"#¤%&'()*+,-./0123456789:;<=>?¡ABCDEFGHIJKLMNOPQRSTUVWXYZÄÖÑÜ§¿abcdefghijklmnopqrstuvwxyzäöñüà"
const gsmExtended = "^{}\\[~]|€\f"

func Segments(text string) (encoding string, units, segments int) {
	encoding = "GSM-7"
	for _, r := range text {
		if strings.ContainsRune(gsmBasic, r) {
			units++
		} else if strings.ContainsRune(gsmExtended, r) {
			units += 2
		} else {
			encoding = "Unicode"
			break
		}
	}
	single, multi := 160, 153
	if encoding == "Unicode" {
		units = len(utf16.Encode([]rune(text)))
		single, multi = 70, 67
	}
	if units == 0 {
		return encoding, 0, 0
	}
	if units <= single {
		return encoding, units, 1
	}
	return encoding, units, (units + multi - 1) / multi
}
