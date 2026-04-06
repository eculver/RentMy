package guaranteefund

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/giits/rentmy/backend/internal/platform/ulid"
)

// Config holds tunable guarantee fund parameters.
type Config struct {
	// ReserveRatioNormal is the threshold above which the fund is healthy (default 0.15 = 15%).
	ReserveRatioNormal float64
	// ReserveRatioAlert is the threshold below which an alert is fired (default 0.10 = 10%).
	ReserveRatioAlert float64
	// ReserveRatioRestrictHigh is the threshold below which high-value listings are restricted (default 0.05 = 5%).
	ReserveRatioRestrictHigh float64
	// LossRatioTarget is the maximum acceptable loss ratio (default 0.6).
	LossRatioTarget float64
}

// Service implements the guarantee fund business logic.
type Service struct {
	repo        *Repository
	riverClient *river.Client[pgx.Tx]
	cfg         Config
}

// NewService creates a Service with the given dependencies.
func NewService(repo *Repository, riverClient *river.Client[pgx.Tx], cfg Config) *Service {
	return &Service{
		repo:        repo,
		riverClient: riverClient,
		cfg:         cfg,
	}
}

// Contribute inserts a CONTRIBUTION entry into the guarantee fund ledger.
func (s *Service) Contribute(ctx context.Context, tx pgx.Tx, transactionID string, amount int64) error {
	if amount <= 0 {
		return nil
	}
	entry := Entry{
		ID:            ulid.New(),
		TransactionID: transactionID,
		EntryType:     EntryTypeContribution,
		Amount:        amount,
	}
	return s.repo.InsertEntry(ctx, tx, entry)
}

// Claim draws from the guarantee fund to cover damage shortfalls.
// The fund balance cannot go negative — only the available amount is disbursed.
// Returns the actual amount claimed (may be less than requested if fund is low).
func (s *Service) Claim(ctx context.Context, transactionID string, amount int64) (int64, error) {
	balance, err := s.repo.GetCurrentBalance(ctx)
	if err != nil {
		return 0, fmt.Errorf("get fund balance: %w", err)
	}
	if balance <= 0 {
		return 0, ErrFundEmpty
	}

	claimAmount := amount
	if claimAmount > balance {
		claimAmount = balance
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	entry := Entry{
		ID:            ulid.New(),
		TransactionID: transactionID,
		EntryType:     EntryTypeClaim,
		Amount:        -claimAmount,
	}
	if err := s.repo.InsertEntry(ctx, tx, entry); err != nil {
		return 0, fmt.Errorf("insert claim entry: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit claim: %w", err)
	}

	slog.Info("guaranteefund: claim processed",
		"transactionId", transactionID,
		"requested", amount,
		"claimed", claimAmount,
	)
	return claimAmount, nil
}

// RecordCardRecovery records a recovery from a renter's card, restoring fund balance.
func (s *Service) RecordCardRecovery(ctx context.Context, transactionID string, amount int64) error {
	if amount <= 0 {
		return nil
	}
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	entry := Entry{
		ID:            ulid.New(),
		TransactionID: transactionID,
		EntryType:     EntryTypeCardRecovery,
		Amount:        amount,
	}
	if err := s.repo.InsertEntry(ctx, tx, entry); err != nil {
		return fmt.Errorf("insert card recovery entry: %w", err)
	}
	return tx.Commit(ctx)
}

// RecordCollectionsReferral records an amount sent to collections, tracking the loss.
func (s *Service) RecordCollectionsReferral(ctx context.Context, transactionID string, amount int64) error {
	if amount <= 0 {
		return nil
	}
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	entry := Entry{
		ID:            ulid.New(),
		TransactionID: transactionID,
		EntryType:     EntryTypeCollectionsRef,
		Amount:        -amount,
	}
	if err := s.repo.InsertEntry(ctx, tx, entry); err != nil {
		return fmt.Errorf("insert collections referral entry: %w", err)
	}
	return tx.Commit(ctx)
}

// GetFundHealth calculates the current fund health including reserve ratio and loss ratio.
func (s *Service) GetFundHealth(ctx context.Context) (FundHealth, error) {
	balance, err := s.repo.GetCurrentBalance(ctx)
	if err != nil {
		return FundHealth{}, fmt.Errorf("get balance: %w", err)
	}

	gaps, err := s.repo.GetOutstandingGaps(ctx)
	if err != nil {
		return FundHealth{}, fmt.Errorf("get outstanding gaps: %w", err)
	}

	claims, err := s.repo.GetRolling90DayClaims(ctx)
	if err != nil {
		return FundHealth{}, fmt.Errorf("get 90-day claims: %w", err)
	}

	contributions, err := s.repo.GetRolling90DayContributions(ctx)
	if err != nil {
		return FundHealth{}, fmt.Errorf("get 90-day contributions: %w", err)
	}

	var reserveRatio float64
	if gaps > 0 {
		reserveRatio = float64(balance) / float64(gaps)
	}

	var lossRatio float64
	if contributions > 0 {
		lossRatio = float64(claims) / float64(contributions)
	}

	action := s.CheckReserveRatio(reserveRatio, gaps)

	return FundHealth{
		Balance:         balance,
		OutstandingGaps: gaps,
		ReserveRatio:    reserveRatio,
		LossRatio:       lossRatio,
		Action:          action,
	}, nil
}

// CheckReserveRatio returns the recommended action based on the reserve ratio.
// When outstandingGaps is 0, the fund is always in NORMAL state.
func (s *Service) CheckReserveRatio(reserveRatio float64, outstandingGaps int64) ReserveAction {
	if outstandingGaps == 0 {
		return ReserveActionNormal
	}
	switch {
	case reserveRatio >= s.cfg.ReserveRatioNormal:
		return ReserveActionNormal
	case reserveRatio >= s.cfg.ReserveRatioAlert:
		return ReserveActionAlert
	case reserveRatio >= s.cfg.ReserveRatioRestrictHigh:
		return ReserveActionRestrictHigh
	default:
		return ReserveActionRestrictAllGap
	}
}

// GetEntries returns paginated ledger entries.
func (s *Service) GetEntries(ctx context.Context, limit, offset int) ([]Entry, int, error) {
	return s.repo.GetEntries(ctx, limit, offset)
}
