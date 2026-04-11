// Package latereturn implements the LateReturnAgent which monitors active rentals
// past their scheduled end, auto-charges late fees, and escalates to DisputeAgent
// when warranted.
package latereturn

import (
	"errors"
	"time"
)

// Sentinel errors for the latereturn package.
var (
	ErrLateReturnNotFound  = errors.New("late return record not found")
	ErrTransactionNotFound = errors.New("transaction not found")
	ErrRentalNotActive     = errors.New("rental is no longer active")
	ErrCapExceeded         = errors.New("late fee would exceed damage reserve cap")
)

// Status tracks the lifecycle of a late return record.
type Status string

const (
	StatusMonitoring         Status = "MONITORING"           // scheduled_end passed, watching
	StatusCharging           Status = "CHARGING"             // actively charging hourly late fees
	StatusEscalatedToDispute Status = "ESCALATED_TO_DISPUTE" // handed off to DisputeAgent
	StatusFlaggedForReview   Status = "FLAGGED_FOR_REVIEW"   // renter unresponsive, potential theft
	StatusResolved           Status = "RESOLVED"             // item returned, late fees finalized
)

// EscalationLevel is the LLM-decided escalation for a late return.
type EscalationLevel string

const (
	EscalationCharging          EscalationLevel = "CHARGING"              // continue charging, no escalation
	EscalationWarning           EscalationLevel = "WARNING"               // send warning, continue monitoring
	EscalationEscalateToDispute EscalationLevel = "ESCALATED_TO_DISPUTE"  // hand off to DisputeAgent
	EscalationFlagForReview     EscalationLevel = "FLAGGED_FOR_REVIEW"    // potential theft
)

// LateReturn is the domain representation of a late return record.
type LateReturn struct {
	ID              string          `json:"id"`
	TransactionID   string          `json:"transactionId"`
	RenterID        string          `json:"renterId"`
	HostID          string          `json:"hostId"`
	ScheduledEnd    time.Time       `json:"scheduledEnd"`
	ActualEnd       *time.Time      `json:"actualEnd,omitempty"`
	LateMinutes     int             `json:"lateMinutes"`
	Status          Status          `json:"status"`
	EscalationLevel *EscalationLevel `json:"escalationLevel,omitempty"`
	TotalFeeCharged int64           `json:"totalFeeCharged"` // cents
	Confidence      *float64        `json:"confidence,omitempty"`
	AgentDecisionID *string         `json:"agentDecisionId,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

// LateReturnInput is the evidence assembled for the LateReturnAgent LLM call.
type LateReturnInput struct {
	TransactionID       string  `json:"transactionId"`
	RenterID            string  `json:"renterId"`
	HostID              string  `json:"hostId"`
	MinutesOverdue      int     `json:"minutesOverdue"`
	HoursOverdue        float64 `json:"hoursOverdue"`
	ItemValue           int64   `json:"itemValue"`           // cents
	HoldAmount          int64   `json:"holdAmount"`          // cents
	HoldRemaining       int64   `json:"holdRemaining"`       // cents
	TotalLateFeesSoFar  int64   `json:"totalLateFeesSoFar"`  // cents
	RenterReputation    int     `json:"renterReputation"`
	RenterMessageCount  int     `json:"renterMessageCount"`  // messages in last 2h
	HasConflictingBooking bool  `json:"hasConflictingBooking"`
	TimeOfDay           string  `json:"timeOfDay"` // e.g. "14:30 PST"
}

// EscalationDecisionOutput is the structured output from the LLM escalation call.
type EscalationDecisionOutput struct {
	EscalationLevel EscalationLevel `json:"escalationLevel"`
	Confidence      float64         `json:"confidence"`
	Reasoning       string          `json:"reasoning"`
}
