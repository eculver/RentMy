// Package guaranteefund implements the platform guarantee fund ledger,
// reserve ratio monitoring, and loss ratio tracking.
package guaranteefund

import (
	"errors"
	"time"
)

// EntryType classifies a guarantee fund ledger entry.
type EntryType string

const (
	EntryTypeContribution      EntryType = "CONTRIBUTION"
	EntryTypeClaim             EntryType = "CLAIM"
	EntryTypeCardRecovery      EntryType = "CARD_RECOVERY"
	EntryTypeCollectionsRef    EntryType = "COLLECTIONS_REFERRAL"
)

// ReserveAction is the recommended action based on reserve ratio health.
type ReserveAction string

const (
	ReserveActionNormal          ReserveAction = "NORMAL"
	ReserveActionAlert           ReserveAction = "ALERT"
	ReserveActionRestrictHigh    ReserveAction = "RESTRICT_HIGH_VALUE"
	ReserveActionRestrictAllGap  ReserveAction = "RESTRICT_ALL_GAP"
)

// Entry represents a single ledger entry in the guarantee fund.
type Entry struct {
	ID            string    `json:"id"`
	TransactionID string    `json:"transactionId"`
	EntryType     EntryType `json:"entryType"`
	Amount        int64     `json:"amount"`       // cents; positive = in, negative = out
	BalanceAfter  int64     `json:"balanceAfter"` // cents
	CreatedAt     time.Time `json:"createdAt"`
}

// FundHealth summarizes the guarantee fund financial state.
type FundHealth struct {
	Balance         int64         `json:"balance"`         // cents
	OutstandingGaps int64         `json:"outstandingGaps"` // cents
	ReserveRatio    float64       `json:"reserveRatio"`    // balance / outstandingGaps
	LossRatio       float64       `json:"lossRatio"`       // rolling 90-day claims / contributions
	Action          ReserveAction `json:"action"`
}

// Sentinel errors.
var (
	ErrFundEmpty         = errors.New("guarantee fund is empty")
	ErrInsufficientFund  = errors.New("guarantee fund has insufficient balance")
)
