package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/delivery"
)

var trackingCode = regexp.MustCompile(`^[A-Za-z0-9]{12}$`)

// A 1x1 transparent GIF.
var pixel = []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00, 0x01, 0x00, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0xff, 0xff, 0x21, 0xf9, 0x04, 0x01, 0x00, 0x00, 0x00, 0x00, 0x2c, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x02, 0x02, 0x44, 0x01, 0x00, 0x3b}

var unsubscribePage = template.Must(template.New("unsubscribe").Parse(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="robots" content="noindex"><title>{{.Title}}</title>
<style>body{margin:0;min-height:100vh;display:grid;place-items:center;background:#f6f7f9;color:#1f2933;font:15px/1.5 system-ui,-apple-system,Segoe UI,sans-serif}main{max-width:420px;margin:24px;padding:32px;background:#fff;border:1px solid #e3e6ea;border-radius:12px}h1{margin:0 0 8px;font-size:20px}p{margin:0 0 20px;color:#52606d}button{font:inherit;font-weight:600;padding:10px 18px;border:0;border-radius:8px;background:#1f2933;color:#fff;cursor:pointer}</style></head>
<body><main><h1>{{.Title}}</h1><p>{{.Body}}</p>{{if .Ask}}<form method="post"><button type="submit">Unsubscribe</button></form>{{end}}</main></body></html>`))

type trackedCode struct {
	workspace, delivery, kind, url, mode, customer, channel string
}

func registerTracking(mux *http.ServeMux, pool *pgxpool.Pool, log *slog.Logger) {
	lookup := func(ctx context.Context, code string) (trackedCode, error) {
		var t trackedCode
		err := pool.QueryRow(ctx, `SELECT t.workspace_id::text,t.delivery_id::text,t.kind,t.url,d.mode,COALESCE(d.customer_id,''),d.channel FROM tracking_codes t JOIN delivery_jobs d ON d.id=t.delivery_id AND d.workspace_id=t.workspace_id WHERE t.code=$1`, code).Scan(&t.workspace, &t.delivery, &t.kind, &t.url, &t.mode, &t.customer, &t.channel)
		return t, err
	}
	record := func(ctx context.Context, t trackedCode, code, kind string, detail map[string]any) error {
		if _, err := pool.Exec(ctx, `UPDATE tracking_codes SET uses=uses+1 WHERE code=$1`, code); err != nil {
			return err
		}
		now := time.Now().UTC()
		_, err := delivery.RecordEngagement(ctx, pool, t.workspace, t.delivery, kind, "tracking", detail, now, now)
		return err
	}
	page := func(w http.ResponseWriter, status int, title, body string, ask bool) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
		w.WriteHeader(status)
		_ = unsubscribePage.Execute(w, map[string]any{"Title": title, "Body": body, "Ask": ask})
	}
	mux.HandleFunc(delivery.TrackingPath+"{code}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		code := r.PathValue("code")
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodPost {
			w.Header().Set("Allow", "GET, HEAD, POST")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !trackingCode.MatchString(code) {
			http.NotFound(w, r)
			return
		}
		t, err := lookup(r.Context(), code)
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, "Temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		// A HEAD request is what link previews and scanners send: it gets the
		// same answer and counts nothing.
		counted := r.Method == http.MethodGet
		fail := func(err error) {
			log.ErrorContext(r.Context(), "tracking record failed", "kind", t.kind, "error", err.Error())
		}
		switch t.kind {
		case "open":
			if r.Method == http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if counted {
				if err := record(r.Context(), t, code, "opened", nil); err != nil {
					fail(err)
				}
			}
			w.Header().Set("Content-Type", "image/gif")
			_, _ = w.Write(pixel)
		case "click":
			if r.Method == http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if counted {
				// A click that cannot be counted still takes the person where they meant to go.
				if err := record(r.Context(), t, code, "clicked", map[string]any{"url": t.url}); err != nil {
					fail(err)
				}
			}
			http.Redirect(w, r, t.url, http.StatusFound)
		case "unsubscribe":
			// Opening the link only asks, because mail scanners open links.
			// The POST comes from the button below or a mail client's own
			// one-click unsubscribe (RFC 8058).
			if r.Method != http.MethodPost {
				page(w, http.StatusOK, "Unsubscribe from these emails?", "You will stop receiving marketing emails from us. Messages we must send, such as receipts, are not affected.", true)
				return
			}
			_ = r.ParseForm()
			if _, err := pool.Exec(r.Context(), `UPDATE tracking_codes SET uses=uses+1 WHERE code=$1`, code); err != nil {
				fail(err)
			}
			if err := delivery.Unsubscribe(r.Context(), pool, t.workspace, t.delivery, time.Now().UTC()); err != nil {
				fail(err)
				page(w, http.StatusServiceUnavailable, "Please try again", "We could not record your choice just now.", true)
				return
			}
			body := "You will not receive these emails any more."
			if t.mode != "LIVE" {
				body = "This was a test message, so no customer's consent changed."
			}
			page(w, http.StatusOK, "You're unsubscribed", body, false)
		}
	})
	mux.HandleFunc("POST /api/provider/events", providerEvents(pool, log))
}

// providerEvents takes delivery reports from an email or SMS provider. The
// body must be signed with PROVIDER_WEBHOOK_SECRET; without a secret the
// endpoint is off. A report names the message by the idempotency key we sent.
func providerEvents(pool *pgxpool.Pool, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		secret := os.Getenv("PROVIDER_WEBHOOK_SECRET")
		if secret == "" {
			http.NotFound(w, r)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 16<<10))
		if err != nil {
			http.Error(w, "Invalid body", http.StatusBadRequest)
			return
		}
		if !ValidProviderSignature(secret, body, r.Header.Get("X-Provider-Signature")) {
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}
		var report struct {
			IdempotencyKey string    `json:"idempotency_key"`
			Type           string    `json:"type"`
			OccurredAt     time.Time `json:"occurred_at"`
			Reason         string    `json:"reason"`
		}
		if err := json.Unmarshal(body, &report); err != nil || report.Type != "delivered" && report.Type != "bounced" {
			http.Error(w, "A report needs idempotency_key and a type of delivered or bounced", http.StatusUnprocessableEntity)
			return
		}
		workspace, key, ok := strings.Cut(report.IdempotencyKey, ":")
		if _, err := uuid.Parse(workspace); err != nil {
			ok = false
		}
		now := time.Now().UTC()
		if report.OccurredAt.IsZero() || report.OccurredAt.After(now) {
			report.OccurredAt = now
		}
		var id string
		if ok {
			err = pool.QueryRow(r.Context(), `SELECT id::text FROM delivery_jobs WHERE workspace_id=$1::uuid AND logical_key=$2`, workspace, key).Scan(&id)
		}
		if !ok || errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "Unknown message", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "Temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		detail := map[string]any{}
		if reason := strings.TrimSpace(report.Reason); reason != "" {
			detail["reason"] = reason[:min(len(reason), 300)]
		}
		if _, err := delivery.RecordEngagement(r.Context(), pool, workspace, id, report.Type, "provider", detail, report.OccurredAt, now); err != nil {
			log.ErrorContext(r.Context(), "provider report failed", "error", err.Error())
			// A 5xx makes the provider send the report again; recording is idempotent.
			http.Error(w, "Temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ValidProviderSignature checks "sha256=<hex HMAC-SHA256 of the body>".
func ValidProviderSignature(secret string, body []byte, header string) bool {
	given, ok := strings.CutPrefix(header, "sha256=")
	if !ok {
		return false
	}
	raw, err := hex.DecodeString(given)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hmac.Equal(raw, mac.Sum(nil))
}
