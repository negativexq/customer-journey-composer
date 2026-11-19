package httpapi

import "testing"

func TestIngestLimitReadsTheEnvironment(t *testing.T) {
	for _, c := range []struct {
		rate, burst       string
		wantRate, wantBur float64
	}{
		{"", "", 500, 1000},
		{"2000", "4000", 2000, 4000},
		{"0", "-1", 500, 1000},
		{"fast", "0.5", 500, 1000},
	} {
		t.Setenv("INGEST_RATE_PER_SECOND", c.rate)
		t.Setenv("INGEST_BURST", c.burst)
		if rate, burst := ingestLimit(); rate != c.wantRate || burst != c.wantBur {
			t.Fatalf("%q/%q: got %v/%v", c.rate, c.burst, rate, burst)
		}
	}
}
