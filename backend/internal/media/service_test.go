package media

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- fakes ---

type fakeRepo struct {
	inserted *Media
	findByID *Media
	findErr  error
}

func (f *fakeRepo) Insert(_ context.Context, m *Media) (*Media, error) {
	f.inserted = m
	return m, nil
}

func (f *fakeRepo) FindByID(_ context.Context, _ string) (*Media, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	return f.findByID, nil
}

func (f *fakeRepo) FindByListingID(_ context.Context, _ string) ([]*Media, error) {
	return nil, nil
}

func (f *fakeRepo) FindByTransactionID(_ context.Context, _ string) ([]*Media, error) {
	return nil, nil
}

type fakeStorage struct {
	uploads []uploadCall
}

type uploadCall struct {
	bucket      string
	key         string
	contentType string
}

func (f *fakeStorage) Upload(_ context.Context, bucket, key string, _ io.Reader, contentType string) error {
	f.uploads = append(f.uploads, uploadCall{bucket: bucket, key: key, contentType: contentType})
	return nil
}

// --- helpers ---

// makeTestJPEG creates a minimal valid JPEG image in memory.
func makeTestJPEG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 100, G: 150, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80})
	return buf.Bytes()
}

// --- tests ---

func TestUpload_DefaultsToListingPhoto(t *testing.T) {
	repo := &fakeRepo{}
	storage := &fakeStorage{}
	svc := NewServiceWithInterfaces(repo, storage, "http://localhost:9002")

	imgData := makeTestJPEG(200, 200)
	m, err := svc.Upload(context.Background(), imgData, "image/jpeg", UploadInput{})
	require.NoError(t, err)

	assert.Equal(t, MediaTypeListingPhoto, m.MediaType)
	assert.NotEmpty(t, m.ID)
	assert.Contains(t, m.OriginalURL, "media-originals")
	assert.NotNil(t, m.ThumbnailURL)
	assert.Contains(t, *m.ThumbnailURL, "media-thumbnails")
}

func TestUpload_ExplicitMediaType(t *testing.T) {
	repo := &fakeRepo{}
	storage := &fakeStorage{}
	svc := NewServiceWithInterfaces(repo, storage, "http://localhost:9002")

	imgData := makeTestJPEG(100, 100)
	m, err := svc.Upload(context.Background(), imgData, "image/jpeg", UploadInput{
		MediaType: MediaTypeCheckIn,
	})
	require.NoError(t, err)
	assert.Equal(t, MediaTypeCheckIn, m.MediaType)
}

func TestUpload_OrientationStored(t *testing.T) {
	repo := &fakeRepo{}
	storage := &fakeStorage{}
	svc := NewServiceWithInterfaces(repo, storage, "http://localhost:9002")

	roll := float32(15.2)
	pitch := float32(45.0)
	yaw := float32(90.3)

	imgData := makeTestJPEG(100, 100)
	m, err := svc.Upload(context.Background(), imgData, "image/jpeg", UploadInput{
		Orientation: &Orientation{Roll: &roll, Pitch: &pitch, Yaw: &yaw},
	})
	require.NoError(t, err)

	require.NotNil(t, m.OrientationRoll)
	require.NotNil(t, m.OrientationPitch)
	require.NotNil(t, m.OrientationYaw)
	assert.InDelta(t, roll, *m.OrientationRoll, 0.001)
	assert.InDelta(t, pitch, *m.OrientationPitch, 0.001)
	assert.InDelta(t, yaw, *m.OrientationYaw, 0.001)
}

func TestUpload_TwoS3Calls(t *testing.T) {
	repo := &fakeRepo{}
	storage := &fakeStorage{}
	svc := NewServiceWithInterfaces(repo, storage, "http://localhost:9002")

	imgData := makeTestJPEG(100, 100)
	_, err := svc.Upload(context.Background(), imgData, "image/jpeg", UploadInput{})
	require.NoError(t, err)

	require.Len(t, storage.uploads, 2)
	assert.Equal(t, bucketOriginals, storage.uploads[0].bucket)
	assert.Equal(t, bucketThumbnails, storage.uploads[1].bucket)
}

func TestUpload_EmptyImageReturnsError(t *testing.T) {
	repo := &fakeRepo{}
	storage := &fakeStorage{}
	svc := NewServiceWithInterfaces(repo, storage, "http://localhost:9002")

	_, err := svc.Upload(context.Background(), nil, "image/jpeg", UploadInput{})
	require.Error(t, err)
}

func TestUpload_ThumbnailDownscalesLargeImage(t *testing.T) {
	// Create an image larger than maxLongestSide on both axes.
	repo := &fakeRepo{}
	storage := &fakeStorage{}
	svc := NewServiceWithInterfaces(repo, storage, "http://localhost:9002")

	imgData := makeTestJPEG(2000, 1500)
	m, err := svc.Upload(context.Background(), imgData, "image/jpeg", UploadInput{})
	require.NoError(t, err)

	// Both calls should have succeeded; the thumbnail bytes are in storage.uploads[1].
	// We can't easily inspect dimensions without decoding, but we verify the call was made.
	assert.NotNil(t, m.ThumbnailURL)
	require.Len(t, storage.uploads, 2)
}

func TestGenerateThumbnail_DownscalesImage(t *testing.T) {
	imgData := makeTestJPEG(2000, 1000)
	thumb, err := generateThumbnail(imgData)
	require.NoError(t, err)
	require.NotEmpty(t, thumb)

	// Decode the thumbnail and verify dimensions.
	decoded, _, err := image.Decode(bytes.NewReader(thumb))
	require.NoError(t, err)
	b := decoded.Bounds()
	assert.LessOrEqual(t, b.Dx(), maxLongestSide)
	assert.LessOrEqual(t, b.Dy(), maxLongestSide)
}

func TestGenerateThumbnail_SmallImageUnchanged(t *testing.T) {
	imgData := makeTestJPEG(400, 300)
	thumb, err := generateThumbnail(imgData)
	require.NoError(t, err)
	require.NotEmpty(t, thumb)

	decoded, _, err := image.Decode(bytes.NewReader(thumb))
	require.NoError(t, err)
	b := decoded.Bounds()
	assert.Equal(t, 400, b.Dx())
	assert.Equal(t, 300, b.Dy())
}

func TestGetByID_NotFound(t *testing.T) {
	repo := &fakeRepo{findErr: ErrNotFound}
	storage := &fakeStorage{}
	svc := NewServiceWithInterfaces(repo, storage, "http://localhost:9002")

	_, err := svc.GetByID(context.Background(), "nonexistent")
	require.Error(t, err)
}
