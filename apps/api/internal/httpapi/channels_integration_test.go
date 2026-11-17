//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/database"
)

func TestChannelHealthSimulationNotificationsAndLongestSMS(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Open(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(New(slog.New(slog.NewTextHandler(io.Discard, nil)), func(context.Context) error { return nil }, pool))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	csrf := ""
	call := func(method, path string, body any, want int) []byte {
		t.Helper()
		raw, _ := json.Marshal(body)
		request, _ := http.NewRequest(method, server.URL+"/api/v1"+path, bytes.NewReader(raw))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-CSRF-Token", csrf)
		response, e := client.Do(request)
		if e != nil {
			t.Fatal(e)
		}
		defer response.Body.Close()
		data, _ := io.ReadAll(response.Body)
		if response.StatusCode != want {
			t.Fatalf("%s %s: %d %s", method, path, response.StatusCode, data)
		}
		return data
	}
	var user Identity
	json.Unmarshal(call("POST", "/auth/register", map[string]any{"name": "Channel owner", "workspace": "Channels", "email": randomToken()[:12] + "@example.invalid", "password": randomToken()}, 200), &user)
	csrf = user.CSRF

	var channels []channelHealth
	json.Unmarshal(call("GET", "/delivery/channels", nil, 200), &channels)
	if len(channels) != 3 || channels[0].Channel != "email" || channels[1].State != "healthy" || channels[1].RatePerSecond != 50 || channels[2].Provider != "mock_push" || channels[2].State != "healthy" {
		t.Fatalf("every channel starts healthy: %+v", channels)
	}
	call("PUT", "/delivery/channels/mock_sms", map[string]any{"rate_per_second": 20, "simulate": "outage"}, 422)
	json.Unmarshal(call("PUT", "/delivery/channels/mock_sms", map[string]any{"rate_per_second": 20, "simulate": "outage", "simulate_minutes": 5}, 200), &channels)
	if channels[1].Simulate != "outage" || channels[1].SimulateUntil == nil || channels[1].RatePerSecond != 20 {
		t.Fatalf("the simulation is shown: %+v", channels[1])
	}

	// A teammate's publish rings the bell; one's own actions arrive read.
	var teammate, journey string
	if err := pool.QueryRow(ctx, `INSERT INTO users(workspace_id,email,name,password_hash) VALUES($1,$2,'Deniz','!disabled') RETURNING id::text`, user.WorkspaceID, randomToken()[:12]+"@example.invalid").Scan(&teammate); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO journeys(workspace_id,name,draft) VALUES($1,'Cart rescue','{}') RETURNING id::text`, user.WorkspaceID).Scan(&journey); err != nil {
		t.Fatal(err)
	}
	for _, actor := range []string{teammate, user.ID} {
		if _, err := pool.Exec(ctx, `INSERT INTO audit_logs(workspace_id,actor_id,action,resource_id,details,created_at) VALUES($1,$2,'journey.publish',$3,'{"version":2}',now()+interval '1 second')`, user.WorkspaceID, actor, journey); err != nil {
			t.Fatal(err)
		}
	}
	var bell notificationList
	json.Unmarshal(call("GET", "/notifications", nil, 200), &bell)
	if bell.Unread != 1 || len(bell.Items) != 2 || bell.Items[0].Title == "" {
		t.Fatalf("one unread from the teammate: %+v", bell)
	}
	found := false
	for _, item := range bell.Items {
		found = found || item.Title == "Deniz published Cart rescue" && item.Body == "Version 2 is live." && item.Unread
	}
	if !found {
		t.Fatalf("%+v", bell.Items)
	}
	if _, err := pool.Exec(ctx, `UPDATE notifications SET created_at=now()-interval '1 minute' WHERE workspace_id=$1`, user.WorkspaceID); err != nil {
		t.Fatal(err)
	}
	call("POST", "/notifications/read", nil, 204)
	json.Unmarshal(call("GET", "/notifications", nil, 200), &bell)
	if bell.Unread != 0 || bell.Items[0].Unread {
		t.Fatalf("reading clears the count: %+v", bell)
	}

	// The SMS is measured with the costliest name among customers it reaches.
	for _, row := range [][2]string{{"short", "Al"}, {"long", "Bartholomew-Maximilian"}, {"unicode", "Şükrü"}} {
		if _, err := pool.Exec(ctx, `INSERT INTO customers(workspace_id,customer_id,phone,attributes,consents) VALUES($1,$2,'+905551112233',jsonb_build_object('first_name',$3::text),'{"sms":true}')`, user.WorkspaceID, row[0], row[1]); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO customers(workspace_id,customer_id,phone,attributes,consents) VALUES($1,'unreachable','+905551112234','{"first_name":"Wolfeschlegelsteinhausenbergerdorff-Ğğ"}','{"sms":false}')`, user.WorkspaceID); err != nil {
		t.Fatal(err)
	}
	var preview struct {
		Segments    int `json:"segments"`
		LongestCase *struct {
			Values       map[string]string `json:"values"`
			Encoding     string            `json:"encoding"`
			Segments     int               `json:"segments"`
			TrackedLinks int               `json:"tracked_links"`
		} `json:"longest_case"`
	}
	json.Unmarshal(call("POST", "/messages/preview", map[string]any{"channel": "sms", "config": map[string]any{"text": "Hi {{ customer.attributes.first_name }}, finish your order at https://shop.example/cart now"}, "customer": map[string]any{"attributes": map[string]any{"first_name": "Ada"}}}, 200), &preview)
	if preview.Segments != 1 || preview.LongestCase == nil {
		t.Fatalf("%+v", preview)
	}
	if got := preview.LongestCase; got.Values["customer.attributes.first_name"] != "Şükrü" || got.Encoding != "Unicode" || got.Segments != 2 || got.TrackedLinks != 1 {
		t.Fatalf("one Turkish letter makes the SMS Unicode, and a customer without consent is left out: %+v", got)
	}
}
