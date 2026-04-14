package messaging_test

import (
	"context"
	"testing"

	"github.com/Brett2thered/RentMy/backend/internal/messaging"
	"github.com/Brett2thered/RentMy/backend/internal/notification"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubRepo is an in-memory implementation of the repo interface used in tests.
type stubRepo struct {
	messages map[string][]messaging.Message // transactionID -> messages
	parties  map[string]messaging.Parties   // transactionID -> Parties
	counter  int
}

func newStubRepo() *stubRepo {
	return &stubRepo{
		messages: make(map[string][]messaging.Message),
		parties:  make(map[string]messaging.Parties),
	}
}

func (r *stubRepo) Insert(_ context.Context, m messaging.Message) (messaging.Message, error) {
	r.counter++
	if m.ID == "" {
		m.ID = "msg-" + m.TransactionID
	}
	r.messages[m.TransactionID] = append(r.messages[m.TransactionID], m)
	return m, nil
}

func (r *stubRepo) FindByTransactionID(_ context.Context, transactionID, cursor string, limit int) ([]messaging.Message, string, error) {
	all := r.messages[transactionID]
	var filtered []messaging.Message
	for _, m := range all {
		if cursor == "" || m.ID > cursor {
			filtered = append(filtered, m)
		}
	}
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	var nextCursor string
	if len(filtered) == limit && len(r.messages[transactionID]) > limit {
		nextCursor = filtered[len(filtered)-1].ID
	}
	return filtered, nextCursor, nil
}

func (r *stubRepo) GetParties(_ context.Context, transactionID string) (messaging.Parties, error) {
	p, ok := r.parties[transactionID]
	if !ok {
		return messaging.Parties{}, messaging.ErrTransactionNotFound
	}
	return p, nil
}

func (r *stubRepo) ListConversations(_ context.Context, _ string) ([]messaging.Conversation, error) {
	return nil, nil
}

// stubPusher records Trigger calls.
type stubPusher struct {
	triggered []struct{ channel, event string }
}

func (p *stubPusher) Trigger(channel, event string, _ interface{}) error {
	p.triggered = append(p.triggered, struct{ channel, event string }{channel, event})
	return nil
}

// stubNotificationSvc records Notify calls.
type stubNotificationSvc struct {
	notified []string // recipient user IDs
}

func (s *stubNotificationSvc) Notify(_ context.Context, userID string, _ notification.Type, _, _ string, _ map[string]string) error {
	s.notified = append(s.notified, userID)
	return nil
}

func TestSendMessage_Success(t *testing.T) {
	repo := newStubRepo()
	pusher := &stubPusher{}
	notifSvc := &stubNotificationSvc{}

	repo.parties["txn-1"] = messaging.Parties{RenterID: "renter-1", HostID: "host-1"}

	svc := messaging.NewServiceFromParts(repo, pusher, notifSvc)

	msg, err := svc.SendMessage(context.Background(), messaging.SendMessageInput{
		TransactionID: "txn-1",
		SenderID:      "renter-1",
		Content:       "On my way!",
	})
	require.NoError(t, err)
	assert.Equal(t, "txn-1", msg.TransactionID)
	assert.Equal(t, "renter-1", msg.SenderID)
	assert.Equal(t, "On my way!", msg.Content)

	// Pusher event fired on the correct channel.
	require.Len(t, pusher.triggered, 1)
	assert.Equal(t, "private-transaction-txn-1", pusher.triggered[0].channel)
	assert.Equal(t, messaging.EventNewMessage, pusher.triggered[0].event)

	// Push notification sent to the host (recipient).
	require.Len(t, notifSvc.notified, 1)
	assert.Equal(t, "host-1", notifSvc.notified[0])
}

func TestSendMessage_HostSendsToRenter(t *testing.T) {
	repo := newStubRepo()
	notifSvc := &stubNotificationSvc{}

	repo.parties["txn-2"] = messaging.Parties{RenterID: "renter-2", HostID: "host-2"}

	svc := messaging.NewServiceFromParts(repo, &stubPusher{}, notifSvc)

	_, err := svc.SendMessage(context.Background(), messaging.SendMessageInput{
		TransactionID: "txn-2",
		SenderID:      "host-2",
		Content:       "I'll meet you at the door.",
	})
	require.NoError(t, err)

	// Push notification sent to the renter (other party).
	require.Len(t, notifSvc.notified, 1)
	assert.Equal(t, "renter-2", notifSvc.notified[0])
}

func TestSendMessage_EmptyContent(t *testing.T) {
	repo := newStubRepo()
	repo.parties["txn-3"] = messaging.Parties{RenterID: "r", HostID: "h"}

	svc := messaging.NewServiceFromParts(repo, nil, nil)

	_, err := svc.SendMessage(context.Background(), messaging.SendMessageInput{
		TransactionID: "txn-3",
		SenderID:      "r",
		Content:       "   ", // whitespace only
	})
	assert.ErrorIs(t, err, messaging.ErrEmptyContent)
}

func TestSendMessage_ContentTooLong(t *testing.T) {
	repo := newStubRepo()
	repo.parties["txn-4"] = messaging.Parties{RenterID: "r", HostID: "h"}

	svc := messaging.NewServiceFromParts(repo, nil, nil)

	longContent := make([]byte, messaging.MaxContentLength+1)
	for i := range longContent {
		longContent[i] = 'a'
	}

	_, err := svc.SendMessage(context.Background(), messaging.SendMessageInput{
		TransactionID: "txn-4",
		SenderID:      "r",
		Content:       string(longContent),
	})
	assert.ErrorIs(t, err, messaging.ErrContentTooLong)
}

func TestSendMessage_NotAParty(t *testing.T) {
	repo := newStubRepo()
	repo.parties["txn-5"] = messaging.Parties{RenterID: "renter-5", HostID: "host-5"}

	svc := messaging.NewServiceFromParts(repo, nil, nil)

	_, err := svc.SendMessage(context.Background(), messaging.SendMessageInput{
		TransactionID: "txn-5",
		SenderID:      "stranger",
		Content:       "Let me in!",
	})
	assert.ErrorIs(t, err, messaging.ErrNotAParty)
}

func TestSendMessage_TransactionNotFound(t *testing.T) {
	repo := newStubRepo()
	svc := messaging.NewServiceFromParts(repo, nil, nil)

	_, err := svc.SendMessage(context.Background(), messaging.SendMessageInput{
		TransactionID: "nonexistent",
		SenderID:      "someone",
		Content:       "Hello?",
	})
	assert.ErrorIs(t, err, messaging.ErrTransactionNotFound)
}

func TestGetMessages_ReturnsChronological(t *testing.T) {
	repo := newStubRepo()
	repo.parties["txn-6"] = messaging.Parties{RenterID: "r6", HostID: "h6"}

	svc := messaging.NewServiceFromParts(repo, &stubPusher{}, nil)

	// Send 3 messages from the renter.
	for i := 0; i < 3; i++ {
		_, err := svc.SendMessage(context.Background(), messaging.SendMessageInput{
			TransactionID: "txn-6",
			SenderID:      "r6",
			Content:       "msg",
		})
		require.NoError(t, err)
	}

	msgs, _, err := svc.GetMessages(context.Background(), "txn-6", "r6", "", 50)
	require.NoError(t, err)
	assert.Len(t, msgs, 3)
}

func TestGetMessages_NotAParty(t *testing.T) {
	repo := newStubRepo()
	repo.parties["txn-7"] = messaging.Parties{RenterID: "r7", HostID: "h7"}

	svc := messaging.NewServiceFromParts(repo, nil, nil)

	_, _, err := svc.GetMessages(context.Background(), "txn-7", "outsider", "", 50)
	assert.ErrorIs(t, err, messaging.ErrNotAParty)
}

func TestGetMessages_TransactionNotFound(t *testing.T) {
	repo := newStubRepo()
	svc := messaging.NewServiceFromParts(repo, nil, nil)

	_, _, err := svc.GetMessages(context.Background(), "no-such-txn", "anyone", "", 50)
	assert.ErrorIs(t, err, messaging.ErrTransactionNotFound)
}

func TestChannelNaming(t *testing.T) {
	assert.Equal(t, "private-transaction-abc123", messaging.TransactionChannel("abc123"))
}
