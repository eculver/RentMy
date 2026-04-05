package verification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/giits/rentmy/backend/internal/agent/decision"
	"github.com/giits/rentmy/backend/internal/agent/router"
	"github.com/giits/rentmy/backend/internal/platform/ulid"
	"github.com/giits/rentmy/backend/internal/user"
)

// attemptRepository is the persistence interface for VerificationAttempt records.
type attemptRepository interface {
	Insert(ctx context.Context, a *VerificationAttempt) (*VerificationAttempt, error)
	FindByUserID(ctx context.Context, userID string) (*VerificationAttempt, error)
	FindBySessionID(ctx context.Context, sessionID string) (*VerificationAttempt, error)
	UpdateStatus(ctx context.Context, id string, in updateStatusInput) error
	IncrementRetryCount(ctx context.Context, id string) error
}

// StripeIdentityAdapter abstracts the Stripe Identity API calls.
// The concrete implementation lives in stripe.go; tests may use fakes.
type StripeIdentityAdapter interface {
	// CreateVerificationSession creates a new Stripe Identity VerificationSession.
	CreateVerificationSession(ctx context.Context, userID string) (StripeSessionResult, error)
	// ConstructWebhookEvent validates a Stripe webhook signature and returns the event type
	// and the raw event.data.object JSON for further unmarshalling.
	ConstructWebhookEvent(body []byte, signature string) (eventType string, dataRaw json.RawMessage, err error)
}

// StripeSessionResult holds the data returned when creating a new verification session.
type StripeSessionResult struct {
	SessionID          string
	SessionURL         string
	EphemeralKeySecret string // Stripe client_secret; used by the mobile Stripe Identity SDK
}

// UserService is the subset of user.Service that the VerificationAgent needs.
type UserService interface {
	GetProfile(ctx context.Context, userID string) (*user.User, error)
	UpdateIdentityStatus(ctx context.Context, userID string, status user.IdentityStatus) error
	AddReputationScore(ctx context.Context, userID string, delta int) error
}

// ErrAlreadyVerified is returned when a user tries to start KYC but is already verified.
var ErrAlreadyVerified = errors.New("user is already verified")

// DecisionService is the subset of decision.Service that the VerificationAgent needs.
type DecisionService interface {
	RecordDecision(ctx context.Context, in decision.CreateDecisionInput) (*decision.AgentDecision, error)
}

// Service is the VerificationAgent business logic.
type Service struct {
	repo        attemptRepository
	stripe      StripeIdentityAdapter
	modelRouter *router.AnthropicRouter // nil when ANTHROPIC_API_KEY is absent
	decisionSvc DecisionService
	userSvc     UserService
	riverClient *river.Client[pgx.Tx]
}

// NewService creates a VerificationAgent Service.
// modelRouter may be nil in dev/test environments without an API key.
func NewService(
	repo attemptRepository,
	stripe StripeIdentityAdapter,
	modelRouter *router.AnthropicRouter,
	decisionSvc DecisionService,
	userSvc UserService,
	riverClient *river.Client[pgx.Tx],
) *Service {
	return &Service{
		repo:        repo,
		stripe:      stripe,
		modelRouter: modelRouter,
		decisionSvc: decisionSvc,
		userSvc:     userSvc,
		riverClient: riverClient,
	}
}

// StartVerification initiates KYC for an authenticated user.
// If the user is already VERIFIED, returns ErrAlreadyVerified.
// If there is an existing PENDING attempt, returns the existing session ID (idempotency).
// Otherwise creates a new Stripe Identity VerificationSession.
func (s *Service) StartVerification(ctx context.Context, userID string) (StartVerificationResult, error) {
	u, err := s.userSvc.GetProfile(ctx, userID)
	if err != nil {
		return StartVerificationResult{}, fmt.Errorf("verification: getting user: %w", err)
	}
	if u.IdentityStatus == user.IdentityStatusVerified {
		return StartVerificationResult{}, ErrAlreadyVerified
	}

	// Idempotency: if there is already a PENDING attempt, return the existing session.
	existing, err := s.repo.FindByUserID(ctx, userID)
	if err == nil && existing.Status == VerificationStatusPending {
		// The Stripe URL is only valid immediately after creation, so we return
		// an empty URL; the client must call Stripe to refresh it if needed.
		return StartVerificationResult{
			SessionID:  existing.StripeSessionID,
			SessionURL: "",
		}, nil
	}

	session, err := s.stripe.CreateVerificationSession(ctx, userID)
	if err != nil {
		return StartVerificationResult{}, fmt.Errorf("verification: creating stripe session: %w", err)
	}

	now := time.Now().UTC()
	attempt := &VerificationAttempt{
		ID:              ulid.New(),
		UserID:          userID,
		StripeSessionID: session.SessionID,
		Status:          VerificationStatusPending,
		FraudIndicators: []string{},
		RetryCount:      0,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if _, err := s.repo.Insert(ctx, attempt); err != nil {
		return StartVerificationResult{}, fmt.Errorf("verification: recording attempt: %w", err)
	}

	// Enqueue a timeout job. Failure here is non-fatal: the session was created successfully.
	if s.riverClient != nil {
		if _, err := s.riverClient.Insert(ctx, VerificationTimeoutJobArgs{
			AttemptID: attempt.ID,
			UserID:    userID,
			SessionID: session.SessionID,
		}, &river.InsertOpts{
			ScheduledAt: time.Now().Add(10 * time.Minute),
		}); err != nil {
			slog.Warn("verification: failed to enqueue timeout job", "attemptId", attempt.ID, "error", err)
		}
	}

	slog.Info("verification session created", "userId", userID, "sessionId", session.SessionID)
	return StartVerificationResult{
		SessionID:          session.SessionID,
		SessionURL:         session.SessionURL,
		EphemeralKeySecret: session.EphemeralKeySecret,
	}, nil
}

// GetStatus returns the current KYC status for a user.
func (s *Service) GetStatus(ctx context.Context, userID string) (VerificationStatusResult, error) {
	u, err := s.userSvc.GetProfile(ctx, userID)
	if err != nil {
		return VerificationStatusResult{}, fmt.Errorf("verification: getting user: %w", err)
	}

	result := VerificationStatusResult{
		IdentityStatus: string(u.IdentityStatus),
		Status:         VerificationStatusPending,
	}

	attempt, err := s.repo.FindByUserID(ctx, userID)
	if errors.Is(err, ErrAttemptNotFound) {
		return result, nil
	}
	if err != nil {
		return VerificationStatusResult{}, fmt.Errorf("verification: finding attempt: %w", err)
	}

	result.Status = attempt.Status
	result.EscalationReason = attempt.EscalationReason
	return result, nil
}

// HandleWebhook processes a Stripe Identity webhook event.
// body is the raw request body; signature is the Stripe-Signature header value.
func (s *Service) HandleWebhook(ctx context.Context, body []byte, signature string) error {
	eventType, dataRaw, err := s.stripe.ConstructWebhookEvent(body, signature)
	if err != nil {
		return fmt.Errorf("verification: constructing webhook event: %w", err)
	}

	switch eventType {
	case "identity.verification_session.verified":
		return s.handleVerified(ctx, dataRaw)
	case "identity.verification_session.requires_input":
		return s.handleRequiresInput(ctx, dataRaw)
	case "identity.verification_session.canceled":
		return s.handleCanceled(ctx, dataRaw)
	default:
		slog.Debug("verification: ignoring stripe identity event", "type", eventType)
		return nil
	}
}

// stripeSessionObject holds the minimal fields we extract from webhook event data.
type stripeSessionObject struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	LastError *struct {
		Code   string `json:"code"`
		Reason string `json:"reason"`
	} `json:"last_error"`
	LastVerificationReport *struct {
		Document *struct {
			Status string `json:"status"`
			Type   string `json:"type"`
		} `json:"document"`
	} `json:"last_verification_report"`
}

func (s *Service) handleVerified(ctx context.Context, dataRaw json.RawMessage) error {
	var sess stripeSessionObject
	if err := json.Unmarshal(dataRaw, &sess); err != nil {
		return fmt.Errorf("verification: unmarshalling verified session: %w", err)
	}

	attempt, err := s.repo.FindBySessionID(ctx, sess.ID)
	if err != nil {
		return fmt.Errorf("verification: finding attempt for session %s: %w", sess.ID, err)
	}

	docType := ""
	if sess.LastVerificationReport != nil && sess.LastVerificationReport.Document != nil {
		docType = sess.LastVerificationReport.Document.Type
	}

	decisionStr := "APPROVE"
	conf := 1.0
	model := "stripe_auto"
	pv := "n/a"
	in := updateStatusInput{
		Status:          VerificationStatusVerified,
		StripeStatus:    sess.Status,
		DocumentType:    docType,
		FraudIndicators: []string{},
		Decision:        &decisionStr,
		Confidence:      &conf,
		Model:           &model,
		PromptVersion:   &pv,
	}
	if err := s.repo.UpdateStatus(ctx, attempt.ID, in); err != nil {
		return fmt.Errorf("verification: updating attempt to verified: %w", err)
	}

	prevUser, _ := s.userSvc.GetProfile(ctx, attempt.UserID)
	if err := s.userSvc.UpdateIdentityStatus(ctx, attempt.UserID, user.IdentityStatusVerified); err != nil {
		return fmt.Errorf("verification: updating identity status: %w", err)
	}
	if prevUser != nil && prevUser.IdentityStatus != user.IdentityStatusVerified {
		if err := s.userSvc.AddReputationScore(ctx, attempt.UserID, 50); err != nil {
			slog.Warn("verification: failed to award KYC reputation bonus", "userId", attempt.UserID, "error", err)
		}
	}

	uid := attempt.UserID
	s.recordDecision(ctx, decision.CreateDecisionInput{
		AgentType:     decision.AgentTypeVerification,
		UserID:        &uid,
		Input:         map[string]any{"session_id": sess.ID, "stripe_status": sess.Status},
		Decision:      map[string]any{"decision": "APPROVE", "confidence": 1.0, "source": "stripe_auto"},
		Model:         &model,
		PromptVersion: &pv,
		Confidence:    &conf,
	})

	slog.Info("verification: user auto-approved by Stripe", "userId", attempt.UserID, "sessionId", sess.ID)
	return nil
}

func (s *Service) handleRequiresInput(ctx context.Context, dataRaw json.RawMessage) error {
	var sess stripeSessionObject
	if err := json.Unmarshal(dataRaw, &sess); err != nil {
		return fmt.Errorf("verification: unmarshalling requires_input session: %w", err)
	}

	attempt, err := s.repo.FindBySessionID(ctx, sess.ID)
	if err != nil {
		return fmt.Errorf("verification: finding attempt for session %s: %w", sess.ID, err)
	}

	errCode := ""
	errReason := ""
	if sess.LastError != nil {
		errCode = sess.LastError.Code
		errReason = sess.LastError.Reason
	}

	if isFraudCode(errCode) {
		return s.autoReject(ctx, attempt, sess, errCode, errReason)
	}
	return s.interpretEdgeCase(ctx, attempt, sess, errCode, errReason)
}

func (s *Service) handleCanceled(ctx context.Context, dataRaw json.RawMessage) error {
	var sess stripeSessionObject
	if err := json.Unmarshal(dataRaw, &sess); err != nil {
		return fmt.Errorf("verification: unmarshalling canceled session: %w", err)
	}

	attempt, err := s.repo.FindBySessionID(ctx, sess.ID)
	if err != nil {
		return fmt.Errorf("verification: finding attempt for session %s: %w", sess.ID, err)
	}

	in := updateStatusInput{
		Status:          VerificationStatusCanceled,
		StripeStatus:    "canceled",
		FraudIndicators: []string{},
	}
	if err := s.repo.UpdateStatus(ctx, attempt.ID, in); err != nil {
		return fmt.Errorf("verification: updating attempt to canceled: %w", err)
	}

	slog.Info("verification: session canceled", "userId", attempt.UserID, "sessionId", sess.ID)
	return nil
}

func (s *Service) autoReject(ctx context.Context, attempt *VerificationAttempt, sess stripeSessionObject, errCode, errReason string) error {
	decisionStr := "REJECT"
	conf := 1.0
	model := "stripe_auto"
	pv := "n/a"
	in := updateStatusInput{
		Status:          VerificationStatusRejected,
		StripeStatus:    sess.Status,
		StripeReason:    errReason,
		FraudIndicators: []string{errCode},
		Decision:        &decisionStr,
		Confidence:      &conf,
		Model:           &model,
		PromptVersion:   &pv,
	}
	if err := s.repo.UpdateStatus(ctx, attempt.ID, in); err != nil {
		return fmt.Errorf("verification: updating attempt to rejected: %w", err)
	}
	if err := s.userSvc.UpdateIdentityStatus(ctx, attempt.UserID, user.IdentityStatusRejected); err != nil {
		return fmt.Errorf("verification: updating identity status to rejected: %w", err)
	}

	uid := attempt.UserID
	s.recordDecision(ctx, decision.CreateDecisionInput{
		AgentType:     decision.AgentTypeVerification,
		UserID:        &uid,
		Input:         map[string]any{"session_id": sess.ID, "error_code": errCode},
		Decision:      map[string]any{"decision": "REJECT", "confidence": 1.0, "reason": errReason},
		Model:         &model,
		PromptVersion: &pv,
		Confidence:    &conf,
	})

	slog.Info("verification: auto-rejected (fraud indicator)", "userId", attempt.UserID, "errorCode", errCode)
	return nil
}

func (s *Service) interpretEdgeCase(ctx context.Context, attempt *VerificationAttempt, sess stripeSessionObject, errCode, errReason string) error {
	if s.modelRouter == nil {
		// No AI available: escalate to human review.
		reason := "model_router_unavailable"
		return s.escalate(ctx, attempt, sess, reason, nil, nil)
	}

	docType := ""
	if sess.LastVerificationReport != nil && sess.LastVerificationReport.Document != nil {
		docType = sess.LastVerificationReport.Document.Type
	}
	fraudStr := "none"
	if errCode != "" {
		fraudStr = errCode
	}

	tmplData := interpretationInput{
		UserID:           attempt.UserID,
		SessionID:        sess.ID,
		StripeStatus:     sess.Status,
		StripeReason:     strings.TrimSpace(errReason),
		DocumentType:     docType,
		SelfieMatchScore: "unknown",
		FraudIndicators:  fraudStr,
	}

	rendered, promptVersion, err := s.modelRouter.RenderPrompt("verification", tmplData)
	if err != nil {
		slog.Warn("verification: failed to render prompt, escalating", "error", err)
		return s.escalate(ctx, attempt, sess, "prompt_render_failed", nil, nil)
	}

	out, err := s.modelRouter.Route(ctx, router.RouteInput{
		Task:       router.TaskKYCInterpretation,
		UserPrompt: rendered,
	})
	if err != nil {
		slog.Warn("verification: model call failed, escalating", "error", err)
		return s.escalate(ctx, attempt, sess, "model_call_failed", nil, nil)
	}
	out.PromptVersion = promptVersion

	var result interpretationResult
	if err := json.Unmarshal([]byte(out.Content), &result); err != nil {
		slog.Warn("verification: failed to parse model response, escalating",
			"content", out.Content, "error", err)
		return s.escalate(ctx, attempt, sess, "parse_failed", &out, nil)
	}

	return s.applyInterpretation(ctx, attempt, sess, result, out)
}

func (s *Service) applyInterpretation(
	ctx context.Context,
	attempt *VerificationAttempt,
	sess stripeSessionObject,
	result interpretationResult,
	out router.RouteOutput,
) error {
	model := out.Model
	pv := out.PromptVersion
	conf := result.Confidence

	switch result.Decision {
	case "APPROVE":
		decisionStr := "APPROVE"
		in := updateStatusInput{
			Status:          VerificationStatusVerified,
			StripeStatus:    sess.Status,
			FraudIndicators: []string{},
			Decision:        &decisionStr,
			Confidence:      &conf,
			Model:           &model,
			PromptVersion:   &pv,
		}
		if err := s.repo.UpdateStatus(ctx, attempt.ID, in); err != nil {
			return fmt.Errorf("verification: updating to verified after AI: %w", err)
		}
		prevUser, _ := s.userSvc.GetProfile(ctx, attempt.UserID)
		if err := s.userSvc.UpdateIdentityStatus(ctx, attempt.UserID, user.IdentityStatusVerified); err != nil {
			return fmt.Errorf("verification: updating identity status: %w", err)
		}
		if prevUser != nil && prevUser.IdentityStatus != user.IdentityStatusVerified {
			if err := s.userSvc.AddReputationScore(ctx, attempt.UserID, 50); err != nil {
				slog.Warn("verification: failed to award KYC reputation bonus",
					"userId", attempt.UserID, "error", err)
			}
		}

	case "REJECT":
		decisionStr := "REJECT"
		in := updateStatusInput{
			Status:          VerificationStatusRejected,
			StripeStatus:    sess.Status,
			FraudIndicators: []string{},
			Decision:        &decisionStr,
			Confidence:      &conf,
			Model:           &model,
			PromptVersion:   &pv,
		}
		if err := s.repo.UpdateStatus(ctx, attempt.ID, in); err != nil {
			return fmt.Errorf("verification: updating to rejected after AI: %w", err)
		}
		if err := s.userSvc.UpdateIdentityStatus(ctx, attempt.UserID, user.IdentityStatusRejected); err != nil {
			return fmt.Errorf("verification: updating identity status to rejected: %w", err)
		}

	default: // ESCALATE or unrecognised decision
		escReason := result.EscalationReason
		if escReason == "" {
			escReason = "low_confidence"
		}
		return s.escalate(ctx, attempt, sess, escReason, &out, &result)
	}

	uid := attempt.UserID
	s.recordDecision(ctx, decision.CreateDecisionInput{
		AgentType:     decision.AgentTypeVerification,
		UserID:        &uid,
		Input:         map[string]any{"session_id": sess.ID, "stripe_status": sess.Status},
		Decision:      map[string]any{"decision": result.Decision, "confidence": conf, "reasoning": result.Reasoning},
		Model:         &model,
		PromptVersion: &pv,
		Confidence:    &conf,
	})

	slog.Info("verification: AI decision applied",
		"userId", attempt.UserID, "decision", result.Decision, "confidence", conf, "model", model)
	return nil
}

func (s *Service) escalate(
	ctx context.Context,
	attempt *VerificationAttempt,
	sess stripeSessionObject,
	reason string,
	out *router.RouteOutput,
	result *interpretationResult,
) error {
	in := updateStatusInput{
		Status:           VerificationStatusEscalated,
		StripeStatus:     sess.Status,
		FraudIndicators:  []string{},
		EscalationReason: &reason,
	}
	var (
		model string
		pv    string
		conf  float64
	)
	if out != nil {
		model = out.Model
		pv = out.PromptVersion
		in.Model = &model
		in.PromptVersion = &pv
	}
	if result != nil {
		conf = result.Confidence
		decisionStr := result.Decision
		in.Decision = &decisionStr
		in.Confidence = &conf
	}

	if err := s.repo.UpdateStatus(ctx, attempt.ID, in); err != nil {
		return fmt.Errorf("verification: escalating attempt: %w", err)
	}

	uid := attempt.UserID
	s.recordDecision(ctx, decision.CreateDecisionInput{
		AgentType:        decision.AgentTypeVerification,
		UserID:           &uid,
		Input:            map[string]any{"session_id": sess.ID, "stripe_status": sess.Status},
		Decision:         map[string]any{"decision": "ESCALATE", "reason": reason},
		Model:            ptrString(model),
		PromptVersion:    ptrString(pv),
		Confidence:       ptrFloat(conf),
		Escalated:        true,
		EscalationReason: &reason,
	})

	slog.Info("verification: escalated to human review",
		"userId", attempt.UserID, "sessionId", sess.ID, "reason", reason)
	return nil
}

func (s *Service) recordDecision(ctx context.Context, in decision.CreateDecisionInput) {
	if _, err := s.decisionSvc.RecordDecision(ctx, in); err != nil {
		slog.Warn("verification: failed to record agent decision", "error", err)
	}
}

// isFraudCode returns true for Stripe error codes indicating document manipulation or identity fraud.
// These trigger auto-rejection without AI interpretation.
func isFraudCode(code string) bool {
	switch code {
	case "selfie_manipulated", "document_fraudulent":
		return true
	}
	return false
}

func ptrString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func ptrFloat(f float64) *float64 {
	if f == 0 {
		return nil
	}
	return &f
}
