// Package outcome implements the Agent Learning Framework's outcome linking
// and calibration system (PRD 31). It evaluates whether past agent decisions
// were correct by examining real-world outcomes, and maintains per-agent
// confidence calibration metrics.
package outcome

import (
	"time"

	"github.com/giits/rentmy/backend/internal/agent/decision"
)

// OutcomeRule defines how correctness is evaluated for a specific agent type.
type OutcomeRule struct {
	AgentType decision.AgentType
	// Evaluate determines whether a decision was correct given the context.
	// Returns (correct, reason).
	Evaluate func(ctx EvalContext) (bool, string)
}

// EvalContext holds the data needed to evaluate a decision's correctness.
type EvalContext struct {
	Decision *decision.AgentDecision

	// DisputeAgent: was the decision overridden by a human reviewer?
	WasOverridden bool

	// LateReturnAgent: did the renter return within grace period?
	ReturnedWithinGrace bool
	// LateReturnAgent: was the escalation warranted?
	EscalationWasWarranted bool

	// AgreementAgent: did a dispute arise from an agreement gap?
	DisputeFromAgreementGap bool

	// VerificationAgent: was the verified user later fraud-flagged?
	LaterFraudFlagged bool

	// AppraisalAgent: estimated vs actual damage diff within 30%?
	AppraisalWithinThreshold bool
	// AppraisalAgent: did the host override the value?
	HostOverrodeValue bool

	// RiskAgent: for high-risk blocks, was the block warranted?
	// For low-risk passes, did an incident occur?
	RiskBlockWarranted bool
	IncidentOccurred   bool

	// FraudAgent: was the flagged account confirmed fraudulent?
	ConfirmedFraudulent bool
}

// CalibrationBucket represents a confidence range for calibration metrics.
type CalibrationBucket struct {
	AgentType        string  `json:"agentType"`
	BucketLow        float64 `json:"bucketLow"`
	BucketHigh       float64 `json:"bucketHigh"`
	ExpectedAccuracy float64 `json:"expectedAccuracy"`
	ActualAccuracy   float64 `json:"actualAccuracy"`
	CalibrationError float64 `json:"calibrationError"`
	TotalDecisions   int     `json:"totalDecisions"`
	CorrectDecisions int     `json:"correctDecisions"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// CalibrationReport is the monthly per-agent calibration summary.
type CalibrationReport struct {
	AgentType        string              `json:"agentType"`
	Buckets          []CalibrationBucket `json:"buckets"`
	OverallAccuracy  float64             `json:"overallAccuracy"`
	MeanCalibration  float64             `json:"meanCalibrationError"`
	TotalDecisions   int                 `json:"totalDecisions"`
	GeneratedAt      time.Time           `json:"generatedAt"`
}

// DecisionWithOutcome is an AgentDecision enriched with outcome context for
// the admin API.
type DecisionWithOutcome struct {
	ID             string              `json:"id"`
	AgentType      string              `json:"agentType"`
	TransactionID  *string             `json:"transactionId,omitempty"`
	Confidence     *float64            `json:"confidence,omitempty"`
	Escalated      bool                `json:"escalated"`
	OutcomeCorrect *bool               `json:"outcomeCorrect,omitempty"`
	OutcomeID      *string             `json:"outcomeId,omitempty"`
	CreatedAt      time.Time           `json:"createdAt"`
}

// Confidence buckets for calibration as defined in PRD 31.
var CalibrationBuckets = []struct {
	Low  float64
	High float64
}{
	{0.5, 0.6},
	{0.6, 0.7},
	{0.7, 0.8},
	{0.8, 0.9},
	{0.9, 1.0},
}
