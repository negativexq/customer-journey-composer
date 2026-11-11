package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	composer "github.com/negativexq/customer-journey-composer/apps/api/internal/ai"
)

type generationEvent struct {
	Type     string             `json:"type"`
	Stage    string             `json:"stage,omitempty"`
	Attempt  int                `json:"attempt,omitempty"`
	Proposal *composer.Proposal `json:"proposal,omitempty"`
	Message  string             `json:"message,omitempty"`
	Status   int                `json:"status,omitempty"`
}

func streamGeneration(ctx context.Context, writer io.Writer, run func(context.Context) (composer.Proposal, error)) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	send := func(event generationEvent) bool {
		if response, ok := writer.(http.ResponseWriter); ok {
			_ = http.NewResponseController(response).SetWriteDeadline(time.Now().Add(10 * time.Second))
		}
		data, err := json.Marshal(event)
		if err != nil {
			return false
		}
		if _, err = fmt.Fprintf(writer, "data: %s\n\n", data); err != nil {
			return false
		}
		if flush, ok := writer.(http.Flusher); ok {
			flush.Flush()
		}
		return true
	}
	if !send(generationEvent{Type: "progress", Stage: "accepted"}) {
		return
	}
	events := make(chan generationEvent, 16)
	request := composer.WithProgress(ctx, func(progress composer.Progress) {
		select {
		case events <- generationEvent{Type: "progress", Stage: progress.Stage, Attempt: progress.Attempt}:
		case <-ctx.Done():
		}
	})
	go func() {
		proposal, err := run(request)
		event := generationEvent{Type: "ready", Stage: "ready", Proposal: &proposal}
		if err != nil {
			event = generationEvent{Type: "error", Message: "Generation failed. Your draft is unchanged.", Status: 500}
			var status huma.StatusError
			if errors.As(err, &status) {
				event.Status, event.Message = status.GetStatus(), err.Error()
			}
		}
		select {
		case events <- event:
		case <-ctx.Done():
		}
	}()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			// A timeout is a terminal event if the client is still connected.
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				send(generationEvent{Type: "error", Status: 504, Message: "Generation timed out. Your draft is unchanged."})
			}
			return
		case event := <-events:
			if !send(event) || event.Type == "ready" || event.Type == "error" {
				return
			}
		case <-ticker.C:
			if !send(generationEvent{Type: "heartbeat"}) {
				return
			}
		}
	}
}
