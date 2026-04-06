// Package dispute implements the DisputeAgent with escalation gate for autonomous
// dispute resolution. It gathers evidence, runs an LLM for damage assessment,
// routes decisions through the escalation gate, and executes hold captures.
package dispute

import (
	"encoding/json"
	"errors"
	"time"
)

// Sentinel errors for the dispute package.
var (
	ErrDisputeNotFound     = errors.New("dispute not found")
	ErrTransactionNotFound = errors.New("transaction not found")
	ErrAlreadyDisputed     = errors.New("transaction already has an open dispute")
	ErrInvalidStatus       = errors.New("invalid dispute status transition")
)

// Status tracks the lifecycle of a dispute.
type Status string

const (
	StatusPending        Status = "PENDING"         // created, evidence gathering not started
	StatusGathering      Status = "GATHERING"       // evidence collection in progress
	StatusAnalyzing      Status = "ANALYZING"       // LLM is evaluating evidence
	StatusAutoResolved   Status = "AUTO_RESOLVED"   // resolved by agent, no human review
	StatusAuditQueued    Status = "AUDIT_QUEUED"     // resolved by agent, flagged for async audit
	StatusHumanReview    Status = "HUMAN_REVIEW"     // queued for human reviewer
	StatusResolved       Status = "RESOLVED"         // human reviewer approved/overrode
	StatusInconclusive   Status = "INCONCLUSIVE"     // needs more evidence, re-prompt sent
)

// EscalationRoute determines how a dispute decision is routed.
type EscalationRoute string

const (
	RouteAutoResolve      EscalationRoute = "AUTO_RESOLVE"
	RouteAutoResolveAudit EscalationRoute = "AUTO_RESOLVE_AUDIT"
	RouteHumanReview      EscalationRoute = "HUMAN_REVIEW"
)

// Dispute is the domain representation of a rental dispute.
type Dispute struct {
	ID              string          `json:"id"`
	TransactionID   string          `json:"transactionId"`
	ReporterID      string          `json:"reporterId"`
	Reason          string          `json:"reason"`
	Description     string          `json:"description"`
	Status          Status          `json:"status"`
	Route           EscalationRoute `json:"route,omitempty"`
	ChargeAmount    int64           `json:"chargeAmount,omitempty"` // cents
	Confidence      float64         `json:"confidence,omitempty"`
	AgentDecisionID *string         `json:"agentDecisionId,omitempty"`
	ReviewerID      *string         `json:"reviewerId,omitempty"`
	ReviewerNotes   *string         `json:"reviewerNotes,omitempty"`
	SLADeadline     *time.Time      `json:"slaDeadline,omitempty"`
	Evidence        json.RawMessage `json:"evidence,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

// FileDisputeInput is the input for creating a new dispute.
type FileDisputeInput struct {
	TransactionID string `json:"transactionId"`
	ReporterID    string `json:"reporterId"`
	Reason        string `json:"reason"`
	Description   string `json:"description"`
}

// ResolveInput is the input for a human reviewer resolving a dispute.
type ResolveInput struct {
	DisputeID    string `json:"disputeId"`
	ReviewerID   string `json:"reviewerId"`
	Action       string `json:"action"` // "APPROVE", "OVERRIDE"
	ChargeAmount *int64 `json:"chargeAmount,omitempty"` // only for OVERRIDE
	Notes        string `json:"notes"`
}

// EvidencePackage is the gathered evidence for a dispute, assembled in memory
// from existing tables (no data duplication).
type EvidencePackage struct {
	AgreementSnapshot json.RawMessage   `json:"agreementSnapshot,omitempty"`
	CheckInMedia      []MediaRef        `json:"checkInMedia"`
	CheckOutMedia     []MediaRef        `json:"checkOutMedia"`
	Messages          []MessageRef      `json:"messages"`
	ProximityProofs   []ProximityRef    `json:"proximityProofs"`
	PhotoDiffResult   *string           `json:"photoDiffResult,omitempty"`
	PhotoDiffConf     *float64          `json:"photoDiffConfidence,omitempty"`
	TransactionData   TransactionRef    `json:"transactionData"`
	ReporterReputation int              `json:"reporterReputation"`
	OtherReputation    int              `json:"otherReputation"`
	HasFraudFlags      bool             `json:"hasFraudFlags"`
}

// MediaRef is a lightweight reference to a media item for evidence gathering.
type MediaRef struct {
	ID        string  `json:"id"`
	MediaType string  `json:"mediaType"`
	URL       string  `json:"url"`
	GpsLat    *float32 `json:"gpsLat,omitempty"`
	GpsLng    *float32 `json:"gpsLng,omitempty"`
}

// MessageRef is a lightweight reference to a message.
type MessageRef struct {
	SenderID  string    `json:"senderId"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

// ProximityRef is a lightweight reference to a proximity proof.
type ProximityRef struct {
	ProofType  string   `json:"proofType"`
	Method     string   `json:"method"`
	GPSDistance *float64 `json:"gpsDistance,omitempty"`
	Verified   bool     `json:"verified"`
}

// TransactionRef holds the transaction details relevant for dispute analysis.
type TransactionRef struct {
	RentalFee      int64     `json:"rentalFee"`
	HoldAmount     int64     `json:"holdAmount"`
	ItemValue      int64     `json:"itemValue"`
	ScheduledStart time.Time `json:"scheduledStart"`
	ScheduledEnd   time.Time `json:"scheduledEnd"`
	Status         string    `json:"status"`
}

// AgentDecisionOutput is the structured output from the DisputeAgent LLM call.
type AgentDecisionOutput struct {
	Verdict       string  `json:"verdict"`       // "NO_DAMAGE", "MINOR_DAMAGE", "MAJOR_DAMAGE", "MISSING_ITEM"
	ChargeAmount  int64   `json:"chargeAmount"`  // cents
	Confidence    float64 `json:"confidence"`
	Reasoning     string  `json:"reasoning"`
}
