package ai

import "context"

// Progress describes observable orchestration boundaries, never model reasoning.
type Progress struct {
	Stage   string `json:"stage"`
	Attempt int    `json:"attempt,omitempty"`
}

type progressKey struct{}

// WithProgress attaches a request-local observer. It must not block or retain data.
func WithProgress(ctx context.Context, observe func(Progress)) context.Context {
	return context.WithValue(ctx, progressKey{}, observe)
}

func progress(ctx context.Context, stage string, attempt int) {
	if observe, ok := ctx.Value(progressKey{}).(func(Progress)); ok && ctx.Err() == nil {
		observe(Progress{Stage: stage, Attempt: attempt})
	}
}
