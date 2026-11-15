package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WebhookCall is what a webhook step asks to be delivered. The step only
// records it; WebhookDispatcher sends it.
type WebhookCall struct {
	URL     string         `json:"url"`
	Payload map[string]any `json:"payload"`
}

// WebhookDispatcher sends recorded webhook calls. A request leaves the system
// only when its host is on the operator's allowlist; every other call is kept
// as RECORDED so the journey can be built and inspected without sending data
// to an address someone typed into a step.
type WebhookDispatcher struct {
	Pool     *pgxpool.Pool
	Client   *http.Client
	Allowed  map[string]bool
	Log      *slog.Logger
	Interval time.Duration
}

const webhookAttempts = 3

// AllowedWebhookHosts reads a comma-separated host list, e.g.
// "hooks.example.com,localhost:9000". Empty means nothing is sent.
func AllowedWebhookHosts(value string) map[string]bool {
	hosts := map[string]bool{}
	for _, host := range strings.Split(value, ",") {
		if host = strings.ToLower(strings.TrimSpace(host)); host != "" {
			hosts[host] = true
		}
	}
	return hosts
}

func NewWebhookDispatcher(pool *pgxpool.Pool, allowed map[string]bool) *WebhookDispatcher {
	return &WebhookDispatcher{
		Pool:    pool,
		Allowed: allowed,
		// Redirects are not followed: an allowed host must not be able to
		// forward the request, and its data, somewhere else.
		Client: &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}},
		Interval: 500 * time.Millisecond,
	}
}

func (d *WebhookDispatcher) Run(ctx context.Context) {
	ticker := time.NewTicker(d.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for {
				handled, err := d.Dispatch(ctx)
				if err != nil && d.Log != nil && ctx.Err() == nil {
					d.Log.Error("webhook dispatch failed", "error", err)
				}
				if err != nil || !handled {
					break
				}
			}
		}
	}
}

// Dispatch handles one pending call and reports whether there was one. The
// row stays locked for the duration of the request, so two dispatchers never
// send the same call concurrently. A crash after the request and before the
// commit sends it again; the Idempotency-Key header lets a receiver drop that.
func (d *WebhookDispatcher) Dispatch(ctx context.Context) (bool, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var id, target string
	var payload []byte
	var attempts int
	err = tx.QueryRow(ctx, `SELECT id::text,url,payload,attempts FROM webhook_calls WHERE state='PENDING' ORDER BY created_at,id LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&id, &target, &payload, &attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	state, status, detail := d.send(ctx, id, target, payload)
	attempts++
	// A failed request is retried on a later pass until the attempts run out.
	if state == "FAILED" && attempts < webhookAttempts {
		_, err = tx.Exec(ctx, `UPDATE webhook_calls SET attempts=$2,response_status=$3,detail=$4 WHERE id=$1`, id, attempts, status, detail)
	} else {
		_, err = tx.Exec(ctx, `UPDATE webhook_calls SET state=$2,attempts=$3,response_status=$4,detail=$5,completed_at=now() WHERE id=$1`, id, state, attempts, status, detail)
	}
	if err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

func (d *WebhookDispatcher) send(ctx context.Context, id, target string, payload []byte) (state string, status *int, detail string) {
	parsed, err := url.Parse(target)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "FAILED", nil, "The address is not a valid http or https URL"
	}
	if !d.Allowed[strings.ToLower(parsed.Host)] {
		return "RECORDED", nil, "Not sent: this host is not on the server's webhook allowlist"
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		return "FAILED", nil, "The request could not be built"
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", id)
	request.Header.Set("User-Agent", "journey-composer-webhook")
	response, err := d.Client.Do(request)
	if err != nil {
		return "FAILED", nil, "The receiver could not be reached"
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	code := response.StatusCode
	if code >= 200 && code < 300 {
		return "SENT", &code, fmt.Sprintf("Receiver answered %d", code)
	}
	return "FAILED", &code, fmt.Sprintf("Receiver answered %d", code)
}
