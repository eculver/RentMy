package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Brett2thered/RentMy/backend/internal/notification"
)

// repo is the interface the service uses to interact with the messaging store.
// Using an interface enables unit tests to inject stubs.
type repo interface {
	Insert(ctx context.Context, m Message) (Message, error)
	FindByTransactionID(ctx context.Context, transactionID, cursor string, limit int) ([]Message, string, error)
	GetParties(ctx context.Context, transactionID string) (Parties, error)
	ListConversations(ctx context.Context, userID string) ([]Conversation, error)
}

// pusherClient is the interface the messaging service uses to publish
// real-time events without a direct import of the platform/pusher package.
type pusherClient interface {
	Trigger(channel, event string, data interface{}) error
}

// notificationSvc is the interface the messaging service uses to dispatch
// push notifications to the message recipient.
type notificationSvc interface {
	Notify(ctx context.Context, userID string, t notification.Type, title, body string, data map[string]string) error
}

// Service implements the messaging domain business logic.
type Service struct {
	repo            repo
	pusherClient    pusherClient
	notificationSvc notificationSvc
}

// NewService creates a Service backed by a concrete Repository.
// pusherClient and notificationSvc may be nil (delivery disabled, e.g. dev).
func NewService(repository *Repository, pusherClient pusherClient, notificationSvc notificationSvc) *Service {
	return NewServiceFromParts(repository, pusherClient, notificationSvc)
}

// NewServiceFromParts creates a Service from interface-typed dependencies.
// Exported so tests can inject stubs without importing internal types.
func NewServiceFromParts(repository repo, pusherClient pusherClient, notificationSvc notificationSvc) *Service {
	return &Service{
		repo:            repository,
		pusherClient:    pusherClient,
		notificationSvc: notificationSvc,
	}
}

// SendMessage validates the sender, persists the message, fires a Pusher event
// on the transaction channel, and sends a push notification to the recipient.
// Pusher and push delivery are best-effort: failures are logged, not returned.
func (s *Service) SendMessage(ctx context.Context, in SendMessageInput) (Message, error) {
	content := strings.TrimSpace(in.Content)
	if content == "" {
		return Message{}, ErrEmptyContent
	}
	if len(content) > MaxContentLength {
		return Message{}, ErrContentTooLong
	}

	parties, err := s.repo.GetParties(ctx, in.TransactionID)
	if err != nil {
		return Message{}, err
	}

	if in.SenderID != parties.RenterID && in.SenderID != parties.HostID {
		return Message{}, ErrNotAParty
	}

	msg, err := s.repo.Insert(ctx, Message{
		TransactionID: in.TransactionID,
		SenderID:      in.SenderID,
		Content:       content,
	})
	if err != nil {
		return Message{}, fmt.Errorf("insert message: %w", err)
	}

	// Determine recipient (the other party).
	recipientID := parties.HostID
	if in.SenderID == parties.HostID {
		recipientID = parties.RenterID
	}

	// Publish real-time event on the transaction channel (best-effort).
	if s.pusherClient != nil {
		channel := TransactionChannel(in.TransactionID)
		if err := s.pusherClient.Trigger(channel, EventNewMessage, msg); err != nil {
			slog.Warn("messaging: pusher trigger failed", "transactionId", in.TransactionID, "error", err)
		}
	}

	// Push notification to the recipient (best-effort).
	if s.notificationSvc != nil {
		if err := s.notificationSvc.Notify(ctx, recipientID, notification.TypeNewMessage,
			"New message",
			content,
			map[string]string{"transactionId": in.TransactionID, "messageId": msg.ID},
		); err != nil {
			slog.Warn("messaging: push notification failed", "recipientId", recipientID, "error", err)
		}
	}

	return msg, nil
}

// ListConversations returns all message threads for the given user.
func (s *Service) ListConversations(ctx context.Context, userID string) ([]Conversation, error) {
	return s.repo.ListConversations(ctx, userID)
}

// GetMessages returns paginated messages for a transaction in chronological order.
// Only the renter or host of the transaction may read messages.
// cursor is an exclusive message ULID; pass "" for the first page.
// nextCursor is "" when no further pages exist.
func (s *Service) GetMessages(ctx context.Context, transactionID, requesterID, cursor string, limit int) ([]Message, string, error) {
	parties, err := s.repo.GetParties(ctx, transactionID)
	if err != nil {
		return nil, "", err
	}

	if requesterID != parties.RenterID && requesterID != parties.HostID {
		return nil, "", ErrNotAParty
	}

	if limit <= 0 {
		limit = 50
	}

	return s.repo.FindByTransactionID(ctx, transactionID, cursor, limit)
}
