package backfill

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestJobProgressCounters verifies that JobProgress atomic counters behave
// correctly under sequential increments.
func TestJobProgressCounters(t *testing.T) {
	p := NewJobProgress()

	p.AppraisalTotal.Store(100)
	p.AppraisalDone.Add(50)
	p.AppraisalErrors.Add(3)

	p.ReputationTotal.Store(200)
	p.ReputationDone.Add(100)
	p.ReputationErrors.Add(0)

	p.RiskTotal.Store(50)
	p.RiskDone.Add(25)
	p.RiskErrors.Add(1)

	status := p.Status()

	assert.Equal(t, int64(100), status["appraisals"].Total)
	assert.Equal(t, int64(50), status["appraisals"].Processed)
	assert.Equal(t, int64(3), status["appraisals"].Errors)

	assert.Equal(t, int64(200), status["reputation"].Total)
	assert.Equal(t, int64(100), status["reputation"].Processed)
	assert.Equal(t, int64(0), status["reputation"].Errors)

	assert.Equal(t, int64(50), status["risk_scores"].Total)
	assert.Equal(t, int64(25), status["risk_scores"].Processed)
	assert.Equal(t, int64(1), status["risk_scores"].Errors)
}

// TestJobKinds verifies that each job args type returns the expected kind string.
func TestJobKinds(t *testing.T) {
	assert.Equal(t, "backfill_appraisal", BackfillAppraisalJobArgs{}.Kind())
	assert.Equal(t, "backfill_reputation", BackfillReputationJobArgs{}.Kind())
	assert.Equal(t, "backfill_risk_scores", BackfillRiskScoreJobArgs{}.Kind())
}
