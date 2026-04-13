package appraisal

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Brett2thered/RentMy/backend/internal/listing"
)

// ErrNotFound is returned when no appraisal record exists for a listing.
var ErrNotFound = errors.New("appraisal not found")

// Repository handles Postgres persistence for the appraisal domain.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a Repository backed by the given connection pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Insert persists a new (PENDING) appraisal row for a listing.
func (r *Repository) Insert(ctx context.Context, a *Appraisal) (*Appraisal, error) {
	const q = `
		INSERT INTO appraisals (id, listing_id, status, tags, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		RETURNING created_at, updated_at`

	err := r.pool.QueryRow(ctx, q,
		a.ID, a.ListingID, string(a.Status), tagsBytes(a.Tags),
	).Scan(&a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert appraisal: %w", err)
	}
	return a, nil
}

// FindByListingID retrieves the appraisal for a given listing.
// Returns ErrNotFound when no record exists.
func (r *Repository) FindByListingID(ctx context.Context, listingID string) (*Appraisal, error) {
	const q = `
		SELECT id, listing_id, status,
		       item_name, category, condition,
		       estimated_value_cents, suggested_price_per_hour_cents, suggested_price_per_day_cents,
		       description, tags, confidence, model, prompt_version,
		       override_approved, override_reasoning, failure_reason,
		       created_at, updated_at
		FROM appraisals
		WHERE listing_id = $1`

	a, err := scanAppraisal(r.pool.QueryRow(ctx, q, listingID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find appraisal by listing id: %w", err)
	}
	return a, nil
}

// updateInput carries fields for a completed or failed appraisal.
type updateInput struct {
	Status                     listing.AppraisalStatus
	ItemName                   *string
	Category                   *string
	Condition                  *string
	EstimatedValueCents        *int
	SuggestedPricePerHourCents *int
	SuggestedPricePerDayCents  *int
	Description                *string
	Tags                       []byte
	Confidence                 *float64
	Model                      *string
	PromptVersion              *string
	FailureReason              *string
}

// Update writes the result of a completed or failed appraisal run.
func (r *Repository) Update(ctx context.Context, id string, in updateInput) error {
	const q = `
		UPDATE appraisals SET
			status                          = $2,
			item_name                       = $3,
			category                        = $4,
			condition                       = $5,
			estimated_value_cents           = $6,
			suggested_price_per_hour_cents  = $7,
			suggested_price_per_day_cents   = $8,
			description                     = $9,
			tags                            = $10,
			confidence                      = $11,
			model                           = $12,
			prompt_version                  = $13,
			failure_reason                  = $14,
			updated_at                      = NOW()
		WHERE id = $1`

	_, err := r.pool.Exec(ctx, q,
		id,
		string(in.Status),
		in.ItemName, in.Category, in.Condition,
		in.EstimatedValueCents, in.SuggestedPricePerHourCents, in.SuggestedPricePerDayCents,
		in.Description, tagsBytes(in.Tags),
		in.Confidence, in.Model, in.PromptVersion,
		in.FailureReason,
	)
	if err != nil {
		return fmt.Errorf("update appraisal: %w", err)
	}
	return nil
}

// updateOverrideInput carries the result of an override review.
type updateOverrideInput struct {
	OverrideApproved  bool
	OverrideReasoning string
}

// UpdateOverride records the AI decision for a host-declared value override.
func (r *Repository) UpdateOverride(ctx context.Context, id string, in updateOverrideInput) error {
	const q = `
		UPDATE appraisals SET
			override_approved  = $2,
			override_reasoning = $3,
			updated_at         = NOW()
		WHERE id = $1`

	_, err := r.pool.Exec(ctx, q, id, in.OverrideApproved, in.OverrideReasoning)
	if err != nil {
		return fmt.Errorf("update appraisal override: %w", err)
	}
	return nil
}

func scanAppraisal(row pgx.Row) (*Appraisal, error) {
	var (
		a      Appraisal
		status string
		tags   []byte
	)
	err := row.Scan(
		&a.ID, &a.ListingID, &status,
		&a.ItemName, &a.Category, &a.Condition,
		&a.EstimatedValueCents, &a.SuggestedPricePerHourCents, &a.SuggestedPricePerDayCents,
		&a.Description, &tags, &a.Confidence,
		&a.Model, &a.PromptVersion,
		&a.OverrideApproved, &a.OverrideReasoning, &a.FailureReason,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	a.Status = listing.AppraisalStatus(status)
	a.Tags = jsonOrDefault(tags, "[]")
	return &a, nil
}

func tagsBytes(raw []byte) []byte {
	if raw == nil {
		return []byte("[]")
	}
	return raw
}

func jsonOrDefault(raw []byte, fallback string) []byte {
	if raw == nil {
		return []byte(fallback)
	}
	return raw
}
