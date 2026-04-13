package notification_test

import (
	"testing"
	"time"

	"github.com/Brett2thered/RentMy/backend/internal/notification"
	"github.com/stretchr/testify/assert"
)

func TestIsTypeDisabled(t *testing.T) {
	t.Run("mandatory types cannot be disabled", func(t *testing.T) {
		prefs := notification.Preferences{
			DisabledTypes: []notification.Type{
				notification.TypeBookingRequest,
				notification.TypeBookingAccepted,
				notification.TypeBookingAutoDeclined,
				notification.TypeCancellation,
			},
		}
		assert.False(t, notification.IsTypeDisabled(prefs, notification.TypeBookingRequest))
		assert.False(t, notification.IsTypeDisabled(prefs, notification.TypeBookingAccepted))
		assert.False(t, notification.IsTypeDisabled(prefs, notification.TypeBookingAutoDeclined))
		assert.False(t, notification.IsTypeDisabled(prefs, notification.TypeCancellation))
	})

	t.Run("non-mandatory types can be disabled", func(t *testing.T) {
		prefs := notification.Preferences{
			DisabledTypes: []notification.Type{notification.TypeNewMessage},
		}
		assert.True(t, notification.IsTypeDisabled(prefs, notification.TypeNewMessage))
	})

	t.Run("non-disabled types return false", func(t *testing.T) {
		prefs := notification.Preferences{}
		assert.False(t, notification.IsTypeDisabled(prefs, notification.TypePickupApproaching))
	})
}

func TestIsQuietHours(t *testing.T) {
	start22 := 22
	end7 := 7
	start8 := 8
	end20 := 20

	tests := []struct {
		name     string
		prefs    notification.Preferences
		now      time.Time
		expected bool
	}{
		{
			name:     "no quiet hours configured returns false",
			prefs:    notification.Preferences{},
			now:      time.Now(),
			expected: false,
		},
		{
			name: "wrapping midnight: hour inside quiet window (23:00)",
			prefs: notification.Preferences{
				QuietHoursStart: &start22,
				QuietHoursEnd:   &end7,
			},
			now:      time.Date(2026, 1, 1, 23, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name: "wrapping midnight: hour inside quiet window (02:00)",
			prefs: notification.Preferences{
				QuietHoursStart: &start22,
				QuietHoursEnd:   &end7,
			},
			now:      time.Date(2026, 1, 1, 2, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name: "wrapping midnight: hour outside quiet window (09:00)",
			prefs: notification.Preferences{
				QuietHoursStart: &start22,
				QuietHoursEnd:   &end7,
			},
			now:      time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC),
			expected: false,
		},
		{
			name: "non-wrapping range: hour inside (12:00)",
			prefs: notification.Preferences{
				QuietHoursStart: &start8,
				QuietHoursEnd:   &end20,
			},
			now:      time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name: "non-wrapping range: hour outside (21:00)",
			prefs: notification.Preferences{
				QuietHoursStart: &start8,
				QuietHoursEnd:   &end20,
			},
			now:      time.Date(2026, 1, 1, 21, 0, 0, 0, time.UTC),
			expected: false,
		},
		{
			name: "boundary: exactly at end hour is not quiet (07:00)",
			prefs: notification.Preferences{
				QuietHoursStart: &start22,
				QuietHoursEnd:   &end7,
			},
			now:      time.Date(2026, 1, 1, 7, 0, 0, 0, time.UTC),
			expected: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := notification.IsQuietHours(tc.prefs, tc.now)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestQuietHoursEndTime(t *testing.T) {
	start22 := 22
	end7 := 7

	t.Run("returns next 07:00 when current time is 23:00", func(t *testing.T) {
		prefs := notification.Preferences{
			QuietHoursStart: &start22,
			QuietHoursEnd:   &end7,
		}
		now := time.Date(2026, 1, 1, 23, 0, 0, 0, time.UTC)
		end := notification.QuietHoursEndTime(prefs, now)
		// Should be 2026-01-02 07:00 UTC
		assert.Equal(t, 2, end.Day())
		assert.Equal(t, 7, end.Hour())
	})

	t.Run("returns zero time when not configured", func(t *testing.T) {
		prefs := notification.Preferences{}
		end := notification.QuietHoursEndTime(prefs, time.Now())
		assert.True(t, end.IsZero())
	})
}
