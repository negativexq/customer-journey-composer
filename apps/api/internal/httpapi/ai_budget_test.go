package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"testing"
	"time"

	composer "github.com/negativexq/customer-journey-composer/apps/api/internal/ai"
)

// Two provider calls must fit in an AI request, and the proxy and the web
// client must wait longer than the request. A 45 s call limit refused a valid
// live request whose reasoning took longer.
func TestAIRequestOutlastsTwoProviderCalls(t *testing.T) {
	if composer.ProviderTimeout < 90*time.Second || composer.RequestBudget < 2*composer.ProviderTimeout {
		t.Fatalf("provider %s, request %s", composer.ProviderTimeout, composer.RequestBudget)
	}
	var left time.Duration
	handler := middleware(slog.New(slog.NewTextHandler(io.Discard, nil)), http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		deadline, _ := r.Context().Deadline()
		left = time.Until(deadline)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/v1/ai/journeys/modify/stream", nil))
	if left < composer.RequestBudget-time.Second || left > composer.RequestBudget {
		t.Fatalf("AI request deadline in %s, want %s", left, composer.RequestBudget)
	}
	for _, c := range []struct {
		file, pattern string
		unit          time.Duration
	}{
		{"../../../web/nginx.conf", `location /api/v1/ai/ \{[^}]*proxy_read_timeout (\d+)s;`, time.Second},
		{"../../../web/src/lib/api.ts", `/stream[\s\S]*?AbortSignal\.timeout\((\d+)\)`, time.Millisecond},
	} {
		raw, err := os.ReadFile(c.file)
		if err != nil {
			t.Fatal(err)
		}
		m := regexp.MustCompile(c.pattern).FindSubmatch(raw)
		if m == nil {
			t.Fatalf("%s: no AI timeout found", c.file)
		}
		n, _ := strconv.Atoi(string(m[1]))
		if wait := time.Duration(n) * c.unit; wait <= composer.RequestBudget {
			t.Fatalf("%s waits %s, shorter than the %s request budget", c.file, wait, composer.RequestBudget)
		}
	}
}
