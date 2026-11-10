package delivery

import (
	"bufio"
	"context"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ofk/customer-journey-composer/apps/api/internal/messaging"
)

func TestMockRetriesAndStableIdempotency(t *testing.T) {
	provider := &Mock{Codes: []int{429, 500, 200}}
	message := messaging.Message{Channel: "sms", To: "+905551234567", Text: "Hello"}
	for _, status := range []string{"RETRY", "RETRY", "ACCEPTED"} {
		if out := provider.Send(context.Background(), message, "key"); out.Status != status {
			t.Fatal(out)
		}
	}
	first := provider.Send(context.Background(), message, "key")
	second := provider.Send(context.Background(), message, "key")
	if first.ProviderID != second.ProviderID || provider.calls != 3 {
		t.Fatal("mock idempotency violated")
	}
}
func TestSMTPRealProtocolAndAmbiguousAcceptance(t *testing.T) {
	for _, ambiguous := range []bool{false, true} {
		t.Run(map[bool]string{false: "accepted", true: "ambiguous"}[ambiguous], func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			received := make(chan string, 1)
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				reader := bufio.NewReader(conn)
				_, _ = io.WriteString(conn, "220 test ESMTP\r\n")
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					switch {
					case strings.HasPrefix(line, "EHLO"):
						_, _ = io.WriteString(conn, "250 test\r\n")
					case strings.HasPrefix(line, "MAIL"), strings.HasPrefix(line, "RCPT"):
						_, _ = io.WriteString(conn, "250 OK\r\n")
					case strings.HasPrefix(line, "DATA"):
						_, _ = io.WriteString(conn, "354 Send data\r\n")
						var body strings.Builder
						for {
							line, err := reader.ReadString('\n')
							if err != nil {
								return
							}
							if line == ".\r\n" {
								break
							}
							body.WriteString(line)
						}
						received <- body.String()
						if ambiguous {
							return
						}
						_, _ = io.WriteString(conn, "250 Accepted\r\n")
					case strings.HasPrefix(line, "QUIT"):
						_, _ = io.WriteString(conn, "221 Bye\r\n")
						return
					default:
						_, _ = io.WriteString(conn, "250 OK\r\n")
					}
				}
			}()
			host, port, _ := net.SplitHostPort(listener.Addr().String())
			provider := &SMTP{Host: host, Port: port, From: "sender@example.invalid"}
			out := provider.Send(context.Background(), messaging.Message{Channel: "email", To: "recipient@example.invalid", Subject: "Merhaba", Text: "Hello", HTML: "<p>Hello</p>"}, "logical-action")
			want := "ACCEPTED"
			if ambiguous {
				want = "UNKNOWN"
			}
			if out.Status != want {
				t.Fatalf("want %s got %+v", want, out)
			}
			body := <-received
			if !strings.Contains(body, "multipart/alternative") || !strings.Contains(body, "text/plain") || !strings.Contains(body, "text/html") || !strings.Contains(body, messageID("logical-action")) {
				t.Fatal("invalid MIME message")
			}
		})
	}
}
