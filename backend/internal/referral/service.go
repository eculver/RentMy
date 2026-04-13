package referral

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/Brett2thered/RentMy/backend/internal/platform/ulid"
)

// ErrSelfReferral is returned when a user tries to apply their own code.
var ErrSelfReferral = errors.New("cannot apply your own referral code")

// ErrCodeExpired is returned when the referral code is past its expiry date.
var ErrCodeExpired = errors.New("referral code has expired")

// ErrCodeExhausted is returned when the referral code has reached its max uses.
var ErrCodeExhausted = errors.New("referral code has reached its maximum uses")

// StripeAdapter is the subset of Stripe operations needed by the referral payout job.
type StripeAdapter interface {
	PayoutHost(ctx context.Context, amount int64, currency string, accountID string) (transferID string, err error)
}

// UserAccountStore is the subset of the payment repository used to look up Stripe account IDs.
type UserAccountStore interface {
	GetStripeAccountID(ctx context.Context, userID string) (string, error)
}

// FraudSignalStore provides shared-device and shared-network checks for referral fraud.
type FraudSignalStore interface {
	SharedDeviceFingerprint(ctx context.Context, userA, userB string) (bool, error)
	SharedWiFiBSSID(ctx context.Context, userA, userB string) (bool, error)
}

// RepositoryInterface declares the persistence operations required by Service.
type RepositoryInterface interface {
	InsertReferralCode(ctx context.Context, rc *ReferralCode) error
	FindReferralCodeByCode(ctx context.Context, code string) (*ReferralCode, error)
	FindReferralCodeByUser(ctx context.Context, userID string) (*ReferralCode, error)
	IncrementCodeUseCount(ctx context.Context, codeID string) error
	InsertReferral(ctx context.Context, ref *Referral) error
	UpdateReferralStatus(ctx context.Context, id string, status ReferralStatus) error
	FindReferralByReferee(ctx context.Context, userID string) (*Referral, error)
	FindReferralByID(ctx context.Context, id string) (*Referral, error)
	CountRecentPayouts(ctx context.Context, userID string) (int, error)
	InsertReferralPayout(ctx context.Context, p *ReferralPayout) error
	UpdatePayoutStatus(ctx context.Context, payoutID string, status PayoutStatus, stripeID *string) error
	FindPayoutByID(ctx context.Context, id string) (*ReferralPayout, error)
	ListReferralsByReferrer(ctx context.Context, userID string, page, limit int) ([]*Referral, error)
	ListAllReferralsPaginated(ctx context.Context, f ListReferralsFilter) ([]*Referral, error)
	GetStats(ctx context.Context) (*ReferralStats, error)
}

// Service implements referral generation, application, and payout logic.
type Service struct {
	repo         RepositoryInterface
	stripe       StripeAdapter
	userAccounts UserAccountStore
	fraudSignals FraudSignalStore
	riverClient  *river.Client[pgx.Tx]
}

// NewService constructs a Service backed by the concrete Repository.
func NewService(
	repo *Repository,
	stripe StripeAdapter,
	userAccounts UserAccountStore,
	fraudSignals FraudSignalStore,
	riverClient *river.Client[pgx.Tx],
) *Service {
	return NewServiceWithFakes(repo, stripe, userAccounts, fraudSignals, riverClient)
}

// NewServiceWithFakes constructs a Service with interface-typed dependencies,
// useful for unit testing.
func NewServiceWithFakes(
	repo RepositoryInterface,
	stripe StripeAdapter,
	userAccounts UserAccountStore,
	fraudSignals FraudSignalStore,
	riverClient *river.Client[pgx.Tx],
) *Service {
	return &Service{
		repo:         repo,
		stripe:       stripe,
		userAccounts: userAccounts,
		fraudSignals: fraudSignals,
		riverClient:  riverClient,
	}
}

// GenerateCode returns the user's existing code, or creates and stores a new 8-char code.
func (s *Service) GenerateCode(ctx context.Context, userID string) (*ReferralCode, error) {
	existing, err := s.repo.FindReferralCodeByUser(ctx, userID)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, ErrCodeNotFound) {
		return nil, fmt.Errorf("find referral code: %w", err)
	}

	code := generateCode()
	rc := &ReferralCode{
		ID:        ulid.New(),
		Code:      code,
		UserID:    userID,
		MaxUses:   0,
		UseCount:  0,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.repo.InsertReferralCode(ctx, rc); err != nil {
		return nil, fmt.Errorf("insert referral code: %w", err)
	}
	return rc, nil
}

// ApplyReferralCode is called during registration. It validates the code and
// creates a Referral record with status SIGNED_UP.
func (s *Service) ApplyReferralCode(ctx context.Context, refereeUserID, code string) (*Referral, error) {
	rc, err := s.repo.FindReferralCodeByCode(ctx, code)
	if err != nil {
		return nil, err // ErrCodeNotFound propagates
	}

	if rc.UserID == refereeUserID {
		return nil, ErrSelfReferral
	}
	if rc.ExpiresAt != nil && rc.ExpiresAt.Before(time.Now().UTC()) {
		return nil, ErrCodeExpired
	}
	if rc.MaxUses > 0 && rc.UseCount >= rc.MaxUses {
		return nil, ErrCodeExhausted
	}

	// Idempotency: already referred.
	existing, err := s.repo.FindReferralByReferee(ctx, refereeUserID)
	if err != nil {
		return nil, fmt.Errorf("check existing referral: %w", err)
	}
	if existing != nil {
		return existing, nil
	}

	// Check for shared device fingerprint at sign-up time.
	if s.fraudSignals != nil {
		shared, err := s.fraudSignals.SharedDeviceFingerprint(ctx, rc.UserID, refereeUserID)
		if err != nil {
			slog.Warn("referral: shared device check failed", "error", err)
		} else if shared {
			slog.Info("referral: blocking — shared device fingerprint",
				"referrer", rc.UserID, "referee", refereeUserID)
			return nil, fmt.Errorf("referral fraud: shared device")
		}
	}

	ref := &Referral{
		ID:             ulid.New(),
		ReferralCodeID: rc.ID,
		ReferrerID:     rc.UserID,
		RefereeID:      refereeUserID,
		Status:         ReferralStatusSignedUp,
		CreatedAt:      time.Now().UTC(),
	}
	if err := s.repo.InsertReferral(ctx, ref); err != nil {
		return nil, fmt.Errorf("insert referral: %w", err)
	}
	if err := s.repo.IncrementCodeUseCount(ctx, rc.ID); err != nil {
		slog.Warn("referral: failed to increment use count", "codeId", rc.ID, "error", err)
	}
	return ref, nil
}

// OnFirstRentalCompleted is called by the booking checkout hook. It checks
// whether the user was referred and, if so, advances the referral to
// FIRST_RENTAL_COMPLETED and enqueues a payout evaluation job.
func (s *Service) OnFirstRentalCompleted(ctx context.Context, userID string) {
	ref, err := s.repo.FindReferralByReferee(ctx, userID)
	if err != nil {
		slog.Warn("referral: find by referee failed", "userID", userID, "error", err)
		return
	}
	if ref == nil || ref.Status != ReferralStatusSignedUp {
		return
	}

	if err := s.repo.UpdateReferralStatus(ctx, ref.ID, ReferralStatusFirstRentalCompleted); err != nil {
		slog.Warn("referral: advance to FIRST_RENTAL_COMPLETED failed", "referralId", ref.ID, "error", err)
		return
	}

	if s.riverClient != nil {
		if _, err := s.riverClient.Insert(ctx, ReferralPayoutJobArgs{ReferralID: ref.ID}, nil); err != nil {
			slog.Warn("referral: failed to enqueue payout job", "referralId", ref.ID, "error", err)
		}
	}
}

// EvaluateAndPayout performs fraud checks and, if clean, creates payout records
// and enqueues Stripe transfer jobs.
func (s *Service) EvaluateAndPayout(ctx context.Context, referralID string) error {
	ref, err := s.repo.FindReferralByID(ctx, referralID)
	if err != nil {
		return fmt.Errorf("load referral: %w", err)
	}
	if ref.Status == ReferralStatusPaid || ref.Status == ReferralStatusFraudulent {
		return nil
	}

	// Fraud check 1: shared device fingerprint → block.
	if s.fraudSignals != nil {
		shared, err := s.fraudSignals.SharedDeviceFingerprint(ctx, ref.ReferrerID, ref.RefereeID)
		if err != nil {
			slog.Warn("referral payout: device check error", "referralId", referralID, "error", err)
		}
		if shared {
			return s.repo.UpdateReferralStatus(ctx, referralID, ReferralStatusFraudulent)
		}

		// Fraud check 2: same WiFi AND same device (device check above already passed if we reach here).
		sameNet, err := s.fraudSignals.SharedWiFiBSSID(ctx, ref.ReferrerID, ref.RefereeID)
		if err != nil {
			slog.Warn("referral payout: wifi check error", "referralId", referralID, "error", err)
		}
		if sameNet {
			slog.Info("referral: same WiFi — monitoring, devices differ",
				"referralId", referralID)
		}
	}

	// Fraud check 3: velocity limit — referrer has >5 PAID payouts in last 30 days.
	recentCount, err := s.repo.CountRecentPayouts(ctx, ref.ReferrerID)
	if err != nil {
		slog.Warn("referral payout: velocity check error", "referralId", referralID, "error", err)
	}
	if recentCount > 5 {
		slog.Info("referral: velocity limit — queueing for manual review",
			"referralId", referralID, "recentPayouts", recentCount)
		return nil // leave as FIRST_RENTAL_COMPLETED for ops review
	}

	// Create payout records for both parties.
	now := time.Now().UTC()
	payouts := []*ReferralPayout{
		{ID: ulid.New(), ReferralID: referralID, UserID: ref.ReferrerID, Amount: ReferralPayoutAmount, Status: PayoutStatusPending, CreatedAt: now},
		{ID: ulid.New(), ReferralID: referralID, UserID: ref.RefereeID, Amount: ReferralPayoutAmount, Status: PayoutStatusPending, CreatedAt: now},
	}
	for _, p := range payouts {
		if err := s.repo.InsertReferralPayout(ctx, p); err != nil {
			return fmt.Errorf("insert payout: %w", err)
		}
	}

	if err := s.repo.UpdateReferralStatus(ctx, referralID, ReferralStatusPaid); err != nil {
		return fmt.Errorf("advance referral to PAID: %w", err)
	}

	if s.riverClient != nil {
		for _, p := range payouts {
			if _, err := s.riverClient.Insert(ctx, ReferralStripeTransferJobArgs{PayoutID: p.ID}, nil); err != nil {
				slog.Warn("referral: failed to enqueue stripe transfer job",
					"payoutId", p.ID, "error", err)
			}
		}
	}
	return nil
}

// ProcessPayout is called by the Stripe transfer River job. It initiates a
// Stripe transfer and updates the payout status.
func (s *Service) ProcessPayout(ctx context.Context, payoutID string) error {
	p, err := s.repo.FindPayoutByID(ctx, payoutID)
	if err != nil {
		return fmt.Errorf("load payout: %w", err)
	}
	if p.Status == PayoutStatusPaid {
		return nil // idempotent
	}

	if err := s.repo.UpdatePayoutStatus(ctx, payoutID, PayoutStatusProcessing, nil); err != nil {
		return fmt.Errorf("mark processing: %w", err)
	}

	accountID, err := s.userAccounts.GetStripeAccountID(ctx, p.UserID)
	if err != nil {
		_ = s.repo.UpdatePayoutStatus(ctx, payoutID, PayoutStatusFailed, nil)
		return fmt.Errorf("get stripe account: %w", err)
	}
	if accountID == "" {
		_ = s.repo.UpdatePayoutStatus(ctx, payoutID, PayoutStatusFailed, nil)
		return fmt.Errorf("user %s has no Stripe account", p.UserID)
	}

	transferID, err := s.stripe.PayoutHost(ctx, int64(p.Amount), "usd", accountID)
	if err != nil {
		_ = s.repo.UpdatePayoutStatus(ctx, payoutID, PayoutStatusFailed, nil)
		return fmt.Errorf("stripe transfer: %w", err)
	}

	if err := s.repo.UpdatePayoutStatus(ctx, payoutID, PayoutStatusPaid, &transferID); err != nil {
		slog.Error("referral: payout succeeded but status update failed",
			"payoutId", payoutID, "transferId", transferID, "error", err)
	}
	return nil
}

// ApplyCode satisfies the user.ReferralApplier interface; it discards the
// returned Referral record so the user service doesn't import this package.
func (s *Service) ApplyCode(ctx context.Context, refereeUserID, code string) error {
	_, err := s.ApplyReferralCode(ctx, refereeUserID, code)
	return err
}

// GetCodeByUser returns the referral code owned by a user.
func (s *Service) GetCodeByUser(ctx context.Context, userID string) (*ReferralCode, error) {
	return s.repo.FindReferralCodeByUser(ctx, userID)
}

// ListReferralsByReferrer returns paginated referrals where the user is the referrer.
func (s *Service) ListReferralsByReferrer(ctx context.Context, userID string, page, limit int) ([]*Referral, error) {
	return s.repo.ListReferralsByReferrer(ctx, userID, page, limit)
}

// ListAllReferralsPaginated returns all referrals for the ops dashboard.
func (s *Service) ListAllReferralsPaginated(ctx context.Context, f ListReferralsFilter) ([]*Referral, error) {
	return s.repo.ListAllReferralsPaginated(ctx, f)
}

// GetStats returns aggregate referral statistics.
func (s *Service) GetStats(ctx context.Context) (*ReferralStats, error) {
	return s.repo.GetStats(ctx)
}

// generateCode produces a random 8-character uppercase alphanumeric string.
func generateCode() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
