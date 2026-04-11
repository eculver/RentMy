// Package rating implements the structured rating system (PRD §15).
// After a completed rental, both parties rate each other using predefined
// bubble tags. No freeform text reviews are allowed.
package rating

import (
	"errors"
	"time"
)

// Sentinel errors for the rating domain.
var (
	ErrRatingNotFound     = errors.New("rating not found")
	ErrTransactionNotFound = errors.New("transaction not found")
	ErrTransactionNotCompleted = errors.New("transaction is not completed")
	ErrAlreadyRated       = errors.New("user has already rated this transaction")
	ErrInvalidBubble      = errors.New("invalid rating bubble for this role")
	ErrNotParticipant     = errors.New("user is not a participant in this transaction")
)

// Bubble is a single structured rating tag.
type Bubble string

// Renter-rates-Host bubbles.
const (
	BubbleGoodCommunication Bubble = "GOOD_COMMUNICATION"
	BubbleOnTime            Bubble = "ON_TIME"
	BubbleItemAsDescribed   Bubble = "ITEM_AS_DESCRIBED"
	BubbleEasyPickup        Bubble = "EASY_PICKUP"
	BubbleFriendly          Bubble = "FRIENDLY"
)

// Host-rates-Renter bubbles.
const (
	BubbleOnTimeReturn    Bubble = "ON_TIME_RETURN"
	BubbleCarefulWithItem Bubble = "CAREFUL_WITH_ITEM"
	BubbleEasyHandoff     Bubble = "EASY_HANDOFF"
	BubbleRespectful      Bubble = "RESPECTFUL"
)

// renterBubbles is the valid set for renter-rates-host submissions.
var renterBubbles = map[Bubble]bool{
	BubbleGoodCommunication: true,
	BubbleOnTime:            true,
	BubbleItemAsDescribed:   true,
	BubbleEasyPickup:        true,
	BubbleFriendly:          true,
}

// hostBubbles is the valid set for host-rates-renter submissions.
var hostBubbles = map[Bubble]bool{
	BubbleGoodCommunication: true,
	BubbleOnTimeReturn:      true,
	BubbleCarefulWithItem:   true,
	BubbleEasyHandoff:       true,
	BubbleRespectful:        true,
}

// ValidBubblesForRenter returns the allowed bubbles when a renter rates a host.
func ValidBubblesForRenter() []Bubble {
	return []Bubble{
		BubbleGoodCommunication,
		BubbleOnTime,
		BubbleItemAsDescribed,
		BubbleEasyPickup,
		BubbleFriendly,
	}
}

// ValidBubblesForHost returns the allowed bubbles when a host rates a renter.
func ValidBubblesForHost() []Bubble {
	return []Bubble{
		BubbleGoodCommunication,
		BubbleOnTimeReturn,
		BubbleCarefulWithItem,
		BubbleEasyHandoff,
		BubbleRespectful,
	}
}

// ValidateBubblesForRenter checks that all bubbles are valid renter-rates-host tags.
func ValidateBubblesForRenter(bubbles []Bubble) error {
	for _, b := range bubbles {
		if !renterBubbles[b] {
			return ErrInvalidBubble
		}
	}
	return nil
}

// ValidateBubblesForHost checks that all bubbles are valid host-rates-renter tags.
func ValidateBubblesForHost(bubbles []Bubble) error {
	for _, b := range bubbles {
		if !hostBubbles[b] {
			return ErrInvalidBubble
		}
	}
	return nil
}

// Rating is the rating domain model.
type Rating struct {
	ID            string
	TransactionID string
	FromUserID    string
	ToUserID      string
	Bubbles       []Bubble
	CreatedAt     time.Time
}

// CreateRatingInput is the input for submitting a new rating.
type CreateRatingInput struct {
	TransactionID string
	FromUserID    string
	Bubbles       []Bubble
}

// BubbleSummaryItem is an aggregated count for a single bubble.
type BubbleSummaryItem struct {
	Bubble Bubble `json:"bubble"`
	Count  int    `json:"count"`
}
