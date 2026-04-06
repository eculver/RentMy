// Package photodiff implements the two-stage photo diff pipeline for comparing
// check-in and check-out rental photos to detect damage.
package photodiff

import "time"

// DiffResult is the classification produced by the photo diff pipeline.
type DiffResult string

const (
	ResultNoChange        DiffResult = "NO_CHANGE"
	ResultCosmeticDamage  DiffResult = "COSMETIC_DAMAGE"
	ResultFunctionalDamage DiffResult = "FUNCTIONAL_DAMAGE"
	ResultMissingItem     DiffResult = "MISSING_ITEM"
	ResultInconclusive    DiffResult = "INCONCLUSIVE"
)

// ValidResults is the set of valid DiffResult values.
var ValidResults = map[DiffResult]bool{
	ResultNoChange:         true,
	ResultCosmeticDamage:   true,
	ResultFunctionalDamage: true,
	ResultMissingItem:      true,
	ResultInconclusive:     true,
}

// PhotoDiff holds the result of a photo diff analysis for a transaction.
type PhotoDiff struct {
	TransactionID string      `json:"transactionId"`
	Result        DiffResult  `json:"result"`
	Confidence    float64     `json:"confidence"`
	PairsCompared int         `json:"pairsCompared"`
	Details       string      `json:"details,omitempty"`
	PromptVersion string      `json:"promptVersion,omitempty"`
	Model         string      `json:"model,omitempty"`
	CreatedAt     time.Time   `json:"createdAt"`
}

// LLMComparisonResponse is the expected JSON structure from the LLM's
// structural comparison analysis.
type LLMComparisonResponse struct {
	Classification string  `json:"classification"`
	Confidence     float64 `json:"confidence"`
	Details        string  `json:"details"`
}
