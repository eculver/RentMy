package notification

import (
	"time"
)

// IsTypeDisabled reports whether the given notification type is turned off in
// the user's preferences.  Mandatory types (booking-critical) always return false.
func IsTypeDisabled(prefs Preferences, t Type) bool {
	if IsMandatory(t) {
		return false
	}
	for _, disabled := range prefs.DisabledTypes {
		if disabled == t {
			return true
		}
	}
	return false
}

// IsQuietHours reports whether now falls inside the user's configured quiet
// hours window.  Returns false if quiet hours are not configured.
func IsQuietHours(prefs Preferences, now time.Time) bool {
	if prefs.QuietHoursStart == nil || prefs.QuietHoursEnd == nil {
		return false
	}
	loc := time.UTC
	if prefs.TimezoneName != "" {
		if l, err := time.LoadLocation(prefs.TimezoneName); err == nil {
			loc = l
		}
	}
	local := now.In(loc)
	hour := local.Hour()
	start := *prefs.QuietHoursStart
	end := *prefs.QuietHoursEnd

	if start <= end {
		// e.g. 22:00 – 07:00 wraps midnight: start > end case below
		// simple range: e.g. 08:00 – 20:00
		return hour >= start && hour < end
	}
	// Wraps midnight: e.g. start=22, end=7 → quiet from 22:00 to 06:59
	return hour >= start || hour < end
}

// QuietHoursEndTime returns the next time quiet hours end, given a now timestamp.
// Returns zero time if quiet hours are not configured.
func QuietHoursEndTime(prefs Preferences, now time.Time) time.Time {
	if prefs.QuietHoursStart == nil || prefs.QuietHoursEnd == nil {
		return time.Time{}
	}
	loc := time.UTC
	if prefs.TimezoneName != "" {
		if l, err := time.LoadLocation(prefs.TimezoneName); err == nil {
			loc = l
		}
	}
	local := now.In(loc)
	end := *prefs.QuietHoursEnd
	// Construct today's end time in local timezone.
	candidate := time.Date(local.Year(), local.Month(), local.Day(), end, 0, 0, 0, loc)
	if !candidate.After(local) {
		// End time is tomorrow.
		candidate = candidate.Add(24 * time.Hour)
	}
	return candidate
}
