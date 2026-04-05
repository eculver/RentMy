package listing

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-playground/validator/v10"

	"github.com/giits/rentmy/backend/internal/platform/ulid"
)

// RepositoryInterface declares the persistence operations required by Service.
type RepositoryInterface interface {
	Insert(ctx context.Context, l *Listing) (*Listing, error)
	FindByID(ctx context.Context, id string) (*Listing, error)
	FindByHostID(ctx context.Context, hostID string, page, limit int) ([]*Listing, int, error)
	Update(ctx context.Context, id string, in UpdateListingInput) (*Listing, error)
	AttachMedia(ctx context.Context, listingID string, mediaIDs []string) error
	UpdateAppraisalFields(ctx context.Context, id string, in AppraisalFieldsUpdate) error
}

// AppraisalEnqueuer can asynchronously trigger AI appraisal for a listing.
// Implemented by the appraisal agent service; injected via WithAppraisalEnqueuer.
type AppraisalEnqueuer interface {
	EnqueueAppraisal(ctx context.Context, listingID string) error
}

// ErrDurationExceedsLimit is returned when the requested max duration exceeds the 7-day ceiling.
var ErrDurationExceedsLimit = fmt.Errorf("max_duration exceeds the 7-day ceiling (%s)", MaxAllowedDuration)

// ErrNotOwner is returned when a user attempts to modify a listing they do not own.
var ErrNotOwner = fmt.Errorf("not the listing owner")

var validate = validator.New()

// Service implements listing business logic.
type Service struct {
	repo             RepositoryInterface
	appraisalEnqueue AppraisalEnqueuer // optional; nil until wired in main
}

// NewService constructs a Service backed by the concrete Repository.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// NewServiceWithInterface constructs a Service with an interface-typed repository,
// useful for unit testing with fakes.
func NewServiceWithInterface(repo RepositoryInterface) *Service {
	return &Service{repo: repo}
}

// WithAppraisalEnqueuer sets the appraisal enqueuer after construction.
// This breaks a potential circular init dependency (listing ↔ appraisal).
func (s *Service) WithAppraisalEnqueuer(e AppraisalEnqueuer) {
	s.appraisalEnqueue = e
}

// UpdateAppraisalResult persists AI-generated fields back onto a listing.
func (s *Service) UpdateAppraisalResult(ctx context.Context, listingID string, in AppraisalFieldsUpdate) error {
	if err := s.repo.UpdateAppraisalFields(ctx, listingID, in); err != nil {
		return fmt.Errorf("update appraisal result: %w", err)
	}
	return nil
}

// Create validates input, enforces the 7-day ceiling, and persists a new listing
// with status PENDING.
func (s *Service) Create(ctx context.Context, hostID string, in CreateListingInput) (*Listing, error) {
	if err := validate.Struct(in); err != nil {
		return nil, err
	}

	if in.MaxDuration != nil && time.Duration(*in.MaxDuration) > MaxAllowedDuration {
		return nil, ErrDurationExceedsLimit
	}

	l := &Listing{
		ID:                ulid.New(),
		HostID:            hostID,
		Title:             in.Title,
		Description:       in.Description,
		PricePerHour:      in.PricePerHour,
		PricePerDay:       in.PricePerDay,
		MinDuration:       in.MinDuration,
		MaxDuration:       in.MaxDuration,
		Location:          in.Location,
		Availability:      in.Availability,
		HostDeclaredValue: in.HostDeclaredValue,
		Status:            ListingStatusPending,
	}

	created, err := s.repo.Insert(ctx, l)
	if err != nil {
		return nil, fmt.Errorf("create listing: %w", err)
	}

	// Enqueue AI appraisal asynchronously. Failure here is non-fatal:
	// the listing was created successfully, and appraisal can be re-triggered.
	if s.appraisalEnqueue != nil {
		if err := s.appraisalEnqueue.EnqueueAppraisal(ctx, created.ID); err != nil {
			slog.Warn("listing: failed to enqueue appraisal job", "listingId", created.ID, "error", err)
		}
	}

	return created, nil
}

// Get retrieves a single listing by ID.
func (s *Service) Get(ctx context.Context, id string) (*Listing, error) {
	l, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get listing: %w", err)
	}
	return l, nil
}

// ListByHost returns a paginated list of listings for the given host.
func (s *Service) ListByHost(ctx context.Context, hostID string, page, limit int) (*ListByHostResult, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	listings, total, err := s.repo.FindByHostID(ctx, hostID, page, limit)
	if err != nil {
		return nil, fmt.Errorf("list listings by host: %w", err)
	}
	return &ListByHostResult{
		Listings: listings,
		Total:    total,
		Page:     page,
	}, nil
}

// Update applies a partial update to a listing. Returns ErrNotOwner if the
// requestor is not the listing's host.
func (s *Service) Update(ctx context.Context, listingID, requestorID string, in UpdateListingInput) (*Listing, error) {
	if err := validate.Struct(in); err != nil {
		return nil, err
	}

	existing, err := s.repo.FindByID(ctx, listingID)
	if err != nil {
		return nil, fmt.Errorf("update listing: %w", err)
	}
	if existing.HostID != requestorID {
		return nil, ErrNotOwner
	}

	if in.MaxDuration != nil && time.Duration(*in.MaxDuration) > MaxAllowedDuration {
		return nil, ErrDurationExceedsLimit
	}

	updated, err := s.repo.Update(ctx, listingID, in)
	if err != nil {
		return nil, fmt.Errorf("update listing: %w", err)
	}
	return updated, nil
}

// AttachMedia links the given media records to a listing. Returns ErrNotOwner
// if the requestor is not the listing's host.
func (s *Service) AttachMedia(ctx context.Context, listingID, requestorID string, in AttachMediaInput) (*Listing, error) {
	if err := validate.Struct(in); err != nil {
		return nil, err
	}

	existing, err := s.repo.FindByID(ctx, listingID)
	if err != nil {
		return nil, fmt.Errorf("attach media: %w", err)
	}
	if existing.HostID != requestorID {
		return nil, ErrNotOwner
	}

	if err := s.repo.AttachMedia(ctx, listingID, in.MediaIDs); err != nil {
		return nil, fmt.Errorf("attach media: %w", err)
	}

	updated, err := s.repo.FindByID(ctx, listingID)
	if err != nil {
		return nil, fmt.Errorf("reload listing after media attach: %w", err)
	}
	return updated, nil
}
