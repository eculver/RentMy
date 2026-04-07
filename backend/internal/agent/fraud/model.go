// Package fraud implements the FraudAgent — per-transaction signal detection,
// cross-platform pattern analysis, and fraud flag lifecycle management.
package fraud

import (
	"encoding/json"
	"errors"
	"time"
)

// SignalType identifies the kind of fraud signal detected.
type SignalType string

const (
	SignalSharedDeviceFingerprint SignalType = "DEVICE_FINGERPRINT"
	SignalLinkedPaymentInstrument SignalType = "PAYMENT_INSTRUMENT"
	SignalCarrierBatchPhone       SignalType = "CARRIER_BATCH"
	SignalSimultaneousCreation    SignalType = "SIMULTANEOUS_CREATION"
	SignalExclusivePair           SignalType = "EXCLUSIVE_PAIR"
	SignalWiFiNetwork             SignalType = "WIFI_NETWORK"
	SignalDamagePattern           SignalType = "DAMAGE_PATTERN"
	SignalValueSpike              SignalType = "VALUE_SPIKE"
)

// Action is the recommended action taken when a FraudFlag is created.
type Action string

const (
	ActionMonitor Action = "MONITOR"
	ActionFlag    Action = "FLAG"
	ActionSuspend Action = "SUSPEND"
)

// FraudSignal represents a single detected fraud indicator for a user.
type FraudSignal struct {
	Type           SignalType      `json:"type"`
	UserID         string          `json:"userId"`
	RelatedUserID  string          `json:"relatedUserId,omitempty"`
	Score          int             `json:"score"`
	IsCompoundOnly bool            `json:"isCompoundOnly"`
	Evidence       json.RawMessage `json:"evidence,omitempty"`
	DetectedAt     time.Time       `json:"detectedAt"`
}

// SignalBundle aggregates all signals detected for a single user during one
// evaluation run.  The WiFi compound-only logic is applied here:
// WiFiNetwork signals contribute to CompoundScore only when HasNonCompoundSignal
// is true (i.e., at least one other non-compound signal was detected).
type SignalBundle struct {
	UserID               string        `json:"userId"`
	Signals              []FraudSignal `json:"signals"`
	CompoundScore        int           `json:"compoundScore"`
	HasNonCompoundSignal bool          `json:"hasNonCompoundSignal"`
}

// TotalScore returns the effective score for the bundle.
// WiFi-only bundles return 0 regardless of the raw CompoundScore.
func (b *SignalBundle) TotalScore() int {
	if !b.HasNonCompoundSignal {
		// No qualifying signal — WiFi alone counts for nothing.
		return 0
	}
	return b.CompoundScore
}

// FraudFlag records a completed fraud investigation against a user.
type FraudFlag struct {
	ID               string        `json:"id"`
	UserID           string        `json:"userId"`
	Signals          []FraudSignal `json:"signals"`
	TotalScore       int           `json:"totalScore"`
	Action           Action        `json:"action"`
	AgentDecisionID  *string       `json:"agentDecisionId,omitempty"`
	ResolvedAt       *time.Time    `json:"resolvedAt,omitempty"`
	ResolvedBy       *string       `json:"resolvedBy,omitempty"`
	ResolutionNotes  *string       `json:"resolutionNotes,omitempty"`
	CreatedAt        time.Time     `json:"createdAt"`
}

// FraudFlagFilters are query parameters for listing fraud flags.
type FraudFlagFilters struct {
	Status string // "OPEN" | "RESOLVED" | "" (all)
	Action Action // "" = all
	Page   int
	Limit  int
}

// resolveRequest is the JSON body for PUT /ops/fraud/flags/:id/resolve.
type resolveRequest struct {
	Outcome string `json:"outcome"`
	Notes   string `json:"notes"`
}

// ScoreThreshold is the default total-score above which a flag is created.
const ScoreThreshold = 80

// actionFromScore maps a score to the recommended action.
func actionFromScore(score int) Action {
	switch {
	case score >= 100:
		return ActionSuspend
	case score >= ScoreThreshold:
		return ActionFlag
	default:
		return ActionMonitor
	}
}

// Sentinel errors.
var (
	ErrFlagNotFound = errors.New("fraud: flag not found")
)
