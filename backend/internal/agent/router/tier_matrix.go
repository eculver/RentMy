package router

// tierMatrix maps every AgentTask to its ModelTier.
// TierNone means the caller handles logic deterministically; no LLM call is made.
// TierCheap uses Haiku. TierFull uses Sonnet.
var tierMatrix = map[AgentTask]ModelTier{
	// AppraisalAgent — item identification needs vision reasoning; tag gen is text-only.
	TaskItemIdentification:  TierFull,
	TaskTagGeneration:       TierCheap,
	TaskValueOverrideReview: TierFull,

	// DisputeAgent — evidence decisions require full reasoning; summaries are cheaper.
	TaskEvidenceAnalysis: TierFull,
	TaskEvidenceSummary:  TierCheap,

	// RiskAgent — scoring is rules-based with a lightweight LLM signal.
	TaskRiskScoring: TierCheap,

	// VerificationAgent — KYC edge-case interpretation (most cases handled by Stripe rules).
	TaskKYCInterpretation: TierCheap,

	// AgreementAgent — clause generation needs careful reasoning; template rendering is Go stdlib.
	TaskCustomClauseGeneration: TierFull,
	TaskTemplateRendering:      TierNone,

	// LateReturnAgent — escalation decisions need reasoning; fee calculation is deterministic.
	TaskEscalationDecision: TierFull,
	TaskLateFeeCalculation: TierNone,

	// FraudAgent — pattern detection over history needs full model; signal aggregation is cheaper.
	TaskPatternDetection:  TierFull,
	TaskSignalAggregation: TierCheap,

	// OpsAgent — anomaly detection and health reports are routine / cheap.
	TaskAnomalyDetection: TierCheap,
	TaskHealthReport:     TierCheap,

	// NotificationService — generating short human-readable text is cheap.
	TaskNotificationText: TierCheap,

	// DiscoveryService — semantic matching is a fast, cheap operation.
	TaskSemanticSearch: TierCheap,

	// PhotoDiffService — structural comparison needs vision reasoning.
	TaskPhotoDiffComparison: TierFull,
}

// TierFor returns the ModelTier for a given AgentTask.
// Returns an error if the task is not registered in the matrix.
func TierFor(task AgentTask) (ModelTier, error) {
	tier, ok := tierMatrix[task]
	if !ok {
		return TierNone, &UnknownTaskError{Task: task}
	}
	return tier, nil
}

// AllTasks returns a slice of every registered AgentTask. Used in tests to verify
// the matrix is complete.
func AllTasks() []AgentTask {
	tasks := make([]AgentTask, 0, len(tierMatrix))
	for t := range tierMatrix {
		tasks = append(tasks, t)
	}
	return tasks
}
