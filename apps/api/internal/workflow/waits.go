package workflow

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var clockTime = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

// MaxWait bounds every wait, however it is expressed.
const MaxWait = 365 * 24 * time.Hour

func waitLocation(name string) *time.Location {
	if name == "" {
		return time.UTC
	}
	if loc, err := time.LoadLocation(name); err == nil {
		return loc
	}
	return time.UTC
}

// isoWeekday numbers Monday as 1 and Sunday as 7.
func isoWeekday(t time.Time) int { return (int(t.Weekday())+6)%7 + 1 }

// parseWaitDate reads a profile or event value as a moment. A bare date means
// the start of that day in the step's timezone.
func parseWaitDate(value any, loc *time.Location) (time.Time, bool) {
	text, ok := value.(string)
	if !ok {
		return time.Time{}, false
	}
	text = strings.TrimSpace(text)
	if t, err := time.Parse(time.RFC3339, text); err == nil {
		return t, true
	}
	if t, err := time.ParseInLocation("2006-01-02", text, loc); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// WaitDeadline is when a wait step lets the customer continue. It is pure: the
// same step, data and clock always give the same moment. A deadline that
// cannot be computed or is already past is "now", so the customer moves on
// instead of being stranded.
func WaitDeadline(c Config, data map[string]any, now time.Time) time.Time {
	switch c.WaitMode {
	case "time":
		loc := waitLocation(c.Timezone)
		local := now.In(loc)
		hour, _ := strconv.Atoi(c.AtTime[:2])
		minute, _ := strconv.Atoi(c.AtTime[3:])
		allowed := map[int]bool{}
		for _, day := range c.Weekdays {
			allowed[day] = true
		}
		for ahead := 0; ahead <= 7; ahead++ {
			candidate := time.Date(local.Year(), local.Month(), local.Day()+ahead, hour, minute, 0, 0, loc)
			if !candidate.After(now) || (len(allowed) > 0 && !allowed[isoWeekday(candidate)]) {
				continue
			}
			return candidate.UTC()
		}
		return now
	case "date":
		value, ok := Lookup(data, c.DatePath)
		if !ok {
			return now
		}
		at, ok := parseWaitDate(value, waitLocation(c.Timezone))
		if !ok {
			return now
		}
		until := at.Add(time.Duration(c.OffsetSeconds) * time.Second).UTC()
		if until.Before(now) {
			return now
		}
		if limit := now.Add(MaxWait); until.After(limit) {
			return limit
		}
		return until
	default:
		return now.Add(time.Duration(c.DurationSeconds) * time.Second)
	}
}

func validateWait(c Config) (field, message string) {
	switch c.WaitMode {
	case "", "duration":
		if c.DurationSeconds < 1 || c.DurationSeconds > 31536000 {
			return "duration_seconds", "Delay must be between 1 second and 365 days"
		}
	case "time":
		if !clockTime.MatchString(c.AtTime) {
			return "at_time", "Choose a time of day as HH:MM"
		}
		seen := map[int]bool{}
		for _, day := range c.Weekdays {
			if day < 1 || day > 7 || seen[day] {
				return "weekdays", "Weekdays must be distinct values from 1 (Monday) to 7 (Sunday)"
			}
			seen[day] = true
		}
	case "date":
		if !strings.HasPrefix(c.DatePath, "customer.") && !strings.HasPrefix(c.DatePath, "event.") {
			return "date_path", "Choose a customer or event field that holds the date"
		}
		if c.OffsetSeconds < -31536000 || c.OffsetSeconds > 31536000 {
			return "offset_seconds", "The offset must be within 365 days of the date"
		}
	default:
		return "wait_mode", fmt.Sprintf("Unsupported wait type: %s", c.WaitMode)
	}
	return "", ""
}

// AudienceEvent names the internal event that records an audience entry. It is
// reserved: customer events with this name never start a journey.
const AudienceEvent = "audience_entry"

// IsAudience reports whether a trigger enrolls a scheduled audience.
func (c Config) IsAudience() bool { return c.EntryMode == "audience" }

func audienceSchedule(c Config) Config {
	return Config{WaitMode: "time", AtTime: c.AtTime, Weekdays: c.Weekdays, Timezone: c.Timezone}
}

// AudienceSlotAfter is the first scheduled audience run strictly after a moment.
func AudienceSlotAfter(c Config, after time.Time) time.Time {
	return WaitDeadline(audienceSchedule(c), nil, after)
}

// LatestAudienceSlot is the last scheduled run after anchor and not after now,
// so a worker that was stopped runs a missed audience once, not once per slot.
func LatestAudienceSlot(c Config, anchor, now time.Time) (time.Time, bool) {
	slot := AudienceSlotAfter(c, anchor)
	if !slot.After(anchor) || slot.After(now) {
		return time.Time{}, false
	}
	for {
		next := AudienceSlotAfter(c, slot)
		if !next.After(slot) || next.After(now) {
			return slot, true
		}
		slot = next
	}
}

func onlyCustomerPaths(r Rule) bool {
	if r.Path != "" && !strings.HasPrefix(r.Path, "customer.") {
		return false
	}
	for _, child := range r.Rules {
		if !onlyCustomerPaths(child) {
			return false
		}
	}
	return true
}

func validateEntry(c Config) (field, message string) {
	switch c.EntryMode {
	case "", "event":
		if c.EventName == AudienceEvent {
			return "event_name", "audience_entry is reserved for audience entry"
		}
	case "audience":
		if c.EventName != AudienceEvent {
			return "event_name", "An audience entry uses the audience_entry event"
		}
		if c.Filters == nil {
			return "filters", "Choose who is in the audience"
		}
		if !onlyCustomerPaths(*c.Filters) {
			return "filters", "An audience is chosen by customer fields only"
		}
		return validateWait(audienceSchedule(c))
	default:
		return "entry_mode", fmt.Sprintf("Unsupported entry type: %s", c.EntryMode)
	}
	return "", ""
}
