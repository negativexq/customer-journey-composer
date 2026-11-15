package workflow

import (
	"testing"
	"time"
)

func TestWaitDeadlines(t *testing.T) {
	// A Friday afternoon in Istanbul (UTC+3).
	now := time.Date(2026, 11, 13, 12, 30, 0, 0, time.UTC)
	data := map[string]any{"customer": map[string]any{"attributes": map[string]any{
		"renews_on":  "2026-12-01",
		"expires_at": "2026-11-20T09:00:00Z",
		"long_ago":   "2020-01-01",
		"far_away":   "2040-01-01",
		"nonsense":   42.0,
	}}}
	for _, tc := range []struct {
		name string
		c    Config
		want time.Time
	}{
		{"fixed delay", Config{DurationSeconds: 3600}, now.Add(time.Hour)},
		{"explicit duration", Config{WaitMode: "duration", DurationSeconds: 60}, now.Add(time.Minute)},
		{"later today in the step timezone", Config{WaitMode: "time", AtTime: "18:00", Timezone: "Europe/Istanbul"}, time.Date(2026, 11, 13, 15, 0, 0, 0, time.UTC)},
		{"already past today goes to tomorrow", Config{WaitMode: "time", AtTime: "09:00", Timezone: "Europe/Istanbul"}, time.Date(2026, 11, 14, 6, 0, 0, 0, time.UTC)},
		{"only on allowed weekdays", Config{WaitMode: "time", AtTime: "09:00", Weekdays: []int{1, 2}}, time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC)},
		{"the same weekday next week", Config{WaitMode: "time", AtTime: "09:00", Weekdays: []int{5}}, time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)},
		{"days before a date", Config{WaitMode: "date", DatePath: "customer.attributes.renews_on", OffsetSeconds: -3 * 86400}, time.Date(2026, 11, 28, 0, 0, 0, 0, time.UTC)},
		{"a bare date starts in the step timezone", Config{WaitMode: "date", DatePath: "customer.attributes.renews_on", Timezone: "Europe/Istanbul"}, time.Date(2026, 11, 30, 21, 0, 0, 0, time.UTC)},
		{"after a timestamp", Config{WaitMode: "date", DatePath: "customer.attributes.expires_at", OffsetSeconds: 7200}, time.Date(2026, 11, 20, 11, 0, 0, 0, time.UTC)},
		{"a past date continues now", Config{WaitMode: "date", DatePath: "customer.attributes.long_ago"}, now},
		{"a missing field continues now", Config{WaitMode: "date", DatePath: "customer.attributes.absent"}, now},
		{"an unreadable value continues now", Config{WaitMode: "date", DatePath: "customer.attributes.nonsense"}, now},
		{"never longer than the maximum wait", Config{WaitMode: "date", DatePath: "customer.attributes.far_away"}, now.Add(MaxWait)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := WaitDeadline(tc.c, data, now); !got.Equal(tc.want) {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestWaitValidation(t *testing.T) {
	for _, tc := range []struct {
		c     Config
		field string
	}{
		{Config{DurationSeconds: 60}, ""},
		{Config{}, "duration_seconds"},
		{Config{WaitMode: "time", AtTime: "09:00", Weekdays: []int{1, 7}}, ""},
		{Config{WaitMode: "time", AtTime: "9:00"}, "at_time"},
		{Config{WaitMode: "time", AtTime: "24:00"}, "at_time"},
		{Config{WaitMode: "time", AtTime: "09:00", Weekdays: []int{1, 1}}, "weekdays"},
		{Config{WaitMode: "time", AtTime: "09:00", Weekdays: []int{8}}, "weekdays"},
		{Config{WaitMode: "date", DatePath: "customer.attributes.renews_on", OffsetSeconds: -86400}, ""},
		{Config{WaitMode: "date", DatePath: "context.anything"}, "date_path"},
		{Config{WaitMode: "date", DatePath: "event.properties.at", OffsetSeconds: 40000000}, "offset_seconds"},
		{Config{WaitMode: "sometime"}, "wait_mode"},
	} {
		if field, _ := validateWait(tc.c); field != tc.field {
			t.Fatalf("%+v: got %q, want %q", tc.c, field, tc.field)
		}
	}
}
