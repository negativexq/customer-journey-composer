package config

import "testing"

func TestParse(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  map[string]string
		fail bool
	}{
		{"defaults", map[string]string{"DATABASE_URL": "postgres://localhost/journeys"}, false},
		{"missing database", map[string]string{}, true},
		{"bad address", map[string]string{"DATABASE_URL": "db", "HTTP_ADDR": "invalid"}, true},
		{"bad log level", map[string]string{"DATABASE_URL": "db", "LOG_LEVEL": "secret-value"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Parse(func(key string) string { return tc.env[key] })
			if (err != nil) != tc.fail {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.fail && cfg.HTTPAddr != ":8090" {
				t.Fatalf("unexpected default address: %s", cfg.HTTPAddr)
			}
		})
	}
}
