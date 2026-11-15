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
