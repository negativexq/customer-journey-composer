package httpapi

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	composer "github.com/negativexq/customer-journey-composer/apps/api/internal/ai"
)

func TestGenerationStreamFlushesBeforeProviderFinishes(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		streamGeneration(r.Context(), w, func(ctx context.Context) (composer.Proposal, error) {
			select {
			case <-release:
			case <-ctx.Done():
				return composer.Proposal{}, ctx.Err()
			}
			return composer.Proposal{Name: "Reviewed only"}, nil
		})
	}))
	defer server.Close()
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	reader := bufio.NewReader(response.Body)
	first, err := reader.ReadString('\n')
	if err != nil || !strings.Contains(first, `"accepted"`) {
		t.Fatalf("first frame: %q %v", first, err)
	}
	close(release)
	rest, _ := io.ReadAll(reader)
	if !strings.Contains(string(rest), `"type":"ready"`) {
		t.Fatalf("missing final proposal: %s", rest)
	}
}

func TestGenerationStreamCancelsOnDisconnect(t *testing.T) {
	stopped := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		streamGeneration(r.Context(), w, func(ctx context.Context) (composer.Proposal, error) {
			<-ctx.Done()
			close(stopped)
			return composer.Proposal{}, ctx.Err()
		})
	}))
	defer server.Close()
	response, err := http.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("provider context not cancelled")
	}
}

func TestGenerationStreamErrorsNeverEmitReady(t *testing.T) {
	w := httptest.NewRecorder()
	streamGeneration(context.Background(), w, func(context.Context) (composer.Proposal, error) {
		return composer.Proposal{}, huma.Error429TooManyRequests("A request is already running")
	})
	body := w.Body.String()
	if !strings.Contains(body, `"status":429`) || strings.Contains(body, `"type":"ready"`) {
		t.Fatal(body)
	}
}
