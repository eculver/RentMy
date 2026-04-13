package photodiff

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

	agentrouter "github.com/Brett2thered/RentMy/backend/internal/agent/router"
	"github.com/Brett2thered/RentMy/backend/internal/media"
	"github.com/Brett2thered/RentMy/backend/internal/platform/cv"
	locals3 "github.com/Brett2thered/RentMy/backend/internal/platform/s3"
)

// Service orchestrates the two-stage photo diff pipeline:
// Stage 1: CV preprocessing via cv-service sidecar
// Stage 2: LLM structural comparison via model router
type Service struct {
	repo        *Repository
	mediaRepo   media.RepositoryInterface
	cvClient    *cv.Client
	modelRouter *agentrouter.AnthropicRouter
	s3Client    *locals3.Client
}

// NewService creates a PhotoDiffService with all required dependencies.
func NewService(
	repo *Repository,
	mediaRepo media.RepositoryInterface,
	cvClient *cv.Client,
	modelRouter *agentrouter.AnthropicRouter,
	s3Client *locals3.Client,
) *Service {
	return &Service{
		repo:        repo,
		mediaRepo:   mediaRepo,
		cvClient:    cvClient,
		modelRouter: modelRouter,
		s3Client:    s3Client,
	}
}

// RunDiff executes the full two-stage photo diff pipeline for a transaction.
func (s *Service) RunDiff(ctx context.Context, transactionID string) (*PhotoDiff, error) {
	slog.Info("photodiff: starting pipeline", "transactionId", transactionID)

	// Step 1: Fetch check-in and check-out media from the database.
	allMedia, err := s.mediaRepo.FindByTransactionID(ctx, transactionID)
	if err != nil {
		return nil, fmt.Errorf("photodiff: fetch media: %w", err)
	}

	var checkinMedia, checkoutMedia []*media.Media
	for _, m := range allMedia {
		switch m.MediaType {
		case media.MediaTypeCheckIn:
			checkinMedia = append(checkinMedia, m)
		case media.MediaTypeCheckOut:
			checkoutMedia = append(checkoutMedia, m)
		}
	}

	if len(checkinMedia) == 0 {
		return s.storeInconclusive(ctx, transactionID, "no check-in photos available")
	}
	if len(checkoutMedia) == 0 {
		return s.storeInconclusive(ctx, transactionID, "no check-out photos available")
	}

	// Step 2: Download original images from S3.
	checkinBytes, err := s.downloadImages(ctx, checkinMedia)
	if err != nil {
		return nil, fmt.Errorf("photodiff: download check-in images: %w", err)
	}
	checkoutBytes, err := s.downloadImages(ctx, checkoutMedia)
	if err != nil {
		return nil, fmt.Errorf("photodiff: download check-out images: %w", err)
	}

	// Build orientation metadata.
	orientations := s.buildOrientations(checkinMedia, checkoutMedia)

	// Step 3 (Stage 1): Call cv-service for preprocessing.
	var preprocessResult *cv.PreprocessResult
	if s.cvClient != nil {
		preprocessResult, err = s.cvClient.Preprocess(ctx, checkinBytes, checkoutBytes, orientations)
		if err != nil {
			slog.Warn("photodiff: cv-service preprocessing failed, falling back to raw images",
				"transactionId", transactionID, "error", err)
		}
	}

	// Step 4 (Stage 2): Send paired images to LLM for structural comparison.
	if s.modelRouter == nil {
		return s.storeInconclusive(ctx, transactionID, "model router not available")
	}

	images, pairsCompared := s.buildLLMImages(preprocessResult, checkinBytes, checkoutBytes)
	if len(images) == 0 {
		return s.storeInconclusive(ctx, transactionID, "no image pairs to compare")
	}

	systemPrompt, promptVersion, err := s.modelRouter.RenderPrompt("photodiff", PhotoDiffPromptData{
		PairsCount: pairsCompared,
	})
	if err != nil {
		return nil, fmt.Errorf("photodiff: render prompt: %w", err)
	}

	userPrompt := fmt.Sprintf(
		"Compare these %d paired check-in and check-out photo(s) of a rental item. "+
			"For each pair, the check-in photo comes first followed by the check-out photo. "+
			"Analyze the structural condition of the item across all pairs.",
		pairsCompared,
	)

	output, err := s.modelRouter.Route(ctx, agentrouter.RouteInput{
		Task:         agentrouter.TaskPhotoDiffComparison,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Images:       images,
		MaxTokens:    2048,
	})
	if err != nil {
		return nil, fmt.Errorf("photodiff: LLM comparison: %w", err)
	}

	// Step 5: Parse LLM response.
	comparison, err := parseLLMResponse(output.Content)
	if err != nil {
		slog.Warn("photodiff: failed to parse LLM response, marking inconclusive",
			"transactionId", transactionID, "error", err, "raw", output.Content)
		return s.storeInconclusive(ctx, transactionID, "failed to parse LLM response")
	}

	// Step 6: Store result.
	result := DiffResult(comparison.Classification)
	if !ValidResults[result] {
		slog.Warn("photodiff: LLM returned invalid classification, marking inconclusive",
			"transactionId", transactionID, "classification", comparison.Classification)
		return s.storeInconclusive(ctx, transactionID, "invalid classification from LLM")
	}

	if err := s.repo.UpdateDiffResult(ctx, transactionID, result, comparison.Confidence); err != nil {
		return nil, fmt.Errorf("photodiff: store result: %w", err)
	}

	pd := &PhotoDiff{
		TransactionID: transactionID,
		Result:        result,
		Confidence:    comparison.Confidence,
		PairsCompared: pairsCompared,
		Details:       comparison.Details,
		PromptVersion: promptVersion,
		Model:         output.Model,
	}

	slog.Info("photodiff: pipeline complete",
		"transactionId", transactionID,
		"result", result,
		"confidence", comparison.Confidence,
		"pairs", pairsCompared,
	)

	return pd, nil
}

// GetResult returns the stored photo diff result for a transaction.
func (s *Service) GetResult(ctx context.Context, transactionID string) (*PhotoDiff, error) {
	return s.repo.GetDiffResult(ctx, transactionID)
}

func (s *Service) storeInconclusive(ctx context.Context, transactionID, reason string) (*PhotoDiff, error) {
	slog.Warn("photodiff: marking inconclusive", "transactionId", transactionID, "reason", reason)
	if err := s.repo.UpdateDiffResult(ctx, transactionID, ResultInconclusive, 0.0); err != nil {
		return nil, fmt.Errorf("photodiff: store inconclusive: %w", err)
	}
	return &PhotoDiff{
		TransactionID: transactionID,
		Result:        ResultInconclusive,
		Confidence:    0.0,
		Details:       reason,
	}, nil
}

func (s *Service) downloadImages(ctx context.Context, mediaItems []*media.Media) ([][]byte, error) {
	result := make([][]byte, 0, len(mediaItems))
	for _, m := range mediaItems {
		bucket, key := parseS3URL(m.OriginalURL)
		if bucket == "" || key == "" {
			return nil, fmt.Errorf("invalid S3 URL: %s", m.OriginalURL)
		}
		reader, err := s.s3Client.Download(ctx, bucket, key)
		if err != nil {
			return nil, fmt.Errorf("download %s: %w", m.ID, err)
		}
		data, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", m.ID, err)
		}
		result = append(result, data)
	}
	return result, nil
}

func (s *Service) buildOrientations(checkin, checkout []*media.Media) []cv.OrientationMeta {
	var orientations []cv.OrientationMeta
	for _, m := range checkin {
		if m.OrientationRoll != nil {
			orientations = append(orientations, cv.OrientationMeta{
				Type:  "checkin",
				Roll:  *m.OrientationRoll,
				Pitch: deref(m.OrientationPitch),
				Yaw:   deref(m.OrientationYaw),
			})
		}
	}
	for _, m := range checkout {
		if m.OrientationRoll != nil {
			orientations = append(orientations, cv.OrientationMeta{
				Type:  "checkout",
				Roll:  *m.OrientationRoll,
				Pitch: deref(m.OrientationPitch),
				Yaw:   deref(m.OrientationYaw),
			})
		}
	}
	return orientations
}

func (s *Service) buildLLMImages(preprocessResult *cv.PreprocessResult, checkinBytes, checkoutBytes [][]byte) ([]agentrouter.ImageInput, int) {
	if preprocessResult != nil && len(preprocessResult.Pairs) > 0 {
		var images []agentrouter.ImageInput
		for _, pair := range preprocessResult.Pairs {
			ciData, err := base64.StdEncoding.DecodeString(pair.CheckinImage)
			if err != nil {
				continue
			}
			coData, err := base64.StdEncoding.DecodeString(pair.CheckoutImage)
			if err != nil {
				continue
			}
			images = append(images,
				agentrouter.ImageInput{MediaType: "image/jpeg", Data: ciData},
				agentrouter.ImageInput{MediaType: "image/jpeg", Data: coData},
			)
		}
		return images, len(preprocessResult.Pairs)
	}

	// Fallback: pair by index order using raw images.
	n := len(checkinBytes)
	if len(checkoutBytes) < n {
		n = len(checkoutBytes)
	}
	var images []agentrouter.ImageInput
	for i := 0; i < n; i++ {
		images = append(images,
			agentrouter.ImageInput{MediaType: "image/jpeg", Data: checkinBytes[i]},
			agentrouter.ImageInput{MediaType: "image/jpeg", Data: checkoutBytes[i]},
		)
	}
	return images, n
}

func parseLLMResponse(content string) (*LLMComparisonResponse, error) {
	content = strings.TrimSpace(content)
	// Strip markdown code fences if present.
	if strings.HasPrefix(content, "```") {
		lines := strings.Split(content, "\n")
		if len(lines) > 2 {
			content = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}

	var resp LLMComparisonResponse
	if err := json.Unmarshal([]byte(content), &resp); err != nil {
		return nil, fmt.Errorf("parse LLM JSON: %w", err)
	}
	if resp.Classification == "" {
		return nil, fmt.Errorf("empty classification in LLM response")
	}
	return &resp, nil
}

// parseS3URL extracts bucket and key from a URL like "http://localhost:9002/media-originals/abc123".
func parseS3URL(url string) (bucket, key string) {
	// Find the path portion after the host.
	idx := strings.Index(url, "://")
	if idx < 0 {
		return "", ""
	}
	path := url[idx+3:]
	// Skip the host portion.
	slashIdx := strings.Index(path, "/")
	if slashIdx < 0 {
		return "", ""
	}
	path = path[slashIdx+1:]
	// Split into bucket/key.
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func deref(p *float32) float32 {
	if p == nil {
		return 0
	}
	return *p
}
