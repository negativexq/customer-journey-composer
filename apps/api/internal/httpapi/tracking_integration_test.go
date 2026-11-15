//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/database"
)

func TestTrackedLinksPixelUnsubscribeAndProviderReports(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Open(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	secret := randomToken()
	t.Setenv("PROVIDER_WEBHOOK_SECRET", secret)
	server := httptest.NewServer(New(slog.New(slog.NewTextHandler(io.Discard, nil)), func(context.Context) error { return nil }, pool))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	csrf := ""
	call := func(method, path string, body any, want int, headers ...string) []byte {
		t.Helper()
		raw, _ := json.Marshal(body)
		request, _ := http.NewRequest(method, server.URL+"/api/v1"+path, bytes.NewReader(raw))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-CSRF-Token", csrf)
		for i := 0; i+1 < len(headers); i += 2 {
			request.Header.Set(headers[i], headers[i+1])
		}
		response, e := client.Do(request)
		if e != nil {
			t.Fatal(e)
		}
		defer response.Body.Close()
		data, _ := io.ReadAll(response.Body)
		if response.StatusCode != want {
			t.Fatalf("%s: %d %s", path, response.StatusCode, data)
		}
		return data
	}
	var user Identity
	json.Unmarshal(call("POST", "/auth/register", map[string]any{"name": "Tracking test", "workspace": "Tracking", "email": randomToken()[:12] + "@example.invalid", "password": randomToken()}, 200), &user)
	csrf = user.CSRF
	recipient := "qa@example.invalid"
	call("PUT", "/settings/messaging", map[string]any{"frequency_limit": 3, "frequency_window_seconds": 86400, "test_allowlist": []string{recipient}}, 200)
	var sent map[string]any
	json.Unmarshal(call("POST", "/messages/test-send", map[string]any{"recipient": recipient, "preview": map[string]any{"channel": "email", "config": map[string]any{"subject": "Hello", "html": `<p><a href="https://shop.example/cart">Open your cart</a></p>`}}}, 200, "Idempotency-Key", randomToken()), &sent)
	delivery, _ := sent["id"].(string)
	codes := map[string]string{}
	rows, err := pool.Query(ctx, `SELECT kind,code FROM tracking_codes WHERE delivery_id=$1`, delivery)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var kind, code string
		rows.Scan(&kind, &code)
		codes[kind] = code
	}
	rows.Close()
	if len(codes) != 3 {
		t.Fatalf("a test email gets a click, open and unsubscribe code: %v", codes)
	}
	var html string
	pool.QueryRow(ctx, `SELECT message->>'html' FROM delivery_jobs WHERE id=$1`, delivery).Scan(&html)
	if !strings.Contains(html, "/api/t/"+codes["click"]) || strings.Contains(html, "shop.example") {
		t.Fatalf("stored email is not tracked: %s", html)
	}
	public := func(method, code string, body io.Reader) *http.Response {
		t.Helper()
		request, _ := http.NewRequest(method, server.URL+"/api/t/"+code, body)
		if body != nil {
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, response.Body)
		response.Body.Close()
		return response
	}
	kinds := func() string {
		var list string
		pool.QueryRow(ctx, `SELECT COALESCE(string_agg(kind,',' ORDER BY kind),'') FROM delivery_events WHERE delivery_id=$1`, delivery).Scan(&list)
		return list
	}
	// A link preview's HEAD gets the redirect and counts nothing.
	if r := public("HEAD", codes["click"], nil); r.StatusCode != 302 || r.Header.Get("Location") != "https://shop.example/cart" || kinds() != "" {
		t.Fatal("HEAD", r.StatusCode, r.Header.Get("Location"), kinds())
	}
	for range 2 {
		if r := public("GET", codes["click"], nil); r.StatusCode != 302 || r.Header.Get("Location") != "https://shop.example/cart" || r.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("click", r.StatusCode, r.Header)
		}
	}
	if r := public("GET", codes["open"], nil); r.StatusCode != 200 || r.Header.Get("Content-Type") != "image/gif" {
		t.Fatal("pixel", r.StatusCode, r.Header)
	}
	var uses int
	pool.QueryRow(ctx, `SELECT uses FROM tracking_codes WHERE code=$1`, codes["click"]).Scan(&uses)
	if uses != 2 || kinds() != "clicked,opened" {
		t.Fatal("every click is counted on its code, the first is kept as the event", uses, kinds())
	}
	// Opening the unsubscribe link only asks; the button's POST records it.
	if r := public("GET", codes["unsubscribe"], nil); r.StatusCode != 200 || kinds() != "clicked,opened" || !strings.Contains(r.Header.Get("Content-Security-Policy"), "default-src 'none'") {
		t.Fatal("unsubscribe GET", r.StatusCode, kinds())
	}
	if r := public("POST", codes["unsubscribe"], strings.NewReader(url.Values{"List-Unsubscribe": {"One-Click"}}.Encode())); r.StatusCode != 200 || kinds() != "clicked,opened,unsubscribed" {
		t.Fatal("unsubscribe POST", r.StatusCode, kinds())
	}
	for _, code := range []string{"short", "AAAAAAAAAAAA", "AAAA-AAAA-AA"} {
		if r := public("GET", code, nil); r.StatusCode != 404 {
			t.Fatal("unknown code", code, r.StatusCode)
		}
	}

	report := func(body map[string]any, sign string) int {
		raw, _ := json.Marshal(body)
		request, _ := http.NewRequest("POST", server.URL+"/api/provider/events", bytes.NewReader(raw))
		if sign != "" {
			mac := hmac.New(sha256.New, []byte(sign))
			mac.Write(raw)
			request.Header.Set("X-Provider-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
		}
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		return response.StatusCode
	}
	var key string
	pool.QueryRow(ctx, `SELECT workspace_id::text||':'||logical_key FROM delivery_jobs WHERE id=$1`, delivery).Scan(&key)
	body := map[string]any{"idempotency_key": key, "type": "bounced", "reason": "Mailbox does not exist"}
	if code := report(body, ""); code != 401 {
		t.Fatal("unsigned report", code)
	}
	if code := report(body, "wrong-secret"); code != 401 {
		t.Fatal("wrongly signed report", code)
	}
	if code := report(map[string]any{"idempotency_key": user.WorkspaceID + ":test:missing", "type": "bounced"}, secret); code != 404 {
		t.Fatal("report for an unknown message", code)
	}
	for range 2 {
		if code := report(body, secret); code != 204 {
			t.Fatal("signed report", code)
		}
	}
	var source, reason string
	pool.QueryRow(ctx, `SELECT source,detail->>'reason' FROM delivery_events WHERE delivery_id=$1 AND kind='bounced'`, delivery).Scan(&source, &reason)
	if source != "provider" || reason != "Mailbox does not exist" || kinds() != "bounced,clicked,opened,unsubscribed" {
		t.Fatal("provider report", source, reason, kinds())
	}
}
