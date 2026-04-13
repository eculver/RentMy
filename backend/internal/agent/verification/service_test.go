package verification

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Brett2thered/RentMy/backend/internal/agent/decision"
	"github.com/Brett2thered/RentMy/backend/internal/user"
)

// --- fakes ---

type fakeStripeAdapter struct {
	sessionID  string
	sessionURL string
	eventType  string
	dataRaw    json.RawMessage
	createErr  error
	webhookErr error
}

func (f *fakeStripeAdapter) CreateVerificationSession(_ context.Context, _ string) (StripeSessionResult, error) {
	if f.createErr != nil {
		return StripeSessionResult{}, f.createErr
	}
	return StripeSessionResult{SessionID: f.sessionID, SessionURL: f.sessionURL}, nil
}

func (f *fakeStripeAdapter) ConstructWebhookEvent(_ []byte, _ string) (string, json.RawMessage, error) {
	if f.webhookErr != nil {
		return "", nil, f.webhookErr
	}
	return f.eventType, f.dataRaw, nil
}

type fakeUserService struct {
	profile            *user.User
	identityStatusSet  user.IdentityStatus
	reputationDelta    int
	updateIdentityErr  error
	addReputationErr   error
}

func (f *fakeUserService) GetProfile(_ context.Context, _ string) (*user.User, error) {
	return f.profile, nil
}

func (f *fakeUserService) UpdateIdentityStatus(_ context.Context, _ string, status user.IdentityStatus) error {
	f.identityStatusSet = status
	return f.updateIdentityErr
}

func (f *fakeUserService) AddReputationScore(_ context.Context, _ string, delta int) error {
	f.reputationDelta += delta
	return f.addReputationErr
}

type fakeDecisionService struct {
	recorded []decision.CreateDecisionInput
}

func (f *fakeDecisionService) RecordDecision(_ context.Context, in decision.CreateDecisionInput) (*decision.AgentDecision, error) {
	f.recorded = append(f.recorded, in)
	return &decision.AgentDecision{
		ID:        "test-decision",
		AgentType: in.AgentType,
		CreatedAt: time.Now(),
	}, nil
}

// fakeSvc wraps Service with a fake decision service for testing.
// We test via the public methods on Service, using an in-memory repository.

// inMemoryRepo is an in-memory implementation of the Repository for testing.
type inMemoryRepo struct {
	attempts map[string]*VerificationAttempt // keyed by ID
}

func newInMemoryRepo() *inMemoryRepo {
	return &inMemoryRepo{attempts: make(map[string]*VerificationAttempt)}
}

func (r *inMemoryRepo) Insert(_ context.Context, a *VerificationAttempt) (*VerificationAttempt, error) {
	cp := *a
	r.attempts[a.ID] = &cp
	return &cp, nil
}

func (r *inMemoryRepo) FindByUserID(_ context.Context, userID string) (*VerificationAttempt, error) {
	var latest *VerificationAttempt
	for _, a := range r.attempts {
		if a.UserID != userID {
			continue
		}
		if latest == nil || a.CreatedAt.After(latest.CreatedAt) {
			cp := *a
			latest = &cp
		}
	}
	if latest == nil {
		return nil, ErrAttemptNotFound
	}
	return latest, nil
}

func (r *inMemoryRepo) FindBySessionID(_ context.Context, sessionID string) (*VerificationAttempt, error) {
	for _, a := range r.attempts {
		if a.StripeSessionID == sessionID {
			cp := *a
			return &cp, nil
		}
	}
	return nil, ErrAttemptNotFound
}

func (r *inMemoryRepo) UpdateStatus(_ context.Context, id string, in updateStatusInput) error {
	a, ok := r.attempts[id]
	if !ok {
		return ErrAttemptNotFound
	}
	a.Status = in.Status
	a.StripeStatus = in.StripeStatus
	a.StripeReason = in.StripeReason
	a.DocumentType = in.DocumentType
	a.SelfieMatchScore = in.SelfieMatchScore
	a.FraudIndicators = in.FraudIndicators
	a.Decision = in.Decision
	a.Confidence = in.Confidence
	a.EscalationReason = in.EscalationReason
	a.Model = in.Model
	a.PromptVersion = in.PromptVersion
	a.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *inMemoryRepo) IncrementRetryCount(_ context.Context, id string) error {
	a, ok := r.attempts[id]
	if !ok {
		return ErrAttemptNotFound
	}
	a.RetryCount++
	return nil
}

// serviceForTest builds a Service wired to in-memory fakes.
// riverClient is nil — StartVerification handles this gracefully (logs a warning).
func serviceForTest(stripeAdapter StripeIdentityAdapter, userSvc UserService) (*Service, *inMemoryRepo) {
	repo := newInMemoryRepo()
	svc := NewService(repo, stripeAdapter, nil, &fakeDecisionService{}, userSvc, nil)
	return svc, repo
}

// --- tests ---

func TestHandleWebhook_Verified_AutoApproves(t *testing.T) {
	sessID := "vs_test_123"
	userID := "user_abc"

	stripeAdapter := &fakeStripeAdapter{
		sessionID:  sessID,
		sessionURL: "https://verify.stripe.com/start/test",
	}
	userSvc := &fakeUserService{
		profile: &user.User{
			ID:             userID,
			IdentityStatus: user.IdentityStatusPending,
		},
	}

	svc, repo := serviceForTest(stripeAdapter, userSvc)

	// Pre-insert a PENDING attempt so HandleWebhook can find it by session ID.
	now := time.Now().UTC()
	_, err := repo.Insert(context.Background(), &VerificationAttempt{
		ID:              "attempt_1",
		UserID:          userID,
		StripeSessionID: sessID,
		Status:          VerificationStatusPending,
		FraudIndicators: []string{},
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	require.NoError(t, err)

	// Build a fake "verified" webhook event.
	sessionObj := map[string]any{
		"id":     sessID,
		"status": "verified",
	}
	dataRaw, err := json.Marshal(sessionObj)
	require.NoError(t, err)

	stripeAdapter.eventType = "identity.verification_session.verified"
	stripeAdapter.dataRaw = dataRaw

	err = svc.HandleWebhook(context.Background(), []byte("body"), "sig")
	require.NoError(t, err)

	// Attempt should be VERIFIED.
	updated, err := repo.FindBySessionID(context.Background(), sessID)
	require.NoError(t, err)
	assert.Equal(t, VerificationStatusVerified, updated.Status)

	// User identity status should be updated.
	assert.Equal(t, user.IdentityStatusVerified, userSvc.identityStatusSet)

	// +50 reputation bonus should be awarded (first-time verification).
	assert.Equal(t, 50, userSvc.reputationDelta)
}

func TestHandleWebhook_RequiresInput_FraudCode_AutoRejects(t *testing.T) {
	sessID := "vs_fraud_456"
	userID := "user_def"

	stripeAdapter := &fakeStripeAdapter{}
	userSvc := &fakeUserService{
		profile: &user.User{
			ID:             userID,
			IdentityStatus: user.IdentityStatusPending,
		},
	}

	svc, repo := serviceForTest(stripeAdapter, userSvc)

	now := time.Now().UTC()
	_, err := repo.Insert(context.Background(), &VerificationAttempt{
		ID:              "attempt_2",
		UserID:          userID,
		StripeSessionID: sessID,
		Status:          VerificationStatusPending,
		FraudIndicators: []string{},
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	require.NoError(t, err)

	sessionObj := map[string]any{
		"id":     sessID,
		"status": "requires_input",
		"last_error": map[string]any{
			"code":   "selfie_manipulated",
			"reason": "Selfie shows signs of manipulation",
		},
	}
	dataRaw, _ := json.Marshal(sessionObj)
	stripeAdapter.eventType = "identity.verification_session.requires_input"
	stripeAdapter.dataRaw = dataRaw

	err = svc.HandleWebhook(context.Background(), []byte("body"), "sig")
	require.NoError(t, err)

	updated, err := repo.FindBySessionID(context.Background(), sessID)
	require.NoError(t, err)
	assert.Equal(t, VerificationStatusRejected, updated.Status)
	assert.Equal(t, user.IdentityStatusRejected, userSvc.identityStatusSet)
	// No reputation bonus for rejected users.
	assert.Equal(t, 0, userSvc.reputationDelta)
}

func TestHandleWebhook_RequiresInput_EdgeCase_EscalatesWhenNoRouter(t *testing.T) {
	sessID := "vs_edge_789"
	userID := "user_ghi"

	stripeAdapter := &fakeStripeAdapter{}
	userSvc := &fakeUserService{
		profile: &user.User{
			ID:             userID,
			IdentityStatus: user.IdentityStatusPending,
		},
	}

	svc, repo := serviceForTest(stripeAdapter, userSvc)
	// modelRouter is nil by default in serviceForTest — edge cases should escalate.

	now := time.Now().UTC()
	_, err := repo.Insert(context.Background(), &VerificationAttempt{
		ID:              "attempt_3",
		UserID:          userID,
		StripeSessionID: sessID,
		Status:          VerificationStatusPending,
		FraudIndicators: []string{},
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	require.NoError(t, err)

	sessionObj := map[string]any{
		"id":     sessID,
		"status": "requires_input",
		"last_error": map[string]any{
			"code":   "document_expired",
			"reason": "Document has expired",
		},
	}
	dataRaw, _ := json.Marshal(sessionObj)
	stripeAdapter.eventType = "identity.verification_session.requires_input"
	stripeAdapter.dataRaw = dataRaw

	err = svc.HandleWebhook(context.Background(), []byte("body"), "sig")
	require.NoError(t, err)

	updated, err := repo.FindBySessionID(context.Background(), sessID)
	require.NoError(t, err)
	assert.Equal(t, VerificationStatusEscalated, updated.Status)
	assert.NotNil(t, updated.EscalationReason)
	// User identity_status should remain PENDING (not updated on escalation).
	assert.Equal(t, user.IdentityStatus(""), userSvc.identityStatusSet)
}

func TestHandleWebhook_Canceled(t *testing.T) {
	sessID := "vs_cancel_999"
	userID := "user_jkl"

	stripeAdapter := &fakeStripeAdapter{}
	userSvc := &fakeUserService{
		profile: &user.User{ID: userID, IdentityStatus: user.IdentityStatusPending},
	}

	svc, repo := serviceForTest(stripeAdapter, userSvc)

	now := time.Now().UTC()
	_, err := repo.Insert(context.Background(), &VerificationAttempt{
		ID:              "attempt_4",
		UserID:          userID,
		StripeSessionID: sessID,
		Status:          VerificationStatusPending,
		FraudIndicators: []string{},
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	require.NoError(t, err)

	sessionObj := map[string]any{"id": sessID, "status": "canceled"}
	dataRaw, _ := json.Marshal(sessionObj)
	stripeAdapter.eventType = "identity.verification_session.canceled"
	stripeAdapter.dataRaw = dataRaw

	err = svc.HandleWebhook(context.Background(), []byte("body"), "sig")
	require.NoError(t, err)

	updated, err := repo.FindBySessionID(context.Background(), sessID)
	require.NoError(t, err)
	assert.Equal(t, VerificationStatusCanceled, updated.Status)
}

func TestGetStatus_NoAttempt_ReturnsPending(t *testing.T) {
	stripeAdapter := &fakeStripeAdapter{}
	userSvc := &fakeUserService{
		profile: &user.User{ID: "user_new", IdentityStatus: user.IdentityStatusPending},
	}
	svc, _ := serviceForTest(stripeAdapter, userSvc)

	result, err := svc.GetStatus(context.Background(), "user_new")
	require.NoError(t, err)
	assert.Equal(t, VerificationStatusPending, result.Status)
	assert.Equal(t, "PENDING", result.IdentityStatus)
}

func TestIsFraudCode(t *testing.T) {
	assert.True(t, isFraudCode("selfie_manipulated"))
	assert.True(t, isFraudCode("document_fraudulent"))
	assert.False(t, isFraudCode("document_expired"))
	assert.False(t, isFraudCode(""))
}
