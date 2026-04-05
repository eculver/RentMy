package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/riverqueue/river"
)

// Config holds tunable parameters for the notification service.
type Config struct {
	// PickupReminderBefore is how far ahead of scheduled_start to send the pickup reminder.
	PickupReminderBefore time.Duration
	// ReturnReminderBefore is how far ahead of scheduled_end to send the return reminder.
	ReturnReminderBefore time.Duration
}

// Service implements the notification domain business logic.
type Service struct {
	repo        *Repository
	pushClient  *PushClient
	riverClient riverInserter
	cfg         Config
}

// NewService creates a Service. pushClient may be nil (push disabled).
// riverClient may be nil (scheduled jobs disabled, e.g. in unit tests).
func NewService(repo *Repository, pushClient *PushClient, riverClient riverInserter, cfg Config) *Service {
	return &Service{
		repo:        repo,
		pushClient:  pushClient,
		riverClient: riverClient,
		cfg:         cfg,
	}
}

// Notify stores a notification and delivers it via push (subject to preferences
// and quiet hours). If the user is in their quiet hours window the push is
// deferred via a River job; the in-app record is always stored immediately.
func (s *Service) Notify(ctx context.Context, userID string, t Type, title, body string, data map[string]string) error {
	prefs, err := s.repo.GetUserPreferences(ctx, userID)
	if err != nil {
		slog.Warn("failed to load user preferences; using defaults", "userID", userID, "error", err)
		prefs = DefaultPreferences()
	}

	if IsTypeDisabled(prefs, t) {
		return nil
	}

	// Always store the in-app record.
	if storeErr := s.storeNotification(ctx, userID, t, title, body, data); storeErr != nil {
		return storeErr
	}

	if !prefs.PushEnabled || s.pushClient == nil {
		return nil
	}

	if IsQuietHours(prefs, time.Now()) {
		s.deferPush(ctx, prefs, userID, t, title, body, data)
		return nil
	}

	s.sendPush(ctx, userID, title, body, data)
	return nil
}

// notifyDirect stores and delivers a notification bypassing quiet-hours checks.
// Used by QuietHoursDeferredWorker after the quiet window has closed.
func (s *Service) notifyDirect(ctx context.Context, userID string, t Type, title, body string, data map[string]string) error {
	prefs, err := s.repo.GetUserPreferences(ctx, userID)
	if err != nil {
		prefs = DefaultPreferences()
	}
	if IsTypeDisabled(prefs, t) {
		return nil
	}
	if err := s.storeNotification(ctx, userID, t, title, body, data); err != nil {
		return err
	}
	if prefs.PushEnabled && s.pushClient != nil {
		s.sendPush(ctx, userID, title, body, data)
	}
	return nil
}

func (s *Service) storeNotification(ctx context.Context, userID string, t Type, title, body string, data map[string]string) error {
	var raw json.RawMessage
	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("marshal notification data: %w", err)
		}
		raw = b
	} else {
		raw = json.RawMessage("{}")
	}
	return s.repo.Insert(ctx, Notification{
		UserID: userID,
		Type:   t,
		Title:  title,
		Body:   body,
		Data:   raw,
	})
}

func (s *Service) sendPush(ctx context.Context, userID, title, body string, data map[string]string) {
	tokens, err := s.repo.GetPushTokens(ctx, userID)
	if err != nil {
		slog.Warn("failed to get push tokens", "userID", userID, "error", err)
		return
	}
	if len(tokens) == 0 {
		return
	}
	stale, err := s.pushClient.SendBatch(ctx, tokens, title, body, data)
	if err != nil {
		slog.Warn("push send failed", "userID", userID, "error", err)
	}
	for _, tok := range stale {
		if delErr := s.repo.DeletePushToken(ctx, tok); delErr != nil {
			slog.Warn("failed to delete stale push token", "token", tok, "error", delErr)
		}
	}
}

func (s *Service) deferPush(ctx context.Context, prefs Preferences, userID string, t Type, title, body string, data map[string]string) {
	if s.riverClient == nil {
		return
	}
	fireAt := QuietHoursEndTime(prefs, time.Now())
	if fireAt.IsZero() {
		return
	}
	_, err := s.riverClient.Insert(ctx, QuietHoursDeferredArgs{
		UserID:    userID,
		NotifType: string(t),
		Title:     title,
		Body:      body,
		Data:      data,
	}, &river.InsertOpts{ScheduledAt: fireAt})
	if err != nil {
		slog.Warn("failed to defer push notification", "userID", userID, "error", err)
	}
}

// RegisterPushToken saves an Expo push token for a user.
func (s *Service) RegisterPushToken(ctx context.Context, userID, token string) error {
	return s.repo.InsertPushToken(ctx, userID, token)
}

// GetNotifications returns paginated in-app notifications for a user.
func (s *Service) GetNotifications(ctx context.Context, userID string, limit, offset int) ([]Notification, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.repo.FindByUserID(ctx, userID, limit, offset)
}

// MarkRead marks a single notification as read.
func (s *Service) MarkRead(ctx context.Context, notificationID, userID string) error {
	return s.repo.MarkRead(ctx, notificationID, userID)
}

// MarkAllRead marks all notifications for a user as read.
func (s *Service) MarkAllRead(ctx context.Context, userID string) error {
	return s.repo.MarkAllRead(ctx, userID)
}

// CountUnread returns the count of unread notifications for a user.
func (s *Service) CountUnread(ctx context.Context, userID string) (int, error) {
	return s.repo.CountUnread(ctx, userID)
}

// GetPreferences returns a user's notification preferences.
func (s *Service) GetPreferences(ctx context.Context, userID string) (Preferences, error) {
	return s.repo.GetUserPreferences(ctx, userID)
}

// UpdatePreferences saves updated notification preferences for a user.
func (s *Service) UpdatePreferences(ctx context.Context, userID string, prefs Preferences) error {
	return s.repo.UpdateUserPreferences(ctx, userID, prefs)
}
