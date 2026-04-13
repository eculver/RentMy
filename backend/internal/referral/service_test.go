package referral_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Brett2thered/RentMy/backend/internal/referral"
)

// --- fakes ---

type fakeRepo struct {
	mu           sync.Mutex
	codes        map[string]*referral.ReferralCode  // id -> code
	codesByCode  map[string]*referral.ReferralCode  // code string -> code
	codesByUser  map[string]*referral.ReferralCode  // userID -> code
	referrals    map[string]*referral.Referral       // id -> referral
	byReferee    map[string]*referral.Referral       // refereeID -> referral
	payouts      map[string]*referral.ReferralPayout // id -> payout
	recentCounts map[string]int                      // userID -> recent payout count
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		codes:        make(map[string]*referral.ReferralCode),
		codesByCode:  make(map[string]*referral.ReferralCode),
		codesByUser:  make(map[string]*referral.ReferralCode),
		referrals:    make(map[string]*referral.Referral),
		byReferee:    make(map[string]*referral.Referral),
		payouts:      make(map[string]*referral.ReferralPayout),
		recentCounts: make(map[string]int),
	}
}

func (f *fakeRepo) InsertReferralCode(_ context.Context, rc *referral.ReferralCode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *rc
	f.codes[rc.ID] = &cp
	f.codesByCode[rc.Code] = &cp
	f.codesByUser[rc.UserID] = &cp
	return nil
}

func (f *fakeRepo) FindReferralCodeByCode(_ context.Context, code string) (*referral.ReferralCode, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rc, ok := f.codesByCode[code]
	if !ok {
		return nil, referral.ErrCodeNotFound
	}
	cp := *rc
	return &cp, nil
}

func (f *fakeRepo) FindReferralCodeByUser(_ context.Context, userID string) (*referral.ReferralCode, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rc, ok := f.codesByUser[userID]
	if !ok {
		return nil, referral.ErrCodeNotFound
	}
	cp := *rc
	return &cp, nil
}

func (f *fakeRepo) IncrementCodeUseCount(_ context.Context, codeID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if rc, ok := f.codes[codeID]; ok {
		rc.UseCount++
		f.codesByCode[rc.Code] = rc
		f.codesByUser[rc.UserID] = rc
	}
	return nil
}

func (f *fakeRepo) InsertReferral(_ context.Context, ref *referral.Referral) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *ref
	f.referrals[ref.ID] = &cp
	f.byReferee[ref.RefereeID] = &cp
	return nil
}

func (f *fakeRepo) UpdateReferralStatus(_ context.Context, id string, status referral.ReferralStatus) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	ref, ok := f.referrals[id]
	if !ok {
		return errors.New("referral not found")
	}
	ref.Status = status
	f.byReferee[ref.RefereeID] = ref
	return nil
}

func (f *fakeRepo) FindReferralByID(_ context.Context, id string) (*referral.Referral, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ref, ok := f.referrals[id]
	if !ok {
		return nil, errors.New("referral not found")
	}
	cp := *ref
	return &cp, nil
}

func (f *fakeRepo) FindReferralByReferee(_ context.Context, userID string) (*referral.Referral, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ref, ok := f.byReferee[userID]
	if !ok {
		return nil, nil
	}
	cp := *ref
	return &cp, nil
}

func (f *fakeRepo) CountRecentPayouts(_ context.Context, userID string) (int, error) {
	return f.recentCounts[userID], nil
}

func (f *fakeRepo) InsertReferralPayout(_ context.Context, p *referral.ReferralPayout) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *p
	f.payouts[p.ID] = &cp
	return nil
}

func (f *fakeRepo) UpdatePayoutStatus(_ context.Context, id string, status referral.PayoutStatus, stripeID *string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.payouts[id]
	if !ok {
		return errors.New("payout not found")
	}
	p.Status = status
	if stripeID != nil {
		p.StripeTransferID = stripeID
	}
	return nil
}

func (f *fakeRepo) FindPayoutByID(_ context.Context, id string) (*referral.ReferralPayout, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.payouts[id]
	if !ok {
		return nil, errors.New("payout not found")
	}
	cp := *p
	return &cp, nil
}

// fakeStripe is a no-op Stripe adapter.
type fakeStripe struct {
	transferID string
	err        error
}

func (fs *fakeStripe) PayoutHost(_ context.Context, _ int64, _ string, _ string) (string, error) {
	return fs.transferID, fs.err
}

// fakeUserAccounts returns a preset stripe account ID.
type fakeUserAccounts struct {
	accounts map[string]string
}

func (f *fakeUserAccounts) GetStripeAccountID(_ context.Context, userID string) (string, error) {
	return f.accounts[userID], nil
}

// fakeFraudSignals controls shared-device and shared-wifi results.
type fakeFraudSignals struct {
	sharedDevice bool
	sharedWiFi   bool
}

func (f *fakeFraudSignals) SharedDeviceFingerprint(_ context.Context, _, _ string) (bool, error) {
	return f.sharedDevice, nil
}

func (f *fakeFraudSignals) SharedWiFiBSSID(_ context.Context, _, _ string) (bool, error) {
	return f.sharedWiFi, nil
}

func (f *fakeRepo) ListReferralsByReferrer(_ context.Context, _ string, _, _ int) ([]*referral.Referral, error) {
	return nil, nil
}

func (f *fakeRepo) ListAllReferralsPaginated(_ context.Context, _ referral.ListReferralsFilter) ([]*referral.Referral, error) {
	return nil, nil
}

func (f *fakeRepo) GetStats(_ context.Context) (*referral.ReferralStats, error) {
	return &referral.ReferralStats{}, nil
}

// newTestService builds a Service wired to fakes.
func newTestService(repo *fakeRepo, fraudSignals *fakeFraudSignals) *referral.Service {
	stripe := &fakeStripe{transferID: "tr_test"}
	accounts := &fakeUserAccounts{accounts: map[string]string{
		"userA": "acct_A",
		"userB": "acct_B",
	}}
	return referral.NewServiceWithFakes(repo, stripe, accounts, fraudSignals, nil)
}

// --- tests ---

func TestGenerateCode_CreatesNewCode(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, nil)

	rc, err := svc.GenerateCode(context.Background(), "userA")
	require.NoError(t, err)
	assert.Len(t, rc.Code, 8)
	assert.Equal(t, "userA", rc.UserID)
}

func TestGenerateCode_Idempotent(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, nil)

	rc1, err := svc.GenerateCode(context.Background(), "userA")
	require.NoError(t, err)

	rc2, err := svc.GenerateCode(context.Background(), "userA")
	require.NoError(t, err)
	assert.Equal(t, rc1.Code, rc2.Code)
}

func TestApplyReferralCode_Success(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, &fakeFraudSignals{})

	rc, err := svc.GenerateCode(context.Background(), "userA")
	require.NoError(t, err)

	ref, err := svc.ApplyReferralCode(context.Background(), "userB", rc.Code)
	require.NoError(t, err)
	assert.Equal(t, "userA", ref.ReferrerID)
	assert.Equal(t, "userB", ref.RefereeID)
	assert.Equal(t, referral.ReferralStatusSignedUp, ref.Status)
}

func TestApplyReferralCode_SelfReferral(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, &fakeFraudSignals{})

	rc, err := svc.GenerateCode(context.Background(), "userA")
	require.NoError(t, err)

	_, err = svc.ApplyReferralCode(context.Background(), "userA", rc.Code)
	assert.ErrorIs(t, err, referral.ErrSelfReferral)
}

func TestApplyReferralCode_CodeNotFound(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, &fakeFraudSignals{})

	_, err := svc.ApplyReferralCode(context.Background(), "userB", "NOTEXIST")
	assert.ErrorIs(t, err, referral.ErrCodeNotFound)
}

func TestApplyReferralCode_CodeExpired(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, &fakeFraudSignals{})

	expired := time.Now().Add(-24 * time.Hour)
	rc := &referral.ReferralCode{
		ID: "rc1", Code: "EXPIRED1", UserID: "userA",
		ExpiresAt: &expired, MaxUses: 0, UseCount: 0, CreatedAt: time.Now(),
	}
	require.NoError(t, repo.InsertReferralCode(context.Background(), rc))

	_, err := svc.ApplyReferralCode(context.Background(), "userB", "EXPIRED1")
	assert.ErrorIs(t, err, referral.ErrCodeExpired)
}

func TestApplyReferralCode_MaxUsesExhausted(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, &fakeFraudSignals{})

	rc := &referral.ReferralCode{
		ID: "rc1", Code: "MAXUSED1", UserID: "userA",
		MaxUses: 1, UseCount: 1, CreatedAt: time.Now(),
	}
	require.NoError(t, repo.InsertReferralCode(context.Background(), rc))

	_, err := svc.ApplyReferralCode(context.Background(), "userB", "MAXUSED1")
	assert.ErrorIs(t, err, referral.ErrCodeExhausted)
}

func TestApplyReferralCode_BlockedSharedDevice(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, &fakeFraudSignals{sharedDevice: true})

	rc, err := svc.GenerateCode(context.Background(), "userA")
	require.NoError(t, err)

	_, err = svc.ApplyReferralCode(context.Background(), "userB", rc.Code)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fraud")
}

func TestOnFirstRentalCompleted_AdvancesStatus(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, &fakeFraudSignals{})

	// Set up a referral.
	rc := &referral.ReferralCode{ID: "rc1", Code: "ABCD1234", UserID: "userA", CreatedAt: time.Now()}
	require.NoError(t, repo.InsertReferralCode(context.Background(), rc))
	ref := &referral.Referral{
		ID: "ref1", ReferralCodeID: "rc1", ReferrerID: "userA", RefereeID: "userB",
		Status: referral.ReferralStatusSignedUp, CreatedAt: time.Now(),
	}
	require.NoError(t, repo.InsertReferral(context.Background(), ref))

	svc.OnFirstRentalCompleted(context.Background(), "userB")

	updated, err := repo.FindReferralByReferee(context.Background(), "userB")
	require.NoError(t, err)
	assert.Equal(t, referral.ReferralStatusFirstRentalCompleted, updated.Status)
}

func TestOnFirstRentalCompleted_NoReferral(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, &fakeFraudSignals{})
	// Should not panic or error when user has no referral.
	svc.OnFirstRentalCompleted(context.Background(), "userZ")
}
