package guaranteefund

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func defaultConfig() Config {
	return Config{
		ReserveRatioNormal:       0.15,
		ReserveRatioAlert:        0.10,
		ReserveRatioRestrictHigh: 0.05,
		LossRatioTarget:          0.6,
	}
}

// mockTx implements pgx.Tx for testing.
type mockTx struct{ committed, rolledBack bool }

func (m *mockTx) Begin(_ context.Context) (pgx.Tx, error)                 { return m, nil }
func (m *mockTx) Commit(_ context.Context) error                          { m.committed = true; return nil }
func (m *mockTx) Rollback(_ context.Context) error                        { m.rolledBack = true; return nil }
func (m *mockTx) CopyFrom(_ context.Context, _ pgx.Identifier, _ []string, _ pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (m *mockTx) SendBatch(_ context.Context, _ *pgx.Batch) pgx.BatchResults { return nil }
func (m *mockTx) LargeObjects() pgx.LargeObjects                             { return pgx.LargeObjects{} }
func (m *mockTx) Prepare(_ context.Context, _, _ string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (m *mockTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (m *mockTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) { return nil, nil }
func (m *mockTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row        { return nil }
func (m *mockTx) Conn() *pgx.Conn                                               { return nil }

// mockRepo implements RepositoryInterface for testing.
type mockRepo struct {
	balance       int64
	gaps          int64
	claims90d     int64
	contribs90d   int64
	entries       []Entry
	insertedEntry *Entry
	balanceErr    error
	insertErr     error
	beginTxErr    error
}

func (m *mockRepo) InsertEntry(_ context.Context, _ pgx.Tx, entry Entry) error {
	if m.insertErr != nil {
		return m.insertErr
	}
	m.insertedEntry = &entry
	// Simulate balance change
	m.balance += entry.Amount
	return nil
}

func (m *mockRepo) GetCurrentBalance(_ context.Context) (int64, error) {
	if m.balanceErr != nil {
		return 0, m.balanceErr
	}
	return m.balance, nil
}

func (m *mockRepo) GetOutstandingGaps(_ context.Context) (int64, error) {
	return m.gaps, nil
}

func (m *mockRepo) GetRolling90DayClaims(_ context.Context) (int64, error) {
	return m.claims90d, nil
}

func (m *mockRepo) GetRolling90DayContributions(_ context.Context) (int64, error) {
	return m.contribs90d, nil
}

func (m *mockRepo) GetEntries(_ context.Context, limit, offset int) ([]Entry, int, error) {
	return m.entries, len(m.entries), nil
}

func (m *mockRepo) BeginTx(_ context.Context) (pgx.Tx, error) {
	if m.beginTxErr != nil {
		return nil, m.beginTxErr
	}
	return &mockTx{}, nil
}

// --- CheckReserveRatio tests (existing, kept) ---

func TestCheckReserveRatio_Normal(t *testing.T) {
	svc := NewService(nil, nil, defaultConfig())
	action := svc.CheckReserveRatio(0.20, 100_000)
	assert.Equal(t, ReserveActionNormal, action)
}

func TestCheckReserveRatio_ExactNormal(t *testing.T) {
	svc := NewService(nil, nil, defaultConfig())
	action := svc.CheckReserveRatio(0.15, 100_000)
	assert.Equal(t, ReserveActionNormal, action)
}

func TestCheckReserveRatio_Alert(t *testing.T) {
	svc := NewService(nil, nil, defaultConfig())
	action := svc.CheckReserveRatio(0.12, 100_000)
	assert.Equal(t, ReserveActionAlert, action)
}

func TestCheckReserveRatio_ExactAlert(t *testing.T) {
	svc := NewService(nil, nil, defaultConfig())
	action := svc.CheckReserveRatio(0.10, 100_000)
	assert.Equal(t, ReserveActionAlert, action)
}

func TestCheckReserveRatio_RestrictHigh(t *testing.T) {
	svc := NewService(nil, nil, defaultConfig())
	action := svc.CheckReserveRatio(0.07, 100_000)
	assert.Equal(t, ReserveActionRestrictHigh, action)
}

func TestCheckReserveRatio_ExactRestrictHigh(t *testing.T) {
	svc := NewService(nil, nil, defaultConfig())
	action := svc.CheckReserveRatio(0.05, 100_000)
	assert.Equal(t, ReserveActionRestrictHigh, action)
}

func TestCheckReserveRatio_RestrictAllGap(t *testing.T) {
	svc := NewService(nil, nil, defaultConfig())
	action := svc.CheckReserveRatio(0.03, 100_000)
	assert.Equal(t, ReserveActionRestrictAllGap, action)
}

func TestCheckReserveRatio_ZeroRatio(t *testing.T) {
	svc := NewService(nil, nil, defaultConfig())
	action := svc.CheckReserveRatio(0.0, 100_000)
	assert.Equal(t, ReserveActionRestrictAllGap, action)
}

func TestCheckReserveRatio_NoOutstandingGaps(t *testing.T) {
	svc := NewService(nil, nil, defaultConfig())
	// When there are no outstanding gaps, fund is always normal regardless of ratio.
	action := svc.CheckReserveRatio(0.0, 0)
	assert.Equal(t, ReserveActionNormal, action)
}

func TestEntryType_Values(t *testing.T) {
	assert.Equal(t, EntryType("CONTRIBUTION"), EntryTypeContribution)
	assert.Equal(t, EntryType("CLAIM"), EntryTypeClaim)
	assert.Equal(t, EntryType("CARD_RECOVERY"), EntryTypeCardRecovery)
	assert.Equal(t, EntryType("COLLECTIONS_REFERRAL"), EntryTypeCollectionsRef)
}

func TestReserveAction_Values(t *testing.T) {
	assert.Equal(t, ReserveAction("NORMAL"), ReserveActionNormal)
	assert.Equal(t, ReserveAction("ALERT"), ReserveActionAlert)
	assert.Equal(t, ReserveAction("RESTRICT_HIGH_VALUE"), ReserveActionRestrictHigh)
	assert.Equal(t, ReserveAction("RESTRICT_ALL_GAP"), ReserveActionRestrictAllGap)
}

func TestCheckReserveRatio_TableDriven(t *testing.T) {
	svc := NewService(nil, nil, defaultConfig())

	tests := []struct {
		name     string
		ratio    float64
		gaps     int64
		expected ReserveAction
	}{
		{"healthy fund", 0.25, 500_000, ReserveActionNormal},
		{"borderline normal", 0.15, 500_000, ReserveActionNormal},
		{"just below normal", 0.149, 500_000, ReserveActionAlert},
		{"borderline alert", 0.10, 500_000, ReserveActionAlert},
		{"just below alert", 0.099, 500_000, ReserveActionRestrictHigh},
		{"borderline restrict", 0.05, 500_000, ReserveActionRestrictHigh},
		{"just below restrict", 0.049, 500_000, ReserveActionRestrictAllGap},
		{"empty fund with gaps", 0.0, 500_000, ReserveActionRestrictAllGap},
		{"no gaps at all", 0.0, 0, ReserveActionNormal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := svc.CheckReserveRatio(tt.ratio, tt.gaps)
			assert.Equal(t, tt.expected, action)
		})
	}
}

// --- New service-level tests with mock repository ---

func TestContribute_InsertsEntryAndUpdatesBalance(t *testing.T) {
	repo := &mockRepo{balance: 10_000}
	svc := NewService(repo, nil, defaultConfig())
	ctx := context.Background()
	tx := &mockTx{}

	err := svc.Contribute(ctx, tx, "txn-001", 5_000)
	require.NoError(t, err)
	require.NotNil(t, repo.insertedEntry)
	assert.Equal(t, EntryTypeContribution, repo.insertedEntry.EntryType)
	assert.Equal(t, int64(5_000), repo.insertedEntry.Amount)
	assert.Equal(t, "txn-001", repo.insertedEntry.TransactionID)
}

func TestContribute_SkipsZeroAmount(t *testing.T) {
	repo := &mockRepo{balance: 10_000}
	svc := NewService(repo, nil, defaultConfig())
	ctx := context.Background()
	tx := &mockTx{}

	err := svc.Contribute(ctx, tx, "txn-001", 0)
	require.NoError(t, err)
	assert.Nil(t, repo.insertedEntry, "should not insert entry for zero amount")
}

func TestContribute_SkipsNegativeAmount(t *testing.T) {
	repo := &mockRepo{balance: 10_000}
	svc := NewService(repo, nil, defaultConfig())
	ctx := context.Background()
	tx := &mockTx{}

	err := svc.Contribute(ctx, tx, "txn-001", -100)
	require.NoError(t, err)
	assert.Nil(t, repo.insertedEntry, "should not insert entry for negative amount")
}

func TestClaim_FullAmount_WhenSufficientBalance(t *testing.T) {
	repo := &mockRepo{balance: 50_000}
	svc := NewService(repo, nil, defaultConfig())
	ctx := context.Background()

	result, err := svc.Claim(ctx, "txn-002", 30_000)
	require.NoError(t, err)
	assert.Equal(t, int64(30_000), result.Requested)
	assert.Equal(t, int64(30_000), result.Claimed)
	assert.Equal(t, int64(0), result.Shortfall)
	require.NotNil(t, repo.insertedEntry)
	assert.Equal(t, EntryTypeClaim, repo.insertedEntry.EntryType)
	assert.Equal(t, int64(-30_000), repo.insertedEntry.Amount)
}

func TestClaim_PartialAmount_WhenInsufficientBalance(t *testing.T) {
	repo := &mockRepo{balance: 20_000}
	svc := NewService(repo, nil, defaultConfig())
	ctx := context.Background()

	result, err := svc.Claim(ctx, "txn-003", 50_000)
	require.NoError(t, err)
	assert.Equal(t, int64(50_000), result.Requested)
	assert.Equal(t, int64(20_000), result.Claimed)
	assert.Equal(t, int64(30_000), result.Shortfall)
	require.NotNil(t, repo.insertedEntry)
	assert.Equal(t, int64(-20_000), repo.insertedEntry.Amount)
}

func TestClaim_FailsWhenFundEmpty(t *testing.T) {
	repo := &mockRepo{balance: 0}
	svc := NewService(repo, nil, defaultConfig())
	ctx := context.Background()

	_, err := svc.Claim(ctx, "txn-004", 10_000)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFundEmpty))
}

func TestClaim_FailsWhenBalanceNegative(t *testing.T) {
	repo := &mockRepo{balance: -500}
	svc := NewService(repo, nil, defaultConfig())
	ctx := context.Background()

	_, err := svc.Claim(ctx, "txn-004b", 10_000)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFundEmpty))
}

func TestGetFundHealth_CalculatesAllMetrics(t *testing.T) {
	repo := &mockRepo{
		balance:     100_000, // $1000
		gaps:        500_000, // $5000
		claims90d:   30_000,  // $300 in claims
		contribs90d: 60_000,  // $600 in contributions
	}
	svc := NewService(repo, nil, defaultConfig())
	ctx := context.Background()

	health, err := svc.GetFundHealth(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(100_000), health.Balance)
	assert.Equal(t, int64(500_000), health.OutstandingGaps)
	assert.InDelta(t, 0.2, health.ReserveRatio, 0.001)  // 100k / 500k = 0.2
	assert.InDelta(t, 0.5, health.LossRatio, 0.001)     // 30k / 60k = 0.5
	assert.Equal(t, ReserveActionNormal, health.Action)  // 0.2 > 0.15 threshold
}

func TestGetFundHealth_ZeroGaps(t *testing.T) {
	repo := &mockRepo{
		balance:     100_000,
		gaps:        0,
		claims90d:   0,
		contribs90d: 0,
	}
	svc := NewService(repo, nil, defaultConfig())
	ctx := context.Background()

	health, err := svc.GetFundHealth(ctx)
	require.NoError(t, err)
	assert.Equal(t, float64(0), health.ReserveRatio) // no gaps → 0 ratio
	assert.Equal(t, float64(0), health.LossRatio)    // no contributions → 0 ratio
	assert.Equal(t, ReserveActionNormal, health.Action)
}

func TestGetFundHealth_AlertState(t *testing.T) {
	repo := &mockRepo{
		balance:     50_000,  // $500
		gaps:        500_000, // $5000 → ratio = 0.1
		claims90d:   40_000,
		contribs90d: 50_000,
	}
	svc := NewService(repo, nil, defaultConfig())
	ctx := context.Background()

	health, err := svc.GetFundHealth(ctx)
	require.NoError(t, err)
	assert.Equal(t, ReserveActionAlert, health.Action) // 0.1 >= alert threshold
}

func TestDoubleEntryIntegrity(t *testing.T) {
	// Verify that every entry's effect on the balance is tracked correctly
	// by simulating a sequence: contribute → claim → card recovery.
	repo := &mockRepo{balance: 0}
	svc := NewService(repo, nil, defaultConfig())
	ctx := context.Background()
	tx := &mockTx{}

	// 1. Contribute $100 (10_000 cents)
	err := svc.Contribute(ctx, tx, "txn-100", 10_000)
	require.NoError(t, err)
	assert.Equal(t, int64(10_000), repo.balance)

	// 2. Claim $60 (6_000 cents)
	result, err := svc.Claim(ctx, "txn-100", 6_000)
	require.NoError(t, err)
	assert.Equal(t, int64(6_000), result.Claimed)
	assert.Equal(t, int64(4_000), repo.balance) // 10_000 - 6_000

	// 3. Card recovery $30 (3_000 cents)
	err = svc.RecordCardRecovery(ctx, "txn-100", 3_000)
	require.NoError(t, err)
	assert.Equal(t, int64(7_000), repo.balance) // 4_000 + 3_000

	// 4. Collections referral $20 (2_000 cents) — decreases balance
	err = svc.RecordCollectionsReferral(ctx, "txn-100", 2_000)
	require.NoError(t, err)
	assert.Equal(t, int64(5_000), repo.balance) // 7_000 - 2_000
}
