// Package decision provides the AgentDecision domain type and persistence layer.
// Every AI agent call records a decision for audit, learning, and calibration (PRD §31).
package decision

import (
	"encoding/json"
	"time"
)

// AgentType identifies which agent produced a decision.
type AgentType string

const (
	AgentTypeRisk          AgentType = "RISK"
	AgentTypeVerification  AgentType = "VERIFICATION"
	AgentTypeAppraisal     AgentType = "APPRAISAL"
	AgentTypeDispute       AgentType = "DISPUTE"
	AgentTypeAgreement     AgentType = "AGREEMENT"
	AgentTypeLateReturn    AgentType = "LATE_RETURN"
	AgentTypeFraud         AgentType = "FRAUD"
	AgentTypeOps           AgentType = "OPS"
	AgentTypeHumanOverride AgentType = "HUMAN_OVERRIDE"
)

// AgentDecision records the full context of a single AI agent decision.
type AgentDecision struct {
	ID               string
	AgentType        AgentType
	TransactionID    *string
	UserID           *string
	Input            json.RawMessage
	Decision         json.RawMessage
	Model            *string // nil for TierNone or HUMAN_OVERRIDE
	PromptVersion    *string // e.g., "v1"
	Confidence       *float64
	Escalated        bool
	EscalationReason *string
	ReviewedBy       *string // user ID of human reviewer
	OverrideOf       *string // agent_decision ID this overrides
	OutcomeID        *string // links to the transaction outcome (set 48h later)
	OutcomeCorrect   *bool   // was the decision validated by the outcome?
	CreatedAt        time.Time
}

// CreateDecisionInput holds the inputs for recording a new decision.
type CreateDecisionInput struct {
	AgentType        AgentType
	TransactionID    *string
	UserID           *string
	Input            any
	Decision         any
	Model            *string
	PromptVersion    *string
	Confidence       *float64
	Escalated        bool
	EscalationReason *string
	OverrideOf       *string
}

// UpdateOutcomeInput links an existing decision to a verified outcome.
type UpdateOutcomeInput struct {
	DecisionID     string
	OutcomeID      string
	OutcomeCorrect bool
}
