package booking

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Brett2thered/RentMy/backend/internal/latereturn"
	"github.com/Brett2thered/RentMy/backend/internal/messaging"
	"github.com/Brett2thered/RentMy/backend/internal/notification"
	"github.com/Brett2thered/RentMy/backend/internal/outcome"
	"github.com/Brett2thered/RentMy/backend/internal/payment"
	"github.com/Brett2thered/RentMy/backend/internal/proximity"

	risk "github.com/Brett2thered/RentMy/backend/internal/agent/risk"
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
	// PickupReminderBefore is how far before scheduled_start to fire the pickup approaching notification.
	PickupReminderBefore time.Duration
	// ReturnReminderBefore is how far before scheduled_end to fire the return approaching notification.
	ReturnReminderBefore time.Duration
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

// notificationSvc is the interface the booking service uses to dispatch
// notifications without a direct import cycle.
type notificationSvc interface {
	Notify(ctx context.Context, userID string, t notification.Type, title, body string, data map[string]string) error
}

// pusherSvc is the interface the booking service uses to publish real-time
// booking status events on transaction channels.
type pusherSvc interface {
	Trigger(channel, event string, data interface{}) error
}

// agreementSvc is the subset of agreement.Service the booking service needs.
// Using an interface avoids an import cycle and allows test injection.
// The booking service only cares about triggering generation (fire-and-forget best-effort),
// so only the error return matters.
type agreementSvc interface {
	TriggerAgreement(ctx context.Context, transactionID string) error
}

// fraudSvc is the subset of fraud.Agent the booking service uses for
// post-transaction signal evaluation.  Nil-safe: when nil, fraud checks are skipped.
type fraudSvc interface {
	EvaluateTransaction(ctx context.Context, transactionID string) error
}

// referralSvc is the subset of referral.Service the booking service calls on
// checkout to trigger first-rental payout evaluation. Nil-safe.
type referralSvc interface {
	OnFirstRentalCompleted(ctx context.Context, userID string)
}

// Service implements the booking domain business logic.
type Service struct {
	repo            *Repository
	paymentSvc      *payment.Service
	riverClient     riverInserter
	proximitySvc    proximitySvc
	notificationSvc notificationSvc
	pusherSvc       pusherSvc
	riskSvc         *risk.Service // nil-safe: when nil, risk checks are skipped
	agreementSvc    agreementSvc  // nil-safe: when nil, agreement generation is skipped
	fraudSvc        fraudSvc      // nil-safe: when nil, fraud evaluation is skipped
	referralSvc     referralSvc   // nil-safe: when nil, referral hook is skipped
	cfg             Config
}

// NewService creates a Service with the given dependencies and config.
func NewService(repo *Repository, paymentSvc *payment.Service, riverClient riverInserter, proximitySvc proximitySvc, notificationSvc notificationSvc, cfg Config) *Service {
	return &Service{
		repo:            repo,
		paymentSvc:      paymentSvc,
		riverClient:     riverClient,
		proximitySvc:    proximitySvc,
		notificationSvc: notificationSvc,
		cfg:             cfg,
	}
}

// WithRiskAgent attaches a RiskAgent service for per-transaction risk scoring.
func (s *Service) WithRiskAgent(r *risk.Service) *Service {
	s.riskSvc = r
	return s
}

// WithAgreementAgent attaches an AgreementAgent service for post-acceptance agreement generation.
func (s *Service) WithAgreementAgent(a agreementSvc) *Service {
	s.agreementSvc = a
	return s
}

// WithReferralService attaches a referral service to receive first-rental hooks.
func (s *Service) WithReferralService(r referralSvc) *Service {
	s.referralSvc = r
	return s
}

// WithFraudAgent attaches a FraudAgent for post-transaction signal evaluation.
func (s *Service) WithFraudAgent(f fraudSvc) *Service {
	s.fraudSvc = f
	return s
}

// WithPusher attaches a Pusher client so the service can fire booking-status-changed
// events on transaction channels. Call this after constructing the service.
func (s *Service) WithPusher(p pusherSvc) *Service {
	s.pusherSvc = p
	return s
}

// triggerStatusChanged fires a Pusher booking-status-changed event on the
// transaction channel. Failures are logged and never returned to the caller.
func (s *Service) triggerStatusChanged(ctx context.Context, transactionID string, status Status) {
	if s.pusherSvc == nil {
		return
	}
	channel := messaging.TransactionChannel(transactionID)
	data := map[string]string{"transactionId": transactionID, "status": string(status)}
	if err := s.pusherSvc.Trigger(channel, messaging.EventBookingStatusChanged, data); err != nil {
		slog.WarnContext(ctx, "booking: pusher status change failed",
			"transactionId", transactionID, "status", status, "error", err)
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

	// Run RiskAgent scoring. Block the booking if the score is >= 71 (CRITICAL).
	// The TransactionID is not yet known at this point; we pass empty string and
	// the risk service will skip DB persistence (score is re-computed after booking
	// creation if needed). The renter/host IDs and listing value are enough for
	// the deterministic rules engine.
	if s.riskSvc != nil {
		riskResult, riskErr := s.riskSvc.ComputeRiskScore(ctx, risk.ComputeRiskInput{
			RenterID: in.RenterID,
			HostID:   hostID,
		})
		if riskErr != nil {
			// Log and continue: a risk scoring failure must not block a booking.
			slog.Warn("booking: risk scoring failed, proceeding without score",
				"renterId", in.RenterID, "hostId", hostID, "error", riskErr)
		} else if riskResult.Control == risk.ControlBlock {
			return payment.BookingResult{}, fmt.Errorf("%w: risk score %d (CRITICAL)",
				ErrFraudBlocked, riskResult.RiskScore)
		}
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
		slog.Warn("failed to schedule auto-decline job", "transactionId", result.TransactionID, "error", err)
	}

	// Run FraudAgent signal evaluation for both renter and host.
	if s.fraudSvc != nil {
		if err := s.fraudSvc.EvaluateTransaction(ctx, result.TransactionID); err != nil {
			// Log and continue: fraud evaluation failure must not block a booking.
			slog.Warn("booking: fraud evaluation failed", "transactionId", result.TransactionID, "error", err)
		}
	}

	// Notify the host of the new booking request.
	if s.notificationSvc != nil {
		if err := s.notificationSvc.Notify(ctx, hostID, notification.TypeBookingRequest,
			"New booking request",
			"Someone wants to rent your item. Check the app to accept or decline.",
			map[string]string{"transactionId": result.TransactionID},
		); err != nil {
			slog.Warn("failed to send booking request notification", "hostId", hostID, "error", err)
		}
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

	// Notify real-time subscribers of the status change (best-effort).
	s.triggerStatusChanged(ctx, in.BookingID, StatusAccepted)

	// Generate the rental agreement now that the host has accepted (best-effort).
	// Both parties will need to accept the agreement before the booking goes ACTIVE.
	if s.agreementSvc != nil {
		if err := s.agreementSvc.TriggerAgreement(ctx, in.BookingID); err != nil {
			slog.Warn("failed to generate agreement after acceptance",
				"bookingId", in.BookingID, "error", err)
		}
	}

	// Generate and store the check-in PIN for the proximity handoff.
	// This is best-effort: if it fails the booking is still accepted and the
	// host can trigger the SMS fallback endpoint to regenerate.
	if _, err := s.proximitySvc.GenerateCheckInPIN(ctx, in.BookingID, in.HostID); err != nil {
		// Log and continue — booking acceptance is already committed.
		slog.Warn("failed to generate check-in PIN", "bookingId", in.BookingID, "error", err)
	}

	// Notify the renter and schedule pickup/return reminders (best-effort).
	if s.notificationSvc != nil {
		data := map[string]string{"transactionId": in.BookingID}
		if err := s.notificationSvc.Notify(ctx, booking.RenterID, notification.TypeBookingAccepted,
			"Booking accepted",
			"Your booking request has been accepted. Get ready for pickup!",
			data,
		); err != nil {
			slog.Warn("failed to send booking accepted notification", "renterId", booking.RenterID, "error", err)
		}
	}

	if s.riverClient != nil && !booking.ScheduledStart.IsZero() && !booking.ScheduledEnd.IsZero() {
		if err := notification.SchedulePickupApproaching(ctx, s.riverClient, in.BookingID,
			booking.RenterID, booking.HostID, booking.ScheduledStart, s.cfg.PickupReminderBefore,
		); err != nil {
			slog.Warn("failed to schedule pickup reminder", "bookingId", in.BookingID, "error", err)
		}
		if err := notification.ScheduleReturnApproaching(ctx, s.riverClient, in.BookingID,
			booking.RenterID, booking.ScheduledEnd, s.cfg.ReturnReminderBefore,
		); err != nil {
			slog.Warn("failed to schedule return reminder", "bookingId", in.BookingID, "error", err)
		}

		// Schedule late return check at scheduled_end — if the rental is still ACTIVE
		// at that point, LateReturnAgent will begin charging late fees.
		if err := latereturn.ScheduleLateReturnCheck(ctx, s.riverClient, in.BookingID, booking.ScheduledEnd); err != nil {
			slog.Warn("failed to schedule late return check", "bookingId", in.BookingID, "error", err)
		}
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
	if err := dbTx.Commit(ctx); err != nil {
		return err
	}
	s.triggerStatusChanged(ctx, bookingID, StatusActive)
	return nil
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
	if err := dbTx.Commit(ctx); err != nil {
		return err
	}
	s.triggerStatusChanged(ctx, bookingID, StatusCompleted)

	// Schedule outcome linking job 48h from now for agent learning framework.
	if s.riverClient != nil {
		if err := outcome.ScheduleOutcomeLink(ctx, s.riverClient, bookingID); err != nil {
			slog.Warn("booking: failed to schedule outcome link",
				"bookingId", bookingID, "error", err)
		}
	}

	// Trigger referral first-rental hook for the renter (fire-and-forget).
	if s.referralSvc != nil {
		s.referralSvc.OnFirstRentalCompleted(ctx, booking.RenterID)
	}

	return nil
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

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	s.triggerStatusChanged(ctx, in.BookingID, StatusDeclined)
	return nil
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

	if err := dbTx.Commit(ctx); err != nil {
		return err
	}

	// Notify real-time subscribers of the status change (best-effort).
	s.triggerStatusChanged(ctx, in.BookingID, StatusCancelled)

	// Notify the other party about the cancellation (best-effort).
	if s.notificationSvc != nil {
		var recipientID string
		switch role {
		case CancellerRenter:
			recipientID = booking.HostID
		case CancellerHost:
			recipientID = booking.RenterID
		}
		if recipientID != "" {
			if err := s.notificationSvc.Notify(ctx, recipientID, notification.TypeCancellation,
				"Booking cancelled",
				"The other party has cancelled the booking.",
				map[string]string{"transactionId": in.BookingID},
			); err != nil {
				slog.Warn("failed to send cancellation notification", "recipientId", recipientID, "error", err)
			}
		}
	}
	return nil
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
