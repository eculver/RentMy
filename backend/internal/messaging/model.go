// Package messaging implements in-transaction chat between renters and hosts
// via Pusher real-time events and push notifications (PRD §6: messages table).
package messaging

import (
	"errors"
	"time"
)

// MaxContentLength is the maximum allowed message length in characters.
const MaxContentLength = 4000

// Message is a single in-transaction chat message.
type Message struct {
	ID            string    `json:"id"`
	TransactionID string    `json:"transactionId"`
	SenderID      string    `json:"senderId"`
	Content       string    `json:"content"`
	CreatedAt     time.Time `json:"createdAt"`
}

// SendMessageInput is the validated input for sending a new message.
type SendMessageInput struct {
	TransactionID string
	SenderID      string
	Content       string
}

// Parties holds the renter and host IDs for a transaction, used for
// authorization checks without importing the booking package.
type Parties struct {
	RenterID string
	HostID   string
}

// Sentinel errors for the messaging domain.
var (
	ErrMessageNotFound     = errors.New("message not found")
	ErrNotAParty           = errors.New("sender is not a party to this transaction")
	ErrEmptyContent        = errors.New("message content cannot be empty")
	ErrContentTooLong      = errors.New("message content exceeds maximum length")
	ErrTransactionNotFound = errors.New("transaction not found")
)
