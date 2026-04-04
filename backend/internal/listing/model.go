// Package listing implements the RentMy listing domain: creation, retrieval, and media attachment.
package listing

import (
	"encoding/json"
	"fmt"
	"time"
)

// ListingStatus represents the lifecycle state of a listing.
type ListingStatus string

const (
	ListingStatusPending   ListingStatus = "PENDING"
	ListingStatusActive    ListingStatus = "ACTIVE"
	ListingStatusFlagged   ListingStatus = "FLAGGED"
	ListingStatusSuspended ListingStatus = "SUSPENDED"
)

// MaxAllowedDuration is the 7-day ceiling enforced on listing max_duration.
const MaxAllowedDuration = 7 * 24 * time.Hour

// Duration is a time.Duration that marshals to/from a Go duration string (e.g. "168h").
type Duration time.Duration

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func (d *Duration) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("duration must be a string: %w", err)
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	*d = Duration(dur)
	return nil
}

// Location is a geographic coordinate pair.
type Location struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// Listing is the domain representation of a RentMy item listing.
type Listing struct {
	ID                 string          `json:"id"`
	HostID             string          `json:"hostId"`
	Title              string          `json:"title"`
	Description        string          `json:"description"`
	AIGeneratedTags    json.RawMessage `json:"aiGeneratedTags"`
	EstimatedValue     *float64        `json:"estimatedValue,omitempty"`
	HostDeclaredValue  *float64        `json:"hostDeclaredValue,omitempty"`
	ValueJustification *string         `json:"valueJustification,omitempty"`
	PricePerHour       *float64        `json:"pricePerHour,omitempty"`
	PricePerDay        *float64        `json:"pricePerDay,omitempty"`
	MinDuration        *Duration       `json:"minDuration,omitempty"`
	MaxDuration        *Duration       `json:"maxDuration,omitempty"`
	Location           *Location       `json:"location,omitempty"`
	Availability       json.RawMessage `json:"availability"`
	HasVideo           bool            `json:"hasVideo"`
	Status             ListingStatus   `json:"status"`
	CreatedAt          time.Time       `json:"createdAt"`
}

// CreateListingInput is the request body for POST /api/v1/listings.
type CreateListingInput struct {
	Title             string          `json:"title"             validate:"required,min=1,max=200"`
	Description       string          `json:"description"       validate:"omitempty,max=2000"`
	PricePerHour      *float64        `json:"pricePerHour"      validate:"omitempty,gt=0"`
	PricePerDay       *float64        `json:"pricePerDay"       validate:"omitempty,gt=0"`
	MinDuration       *Duration       `json:"minDuration"`
	MaxDuration       *Duration       `json:"maxDuration"`
	Location          *Location       `json:"location"`
	Availability      json.RawMessage `json:"availability"`
	HostDeclaredValue *float64        `json:"hostDeclaredValue" validate:"omitempty,gt=0"`
}

// UpdateListingInput is the request body for PUT /api/v1/listings/:id.
type UpdateListingInput struct {
	Title        *string         `json:"title"        validate:"omitempty,min=1,max=200"`
	Description  *string         `json:"description"  validate:"omitempty,max=2000"`
	PricePerHour *float64        `json:"pricePerHour" validate:"omitempty,gt=0"`
	PricePerDay  *float64        `json:"pricePerDay"  validate:"omitempty,gt=0"`
	MinDuration  *Duration       `json:"minDuration"`
	MaxDuration  *Duration       `json:"maxDuration"`
	Availability json.RawMessage `json:"availability"`
}

// AttachMediaInput is the request body for POST /api/v1/listings/:id/media.
type AttachMediaInput struct {
	MediaIDs []string `json:"mediaIds" validate:"required,min=1,dive,required"`
}

// ListByHostResult is the paginated response for GET /api/v1/users/me/listings.
type ListByHostResult struct {
	Listings []*Listing `json:"listings"`
	Total    int        `json:"total"`
	Page     int        `json:"page"`
}
