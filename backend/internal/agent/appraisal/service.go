package appraisal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/Brett2thered/RentMy/backend/internal/agent/decision"
	"github.com/Brett2thered/RentMy/backend/internal/agent/router"
	"github.com/Brett2thered/RentMy/backend/internal/listing"
	"github.com/Brett2thered/RentMy/backend/internal/media"
	"github.com/Brett2thered/RentMy/backend/internal/platform/ulid"
)

// appraisalRepository is the persistence interface for Appraisal records.
type appraisalRepository interface {
	Insert(ctx context.Context, a *Appraisal) (*Appraisal, error)
	FindByListingID(ctx context.Context, listingID string) (*Appraisal, error)
	Update(ctx context.Context, id string, in updateInput) error
	UpdateOverride(ctx context.Context, id string, in updateOverrideInput) error
}

// ListingService is the subset of listing.Service that the AppraisalAgent needs.
type ListingService interface {
	Get(ctx context.Context, id string) (*listing.Listing, error)
	UpdateAppraisalResult(ctx context.Context, listingID string, in listing.AppraisalFieldsUpdate) error
}

// MediaService is the subset of media.Service that the AppraisalAgent needs.
type MediaService interface {
	GetByListingID(ctx context.Context, listingID string) ([]*media.Media, error)
}

// DecisionService is the subset of decision.Service that the AppraisalAgent needs.
type DecisionService interface {
	RecordDecision(ctx context.Context, in decision.CreateDecisionInput) (*decision.AgentDecision, error)
}

// ErrNoMedia is returned when a listing has no photos to appraise.
var ErrNoMedia = errors.New("listing has no photos to appraise")

// ErrAppraisalNotFound is returned when no appraisal exists for a listing.
var ErrAppraisalNotFound = errors.New("no appraisal found for this listing")

// Service is the AppraisalAgent business logic.
type Service struct {
	repo        appraisalRepository
	listingSvc  ListingService
	mediaSvc    MediaService
	modelRouter *router.AnthropicRouter // nil when ANTHROPIC_API_KEY is absent
	decisionSvc DecisionService
	riverClient *river.Client[pgx.Tx]
	httpClient  *http.Client
}

// NewService creates an AppraisalAgent Service.
// modelRouter and riverClient may be nil in dev/test environments.
func NewService(
	repo appraisalRepository,
	listingSvc ListingService,
	mediaSvc MediaService,
	modelRouter *router.AnthropicRouter,
	decisionSvc DecisionService,
	riverClient *river.Client[pgx.Tx],
) *Service {
	return &Service{
		repo:        repo,
		listingSvc:  listingSvc,
		mediaSvc:    mediaSvc,
		modelRouter: modelRouter,
		decisionSvc: decisionSvc,
		riverClient: riverClient,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

// SetDeps injects listing/media services and the River client into a pre-river
// service that was constructed with nil dependencies for early worker registration.
// Must be called before the service processes any jobs.
func (s *Service) SetDeps(listingSvc ListingService, mediaSvc MediaService, riverClient *river.Client[pgx.Tx]) {
	s.listingSvc = listingSvc
	s.mediaSvc = mediaSvc
	s.riverClient = riverClient
}

// EnqueueAppraisal enqueues an async AppraisalJob for the given listing.
// Implements listing.AppraisalEnqueuer.
// When riverClient is nil (dev without River), falls back to synchronous appraisal.
func (s *Service) EnqueueAppraisal(ctx context.Context, listingID string) error {
	if s.riverClient == nil {
		slog.Warn("appraisal: no riverClient — skipping appraisal enqueue", "listingId", listingID)
		return nil
	}
	_, err := s.riverClient.Insert(ctx, AppraisalJobArgs{ListingID: listingID}, nil)
	if err != nil {
		return fmt.Errorf("appraisal: enqueue job: %w", err)
	}
	slog.Info("appraisal: enqueued job", "listingId", listingID)
	return nil
}

// Appraise runs the full AI appraisal pipeline for a listing.
// It fetches listing media, sends images to Sonnet for identification + pricing,
// merges tags from Haiku, persists the result, and updates the listing fields.
func (s *Service) Appraise(ctx context.Context, listingID string) error {
	l, err := s.listingSvc.Get(ctx, listingID)
	if err != nil {
		return fmt.Errorf("appraisal: getting listing: %w", err)
	}

	// Upsert a PENDING appraisal row (idempotent on re-runs).
	existing, err := s.repo.FindByListingID(ctx, listingID)
	var appraisalID string
	if errors.Is(err, ErrNotFound) {
		a := &Appraisal{
			ID:        ulid.New(),
			ListingID: listingID,
			Status:    listing.AppraisalStatusPending,
			Tags:      []byte("[]"),
		}
		inserted, insertErr := s.repo.Insert(ctx, a)
		if insertErr != nil {
			return fmt.Errorf("appraisal: inserting pending record: %w", insertErr)
		}
		appraisalID = inserted.ID
	} else if err != nil {
		return fmt.Errorf("appraisal: checking existing record: %w", err)
	} else {
		appraisalID = existing.ID
	}

	// Fetch listing media.
	mediaItems, err := s.mediaSvc.GetByListingID(ctx, listingID)
	if err != nil {
		return s.markFailed(ctx, appraisalID, listingID, "failed to fetch media")
	}
	var photoMedia []*media.Media
	for _, m := range mediaItems {
		if m.MediaType == media.MediaTypeListingPhoto {
			photoMedia = append(photoMedia, m)
		}
	}
	if len(photoMedia) == 0 {
		return s.markFailed(ctx, appraisalID, listingID, "no listing photos found")
	}

	if s.modelRouter == nil {
		slog.Warn("appraisal: no model router — marking failed", "listingId", listingID)
		return s.markFailed(ctx, appraisalID, listingID, "model_router_unavailable")
	}

	// Download and encode images (up to 5 photos for vision).
	images, err := s.fetchImages(ctx, photoMedia, 5)
	if err != nil {
		return s.markFailed(ctx, appraisalID, listingID, "failed to download images")
	}

	// Render the appraisal prompt.
	rendered, promptVersion, err := s.modelRouter.RenderPrompt("appraisal", appraisalPromptData{})
	if err != nil {
		return s.markFailed(ctx, appraisalID, listingID, "prompt_render_failed")
	}

	// Call Sonnet with vision for item identification and pricing.
	out, err := s.modelRouter.Route(ctx, router.RouteInput{
		Task:       router.TaskItemIdentification,
		UserPrompt: rendered,
		Images:     images,
	})
	if err != nil {
		slog.Warn("appraisal: model call failed", "listingId", listingID, "error", err)
		return s.markFailed(ctx, appraisalID, listingID, "model_call_failed")
	}
	out.PromptVersion = promptVersion

	var aiResult appraisalAIResult
	if err := json.Unmarshal([]byte(out.Content), &aiResult); err != nil {
		slog.Warn("appraisal: failed to parse model response",
			"listingId", listingID, "content", out.Content, "error", err)
		return s.markFailed(ctx, appraisalID, listingID, "parse_failed")
	}

	// Run Haiku for additional semantic tags (text-only, cheaper).
	extraTags := s.fetchExtraTags(ctx, l, aiResult)
	mergedTags := mergeTags(aiResult.Tags, extraTags)
	tagsJSON, _ := json.Marshal(mergedTags)

	// Persist the appraisal result.
	itemName := aiResult.ItemName
	category := aiResult.Category
	condition := aiResult.Condition
	desc := aiResult.Description
	estCents := aiResult.EstimatedValueCents
	hourCents := aiResult.SuggestedPricePerHourCents
	dayCents := aiResult.SuggestedPricePerDayCents
	conf := aiResult.Confidence
	model := out.Model
	pv := out.PromptVersion

	if err := s.repo.Update(ctx, appraisalID, updateInput{
		Status:                     listing.AppraisalStatusComplete,
		ItemName:                   &itemName,
		Category:                   &category,
		Condition:                  &condition,
		EstimatedValueCents:        &estCents,
		SuggestedPricePerHourCents: &hourCents,
		SuggestedPricePerDayCents:  &dayCents,
		Description:                &desc,
		Tags:                       tagsJSON,
		Confidence:                 &conf,
		Model:                      &model,
		PromptVersion:              &pv,
	}); err != nil {
		return fmt.Errorf("appraisal: persisting result: %w", err)
	}

	// Push AI-generated fields back to the listing row.
	titlePtr := &itemName
	descPtr := &desc
	hourCentsCopy := hourCents
	dayCentsCopy := dayCents
	estCentsCopy := estCents
	if err := s.listingSvc.UpdateAppraisalResult(ctx, listingID, listing.AppraisalFieldsUpdate{
		AIGeneratedTags:            tagsJSON,
		EstimatedValueCents:        &estCentsCopy,
		SuggestedTitle:             titlePtr,
		SuggestedDescription:       descPtr,
		SuggestedPricePerHourCents: &hourCentsCopy,
		SuggestedPricePerDayCents:  &dayCentsCopy,
		AppraisalStatus:            listing.AppraisalStatusComplete,
	}); err != nil {
		slog.Warn("appraisal: failed to update listing fields", "listingId", listingID, "error", err)
	}

	// Record the agent decision.
	s.recordDecision(ctx, decision.CreateDecisionInput{
		AgentType: decision.AgentTypeAppraisal,
		Input: map[string]any{
			"listing_id":   listingID,
			"photo_count":  len(images),
			"prompt_version": promptVersion,
		},
		Decision: map[string]any{
			"item_name":    itemName,
			"category":     category,
			"estimated_value_cents": estCents,
			"confidence":   conf,
		},
		Model:         &model,
		PromptVersion: &pv,
		Confidence:    &conf,
	})

	slog.Info("appraisal: complete",
		"listingId", listingID, "itemName", itemName, "confidence", conf, "model", model)
	return nil
}

// ReviewOverride reviews a host's declared value that exceeds the AI estimate.
// The override is approved if the justification is compelling (rare/vintage item,
// documented proof of purchase, custom modifications).
func (s *Service) ReviewOverride(ctx context.Context, listingID string, req OverrideRequest) (*overrideAIResult, error) {
	existing, err := s.repo.FindByListingID(ctx, listingID)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrAppraisalNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("appraisal: finding appraisal: %w", err)
	}

	if s.modelRouter == nil {
		return nil, fmt.Errorf("appraisal: model router unavailable")
	}
	if existing.EstimatedValueCents == nil {
		return nil, fmt.Errorf("appraisal: no AI estimate available yet")
	}

	itemName := ""
	if existing.ItemName != nil {
		itemName = *existing.ItemName
	}
	category := ""
	if existing.Category != nil {
		category = *existing.Category
	}

	rendered, promptVersion, err := s.modelRouter.RenderPrompt("appraisal_override", overridePromptData{
		ItemName:          itemName,
		Category:          category,
		AIEstimateCents:   *existing.EstimatedValueCents,
		HostDeclaredCents: req.DeclaredValueCents,
		Justification:     req.Justification,
	})
	if err != nil {
		return nil, fmt.Errorf("appraisal: rendering override prompt: %w", err)
	}

	out, err := s.modelRouter.Route(ctx, router.RouteInput{
		Task:       router.TaskValueOverrideReview,
		UserPrompt: rendered,
	})
	if err != nil {
		return nil, fmt.Errorf("appraisal: override model call: %w", err)
	}
	out.PromptVersion = promptVersion

	var result overrideAIResult
	if err := json.Unmarshal([]byte(out.Content), &result); err != nil {
		return nil, fmt.Errorf("appraisal: parsing override response: %w", err)
	}

	// Persist the override decision.
	if err := s.repo.UpdateOverride(ctx, existing.ID, updateOverrideInput{
		OverrideApproved:  result.Approved,
		OverrideReasoning: result.Reasoning,
	}); err != nil {
		slog.Warn("appraisal: failed to persist override decision", "listingId", listingID, "error", err)
	}

	model := out.Model
	pv := out.PromptVersion
	conf := result.Confidence
	s.recordDecision(ctx, decision.CreateDecisionInput{
		AgentType: decision.AgentTypeAppraisal,
		Input: map[string]any{
			"listing_id":           listingID,
			"ai_estimate_cents":    *existing.EstimatedValueCents,
			"host_declared_cents":  req.DeclaredValueCents,
			"justification_length": len(req.Justification),
		},
		Decision: map[string]any{
			"override_approved": result.Approved,
			"reasoning":         result.Reasoning,
			"confidence":        conf,
		},
		Model:         &model,
		PromptVersion: &pv,
		Confidence:    &conf,
	})

	slog.Info("appraisal: override review complete",
		"listingId", listingID, "approved", result.Approved, "confidence", conf)
	return &result, nil
}

// GetAppraisal returns the appraisal record for a listing.
func (s *Service) GetAppraisal(ctx context.Context, listingID string) (*Appraisal, error) {
	a, err := s.repo.FindByListingID(ctx, listingID)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrAppraisalNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("appraisal: getting appraisal: %w", err)
	}
	return a, nil
}

// markFailed updates the appraisal and listing to FAILED status.
func (s *Service) markFailed(ctx context.Context, appraisalID, listingID, reason string) error {
	failedStatus := listing.AppraisalStatusFailed
	_ = s.repo.Update(ctx, appraisalID, updateInput{
		Status:        failedStatus,
		FailureReason: &reason,
	})
	_ = s.listingSvc.UpdateAppraisalResult(ctx, listingID, listing.AppraisalFieldsUpdate{
		AppraisalStatus: listing.AppraisalStatusFailed,
	})
	slog.Warn("appraisal: marked failed", "listingId", listingID, "reason", reason)
	return fmt.Errorf("appraisal: %s", reason)
}

// fetchImages downloads up to maxImages listing photos and returns them as ImageInput slices.
func (s *Service) fetchImages(ctx context.Context, photos []*media.Media, maxImages int) ([]router.ImageInput, error) {
	if len(photos) > maxImages {
		photos = photos[:maxImages]
	}

	var images []router.ImageInput
	for _, m := range photos {
		imgData, mediaType, err := s.downloadImage(ctx, m.OriginalURL)
		if err != nil {
			slog.Warn("appraisal: failed to download image, skipping",
				"url", m.OriginalURL, "error", err)
			continue
		}
		images = append(images, router.ImageInput{
			MediaType: mediaType,
			Data:      imgData,
		})
	}
	if len(images) == 0 {
		return nil, fmt.Errorf("no images could be downloaded")
	}
	return images, nil
}

// downloadImage fetches an image from a URL and returns its bytes and content type.
func (s *Service) downloadImage(ctx context.Context, url string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("fetch image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("unexpected status %d fetching image", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read image body: %w", err)
	}

	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "image/jpeg"
	}
	// Strip charset and other params, keep just the MIME type.
	if idx := strings.Index(ct, ";"); idx != -1 {
		ct = strings.TrimSpace(ct[:idx])
	}
	return data, ct, nil
}

// fetchExtraTags calls Haiku to generate additional semantic tags for the item.
// Returns an empty slice on any error (graceful degradation).
func (s *Service) fetchExtraTags(ctx context.Context, l *listing.Listing, ai appraisalAIResult) []string {
	if s.modelRouter == nil {
		return nil
	}

	prompt := fmt.Sprintf(
		"Generate 5 additional semantic search tags for this rental item. "+
			"Item: %s. Category: %s. Description: %s. "+
			"Return only a JSON array of strings, no explanation.",
		ai.ItemName, ai.Category, ai.Description,
	)

	out, err := s.modelRouter.Route(ctx, router.RouteInput{
		Task:       router.TaskTagGeneration,
		UserPrompt: prompt,
	})
	if err != nil {
		slog.Warn("appraisal: tag generation failed, using AI tags only", "error", err)
		return nil
	}

	var tags []string
	content := strings.TrimSpace(out.Content)
	if err := json.Unmarshal([]byte(content), &tags); err != nil {
		slog.Warn("appraisal: failed to parse extra tags", "content", content, "error", err)
		return nil
	}
	return tags
}

// mergeTags deduplicates and combines the primary and extra tag slices.
func mergeTags(primary, extra []string) []string {
	seen := make(map[string]struct{}, len(primary)+len(extra))
	result := make([]string, 0, len(primary)+len(extra))
	for _, t := range append(primary, extra...) {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" {
			continue
		}
		if _, ok := seen[t]; !ok {
			seen[t] = struct{}{}
			result = append(result, t)
		}
	}
	return result
}

func (s *Service) recordDecision(ctx context.Context, in decision.CreateDecisionInput) {
	if s.decisionSvc == nil {
		return
	}
	if _, err := s.decisionSvc.RecordDecision(ctx, in); err != nil {
		slog.Warn("appraisal: failed to record agent decision", "error", err)
	}
}
