package dispute

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildUserPrompt(t *testing.T) {
	evidence := EvidencePackage{
		TransactionData: TransactionRef{
			RentalFee:  5000,
			HoldAmount: 10000,
			ItemValue:  25000,
			Status:     "DISPUTED",
		},
		ReporterReputation: 850,
		OtherReputation:    720,
		HasFraudFlags:      false,
	}

	prompt := buildUserPrompt(evidence)

	assert.True(t, strings.Contains(prompt, "Evaluate this dispute evidence"))
	assert.True(t, strings.Contains(prompt, "25000"))
	assert.True(t, strings.Contains(prompt, "DISPUTED"))
}

func TestSystemPromptContainsVerdicts(t *testing.T) {
	assert.Contains(t, systemPrompt, "NO_DAMAGE")
	assert.Contains(t, systemPrompt, "MINOR_DAMAGE")
	assert.Contains(t, systemPrompt, "MAJOR_DAMAGE")
	assert.Contains(t, systemPrompt, "MISSING_ITEM")
}
