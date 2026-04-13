package media

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/disintegration/imaging"

	"github.com/Brett2thered/RentMy/backend/internal/platform/ulid"
)

const (
	bucketOriginals  = "media-originals"
	bucketThumbnails = "media-thumbnails"

	// maxLongestSide is the longest dimension in pixels for generated thumbnails.
	maxLongestSide = 800

	// maxUploadBytes is the enforced per-upload size limit (10 MB).
	MaxUploadBytes int64 = 10 << 20
)

// StorageClient is the subset of S3 operations required by Service.
type StorageClient interface {
	Upload(ctx context.Context, bucket, key string, body io.Reader, contentType string) error
}

// RepositoryInterface declares the persistence operations required by Service.
type RepositoryInterface interface {
	Insert(ctx context.Context, m *Media) (*Media, error)
	FindByID(ctx context.Context, id string) (*Media, error)
	FindByListingID(ctx context.Context, listingID string) ([]*Media, error)
	FindByTransactionID(ctx context.Context, transactionID string) ([]*Media, error)
}

// Service implements photo upload, thumbnail generation, and media retrieval.
type Service struct {
	repo       RepositoryInterface
	storage    StorageClient
	storageURL string // public base URL for constructing object URLs, e.g. "http://localhost:9002"
}

// NewService constructs a Service backed by the concrete Repository.
func NewService(repo *Repository, storage StorageClient, storageURL string) *Service {
	return NewServiceWithInterfaces(repo, storage, storageURL)
}

// NewServiceWithInterfaces constructs a Service with interface-typed dependencies,
// useful for testing with fakes.
func NewServiceWithInterfaces(repo RepositoryInterface, storage StorageClient, storageURL string) *Service {
	return &Service{
		repo:       repo,
		storage:    storage,
		storageURL: storageURL,
	}
}

// Upload validates the image, stores original and thumbnail in S3, and persists
// metadata in Postgres. The caller must enforce MaxUploadBytes before calling.
func (s *Service) Upload(ctx context.Context, imageData []byte, contentType string, in UploadInput) (*Media, error) {
	if len(imageData) == 0 {
		return nil, fmt.Errorf("image data is empty")
	}

	mediaType := in.MediaType
	if mediaType == "" {
		mediaType = MediaTypeListingPhoto
	}

	id := ulid.New()

	// Upload original.
	originalKey := id
	if err := s.storage.Upload(ctx, bucketOriginals, originalKey, bytes.NewReader(imageData), contentType); err != nil {
		return nil, fmt.Errorf("upload original: %w", err)
	}
	originalURL := s.objectURL(bucketOriginals, originalKey)

	// Generate thumbnail.
	thumb, err := generateThumbnail(imageData)
	if err != nil {
		return nil, fmt.Errorf("generate thumbnail: %w", err)
	}

	thumbnailKey := id
	if err := s.storage.Upload(ctx, bucketThumbnails, thumbnailKey, bytes.NewReader(thumb), "image/jpeg"); err != nil {
		return nil, fmt.Errorf("upload thumbnail: %w", err)
	}
	thumbnailURL := s.objectURL(bucketThumbnails, thumbnailKey)

	m := &Media{
		ID:           id,
		MediaType:    mediaType,
		OriginalURL:  originalURL,
		ThumbnailURL: &thumbnailURL,
		DeviceID:     in.DeviceID,
		CapturedAt:   in.CapturedAt,
		GpsLat:       in.GpsLat,
		GpsLng:       in.GpsLng,
	}
	if in.Orientation != nil {
		m.OrientationRoll = in.Orientation.Roll
		m.OrientationPitch = in.Orientation.Pitch
		m.OrientationYaw = in.Orientation.Yaw
	}

	inserted, err := s.repo.Insert(ctx, m)
	if err != nil {
		return nil, fmt.Errorf("persist media: %w", err)
	}
	return inserted, nil
}

// GetByID returns a single media record.
func (s *Service) GetByID(ctx context.Context, id string) (*Media, error) {
	m, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get media: %w", err)
	}
	return m, nil
}

// GetByListingID returns all media records attached to a listing.
func (s *Service) GetByListingID(ctx context.Context, listingID string) ([]*Media, error) {
	media, err := s.repo.FindByListingID(ctx, listingID)
	if err != nil {
		return nil, fmt.Errorf("get media by listing: %w", err)
	}
	return media, nil
}

// objectURL constructs the public URL for an S3 object.
func (s *Service) objectURL(bucket, key string) string {
	return fmt.Sprintf("%s/%s/%s", s.storageURL, bucket, key)
}

// generateThumbnail decodes imageData, resizes it so the longest side is at
// most maxLongestSide pixels, and re-encodes it as JPEG.
func generateThumbnail(imageData []byte) ([]byte, error) {
	src, err := imaging.Decode(bytes.NewReader(imageData), imaging.AutoOrientation(true))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	if w > maxLongestSide || h > maxLongestSide {
		src = imaging.Fit(src, maxLongestSide, maxLongestSide, imaging.Lanczos)
	}

	var buf bytes.Buffer
	if err := imaging.Encode(&buf, src, imaging.JPEG, imaging.JPEGQuality(85)); err != nil {
		return nil, fmt.Errorf("encode thumbnail: %w", err)
	}
	return buf.Bytes(), nil
}

