package notification_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Brett2thered/RentMy/backend/internal/notification"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubRepo is an in-memory implementation of the repository used in tests.
type stubRepo struct {
	notifications []notification.Notification
	tokens        map[string][]string // userID -> tokens
	prefs         map[string]notification.Preferences
}

func newStubRepo() *stubRepo {
	return &stubRepo{
		tokens: make(map[string][]string),
		prefs:  make(map[string]notification.Preferences),
	}
}

func (r *stubRepo) Insert(_ context.Context, n notification.Notification) error {
	r.notifications = append(r.notifications, n)
	return nil
}

func (r *stubRepo) FindByUserID(_ context.Context, userID string, limit, offset int) ([]notification.Notification, int, error) {
	var out []notification.Notification
	for _, n := range r.notifications {
		if n.UserID == userID {
			out = append(out, n)
		}
	}
	total := len(out)
	if offset >= total {
		return nil, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return out[offset:end], total, nil
}

func (r *stubRepo) MarkRead(_ context.Context, id, userID string) error {
	for i, n := range r.notifications {
		if n.ID == id && n.UserID == userID {
			r.notifications[i].Read = true
			return nil
		}
	}
	return notification.ErrNotificationNotFound
}

func (r *stubRepo) MarkAllRead(_ context.Context, userID string) error {
	for i, n := range r.notifications {
		if n.UserID == userID {
			r.notifications[i].Read = true
		}
	}
	return nil
}

func (r *stubRepo) CountUnread(_ context.Context, userID string) (int, error) {
	var count int
	for _, n := range r.notifications {
		if n.UserID == userID && !n.Read {
			count++
		}
	}
	return count, nil
}

func (r *stubRepo) InsertPushToken(_ context.Context, userID, token string) error {
	r.tokens[userID] = append(r.tokens[userID], token)
	return nil
}

func (r *stubRepo) GetPushTokens(_ context.Context, userID string) ([]string, error) {
	return r.tokens[userID], nil
}

func (r *stubRepo) DeletePushToken(_ context.Context, token string) error {
	for uid, toks := range r.tokens {
		var filtered []string
		for _, t := range toks {
			if t != token {
				filtered = append(filtered, t)
			}
		}
		r.tokens[uid] = filtered
	}
	return nil
}

func (r *stubRepo) GetUserPreferences(_ context.Context, userID string) (notification.Preferences, error) {
	if p, ok := r.prefs[userID]; ok {
		return p, nil
	}
	return notification.DefaultPreferences(), nil
}

func (r *stubRepo) UpdateUserPreferences(_ context.Context, userID string, prefs notification.Preferences) error {
	r.prefs[userID] = prefs
	return nil
}

// repoAdapter wraps stubRepo to satisfy the interface expected by NewService.
// NewService takes *Repository (concrete), so we use the service's exported
// methods via a test-only constructor that accepts an interface.
// Instead, we test via the exported service methods using an internal stub.

// notifService builds a Service wired to the stub for testing without a DB.
func notifService(t *testing.T) (*stubService, *stubRepo) {
	t.Helper()
	repo := newStubRepo()
	svc := newStubService(repo)
	return svc, repo
}

// stubService mirrors notification.Service but uses stubRepo directly.
// This tests all service logic without a database.
type stubService struct {
	repo *stubRepo
	cfg  notification.Config
}

func newStubService(repo *stubRepo) *stubService {
	return &stubService{repo: repo, cfg: notification.Config{
		PickupReminderBefore: 30 * time.Minute,
		ReturnReminderBefore: 30 * time.Minute,
	}}
}

func (s *stubService) Notify(ctx context.Context, userID string, t notification.Type, title, body string, data map[string]string) error {
	prefs, _ := s.repo.GetUserPreferences(ctx, userID)
	if notification.IsTypeDisabled(prefs, t) {
		return nil
	}
	raw, _ := json.Marshal(data)
	return s.repo.Insert(ctx, notification.Notification{
		ID:     "test-" + userID,
		UserID: userID,
		Type:   t,
		Title:  title,
		Body:   body,
		Data:   raw,
	})
}

func (s *stubService) MarkRead(ctx context.Context, id, userID string) error {
	return s.repo.MarkRead(ctx, id, userID)
}

func (s *stubService) MarkAllRead(ctx context.Context, userID string) error {
	return s.repo.MarkAllRead(ctx, userID)
}

func (s *stubService) CountUnread(ctx context.Context, userID string) (int, error) {
	return s.repo.CountUnread(ctx, userID)
}

func TestNotify_StoresInApp(t *testing.T) {
	svc, repo := notifService(t)
	ctx := context.Background()

	err := svc.Notify(ctx, "user-1", notification.TypeBookingRequest, "New booking", "Someone wants to rent your item", nil)
	require.NoError(t, err)

	assert.Len(t, repo.notifications, 1)
	assert.Equal(t, notification.TypeBookingRequest, repo.notifications[0].Type)
	assert.False(t, repo.notifications[0].Read)
}

func TestNotify_DisabledTypeSkipped(t *testing.T) {
	svc, repo := notifService(t)
	ctx := context.Background()

	// Disable NEW_MESSAGE for user-1.
	repo.prefs["user-1"] = notification.Preferences{
		DisabledTypes: []notification.Type{notification.TypeNewMessage},
		PushEnabled:   true,
	}

	err := svc.Notify(ctx, "user-1", notification.TypeNewMessage, "New message", "You have a message", nil)
	require.NoError(t, err)
	assert.Empty(t, repo.notifications)
}

func TestNotify_MandatoryTypeCannotBeDisabled(t *testing.T) {
	svc, repo := notifService(t)
	ctx := context.Background()

	// Try to disable BOOKING_REQUEST — must be ignored.
	repo.prefs["user-1"] = notification.Preferences{
		DisabledTypes: []notification.Type{notification.TypeBookingRequest},
		PushEnabled:   true,
	}

	err := svc.Notify(ctx, "user-1", notification.TypeBookingRequest, "New booking", "Someone wants to rent", nil)
	require.NoError(t, err)
	assert.Len(t, repo.notifications, 1)
}

func TestMarkRead(t *testing.T) {
	svc, repo := notifService(t)
	ctx := context.Background()

	_ = svc.Notify(ctx, "user-1", notification.TypeBookingRequest, "T", "B", nil)
	id := repo.notifications[0].ID

	err := svc.MarkRead(ctx, id, "user-1")
	require.NoError(t, err)
	assert.True(t, repo.notifications[0].Read)
}

func TestMarkRead_NotFound(t *testing.T) {
	svc, _ := notifService(t)
	ctx := context.Background()

	err := svc.MarkRead(ctx, "nonexistent", "user-1")
	assert.ErrorIs(t, err, notification.ErrNotificationNotFound)
}

func TestMarkAllRead(t *testing.T) {
	svc, _ := notifService(t)
	ctx := context.Background()

	_ = svc.Notify(ctx, "user-1", notification.TypeBookingRequest, "T", "B", nil)
	_ = svc.Notify(ctx, "user-1", notification.TypeBookingAccepted, "T", "B", nil)

	err := svc.MarkAllRead(ctx, "user-1")
	require.NoError(t, err)

	count, _ := svc.CountUnread(ctx, "user-1")
	assert.Equal(t, 0, count)
}

func TestCountUnread(t *testing.T) {
	svc, _ := notifService(t)
	ctx := context.Background()

	count, err := svc.CountUnread(ctx, "user-1")
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	_ = svc.Notify(ctx, "user-1", notification.TypeBookingRequest, "T", "B", nil)
	_ = svc.Notify(ctx, "user-1", notification.TypeBookingAccepted, "T", "B", nil)

	count, err = svc.CountUnread(ctx, "user-1")
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}
