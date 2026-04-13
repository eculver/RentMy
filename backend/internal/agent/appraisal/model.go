// Package appraisal implements the AppraisalAgent: AI-powered item identification,
// pricing suggestions, and host-declared value override review.
package appraisal

import (
	"encoding/json"
	"time"

	"github.com/Brett2thered/RentMy/backend/internal/listing"
)

// Appraisal is the stored appraisal record for a listing.
type Appraisal struct {
	ID                          string
	ListingID                   string
	Status                      listing.AppraisalStatus
	ItemName                    *string
	Category                    *string
	Condition                   *string
	EstimatedValueCents         *int
	SuggestedPricePerHourCents  *int
	SuggestedPricePerDayCents   *int
	Description                 *string
	Tags                        json.RawMessage
	Confidence                  *float64
	Model                       *string
	PromptVersion               *string
	OverrideApproved            *bool
	OverrideReasoning           *string
	FailureReason               *string
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
}

// appraisalPromptData is the template data for the main appraisal prompt.
type appraisalPromptData struct {
	Notes string
}

// appraisalAIResult is the structured JSON response from the AI model.
type appraisalAIResult struct {
	ItemName                   string   `json:"item_name"`
	Category                   string   `json:"category"`
	Condition                  string   `json:"condition"`
	EstimatedValueCents        int      `json:"estimated_value_usd"`
	SuggestedPricePerHourCents int      `json:"suggested_price_per_hour_usd"`
	SuggestedPricePerDayCents  int      `json:"suggested_price_per_day_usd"`
	Description                string   `json:"description"`
	Tags                       []string `json:"tags"`
	Confidence                 float64  `json:"confidence"`
}

// overridePromptData is the template data for the override review prompt.
type overridePromptData struct {
	ItemName          string
	Category          string
	AIEstimateCents   int
	HostDeclaredCents int
	Justification     string
}

// overrideAIResult is the structured JSON response from the override review model.
type overrideAIResult struct {
	Approved   bool    `json:"approved"`
	Reasoning  string  `json:"reasoning"`
	Confidence float64 `json:"confidence"`
}

// OverrideRequest is the request body for POST /api/v1/listings/:id/override.
type OverrideRequest struct {
	DeclaredValueCents int    `json:"declaredValueCents"`
	Justification      string `json:"justification"`
}

// AppraisalResponse is the JSON response for GET /api/v1/listings/:id/appraisal.
type AppraisalResponse struct {
	ID                         string                  `json:"id"`
	ListingID                  string                  `json:"listingId"`
	Status                     listing.AppraisalStatus `json:"status"`
	ItemName                   *string                 `json:"itemName,omitempty"`
	Category                   *string                 `json:"category,omitempty"`
	Condition                  *string                 `json:"condition,omitempty"`
	EstimatedValueCents        *int                    `json:"estimatedValueCents,omitempty"`
	SuggestedPricePerHourCents *int                    `json:"suggestedPricePerHourCents,omitempty"`
	SuggestedPricePerDayCents  *int                    `json:"suggestedPricePerDayCents,omitempty"`
	Description                *string                 `json:"description,omitempty"`
	Tags                       json.RawMessage         `json:"tags"`
	Confidence                 *float64                `json:"confidence,omitempty"`
	OverrideApproved           *bool                   `json:"overrideApproved,omitempty"`
	OverrideReasoning          *string                 `json:"overrideReasoning,omitempty"`
	FailureReason              *string                 `json:"failureReason,omitempty"`
}
