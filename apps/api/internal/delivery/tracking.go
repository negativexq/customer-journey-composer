package delivery

import (
	"crypto/rand"
	"html"
	"os"
	"regexp"
	"strings"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/messaging"
)

// TrackingCode is one tracked address of a sent message.
type TrackingCode struct {
	Code string `json:"code"`
	Kind string `json:"kind"`
	URL  string `json:"url"`
}

// TrackingPath is where every tracked address is served from.
const TrackingPath = "/api/t/"

// TrackingBase is the public address that tracked links, the open pixel and
// the unsubscribe link point at. A deployment sets PUBLIC_BASE_URL to the
// address customers can reach; the default suits the local stack.
func TrackingBase() string {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("PUBLIC_BASE_URL")), "/")
	if base == "" {
		base = "http://localhost:5173"
	}
	return base
}

const codeAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

// NewTrackingCode returns 12 random letters and digits, about 71 bits: a code
// is a capability, so it must not be guessed from another one.
func NewTrackingCode() string {
	out := make([]byte, 0, 12)
	var buf [32]byte
	for len(out) < 12 {
		_, _ = rand.Read(buf[:])
		for _, b := range buf {
			// 248 is the largest multiple of 62 below 256, so every letter is equally likely.
			if b < 248 && len(out) < 12 {
				out = append(out, codeAlphabet[int(b)%len(codeAlphabet)])
			}
		}
	}
	return string(out)
}

var (
	trackedHref = regexp.MustCompile(`href="(https?://[^"]*)"`)
	textLink    = regexp.MustCompile(`https?://[^\s<>"]+`)
)

// Track prepares a rendered message for one delivery: each http(s) link goes
// through its own code, and an email gets an open pixel and an unsubscribe
// link in its footer and headers. The returned codes must be stored with the
// delivery before the message can be sent.
func Track(m *messaging.Message, base string) []TrackingCode {
	codes := []TrackingCode{}
	byURL := map[string]string{}
	address := func(code string) string { return base + TrackingPath + code }
	link := func(raw string) string {
		if code, ok := byURL[raw]; ok {
			return address(code)
		}
		code := NewTrackingCode()
		byURL[raw] = code
		codes = append(codes, TrackingCode{Code: code, Kind: "click", URL: raw})
		return address(code)
	}
	text := func(content string) string {
		return textLink.ReplaceAllStringFunc(content, func(found string) string {
			// Sentence punctuation after a link is not part of it.
			trimmed := strings.TrimRight(found, ".,;:!?)'")
			if len(trimmed) > 2000 {
				return found
			}
			return link(trimmed) + found[len(trimmed):]
		})
	}
	switch m.Channel {
	case "email":
		m.HTML = trackedHref.ReplaceAllStringFunc(m.HTML, func(attr string) string {
			raw := html.UnescapeString(trackedHref.FindStringSubmatch(attr)[1])
			if len(raw) > 2000 {
				return attr
			}
			return `href="` + html.EscapeString(link(raw)) + `"`
		})
		m.Text = text(m.Text)
		unsubscribe, open := NewTrackingCode(), NewTrackingCode()
		codes = append(codes, TrackingCode{Code: unsubscribe, Kind: "unsubscribe"}, TrackingCode{Code: open, Kind: "open"})
		m.Unsubscribe = address(unsubscribe)
		footer := `<p style="margin:24px 0 0;font:12px/1.5 Arial,Helvetica,sans-serif;color:#6b7280;text-align:center">Don't want these emails? <a href="` + html.EscapeString(m.Unsubscribe) + `" style="color:#6b7280">Unsubscribe</a></p><img src="` + html.EscapeString(address(open)) + `" width="1" height="1" alt="" style="display:block;width:1px;height:1px;border:0">`
		if m.HTML != "" {
			if i := strings.LastIndex(strings.ToLower(m.HTML), "</body>"); i >= 0 {
				m.HTML = m.HTML[:i] + footer + m.HTML[i:]
			} else {
				m.HTML += footer
			}
		}
		m.Text = strings.TrimRight(m.Text, "\n") + "\n\nUnsubscribe: " + m.Unsubscribe
	case "sms":
		m.Text = text(m.Text)
		m.Encoding, m.Units, m.Segments = messaging.Segments(m.Text)
	}
	return codes
}
