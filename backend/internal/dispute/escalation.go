package dispute

// RouteDecision implements the escalation gate routing table (PRD §20).
// Rules evaluated top-to-bottom with priority: fraud > inconclusive > low confidence > dollar thresholds.
func RouteDecision(confidence float64, chargeAmountCents int64, photoDiffResult string, hasFraudFlags bool) EscalationRoute {
	if hasFraudFlags {
		return RouteHumanReview
	}
	if photoDiffResult == "INCONCLUSIVE" {
		return RouteHumanReview
	}
	if confidence < 0.85 {
		return RouteHumanReview
	}
	// chargeAmount thresholds are in cents; $200 = 20000, $1000 = 100000
	if chargeAmountCents > 100000 {
		return RouteHumanReview
	}
	if chargeAmountCents > 20000 {
		return RouteAutoResolveAudit
	}
	return RouteAutoResolve
}
