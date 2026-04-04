package booking

import (
	"context"
	"fmt"
	"time"

	"github.com/giits/rentmy/backend/internal/payment"
	"github.com/giits/rentmy/backend/internal/proximity"
)

// Config holds tunable parameters for the booking service.
type Config struct {
	// AutoDeclineTimeout is how long to wait for a host response before auto-declining.
	AutoDeclineTimeout time.Duration
	// FraudNewAccountDays is the minimum account age for both-party lockout rule.
	FraudNewAccountDays int
	// FraudFirstNTransactions is the early-transaction payout delay threshold.
	FraudFirstNTransactions int
	// FraudPayoutDelay is the delay applied to first-N transactions.
	FraudPayoutDelay time.Duration
	// FraudDamageClaimCapCents is the max damage claimed for a host within the cap window.
	FraudDamageClaimCapCents int64
	// FraudDamageClaimWindowDays is the number of days from account creation in which the cap applies.
	FraudDamageClaimWindowDays int
	// HostCancelLateBPS is the cancellation fee BPS for hosts cancelling < 2h before pickup.
	HostCancelLateBPS int
	// HostCancelVeryLateBPS is the cancellation fee BPS for hosts cancelling after scheduled start.
	HostCancelVeryLateBPS int
}

// autoDeclineAt returns the timestamp when the auto-decline job should fire.
func (c Config) autoDeclineAt() time.Time {
	return time.Now().Add(c.AutoDeclineTimeout)
}

// proximitySvc is the interface the booking service uses to interact with
// the proximity domain.  Using an interface breaks the import cycle and
// allows tests to inject a stub.
type proximitySvc interface {
	GenerateCheckInPIN(ctx context.Context, transactionID, hostID string) (string, error)
	CheckHandoffComplete(ctx context.Context, transactionID string, proofType proximity.ProofType) (bool, error)
}

// Service implements the booking domain business logic.
type Service struct {
	repo         *Repository
	paymentSvc   *payment.Service
	riverClient  riverInserter
	proximitySvc proximitySvc
	cfg          Config
}

// NewService creates a Service with the given dependencies and config.
func NewService(repo *Repository, paymentSvc *payment.Service, riverClient riverInserter, proximitySvc proximitySvc, cfg Config) *Service {
	return &Service{
		repo:         repo,
		paymentSvc:   paymentSvc,
		riverClient:  riverClient,
		proximitySvc: proximitySvc,
		cfg:          cfg,
	}
}

// CreateBooking validates the request, runs fraud checks, delegates payment to
// the payment service, and schedules the auto-decline River job.
func (s *Service) CreateBooking(ctx context.Context, in CreateInput) (payment.BookingResult, error) {
	// Enforce 7-day ceiling (PRD section 7).
	duration := in.ScheduledEnd.Sub(in.ScheduledStart)
	if duration > MaxDuration {
		return payment.BookingResult{}, ErrDurationExceedsLimit
	}
	if duration <= 0 {
		return payment.BookingResult{}, fmt.Errorf("scheduledEnd must be after scheduledStart")
	}

	// Check listing availability (no overlapping active bookings).
	available, err := s.repo.IsListingAvailable(ctx, in.ListingID, in.ScheduledStart, in.ScheduledEnd, "")
	if err != nil {
		return payment.BookingResult{}, fmt.Errorf("check availability: %w", err)
	}
	if !available {
		return payment.BookingResult{}, ErrListingNotAvailable
	}

	// Run fraud velocity checks. We need the host ID — fetch listing via payment service.
	hostID, err := s.getListingHostID(ctx, in.ListingID)
	if err != nil {
		return payment.BookingResult{}, fmt.Errorf("get listing host: %w", err)
	}

	fraudResult, err := CheckFraudVelocity(ctx, s.repo, in.RenterID, hostID, FraudVelocityConfig{
		NewAccountThresholdDays:    s.cfg.FraudNewAccountDays,
		FirstNTransactions:         s.cfg.FraudFirstNTransactions,
		PayoutDelay:                s.cfg.FraudPayoutDelay,
		DamageClaimCapCents:        s.cfg.FraudDamageClaimCapCents,
		DamageClaimWindowDays:      s.cfg.FraudDamageClaimWindowDays,
	})
	if err != nil {
		return payment.BookingResult{}, fmt.Errorf("fraud check: %w", err)
	}
	if fraudResult.Blocked {
		return payment.BookingResult{}, fmt.Errorf("%w: %s", ErrFraudBlocked, fraudResult.BlockReason)
	}

	// Delegate payment processing to the payment service.
	result, err := s.paymentSvc.CreateBooking(ctx, payment.BookingInput{
		RenterID:        in.RenterID,
		ListingID:       in.ListingID,
		PaymentMethodID: in.PaymentMethodID,
		ScheduledStart:  in.ScheduledStart,
		ScheduledEnd:    in.ScheduledEnd,
	})
	if err != nil {
		return payment.BookingResult{}, fmt.Errorf("create payment booking: %w", err)
	}

	// Schedule auto-decline River job.
	if err := scheduleAutoDecline(ctx, s.riverClient, result.TransactionID, s.cfg); err != nil {
		// Log and continue: auto-decline failure is not booking-critical.
		// The booking is already created and paid; a manual process can handle stale requests.
		_ = err // TODO: add structured logging here when NotificationService is available
	}

	return result, nil
}

// Accept transitions a booking from REQUESTED to ACCEPTED and generates the
// check-in PIN that the host will display to the renter at handoff.
// Only the listing's host is authorized to accept.
func (s *Service) Accept(ctx context.Context, in AcceptInput) error {
	booking, err := s.repo.FindByID(ctx, in.BookingID)
	if err != nil {
		return err
	}

	if booking.HostID != in.HostID {
		return ErrNotAuthorized
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := s.repo.UpdateStatus(ctx, tx, in.BookingID, StatusAccepted); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit accept: %w", err)
	}

	// Generate and store the check-in PIN for the proximity handoff.
	// This is best-effort: if it fails the booking is still accepted and the
	// host can trigger the SMS fallback endpoint to regenerate.
	if _, err := s.proximitySvc.GenerateCheckInPIN(ctx, in.BookingID, in.HostID); err != nil {
		// Log and continue — booking acceptance is already committed.
		_ = err
	}

	return nil
}

// CheckIn transitions an ACCEPTED booking to ACTIVE once both parties have
// completed GPS + PIN proximity verification (PRD §17: ACCEPTED → ACTIVE).
func (s *Service) CheckIn(ctx context.Context, bookingID, requesterID string) error {
	booking, err := s.repo.FindByID(ctx, bookingID)
	if err != nil {
		return err
	}

	if booking.RenterID != requesterID && booking.HostID != requesterID {
		return ErrNotAuthorized
	}

	complete, err := s.proximitySvc.CheckHandoffComplete(ctx, bookingID, proximity.ProofTypeCheckIn)
	if err != nil {
		return fmt.Errorf("check handoff complete: %w", err)
	}
	if !complete {
		return ErrHandoffIncomplete
	}

	dbTx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = dbTx.Rollback(ctx) }()

	if err := s.repo.UpdateStatus(ctx, dbTx, bookingID, StatusActive); err != nil {
		return err
	}
	return dbTx.Commit(ctx)
}

// CheckOut transitions an ACTIVE booking to COMPLETED once both parties have
// completed GPS proximity verification for the return handoff (PRD §17: ACTIVE → COMPLETED).
func (s *Service) CheckOut(ctx context.Context, bookingID, requesterID string) error {
	booking, err := s.repo.FindByID(ctx, bookingID)
	if err != nil {
		return err
	}

	if booking.RenterID != requesterID && booking.HostID != requesterID {
		return ErrNotAuthorized
	}

	complete, err := s.proximitySvc.CheckHandoffComplete(ctx, bookingID, proximity.ProofTypeCheckOut)
	if err != nil {
		return fmt.Errorf("check handoff complete: %w", err)
	}
	if !complete {
		return ErrHandoffIncomplete
	}

	dbTx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = dbTx.Rollback(ctx) }()

	if err := s.repo.UpdateStatus(ctx, dbTx, bookingID, StatusCompleted); err != nil {
		return err
	}
	return dbTx.Commit(ctx)
}

// Decline transitions a booking from REQUESTED to DECLINED.
// Only the listing's host is authorized to decline.
func (s *Service) Decline(ctx context.Context, in DeclineInput) error {
	booking, err := s.repo.FindByID(ctx, in.BookingID)
	if err != nil {
		return err
	}

	if booking.HostID != in.HostID {
		return ErrNotAuthorized
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := s.repo.UpdateStatus(ctx, tx, in.BookingID, StatusDeclined); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// Cancel transitions a booking to CANCELLED for either the renter or the host,
// calculating and recording any applicable cancellation fee.
func (s *Service) Cancel(ctx context.Context, in CancelInput) error {
	booking, err := s.repo.FindByID(ctx, in.BookingID)
	if err != nil {
		return err
	}

	var role CancellerRole
	switch in.CancellerID {
	case booking.RenterID:
		role = CancellerRenter
	case booking.HostID:
		role = CancellerHost
	default:
		return ErrNotAuthorized
	}

	// Get the rental fee for cancellation fee calculation.
	txn, err := s.paymentSvc.GetTransaction(ctx, in.BookingID)
	if err != nil {
		return fmt.Errorf("get transaction: %w", err)
	}

	hostCfg := HostCancelConfig{
		LateFeeBPS:     s.cfg.HostCancelLateBPS,
		VeryLateFeeBPS: s.cfg.HostCancelVeryLateBPS,
	}
	fee := CalculateCancellationFee(role, txn.RentalFee, booking.ScheduledStart, time.Now(), hostCfg)

	dbTx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = dbTx.Rollback(ctx) }()

	if err := s.repo.UpdateCancellation(ctx, dbTx, in.BookingID, StatusCancelled, role, fee); err != nil {
		return err
	}

	return dbTx.Commit(ctx)
}

// GetBooking returns a booking by ID. Either the renter or host may retrieve it.
func (s *Service) GetBooking(ctx context.Context, bookingID, requesterID string) (Booking, error) {
	booking, err := s.repo.FindByID(ctx, bookingID)
	if err != nil {
		return Booking{}, err
	}
	if booking.RenterID != requesterID && booking.HostID != requesterID {
		return Booking{}, ErrNotAuthorized
	}
	return booking, nil
}

// ListByRenter returns paginated bookings for a renter.
func (s *Service) ListByRenter(ctx context.Context, renterID string, limit, offset int) ([]Booking, int, error) {
	return s.repo.FindByRenterID(ctx, renterID, limit, offset)
}

// ListByHost returns paginated bookings for a host.
func (s *Service) ListByHost(ctx context.Context, hostID string, limit, offset int) ([]Booking, int, error) {
	return s.repo.FindByHostID(ctx, hostID, limit, offset)
}

// getListingHostID fetches the host_id for an active listing.
func (s *Service) getListingHostID(ctx context.Context, listingID string) (string, error) {
	return s.repo.GetListingHostID(ctx, listingID)
}
