package httpapi

import (
	"testing"
	"time"
)

func TestTrendWindow(t *testing.T) {
	now := time.Date(2026, 11, 17, 15, 4, 0, 0, time.UTC)
	day := func(s string) time.Time { d, _ := time.Parse(time.DateOnly, s); return d }

	start, end, err := trendWindow(now, 30, "", "")
	if err != nil || !start.Equal(day("2026-10-19")) || !end.Equal(day("2026-11-18")) {
		t.Fatalf("last 30 days = %v..%v, %v", start, end, err)
	}
	// A custom range is inclusive of its last day, and may end today.
	start, end, err = trendWindow(now, 30, "2026-11-01", "2026-11-17")
	if err != nil || !start.Equal(day("2026-11-01")) || !end.Equal(day("2026-11-18")) {
		t.Fatalf("custom = %v..%v, %v", start, end, err)
	}
	if _, _, err := trendWindow(now, 30, "2026-11-17", "2026-11-17"); err != nil {
		t.Fatalf("a single day is a valid range: %v", err)
	}
	for name, r := range map[string][2]string{
		"missing end":   {"2026-11-01", ""},
		"missing start": {"", "2026-11-01"},
		"bad date":      {"2026/11/01", "2026-11-02"},
		"reversed":      {"2026-11-10", "2026-11-01"},
		"future":        {"2026-11-01", "2026-11-18"},
		"too long":      {"2025-11-01", "2026-11-17"},
	} {
		if _, _, err := trendWindow(now, 30, r[0], r[1]); err == nil {
			t.Errorf("%s: %v was accepted", name, r)
		}
	}
}
