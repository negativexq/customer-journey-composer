package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthAndReadiness(t *testing.T) {
	for _, tc := range []struct {
		path string
		fail bool
		want int
	}{
		{"/healthz", false, 200},
		{"/healthz", true, 200},
		{"/readyz", false, 200},
		{"/readyz", true, 503},
	} {
		t.Run(tc.path, func(t *testing.T) {
			h := New(slog.New(slog.NewTextHandler(io.Discard, nil)), func(ctx context.Context) error {
				if _, ok := ctx.Deadline(); !ok {
					t.Error("readiness dependency must have a deadline")
				}
				if tc.fail {
					return errors.New("postgres://private-credentials")
				}
				return nil
			})
			r := httptest.NewRecorder()
			h.ServeHTTP(r, httptest.NewRequest("GET", tc.path, nil))
			if r.Code != tc.want {
				t.Fatalf("want %d, got %d: %s", tc.want, r.Code, r.Body)
			}
			if strings.Contains(r.Body.String(), "private-credentials") {
				t.Fatal("response leaked dependency error")
			}
			if len(r.Header().Get("X-Request-ID")) != 32 {
				t.Fatal("request id missing")
			}
		})
	}
}

func TestGeneratedOpenAPI(t *testing.T) {
	h := New(slog.Default(), func(context.Context) error { return nil })
	r := httptest.NewRecorder()
	h.ServeHTTP(r, httptest.NewRequest("GET", "/api/openapi.json", nil))
	var spec struct {
		OpenAPI string                     `json:"openapi"`
		Paths   map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &spec); err != nil {
		t.Fatal(err)
	}
	if r.Code != 200 || spec.OpenAPI == "" || spec.Paths["/healthz"] == nil || spec.Paths["/readyz"] == nil {
		t.Fatalf("incomplete generated spec: %s", r.Body)
	}
}
