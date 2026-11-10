package delivery

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/messaging"
)

type Outcome struct {
	Status     string
	Code       int
	ProviderID string
	Response   string
}
type Provider interface {
	Send(context.Context, messaging.Message, string) Outcome
	Idempotent() bool
}
type Mock struct {
	mu       sync.Mutex
	Codes    []int
	calls    int
	accepted map[string]Outcome
}

func (*Mock) Idempotent() bool { return true }
func (m *Mock) Send(ctx context.Context, message messaging.Message, key string) Outcome {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.accepted == nil {
		m.accepted = map[string]Outcome{}
	}
	if existing, ok := m.accepted[key]; ok {
		return existing
	}
	if ctx.Err() != nil {
		return Outcome{Status: "RETRY", Code: 408, Response: "Mock request cancelled"}
	}
	code := 200
	if m.calls < len(m.Codes) {
		code = m.Codes[m.calls]
	}
	m.calls++
	if code == 429 || code >= 500 {
		return Outcome{Status: "RETRY", Code: code, Response: "Injected provider failure"}
	}
	if code == 408 {
		return Outcome{Status: "UNKNOWN", Code: 408, Response: "Injected ambiguous timeout"}
	}
	if code >= 400 {
		return Outcome{Status: "FAILED", Code: code, Response: "Injected permanent failure"}
	}
	sum := sha256.Sum256([]byte(key))
	out := Outcome{Status: "ACCEPTED", Code: 200, ProviderID: "mock_" + hex.EncodeToString(sum[:12]), Response: "Accepted by mock provider; no external message sent"}
	m.accepted[key] = out
	return out
}

type SMTP struct {
	Host, Port, From, User, Password string
	RequireTLS                       bool
}

func (*SMTP) Idempotent() bool { return false }
func SMTPFromEnv() *SMTP {
	host := os.Getenv("SMTP_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	port := os.Getenv("SMTP_PORT")
	if port == "" {
		port = "1025"
	}
	from := os.Getenv("SMTP_FROM")
	if from == "" {
		from = "journeys@localhost.invalid"
	}
	return &SMTP{Host: host, Port: port, From: from, User: os.Getenv("SMTP_USER"), Password: os.Getenv("SMTP_PASSWORD"), RequireTLS: os.Getenv("SMTP_REQUIRE_TLS") == "true"}
}
func (s *SMTP) Send(ctx context.Context, message messaging.Message, key string) Outcome {
	if message.Channel != "email" {
		return Outcome{Status: "FAILED", Response: "SMTP only supports email"}
	}
	sender, err := mail.ParseAddress(s.From)
	if err != nil {
		return Outcome{Status: "FAILED", Response: "Invalid SMTP sender configuration"}
	}
	recipient, err := mail.ParseAddress(message.To)
	if err != nil || recipient.Address != message.To {
		return Outcome{Status: "FAILED", Response: "Invalid recipient"}
	}
	if strings.ContainsAny(message.Subject, "\r\n") {
		return Outcome{Status: "FAILED", Response: "Invalid subject"}
	}
	data, err := emailBytes(message, sender.Address, key)
	if err != nil {
		return Outcome{Status: "FAILED", Response: "Could not encode email"}
	}
	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(s.Host, s.Port))
	if err != nil {
		return smtpError(err, false)
	}
	defer conn.Close()
	deadline := time.Now().Add(10 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	client, err := smtp.NewClient(conn, s.Host)
	if err != nil {
		return smtpError(err, false)
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return smtpError(err, false)
		}
	} else if s.RequireTLS {
		return Outcome{Status: "FAILED", Response: "SMTP provider does not offer required TLS"}
	}
	if s.User != "" {
		if err := client.Auth(smtp.PlainAuth("", s.User, s.Password, s.Host)); err != nil {
			return smtpError(err, false)
		}
	}
	if err := client.Mail(sender.Address); err != nil {
		return smtpError(err, false)
	}
	if err := client.Rcpt(recipient.Address); err != nil {
		return smtpError(err, false)
	}
	writer, err := client.Data()
	if err != nil {
		return smtpError(err, false)
	}
	if _, err := writer.Write(data); err != nil {
		return smtpError(err, true)
	}
	if err := writer.Close(); err != nil {
		return smtpError(err, true)
	}
	_ = client.Quit()
	return Outcome{Status: "ACCEPTED", Code: 250, ProviderID: messageID(key), Response: "Accepted by SMTP provider; delivery is not confirmed"}
}
func smtpError(err error, ambiguous bool) Outcome {
	var status *textproto.Error
	if errors.As(err, &status) {
		if status.Code >= 500 {
			return Outcome{Status: "FAILED", Code: status.Code, Response: "SMTP provider rejected the message"}
		}
		return Outcome{Status: "RETRY", Code: status.Code, Response: "SMTP provider requested retry"}
	}
	if ambiguous {
		return Outcome{Status: "UNKNOWN", Response: "SMTP acceptance is unknown; automatic retry disabled"}
	}
	return Outcome{Status: "RETRY", Response: "SMTP connection failed before acceptance"}
}
func messageID(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:]) + "@journey.local"
}
func emailBytes(message messaging.Message, from, key string) ([]byte, error) {
	var data bytes.Buffer
	writer := multipart.NewWriter(&data)
	boundary := writer.Boundary()
	data.Reset()
	fmt.Fprintf(&data, "From: %s\r\nTo: %s\r\nSubject: %s\r\nMessage-ID: <%s>\r\nMIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=%q\r\n\r\n", (&mail.Address{Address: from}).String(), (&mail.Address{Address: message.To}).String(), mime.QEncoding.Encode("UTF-8", message.Subject), messageID(key), boundary)
	for _, part := range []struct{ kind, body string }{{"text/plain", message.Text}, {"text/html", message.HTML}} {
		if part.body == "" {
			continue
		}
		headers := textproto.MIMEHeader{"Content-Type": {part.kind + "; charset=UTF-8"}, "Content-Transfer-Encoding": {"quoted-printable"}}
		body, err := writer.CreatePart(headers)
		if err != nil {
			return nil, err
		}
		encoded := quotedprintable.NewWriter(body)
		if _, err := io.WriteString(encoded, part.body); err != nil {
			return nil, err
		}
		if err := encoded.Close(); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return data.Bytes(), nil
}
