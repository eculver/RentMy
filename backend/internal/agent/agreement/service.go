package agreement

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/giits/rentmy/backend/internal/agent/decision"
	"github.com/giits/rentmy/backend/internal/agent/router"
	"github.com/giits/rentmy/backend/internal/platform/ulid"
)


// DecisionService is the subset of decision.Service the AgreementAgent needs.
type DecisionService interface {
	RecordDecision(ctx context.Context, in decision.CreateDecisionInput) (*decision.AgentDecision, error)
}

// bannedPatterns is a list of regexes that item-specific clauses must not match.
// These protect the immutable sections of the base agreement.
var bannedPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\blimit.*liability\b`),
	regexp.MustCompile(`(?i)\bno.*liability\b`),
	regexp.MustCompile(`(?i)\bwaive.*arbitration\b`),
	regexp.MustCompile(`(?i)\bnot.*subject.*arbitration\b`),
	regexp.MustCompile(`(?i)\bno.*hold\b`),
	regexp.MustCompile(`(?i)\brefund\b`),
	regexp.MustCompile(`(?i)\bno.*fee\b`),
	regexp.MustCompile(`(?i)\bwaive.*fee\b`),
}

// Service is the AgreementAgent business logic.
type Service struct {
	repo        *Repository              // Postgres persistence
	modelRouter *router.AnthropicRouter  // nil when ANTHROPIC_API_KEY is absent
	decisionSvc DecisionService
	// promptsDir is the base directory for prompt files (defaults to project prompts/).
	promptsDir string
}

// NewService creates an AgreementAgent Service.
// modelRouter may be nil in dev/test environments — clause generation will be skipped.
func NewService(
	repo *Repository,
	modelRouter *router.AnthropicRouter,
	decisionSvc DecisionService,
) *Service {
	return &Service{
		repo:        repo,
		modelRouter: modelRouter,
		decisionSvc: decisionSvc,
		promptsDir:  defaultPromptsDir(),
	}
}

// defaultPromptsDir resolves the prompts/ directory relative to the repo root.
// In tests the caller may override s.promptsDir directly.
func defaultPromptsDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "prompts"
	}
	// file is …/backend/internal/agent/agreement/service.go — walk up 3 levels to backend/
	base := filepath.Join(filepath.Dir(file), "..", "..", "..", "prompts")
	abs, err := filepath.Abs(base)
	if err != nil {
		return "prompts"
	}
	return abs
}

// GenerateAgreement creates an agreement for a transaction:
//  1. Load base template
//  2. Fetch item context from the listing
//  3. Call Sonnet to generate item-specific clauses
//  4. Validate clauses against guardrails
//  5. Merge into final agreement and persist
//
// Idempotent: if an agreement already exists for the transaction, returns it.
func (s *Service) GenerateAgreement(ctx context.Context, transactionID string) (*Agreement, error) {
	// Idempotency: return existing agreement if already generated.
	existing, err := s.repo.FindByTransactionID(ctx, transactionID)
	if err == nil {
		return existing, nil
	}
	if err != ErrAgreementNotFound {
		return nil, fmt.Errorf("agreement: checking existing: %w", err)
	}

	// Load base template.
	tmpl, tmplVersion, err := s.loadBaseTemplate()
	if err != nil {
		return nil, fmt.Errorf("agreement: loading base template: %w", err)
	}

	// Fetch listing context.
	listingID, title, description, estimatedValueCents, err := s.repo.GetListingForTransaction(ctx, transactionID)
	if err != nil {
		return nil, err
	}
	category, _ := s.repo.GetAppraisalCategory(ctx, listingID) // best-effort

	// Generate item-specific clauses via Sonnet (skip if no router).
	var customClauses []Clause
	var promptVersion, modelUsed string
	var guardrailWarnings []string

	if s.modelRouter != nil {
		customClauses, promptVersion, modelUsed, guardrailWarnings, err = s.generateClauses(ctx, agreementPromptData{
			ItemName:            title,
			Category:            category,
			EstimatedValueCents: estimatedValueCents,
			ConditionNotes:      description,
		})
		if err != nil {
			slog.Warn("agreement: clause generation failed, proceeding with base template only",
				"transactionId", transactionID, "error", err)
			customClauses = nil
		}
	} else {
		slog.Warn("agreement: no model router — using base template without item-specific clauses",
			"transactionId", transactionID)
	}

	// Inject custom clauses into the item_specific section of the base template.
	finalTemplate := mergeClauses(tmpl, customClauses)
	fullAgreementJSON, err := json.Marshal(finalTemplate)
	if err != nil {
		return nil, fmt.Errorf("agreement: marshalling final agreement: %w", err)
	}
	customClausesJSON, err := json.Marshal(customClauses)
	if err != nil {
		return nil, fmt.Errorf("agreement: marshalling custom clauses: %w", err)
	}
	warningsJSON, err := json.Marshal(guardrailWarnings)
	if err != nil {
		warningsJSON = []byte("[]")
	}

	var pvPtr, modelPtr *string
	if promptVersion != "" {
		pvPtr = &promptVersion
	}
	if modelUsed != "" {
		modelPtr = &modelUsed
	}

	a := &Agreement{
		ID:                ulid.New(),
		TransactionID:     transactionID,
		Version:           tmplVersion,
		FullAgreement:     fullAgreementJSON,
		CustomClauses:     customClausesJSON,
		PromptVersion:     pvPtr,
		Model:             modelPtr,
		GuardrailWarnings: warningsJSON,
	}
	inserted, err := s.repo.Insert(ctx, a)
	if err != nil {
		return nil, fmt.Errorf("agreement: persisting: %w", err)
	}

	// Write snapshot to the transactions.agreement_snapshot column.
	if snapErr := s.repo.UpdateAgreementSnapshot(ctx, transactionID, fullAgreementJSON); snapErr != nil {
		slog.Warn("agreement: failed to update transaction snapshot",
			"transactionId", transactionID, "error", snapErr)
	}

	// Record agent decision.
	s.recordDecision(ctx, decision.CreateDecisionInput{
		AgentType: decision.AgentTypeAgreement,
		Input: map[string]any{
			"transaction_id": transactionID,
			"listing_id":     listingID,
			"item_name":      title,
			"category":       category,
		},
		Decision: map[string]any{
			"clause_count":      len(customClauses),
			"guardrail_strips":  len(guardrailWarnings),
			"template_version":  tmplVersion,
		},
		Model:         modelPtr,
		PromptVersion: pvPtr,
	})

	slog.Info("agreement: generated",
		"transactionId", transactionID, "clauses", len(customClauses),
		"guardrailStrips", len(guardrailWarnings))
	return inserted, nil
}

// ValidateAcceptance records a party's acceptance of the agreement.
// Both the host and renter must accept (identified by userID matching transaction parties).
func (s *Service) ValidateAcceptance(ctx context.Context, transactionID, userID, ipAddress, deviceID string) error {
	a, err := s.repo.FindByTransactionID(ctx, transactionID)
	if err != nil {
		return err
	}

	// Verify the user is actually a party to this transaction.
	renterID, hostID, err := s.repo.GetTransactionParties(ctx, transactionID)
	if err != nil {
		return err
	}
	if userID != renterID && userID != hostID {
		return ErrNotParty
	}

	var ipPtr, devPtr *string
	if ipAddress != "" {
		ipPtr = &ipAddress
	}
	if deviceID != "" {
		devPtr = &deviceID
	}

	if _, err := s.repo.InsertAcceptance(ctx, a.ID, userID, ipPtr, devPtr); err != nil {
		return err
	}

	slog.Info("agreement: accepted", "transactionId", transactionID, "userId", userID)
	return nil
}

// GetAcceptanceStatus returns which parties have accepted the agreement.
func (s *Service) GetAcceptanceStatus(ctx context.Context, transactionID string) (*AcceptanceStatus, error) {
	a, err := s.repo.FindByTransactionID(ctx, transactionID)
	if err != nil {
		return nil, err
	}

	renterID, hostID, err := s.repo.GetTransactionParties(ctx, transactionID)
	if err != nil {
		return nil, err
	}

	accs, err := s.repo.FindAcceptances(ctx, a.ID)
	if err != nil {
		return nil, err
	}

	status := &AcceptanceStatus{
		AgreementID:   a.ID,
		TransactionID: transactionID,
	}
	for _, acc := range accs {
		t := acc.AcceptedAt
		if acc.UserID == hostID {
			status.HostAccepted = true
			status.HostAcceptedAt = &t
		}
		if acc.UserID == renterID {
			status.RenterAccepted = true
			status.RenterAcceptedAt = &t
		}
	}
	status.BothAccepted = status.HostAccepted && status.RenterAccepted
	return status, nil
}

// TriggerAgreement implements the booking.agreementSvc interface.
// It generates an agreement and discards the return value, suitable for fire-and-forget use.
func (s *Service) TriggerAgreement(ctx context.Context, transactionID string) error {
	_, err := s.GenerateAgreement(ctx, transactionID)
	return err
}

// GetAgreement returns the agreement for a transaction.
func (s *Service) GetAgreement(ctx context.Context, transactionID string) (*Agreement, error) {
	return s.repo.FindByTransactionID(ctx, transactionID)
}

// BothPartiesAccepted returns true when both host and renter have accepted.
// Used by the booking service to gate the ACTIVE transition.
func (s *Service) BothPartiesAccepted(ctx context.Context, transactionID string) (bool, error) {
	status, err := s.GetAcceptanceStatus(ctx, transactionID)
	if err != nil {
		return false, err
	}
	return status.BothAccepted, nil
}

// generateClauses calls Sonnet to produce item-specific clauses, then validates them.
func (s *Service) generateClauses(ctx context.Context, data agreementPromptData) (
	clauses []Clause, promptVersion, model string, warnings []string, err error,
) {
	rendered, pv, err := s.modelRouter.RenderPrompt("agreement", data)
	if err != nil {
		return nil, "", "", nil, fmt.Errorf("render prompt: %w", err)
	}

	out, err := s.modelRouter.Route(ctx, router.RouteInput{
		Task:       router.TaskCustomClauseGeneration,
		UserPrompt: rendered,
	})
	if err != nil {
		return nil, "", "", nil, fmt.Errorf("model call: %w", err)
	}

	var result clauseAIResult
	content := strings.TrimSpace(out.Content)
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, "", "", nil, fmt.Errorf("parse response: %w", err)
	}

	// Guardrail validation — strip clauses that violate base agreement invariants.
	var valid []Clause
	for _, c := range result.Clauses {
		if violation := guardrailCheck(c); violation != "" {
			warnings = append(warnings, fmt.Sprintf("stripped clause %q: %s", c.Title, violation))
			slog.Warn("agreement: guardrail stripped clause", "title", c.Title, "reason", violation)
			continue
		}
		valid = append(valid, c)
	}

	return valid, pv, out.Model, warnings, nil
}

// guardrailCheck returns a non-empty reason string if the clause violates guardrails.
func guardrailCheck(c Clause) string {
	combined := c.Title + " " + c.Text
	for _, re := range bannedPatterns {
		if re.MatchString(combined) {
			return fmt.Sprintf("matches banned pattern %q", re.String())
		}
	}
	return ""
}

// loadBaseTemplate reads and parses the latest base_template_v{N}.json from prompts/agreement/.
func (s *Service) loadBaseTemplate() (*baseTemplate, string, error) {
	// Find the highest-numbered base template.
	pattern := filepath.Join(s.promptsDir, "agreement", "base_template_v*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return nil, "", fmt.Errorf("no base template found at %s", pattern)
	}

	// Use the last match (sorted lexicographically — v1, v2, ... v9, v10 works for <10 versions).
	latest := matches[len(matches)-1]
	data, err := os.ReadFile(latest)
	if err != nil {
		return nil, "", fmt.Errorf("reading base template %s: %w", latest, err)
	}

	var tmpl baseTemplate
	if err := json.Unmarshal(data, &tmpl); err != nil {
		return nil, "", fmt.Errorf("parsing base template: %w", err)
	}

	// Extract version from filename: base_template_v1.json → "1"
	base := filepath.Base(latest)
	version := strings.TrimSuffix(strings.TrimPrefix(base, "base_template_v"), ".json")

	return &tmpl, version, nil
}

// mergeClauses injects the custom clauses into the item_specific_clauses section.
func mergeClauses(tmpl *baseTemplate, clauses []Clause) *baseTemplate {
	result := *tmpl
	sections := make([]baseSection, len(tmpl.Sections))
	copy(sections, tmpl.Sections)

	for i, sec := range sections {
		if sec.Dynamic && sec.Source == "agreement_agent" {
			sections[i].Clauses = clauses
		}
	}
	result.Sections = sections
	return &result
}

func (s *Service) recordDecision(ctx context.Context, in decision.CreateDecisionInput) {
	if s.decisionSvc == nil {
		return
	}
	if _, err := s.decisionSvc.RecordDecision(ctx, in); err != nil {
		slog.Warn("agreement: failed to record agent decision", "error", err)
	}
}
