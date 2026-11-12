package ai

import (
	"context"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/semantic"
)

type catalogContextKey struct{}

// Catalog is supplied by trusted HTTP orchestration, never by provider input.
func WithCatalog(ctx context.Context, entries []semantic.Entry) context.Context {
	return context.WithValue(ctx, catalogContextKey{}, entries)
}
func CatalogFromContext(ctx context.Context) ([]semantic.Entry, bool) {
	entries, ok := ctx.Value(catalogContextKey{}).([]semantic.Entry)
	return entries, ok
}
