package messaging

import "fmt"

// Pusher event names used on transaction channels.
const (
	EventNewMessage           = "new-message"
	EventBookingStatusChanged = "booking-status-changed"
)

// TransactionChannel returns the private Pusher channel name for a transaction.
// All messaging and booking-status events are published on this channel.
func TransactionChannel(transactionID string) string {
	return fmt.Sprintf("private-transaction-%s", transactionID)
}
