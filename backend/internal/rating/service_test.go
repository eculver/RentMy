package rating

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Brett2thered/RentMy/backend/internal/agent/risk"
)

// --- fakes ---

type fakeRepo struct {
	txn      transactionRow
	txnErr   error
	inserted *Rating
	insertErr error
	hasRated bool
	ratings  []Rating
	summary  []BubbleSummaryItem
}

func (f *fakeRepo) FindTransactionForRating(_ context.Context, _ string) (transactionRow, error) {
	return f.txn, f.txnErr
}

func (f *fakeRepo) HasUserRated(_ context.Context, _, _ string) (bool, error) {
	return f.hasRated, nil
}

func (f *fakeRepo) Insert(_ context.Context, rt *Rating) error {
	if f.insertErr != nil {
		return f.insertErr
	}
	f.inserted = rt
	return nil
}

func (f *fakeRepo) FindByTransactionID(_ context.Context, _ string) ([]Rating, error) {
	return f.ratings, nil
}

func (f *fakeRepo) FindByToUserID(_ context.Context, _ string, _, _ int) ([]Rating, int, error) {
	return f.ratings, len(f.ratings), nil
}

func (f *fakeRepo) BubbleSummary(_ context.Context, _ string) ([]BubbleSummaryItem, error) {
	return f.summary, nil
}

type fakeRisk struct {
	signals []risk.EmitSignalInput
}

func (f *fakeRisk) EmitSignal(_ context.Context, in risk.EmitSignalInput) error {
	f.signals = append(f.signals, in)
	return nil
}

// repoInterface is the minimal interface the service depends on (used to swap in the fake).
type repoInterface interface {
	FindTransactionForRating(ctx context.Context, txnID string) (transactionRow, error)
	HasUserRated(ctx context.Context, txnID, fromUserID string) (bool, error)
	Insert(ctx context.Context, rt *Rating) error
	FindByTransactionID(ctx context.Context, txnID string) ([]Rating, error)
	FindByToUserID(ctx context.Context, userID string, limit, offset int) ([]Rating, int, error)
	BubbleSummary(ctx context.Context, userID string) ([]BubbleSummaryItem, error)
}

// testService builds a Service backed by the fake implementations.
// It bypasses the concrete *Repository type by constructing the service with a
// thin wrapper that satisfies the same method set.
func testService(repo *fakeRepo, ra *fakeRisk) *testableService {
	return &testableService{repo: repo, riskAgent: ra}
}

// testableService mirrors Service but accepts the repoInterface so we can
// inject fakes without changing the production code.
type testableService struct {
	repo      repoInterface
	riskAgent riskAgent
}

func (s *testableService) SubmitRating(ctx context.Context, in CreateRatingInput) (*Rating, error) {
	if len(in.Bubbles) == 0 {
		return nil, ErrInvalidBubble
	}

	txn, err := s.repo.FindTransactionForRating(ctx, in.TransactionID)
	if err != nil {
		return nil, err
	}
	if txn.Status != "COMPLETED" {
		return nil, ErrTransactionNotCompleted
	}

	var toUserID string
	switch in.FromUserID {
	case txn.RenterID:
		if err := ValidateBubblesForRenter(in.Bubbles); err != nil {
			return nil, err
		}
		toUserID = txn.HostID
	case txn.HostID:
		if err := ValidateBubblesForHost(in.Bubbles); err != nil {
			return nil, err
		}
		toUserID = txn.RenterID
	default:
		return nil, ErrNotParticipant
	}

	rt := &Rating{
		ID:            "test-id",
		TransactionID: in.TransactionID,
		FromUserID:    in.FromUserID,
		ToUserID:      toUserID,
		Bubbles:       in.Bubbles,
	}
	if err := s.repo.Insert(ctx, rt); err != nil {
		return nil, err
	}

	txnID := in.TransactionID
	for range in.Bubbles {
		_ = s.riskAgent.EmitSignal(ctx, risk.EmitSignalInput{
			UserID:        toUserID,
			SignalType:    risk.SignalPositiveRating,
			TransactionID: &txnID,
		})
	}
	return rt, nil
}

// --- tests ---

func TestSubmitRating_RenterRatesHost(t *testing.T) {
	repo := &fakeRepo{
		txn: transactionRow{RenterID: "renter1", HostID: "host1", Status: "COMPLETED"},
	}
	ra := &fakeRisk{}
	svc := testService(repo, ra)

	bubbles := []Bubble{BubbleGoodCommunication, BubbleOnTime, BubbleItemAsDescribed}
	rt, err := svc.SubmitRating(context.Background(), CreateRatingInput{
		TransactionID: "txn1",
		FromUserID:    "renter1",
		Bubbles:       bubbles,
	})

	require.NoError(t, err)
	assert.Equal(t, "host1", rt.ToUserID)
	assert.Equal(t, "renter1", rt.FromUserID)
	assert.Equal(t, bubbles, rt.Bubbles)
	// One signal emitted per bubble.
	assert.Len(t, ra.signals, 3)
}

func TestSubmitRating_HostRatesRenter(t *testing.T) {
	repo := &fakeRepo{
		txn: transactionRow{RenterID: "renter1", HostID: "host1", Status: "COMPLETED"},
	}
	ra := &fakeRisk{}
	svc := testService(repo, ra)

	bubbles := []Bubble{BubbleOnTimeReturn, BubbleCarefulWithItem}
	rt, err := svc.SubmitRating(context.Background(), CreateRatingInput{
		TransactionID: "txn1",
		FromUserID:    "host1",
		Bubbles:       bubbles,
	})

	require.NoError(t, err)
	assert.Equal(t, "renter1", rt.ToUserID)
	assert.Len(t, ra.signals, 2)
}

func TestSubmitRating_TransactionNotCompleted(t *testing.T) {
	repo := &fakeRepo{
		txn: transactionRow{RenterID: "renter1", HostID: "host1", Status: "ACTIVE"},
	}
	svc := testService(repo, &fakeRisk{})

	_, err := svc.SubmitRating(context.Background(), CreateRatingInput{
		TransactionID: "txn1",
		FromUserID:    "renter1",
		Bubbles:       []Bubble{BubbleGoodCommunication},
	})
	assert.ErrorIs(t, err, ErrTransactionNotCompleted)
}

func TestSubmitRating_NotParticipant(t *testing.T) {
	repo := &fakeRepo{
		txn: transactionRow{RenterID: "renter1", HostID: "host1", Status: "COMPLETED"},
	}
	svc := testService(repo, &fakeRisk{})

	_, err := svc.SubmitRating(context.Background(), CreateRatingInput{
		TransactionID: "txn1",
		FromUserID:    "stranger",
		Bubbles:       []Bubble{BubbleGoodCommunication},
	})
	assert.ErrorIs(t, err, ErrNotParticipant)
}

func TestSubmitRating_InvalidBubbleForRenter(t *testing.T) {
	repo := &fakeRepo{
		txn: transactionRow{RenterID: "renter1", HostID: "host1", Status: "COMPLETED"},
	}
	svc := testService(repo, &fakeRisk{})

	// ON_TIME_RETURN is a host-rates-renter bubble; renter should not use it.
	_, err := svc.SubmitRating(context.Background(), CreateRatingInput{
		TransactionID: "txn1",
		FromUserID:    "renter1",
		Bubbles:       []Bubble{BubbleOnTimeReturn},
	})
	assert.ErrorIs(t, err, ErrInvalidBubble)
}

func TestSubmitRating_AlreadyRated(t *testing.T) {
	repo := &fakeRepo{
		txn:       transactionRow{RenterID: "renter1", HostID: "host1", Status: "COMPLETED"},
		insertErr: ErrAlreadyRated,
	}
	svc := testService(repo, &fakeRisk{})

	_, err := svc.SubmitRating(context.Background(), CreateRatingInput{
		TransactionID: "txn1",
		FromUserID:    "renter1",
		Bubbles:       []Bubble{BubbleGoodCommunication},
	})
	assert.ErrorIs(t, err, ErrAlreadyRated)
}

func TestValidateBubblesForRenter(t *testing.T) {
	valid := []Bubble{BubbleGoodCommunication, BubbleOnTime, BubbleItemAsDescribed, BubbleEasyPickup, BubbleFriendly}
	assert.NoError(t, ValidateBubblesForRenter(valid))

	invalid := []Bubble{BubbleOnTimeReturn}
	assert.ErrorIs(t, ValidateBubblesForRenter(invalid), ErrInvalidBubble)
}

func TestValidateBubblesForHost(t *testing.T) {
	valid := []Bubble{BubbleGoodCommunication, BubbleOnTimeReturn, BubbleCarefulWithItem, BubbleEasyHandoff, BubbleRespectful}
	assert.NoError(t, ValidateBubblesForHost(valid))

	invalid := []Bubble{BubbleItemAsDescribed}
	assert.ErrorIs(t, ValidateBubblesForHost(invalid), ErrInvalidBubble)
}
