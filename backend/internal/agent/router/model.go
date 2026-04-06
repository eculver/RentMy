// Package router provides the AI model router for dispatching agent tasks to the
// appropriate Claude model tier (Haiku = cheap/fast, Sonnet = full reasoning).
package router

import "time"

// ModelTier determines which model handles a task.
type ModelTier int

const (
	TierNone  ModelTier = iota // deterministic — no LLM call
	TierCheap                  // Haiku — fast, low-cost
	TierFull                   // Sonnet — complex reasoning / vision
)

// AgentTask identifies a specific agent operation for routing.
type AgentTask string

const (
	// AppraisalAgent tasks.
	TaskItemIdentification  AgentTask = "appraisal.item_identification"
	TaskTagGeneration       AgentTask = "appraisal.tag_generation"
	TaskValueOverrideReview AgentTask = "appraisal.value_override_review"

	// DisputeAgent tasks.
	TaskEvidenceAnalysis AgentTask = "dispute.evidence_analysis"
	TaskEvidenceSummary  AgentTask = "dispute.evidence_summary"

	// RiskAgent tasks.
	TaskRiskScoring AgentTask = "risk.scoring"

	// VerificationAgent tasks.
	TaskKYCInterpretation AgentTask = "verification.kyc_interpretation"

	// AgreementAgent tasks.
	TaskCustomClauseGeneration AgentTask = "agreement.custom_clause_generation"
	TaskTemplateRendering      AgentTask = "agreement.template_rendering"

	// LateReturnAgent tasks.
	TaskEscalationDecision AgentTask = "late_return.escalation_decision"
	TaskLateFeeCalculation AgentTask = "late_return.fee_calculation"

	// FraudAgent tasks.
	TaskPatternDetection  AgentTask = "fraud.pattern_detection"
	TaskSignalAggregation AgentTask = "fraud.signal_aggregation"

	// OpsAgent tasks.
	TaskAnomalyDetection AgentTask = "ops.anomaly_detection"
	TaskHealthReport     AgentTask = "ops.health_report"

	// NotificationService tasks.
	TaskNotificationText AgentTask = "notification.text_generation"

	// DiscoveryService tasks.
	TaskSemanticSearch AgentTask = "discovery.semantic_search"

	// PhotoDiffService tasks.
	TaskPhotoDiffComparison AgentTask = "photodiff.comparison"
)

// RouteInput is the payload sent to the model.
type RouteInput struct {
	Task         AgentTask
	SystemPrompt string
	UserPrompt   string
	Images       []ImageInput // vision inputs (base64-encoded)
	MaxTokens    int
}

// ImageInput holds a base64-encoded image for vision tasks.
type ImageInput struct {
	MediaType string // "image/jpeg", "image/png", etc.
	Data      []byte // raw image bytes (will be base64-encoded before sending)
}

// RouteOutput is the model response with metadata.
type RouteOutput struct {
	Content       string
	Model         string        // actual model ID used (e.g., "claude-sonnet-4-6")
	PromptVersion string        // e.g., "v1"
	InputTokens   int
	OutputTokens  int
	Latency       time.Duration
	Cached        bool // whether prompt caching was used
}
