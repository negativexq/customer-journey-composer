//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/ofk/customer-journey-composer/apps/api/internal/database"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"
)

func TestTemplateIsolationAndIndexedJourneyPagination(t *testing.T) {
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
		req, _ := http.NewRequest(method, server.URL+"/api/v1"+path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", csrf)
		r, e := client.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer r.Body.Close()
		b, _ := io.ReadAll(r.Body)
		if r.StatusCode != want {
			t.Fatalf("%s: %d %s", path, r.StatusCode, b)
		}
		return b
	}
	var user Identity
	json.Unmarshal(call("POST", "/auth/register", map[string]string{"name": "Catalog", "workspace": "Catalog", "email": randomToken()[:12] + "@example.invalid", "password": randomToken()}, 200), &user)
	csrf = user.CSRF
	var journey Journey
	json.Unmarshal(call("POST", "/templates/abandoned-cart/create-journey", map[string]string{}, 200), &journey)
	if journey.Status != "Draft" {
		t.Fatal("template published itself")
	}
	var saved map[string]any
	json.Unmarshal(call("POST", "/templates", map[string]any{"journey_id": journey.ID, "name": "Saved cart"}, 200), &saved)
	templateID := saved["id"].(string)
	call("POST", "/templates/"+templateID+"/create-journey", map[string]string{"name": "Saved copy"}, 200)
	var templates []map[string]any
	json.Unmarshal(call("GET", "/templates", nil, 200), &templates)
	if len(templates) != 3 || templates[2]["workflow"] != nil {
		t.Fatal("template list contains bodies or wrong scope")
	}
	for _, name := range []string{"Prefix A", "Prefix C", "Prefix B", "Prefix_% literal"} {
		call("POST", "/journeys", map[string]string{"name": name}, 200)
	}
	var first, second []Journey
	json.Unmarshal(call("GET", "/journeys?sort=name&search=Prefix&limit=2", nil, 200), &first)
	if len(first) != 2 || first[0].Name != "Prefix A" || first[1].Name != "Prefix B" || first[0].Stats == nil || string(first[0].Draft) != "null" {
		t.Fatal("invalid metadata page", first)
	}
	params := url.Values{"sort": {"name"}, "search": {"Prefix"}, "limit": {"2"}, "after_name": {first[1].Name}, "before_id": {first[1].ID}}
	json.Unmarshal(call("GET", "/journeys?"+params.Encode(), nil, 200), &second)
	if len(second) != 2 || second[0].Name != "Prefix C" {
		t.Fatal("name cursor failed", second)
	}
	var literal []Journey
	json.Unmarshal(call("GET", "/journeys?search="+url.QueryEscape("Prefix_%"), nil, 200), &literal)
	if len(literal) != 1 || literal[0].Name != "Prefix_% literal" {
		t.Fatal("LIKE wildcards were not escaped")
	}
	same := time.Now().UTC().Truncate(time.Microsecond)
	if _, err = pool.Exec(ctx, `UPDATE journeys SET updated_at=$2 WHERE workspace_id=$1`, user.WorkspaceID, same); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	path := "/journeys?limit=2"
	for range 5 {
		var page []Journey
		json.Unmarshal(call("GET", path, nil, 200), &page)
		if len(page) == 0 {
			break
		}
		for _, j := range page {
			if seen[j.ID] {
				t.Fatal("timestamp tie duplicated a journey")
			}
			seen[j.ID] = true
		}
		last := page[len(page)-1]
		path = "/journeys?" + url.Values{"limit": {"2"}, "before": {last.UpdatedAt.Format(time.RFC3339Nano)}, "before_id": {last.ID}}.Encode()
	}
	if len(seen) != 6 {
		t.Fatal("cursor lost journeys", len(seen))
	}
	if _, err = pool.Exec(ctx, `UPDATE users SET role='Viewer' WHERE id=$1`, user.ID); err != nil {
		t.Fatal(err)
	}
	call("POST", "/templates/customer-onboarding/create-journey", map[string]string{}, 403)
	json.Unmarshal(call("POST", "/auth/register", map[string]string{"name": "Other", "workspace": "Other", "email": randomToken()[:12] + "@example.invalid", "password": randomToken()}, 200), &user)
	csrf = user.CSRF
	call("GET", "/templates/"+templateID, nil, 404)
	call("POST", "/templates/"+templateID+"/create-journey", map[string]string{}, 404)
}
