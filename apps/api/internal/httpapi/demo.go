package httpapi

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"github.com/ofk/customer-journey-composer/apps/api/internal/demo"
	"os"
)

func registerDemo(api huma.API) {
	huma.Get(api, "/api/v1/auth/demo", func(_ context.Context, _ *struct{}) (*struct {
		CacheControl string `header:"Cache-Control"`
		Body         demo.Credentials
	}, error) {
		c, err := demo.Load(os.Getenv)
		if err != nil {
			return nil, huma.Error503ServiceUnavailable("Demo account is not configured")
		}
		return &struct {
			CacheControl string `header:"Cache-Control"`
			Body         demo.Credentials
		}{CacheControl: "no-store", Body: c}, nil
	})
}
