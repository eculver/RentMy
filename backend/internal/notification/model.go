// Package notification implements in-app and push notification delivery,
// preference management, and quiet-hours enforcement (PRD §16).
package notification

import (
	"encoding/json"
	"errors"
	"time"
)

// Type identifies the category of a notification.
// Booking-related types (BOOKING_REQUEST, BOOKING_ACCEPTED, BOOKING_AUTO_DECLINED, CANCELLATION)
// cannot be disabled by the user — they are safety-critical.
type Type string

const (
	TypeBookingRequest        Type = "BOOKING_REQUEST"
	TypeBookingAccepted       Type = "BOOKING_ACCEPTED"
	TypeBookingAutoDeclined   Type = "BOOKING_AUTO_DECLINED"
	TypeCancellation          Type = "CANCELLATION"
	TypePickupApproaching     Type = "PICKUP_APPROACHING"
	TypeProximityVerified     Type = "PROXIMITY_VERIFIED"
	TypeReturnApproaching     Type = "RETURN_APPROACHING"
	TypeLateReturnWarning     Type = "LATE_RETURN_WARNING"
	TypeLateReturnEscalation  Type = "LATE_RETURN_ESCALATION"
	TypeDisputeOpened         Type = "DISPUTE_OPENED"
	TypeDisputeResolved       Type = "DISPUTE_RESOLVED"
	TypePayoutSent            Type = "PAYOUT_SENT"
	TypeNewMessage            Type = "NEW_MESSAGE"
	TypeKYCStatus             Type = "KYC_STATUS"
	TypeListingFlagged        Type = "LISTING_FLAGGED"
)

// mandatoryTypes are notification types that cannot be disabled by user preference.
// These are safety-critical booking events (PRD §16: "booking notifications always enabled").
var mandatoryTypes = map[Type]bool{
	TypeBookingRequest:      true,
	TypeBookingAccepted:     true,
	TypeBookingAutoDeclined: true,
	TypeCancellation:        true,
}

// IsMandatory reports whether a notification type cannot be disabled by the user.
func IsMandatory(t Type) bool {
	return mandatoryTypes[t]
}

// Notification is a single in-app notification record.
type Notification struct {
	ID        string          `json:"id"`
	UserID    string          `json:"userId"`
	Type      Type            `json:"type"`
	Title     string          `json:"title"`
	Body      string          `json:"body"`
	Data      json.RawMessage `json:"data,omitempty"`
	Read      bool            `json:"read"`
	CreatedAt time.Time       `json:"createdAt"`
}

// Preferences holds a user's notification settings, stored as JSONB in the
// users.notification_preferences column.
type Preferences struct {
	// DisabledTypes lists notification types the user has opted out of.
	// Mandatory types are ignored even if listed here.
	DisabledTypes []Type `json:"disabledTypes"`

	// PushEnabled controls whether push notifications are sent at all.
	PushEnabled bool `json:"pushEnabled"`

	// SMSFallbackEnabled controls whether SMS is used when push fails for
	// escalation-level notifications (LATE_RETURN_ESCALATION).
	SMSFallbackEnabled bool `json:"smsFallbackEnabled"`

	// QuietHoursStart is the hour (0-23, local device time) when quiet hours begin.
	// Zero value means quiet hours are not configured.
	QuietHoursStart *int `json:"quietHoursStart,omitempty"`

	// QuietHoursEnd is the hour (0-23) when quiet hours end.
	QuietHoursEnd *int `json:"quietHoursEnd,omitempty"`

	// TimezoneName is the IANA timezone name used to evaluate quiet hours.
	// Defaults to UTC if absent.
	TimezoneName string `json:"timezoneName,omitempty"`
}

// DefaultPreferences returns a sensible default preference set.
func DefaultPreferences() Preferences {
	return Preferences{
		PushEnabled:        true,
		SMSFallbackEnabled: false,
	}
}

// PushToken represents a device's Expo push token registered by a user.
type PushToken struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"createdAt"`
}

// Sentinel errors.
var (
	ErrNotificationNotFound = errors.New("notification not found")
	ErrTokenNotFound        = errors.New("push token not found")
)
