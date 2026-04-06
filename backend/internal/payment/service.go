package payment

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/giits/rentmy/backend/internal/platform/ulid"
)

// Config holds tunable payment parameters loaded from the environment.
type Config struct {
	TakeRateBPS         int // platform fee in basis points (e.g. 2000 = 20%)
	GuaranteeRateBPS    int // guarantee fund contribution as BPS of platform fee
	DamageReserveRate   int // portion of hold reserved for damage, in BPS
	PayoutDelayNewHostH int // hours to delay payout for new/high-risk hosts
}

// Service implements the payment domain business logic.
type Service struct {
	repo        *Repository
	adapter     PaymentAdapter
	riverClient *river.Client[pgx.Tx]
	cfg         Config
}

// NewService creates a Service with the given dependencies and config.
func NewService(repo *Repository, adapter PaymentAdapter, riverClient *river.Client[pgx.Tx], cfg Config) *Service {
	return &Service{
		repo:        repo,
		adapter:     adapter,
		riverClient: riverClient,
		cfg:         cfg,
	}
}

// CreateBooking orchestrates a full booking: hold authorization, rental fee charge,
// transaction creation, and guarantee fund contribution.
func (s *Service) CreateBooking(ctx context.Context, in BookingInput) (BookingResult, error) {
	// Fetch listing to get pricing and item value.
	snap, err := s.repo.GetListingForBooking(ctx, in.ListingID)
	if err != nil {
		return BookingResult{}, fmt.Errorf("get listing: %w", err)
	}

	// Compute hold and fee amounts.
	iv := itemValue(snap)
	holdAmount := TieredHold(iv)
	guaranteeGap := GuaranteeGap(iv, holdAmount)

	perHour, perDay := listingPriceCents(snap)
	duration := in.ScheduledEnd.Sub(in.ScheduledStart)
	rentalFee := RentalFee(perHour, perDay, duration)
	platformFee := PlatformFee(rentalFee, s.cfg.TakeRateBPS)
	hostPayout := HostPayout(rentalFee, platformFee)
	guaranteeContribution := GuaranteeFundContribution(platformFee, s.cfg.GuaranteeRateBPS)

	// Compute damage reserve within the hold.
	damageReserve := holdAmount * int64(s.cfg.DamageReserveRate) / 10_000

	// Resolve renter's Stripe customer ID.
	customerID, err := s.repo.GetStripeCustomerID(ctx, in.RenterID)
	if err != nil {
		return BookingResult{}, fmt.Errorf("get stripe customer id: %w", err)
	}
	if customerID == "" {
		return BookingResult{}, ErrNoPaymentMethod
	}

	// Authorize the security hold (manual capture PaymentIntent).
	holdIntentID, err := s.adapter.AuthorizeHold(ctx, holdAmount, "usd", in.PaymentMethodID, customerID)
	if err != nil {
		return BookingResult{}, fmt.Errorf("authorize hold: %w", err)
	}

	// Charge the rental fee immediately (automatic capture).
	chargeID, err := s.adapter.ChargeRentalFee(ctx, rentalFee, "usd", in.PaymentMethodID, customerID)
	if err != nil {
		// Best-effort: release the hold to avoid leaving an orphaned authorization.
		_ = s.adapter.ReleaseHold(ctx, holdIntentID)
		return BookingResult{}, fmt.Errorf("charge rental fee: %w", err)
	}

	transactionID := ulid.New()

	initAlloc := HoldAllocation{
		TotalAuthorized:     holdAmount,
		CapturedForLateFees: 0,
		CapturedForDamage:   0,
		DamageReserve:       damageReserve,
		Released:            0,
		Remaining:           holdAmount,
	}

	txn := Transaction{
		ID:                    transactionID,
		RenterID:              in.RenterID,
		HostID:                snap.HostID,
		ListingID:             in.ListingID,
		RentalFee:             rentalFee,
		HoldAmount:            holdAmount,
		ItemValue:             iv,
		GuaranteeGap:          guaranteeGap,
		PlatformFee:           platformFee,
		HostPayout:            hostPayout,
		GuaranteeContribution: guaranteeContribution,
		EscrowStatus:          "HELD",
		HoldStatus:            "AUTHORIZED",
		HoldAllocation:        initAlloc,
		StripePaymentIntentID: holdIntentID,
		StripeChargeID:        chargeID,
		ScheduledStart:        in.ScheduledStart,
		ScheduledEnd:          in.ScheduledEnd,
		Status:                "REQUESTED",
	}

	// Persist the transaction and guarantee fund entry atomically.
	dbTx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return BookingResult{}, fmt.Errorf("begin db transaction: %w", err)
	}
	defer func() { _ = dbTx.Rollback(ctx) }()

	if err := s.repo.CreateTransaction(ctx, dbTx, txn); err != nil {
		return BookingResult{}, fmt.Errorf("create transaction record: %w", err)
	}

	fundEntry := GuaranteeFundEntry{
		ID:            ulid.New(),
		TransactionID: transactionID,
		EntryType:     "CONTRIBUTION",
		Amount:        guaranteeContribution,
	}
	if err := s.repo.InsertGuaranteeFundEntry(ctx, dbTx, fundEntry); err != nil {
		return BookingResult{}, fmt.Errorf("insert guarantee fund entry: %w", err)
	}

	if err := dbTx.Commit(ctx); err != nil {
		return BookingResult{}, fmt.Errorf("commit booking transaction: %w", err)
	}

	return BookingResult{
		TransactionID:         transactionID,
		HoldAmount:            holdAmount,
		RentalFee:             rentalFee,
		PlatformFee:           platformFee,
		HostPayout:            hostPayout,
		GuaranteeGap:          guaranteeGap,
		GuaranteeContribution: guaranteeContribution,
		TotalImpact:           holdAmount + rentalFee,
	}, nil
}

// ReleaseHold releases the security hold for a completed transaction.
func (s *Service) ReleaseHold(ctx context.Context, transactionID string) error {
	txn, err := s.repo.GetTransaction(ctx, transactionID)
	if err != nil {
		return fmt.Errorf("get transaction: %w", err)
	}

	if err := s.adapter.ReleaseHold(ctx, txn.StripePaymentIntentID); err != nil {
		return fmt.Errorf("release hold: %w", err)
	}

	dbTx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin db transaction: %w", err)
	}
	defer func() { _ = dbTx.Rollback(ctx) }()

	updated := txn.HoldAllocation
	updated.Released = updated.Remaining
	updated.Remaining = 0

	if err := s.repo.UpdateHoldAllocation(ctx, dbTx, transactionID, updated, "RELEASED"); err != nil {
		return fmt.Errorf("update hold allocation: %w", err)
	}

	return dbTx.Commit(ctx)
}

// CaptureFromHold atomically captures an amount from the security hold.
// reason must be CaptureReasonLateFee or CaptureReasonDamage.
func (s *Service) CaptureFromHold(ctx context.Context, transactionID string, amount int64, reason CaptureReason) (string, error) {
	if reason != CaptureReasonLateFee && reason != CaptureReasonDamage {
		return "", ErrInvalidCapture
	}

	txn, err := s.repo.GetTransaction(ctx, transactionID)
	if err != nil {
		return "", fmt.Errorf("get transaction: %w", err)
	}

	if amount > txn.HoldAllocation.Remaining {
		return "", ErrInsufficientHold
	}

	chargeID, err := s.adapter.CaptureHold(ctx, txn.StripePaymentIntentID, amount)
	if err != nil {
		return "", fmt.Errorf("capture hold: %w", err)
	}

	dbTx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return "", fmt.Errorf("begin db transaction: %w", err)
	}
	defer func() { _ = dbTx.Rollback(ctx) }()

	updated := txn.HoldAllocation
	updated.Remaining -= amount
	switch reason {
	case CaptureReasonLateFee:
		updated.CapturedForLateFees += amount
	case CaptureReasonDamage:
		updated.CapturedForDamage += amount
	}

	holdStatus := "PARTIALLY_CAPTURED"
	if updated.Remaining == 0 {
		holdStatus = "CAPTURED"
	}

	if err := s.repo.UpdateHoldAllocation(ctx, dbTx, transactionID, updated, holdStatus); err != nil {
		return "", fmt.Errorf("update hold allocation: %w", err)
	}

	if err := dbTx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit capture: %w", err)
	}
	return chargeID, nil
}

// ScheduleHostPayout enqueues a River job to transfer rental proceeds to the host.
// The delay depends on the host's history and reputation (PRD section 7):
//   - First 3 transactions: 48h mandatory delay
//   - reputation_score < 200 (high-risk): 48h delay
//   - Established host: no delay
func (s *Service) ScheduleHostPayout(ctx context.Context, transactionID string) error {
	txn, err := s.repo.GetTransaction(ctx, transactionID)
	if err != nil {
		return fmt.Errorf("get transaction: %w", err)
	}

	accountID, err := s.repo.GetStripeAccountID(ctx, txn.HostID)
	if err != nil {
		return fmt.Errorf("get stripe account id: %w", err)
	}
	if accountID == "" {
		return ErrNoStripeAccount
	}

	delay, err := s.payoutDelay(ctx, txn.HostID)
	if err != nil {
		return fmt.Errorf("compute payout delay: %w", err)
	}

	opts := &river.InsertOpts{}
	if delay > 0 {
		opts.ScheduledAt = time.Now().Add(delay)
	}

	_, err = s.riverClient.Insert(ctx, PayoutJobArgs{
		TransactionID: transactionID,
		HostAccountID: accountID,
		Amount:        txn.HostPayout,
		Currency:      "usd",
	}, opts)
	if err != nil {
		return fmt.Errorf("enqueue payout job: %w", err)
	}
	return nil
}

// payoutDelay returns the delay duration before a host payout should be released.
func (s *Service) payoutDelay(ctx context.Context, hostID string) (time.Duration, error) {
	count, err := s.repo.GetHostTransactionCount(ctx, hostID)
	if err != nil {
		return 0, fmt.Errorf("get host transaction count: %w", err)
	}
	if count < 3 {
		return time.Duration(s.cfg.PayoutDelayNewHostH) * time.Hour, nil
	}

	repScore, err := s.repo.GetHostReputationScore(ctx, hostID)
	if err != nil {
		return 0, fmt.Errorf("get host reputation score: %w", err)
	}
	if repScore < 200 {
		return time.Duration(s.cfg.PayoutDelayNewHostH) * time.Hour, nil
	}

	return 0, nil
}

// GetGuaranteeFundHealth returns the current state of the guarantee fund.
func (s *Service) GetGuaranteeFundHealth(ctx context.Context) (GuaranteeFundHealth, error) {
	balance, err := s.repo.GetGuaranteeFundBalance(ctx)
	if err != nil {
		return GuaranteeFundHealth{}, fmt.Errorf("get guarantee fund balance: %w", err)
	}

	gaps, err := s.repo.GetTotalOutstandingGuaranteeGaps(ctx)
	if err != nil {
		return GuaranteeFundHealth{}, fmt.Errorf("get outstanding guarantee gaps: %w", err)
	}

	var ratio float64
	if gaps > 0 {
		ratio = float64(balance) / float64(gaps)
	}

	return GuaranteeFundHealth{
		Balance:         balance,
		OutstandingGaps: gaps,
		ReserveRatio:    ratio,
	}, nil
}

// OnboardHost creates a Stripe Express connected account for a host and returns the onboarding URL.
func (s *Service) OnboardHost(ctx context.Context, userID string) (OnboardHostResult, error) {
	email, name, err := s.repo.GetUserEmailAndName(ctx, userID)
	if err != nil {
		return OnboardHostResult{}, fmt.Errorf("get user: %w", err)
	}
	_ = name // Stripe account creation only needs email for Express accounts

	accountID, onboardingURL, err := s.adapter.CreateConnectedAccount(ctx, email)
	if err != nil {
		return OnboardHostResult{}, fmt.Errorf("create connected account: %w", err)
	}

	if err := s.repo.StoreStripeAccountID(ctx, userID, accountID); err != nil {
		return OnboardHostResult{}, fmt.Errorf("store stripe account id: %w", err)
	}

	return OnboardHostResult{
		AccountID:     accountID,
		OnboardingURL: onboardingURL,
	}, nil
}

// SetupRenterPayment ensures a Stripe customer exists for the renter, then creates
// a SetupIntent so the mobile app can save a payment method.
func (s *Service) SetupRenterPayment(ctx context.Context, userID string) (SetupPaymentResult, error) {
	customerID, err := s.repo.GetStripeCustomerID(ctx, userID)
	if err != nil {
		return SetupPaymentResult{}, fmt.Errorf("get stripe customer id: %w", err)
	}

	if customerID == "" {
		email, name, err := s.repo.GetUserEmailAndName(ctx, userID)
		if err != nil {
			return SetupPaymentResult{}, fmt.Errorf("get user: %w", err)
		}

		customerID, err = s.adapter.CreateCustomer(ctx, email, name)
		if err != nil {
			return SetupPaymentResult{}, fmt.Errorf("create stripe customer: %w", err)
		}

		if err := s.repo.StoreStripeCustomerID(ctx, userID, customerID); err != nil {
			return SetupPaymentResult{}, fmt.Errorf("store stripe customer id: %w", err)
		}
	}

	clientSecret, err := s.adapter.CreateSetupIntent(ctx, customerID)
	if err != nil {
		return SetupPaymentResult{}, fmt.Errorf("create setup intent: %w", err)
	}

	return SetupPaymentResult{
		CustomerID:   customerID,
		ClientSecret: clientSecret,
	}, nil
}

// GetHoldEstimate returns the tiered hold amount for a given listing.
func (s *Service) GetHoldEstimate(ctx context.Context, listingID string) (HoldEstimateResult, error) {
	snap, err := s.repo.GetListingForBooking(ctx, listingID)
	if err != nil {
		return HoldEstimateResult{}, fmt.Errorf("get listing: %w", err)
	}

	iv := itemValue(snap)
	hold := TieredHold(iv)
	gap := GuaranteeGap(iv, hold)

	return HoldEstimateResult{
		ItemValue:    iv,
		HoldAmount:   hold,
		GuaranteeGap: gap,
	}, nil
}

// GetTransaction returns a transaction by ID.
func (s *Service) GetTransaction(ctx context.Context, id string) (Transaction, error) {
	return s.repo.GetTransaction(ctx, id)
}

// GetRenterBookings returns paginated bookings for a renter.
func (s *Service) GetRenterBookings(ctx context.Context, renterID string, limit, offset int) ([]Transaction, int, error) {
	return s.repo.GetRenterTransactions(ctx, renterID, limit, offset)
}

// UpdateTransactionStatus updates the status of a transaction.
func (s *Service) UpdateTransactionStatus(ctx context.Context, transactionID, status string) error {
	return s.repo.UpdateTransactionStatus(ctx, transactionID, status)
}

// ChargeForDamageOverflow attempts to charge the renter's card for damage that
// exceeds the hold amount. Requires the renter to have a Stripe customer with
// a default payment method on file.
func (s *Service) ChargeForDamageOverflow(ctx context.Context, transactionID string, amount int64) error {
	txn, err := s.repo.GetTransaction(ctx, transactionID)
	if err != nil {
		return fmt.Errorf("get transaction: %w", err)
	}
	customerID, err := s.repo.GetStripeCustomerID(ctx, txn.RenterID)
	if err != nil {
		return fmt.Errorf("get renter customer id: %w", err)
	}
	if customerID == "" {
		return ErrNoPaymentMethod
	}
	// Use default payment method via the Stripe customer.
	_, err = s.adapter.ChargeRentalFee(ctx, amount, "usd", "", customerID)
	if err != nil {
		return fmt.Errorf("charge damage overflow: %w", err)
	}
	return nil
}

// ClaimGuaranteeFund draws from the guarantee fund to cover damage shortfalls.
// The fund balance cannot go negative — only the available amount is disbursed.
func (s *Service) ClaimGuaranteeFund(ctx context.Context, transactionID string, amount int64) error {
	balance, err := s.repo.GetGuaranteeFundBalance(ctx)
	if err != nil {
		return fmt.Errorf("get fund balance: %w", err)
	}
	if balance <= 0 {
		return fmt.Errorf("guarantee fund empty, cannot claim %d cents", amount)
	}
	claimAmount := amount
	if claimAmount > balance {
		claimAmount = balance
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	entry := GuaranteeFundEntry{
		ID:            ulid.New(),
		TransactionID: transactionID,
		EntryType:     "CLAIM",
		Amount:        -claimAmount,
	}
	if err := s.repo.InsertGuaranteeFundEntry(ctx, tx, entry); err != nil {
		return fmt.Errorf("insert claim entry: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit claim: %w", err)
	}
	return nil
}
