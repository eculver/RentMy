package proximity

import (
	"context"
	"fmt"
	"time"

	"github.com/giits/rentmy/backend/internal/platform/ulid"
)

// Config holds tunable parameters for the proximity service.
type Config struct {
	// GPSThresholdMeters is the maximum distance (in meters) a user may be from
	// the listing location and still pass GPS verification. PRD §12: ≤100 m.
	GPSThresholdMeters float64
	// PINValidityDuration is how long a generated PIN remains valid.
	PINValidityDuration time.Duration
}

// SMSSender is satisfied by TwilioClient and any test stub that can deliver an SMS.
type SMSSender interface {
	SendSMS(ctx context.Context, to, body string) error
}

// Service implements proximity verification business logic.
type Service struct {
	repo *Repository
	sms  SMSSender
	cfg  Config
}

// NewService creates a Service with the given repository and optional SMS sender.
// Pass nil for sms to disable SMS fallback (useful in dev / test environments).
func NewService(repo *Repository, sms SMSSender, cfg Config) *Service {
	return &Service{repo: repo, sms: sms, cfg: cfg}
}

// GenerateCheckInPIN creates a host CHECK_IN proof record and generates a 4-digit
// PIN for the renter to enter during handoff. Called by BookingService.Accept.
func (s *Service) GenerateCheckInPIN(ctx context.Context, transactionID, hostID string) (string, error) {
	pin, err := GeneratePIN()
	if err != nil {
		return "", fmt.Errorf("generate check-in PIN: %w", err)
	}

	expiresAt := time.Now().Add(s.cfg.PINValidityDuration)
	proof := ProximityProof{
		ID:            ulid.New(),
		TransactionID: transactionID,
		UserID:        hostID,
		ProofType:     ProofTypeCheckIn,
		PIN:           pin,
		PINExpiresAt:  &expiresAt,
		Verified:      false,
		Method:        MethodGPS,
		CreatedAt:     time.Now(),
	}

	if err := s.repo.Insert(ctx, proof); err != nil {
		return "", fmt.Errorf("store host check-in proof: %w", err)
	}

	return pin, nil
}

// VerifyGPS records a party's GPS location against the listing and marks the proof
// as verified if within the configured threshold.
func (s *Service) VerifyGPS(ctx context.Context, in VerifyGPSInput) (bool, error) {
	_, hostID, listingID, err := s.repo.GetTransactionParties(ctx, in.TransactionID)
	if err != nil {
		return false, err
	}

	lat, lng, err := s.repo.GetListingLocation(ctx, listingID)
	if err != nil {
		return false, err
	}

	dist := Haversine(in.Lat, in.Lng, lat, lng)
	withinThreshold := IsWithinThreshold(dist, s.cfg.GPSThresholdMeters)

	// Ensure a proof record exists for this (transaction, type, user) triple.
	proof, err := s.repo.FindByTransactionAndType(ctx, in.TransactionID, in.ProofType, in.UserID)
	if err == ErrProofNotFound {
		// Create new record.
		proof = &ProximityProof{
			ID:            ulid.New(),
			TransactionID: in.TransactionID,
			UserID:        in.UserID,
			ProofType:     in.ProofType,
			Verified:      false,
			Method:        MethodGPS,
			CreatedAt:     time.Now(),
		}
		// For the renter's check-in record we also need to copy PIN expiry from
		// the host's record so ValidatePIN can be called later.
		if in.UserID != hostID && in.ProofType == ProofTypeCheckIn {
			hostProof, herr := s.repo.FindByTransactionAndType(ctx, in.TransactionID, ProofTypeCheckIn, hostID)
			if herr == nil {
				proof.PIN = hostProof.PIN
				proof.PINExpiresAt = hostProof.PINExpiresAt
			}
		}
		if err := s.repo.Insert(ctx, *proof); err != nil {
			return false, fmt.Errorf("insert GPS proof: %w", err)
		}
	} else if err != nil {
		return false, err
	}

	if err := s.repo.UpdateVerified(ctx, proof.ID, dist, withinThreshold); err != nil {
		return false, err
	}

	if !withinThreshold {
		return false, ErrTooFarAway
	}
	return true, nil
}

// VerifyPIN validates the PIN entered by the renter and marks their proof record
// as PIN-verified. Returns ErrPINInvalid or ErrPINExpired on failure.
func (s *Service) VerifyPIN(ctx context.Context, in VerifyPINInput) error {
	_, hostID, _, err := s.repo.GetTransactionParties(ctx, in.TransactionID)
	if err != nil {
		return err
	}

	// PIN is stored on the host's CHECK_IN record.
	hostProof, err := s.repo.FindByTransactionAndType(ctx, in.TransactionID, ProofTypeCheckIn, hostID)
	if err != nil {
		return fmt.Errorf("no host check-in proof found: %w", err)
	}

	if err := ValidatePIN(hostProof.PIN, in.PIN, hostProof.PINExpiresAt); err != nil {
		return err
	}

	// Find the renter's check-in proof record and mark it verified.
	renterProof, err := s.repo.FindByTransactionAndType(ctx, in.TransactionID, ProofTypeCheckIn, in.RenterID)
	if err != nil {
		return fmt.Errorf("renter has not completed GPS verification yet: %w", err)
	}

	// Copy the actual GPS distance when marking PIN-verified (preserve previous value).
	dist := 0.0
	if renterProof.GPSDistance != nil {
		dist = *renterProof.GPSDistance
	}
	return s.repo.UpdateVerified(ctx, renterProof.ID, dist, true)
}

// CheckHandoffComplete returns true when both parties have verified GPS proximity
// for the given proof type. For CHECK_IN, the renter must also have passed PIN
// verification (proof.Verified == true).
func (s *Service) CheckHandoffComplete(ctx context.Context, transactionID string, proofType ProofType) (bool, error) {
	proofs, err := s.repo.FindByTransactionID(ctx, transactionID)
	if err != nil {
		return false, err
	}

	_, hostID, _, err := s.repo.GetTransactionParties(ctx, transactionID)
	if err != nil {
		return false, err
	}

	var hostVerified, renterVerified bool
	for _, p := range proofs {
		if p.ProofType != proofType {
			continue
		}
		if p.UserID == hostID && p.Verified {
			hostVerified = true
		}
		if p.UserID != hostID && p.Verified {
			renterVerified = true
		}
	}

	return hostVerified && renterVerified, nil
}

// SMSFallback sends the current check-in PIN to the given phone number via SMS.
// Only the host may trigger this (they supply their hostID for authorisation).
func (s *Service) SMSFallback(ctx context.Context, in SMSFallbackInput) error {
	if s.sms == nil {
		return fmt.Errorf("SMS fallback is not configured")
	}

	_, hostID, _, err := s.repo.GetTransactionParties(ctx, in.TransactionID)
	if err != nil {
		return err
	}

	if in.HostID != hostID {
		return ErrNotAuthorized
	}

	hostProof, err := s.repo.FindByTransactionAndType(ctx, in.TransactionID, ProofTypeCheckIn, hostID)
	if err != nil {
		return fmt.Errorf("host check-in proof not found: %w", err)
	}

	if hostProof.PINExpiresAt != nil && time.Now().After(*hostProof.PINExpiresAt) {
		return ErrPINExpired
	}

	body := fmt.Sprintf("Your RentMy check-in PIN is: %s", hostProof.PIN)
	if err := s.sms.SendSMS(ctx, in.ToPhone, body); err != nil {
		return fmt.Errorf("send SMS: %w", err)
	}
	return nil
}

// GetStatus returns the aggregated proximity status for a booking.
// Both the renter and host may call this.
func (s *Service) GetStatus(ctx context.Context, transactionID, requesterID string) (*ProximityStatus, error) {
	renterID, hostID, _, err := s.repo.GetTransactionParties(ctx, transactionID)
	if err != nil {
		return nil, err
	}

	if requesterID != renterID && requesterID != hostID {
		return nil, ErrNotAuthorized
	}

	proofs, err := s.repo.FindByTransactionID(ctx, transactionID)
	if err != nil {
		return nil, err
	}

	status := &ProximityStatus{TransactionID: transactionID}
	for _, p := range proofs {
		switch p.ProofType {
		case ProofTypeCheckIn:
			status.CheckInProofs = append(status.CheckInProofs, p)
		case ProofTypeCheckOut:
			status.CheckOutProofs = append(status.CheckOutProofs, p)
		}
	}

	checkIn, _ := s.CheckHandoffComplete(ctx, transactionID, ProofTypeCheckIn)
	checkOut, _ := s.CheckHandoffComplete(ctx, transactionID, ProofTypeCheckOut)
	status.CheckInComplete = checkIn
	status.CheckOutComplete = checkOut

	return status, nil
}
