// Package cv provides an HTTP client for the cv-service sidecar
// that handles computer vision preprocessing for the photo diff pipeline.
package cv

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"
)

const (
	defaultPreprocessTimeout = 30 * time.Second
	defaultQualityTimeout    = 5 * time.Second
)

// Client talks to the cv-service sidecar over HTTP.
type Client struct {
	baseURL           string
	httpClient        *http.Client
	preprocessTimeout time.Duration
	qualityTimeout    time.Duration
}

// Option configures a Client.
type Option func(*Client)

// WithPreprocessTimeout overrides the default preprocess call timeout.
func WithPreprocessTimeout(d time.Duration) Option {
	return func(c *Client) { c.preprocessTimeout = d }
}

// WithQualityTimeout overrides the default quality check call timeout.
func WithQualityTimeout(d time.Duration) Option {
	return func(c *Client) { c.qualityTimeout = d }
}

// New creates a cv-service client pointing at the given base URL.
func New(baseURL string, opts ...Option) *Client {
	c := &Client{
		baseURL:           baseURL,
		httpClient:        &http.Client{},
		preprocessTimeout: defaultPreprocessTimeout,
		qualityTimeout:    defaultQualityTimeout,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// OrientationMeta carries gyroscope orientation for a single photo.
type OrientationMeta struct {
	Type  string  `json:"type"` // "checkin" or "checkout"
	Roll  float32 `json:"roll"`
	Pitch float32 `json:"pitch"`
	Yaw   float32 `json:"yaw"`
}

// PreprocessResult is the response from the /preprocess endpoint.
type PreprocessResult struct {
	Pairs         []ImagePair `json:"pairs"`
	TotalCheckin  int         `json:"total_checkin"`
	TotalCheckout int         `json:"total_checkout"`
	PairsMatched  int         `json:"pairs_matched"`
}

// ImagePair holds a matched pair of base64-encoded preprocessed images.
type ImagePair struct {
	CheckinImage  string `json:"checkin_image"`  // base64
	CheckoutImage string `json:"checkout_image"` // base64
	CheckinIndex  int    `json:"checkin_index"`
	CheckoutIndex int    `json:"checkout_index"`
}

// QualityResult is the response from the /quality endpoint.
type QualityResult struct {
	Passed           bool     `json:"passed"`
	BlurScore        float64  `json:"blur_score"`
	BlurPassed       bool     `json:"blur_passed"`
	ResolutionPassed bool     `json:"resolution_passed"`
	Width            int      `json:"width"`
	Height           int      `json:"height"`
	Issues           []string `json:"issues"`
}

// Preprocess sends check-in and check-out images to the cv-service for
// normalization, segmentation, and angle-based pairing.
func (c *Client) Preprocess(ctx context.Context, checkinImages, checkoutImages [][]byte, orientations []OrientationMeta) (*PreprocessResult, error) {
	ctx, cancel := context.WithTimeout(ctx, c.preprocessTimeout)
	defer cancel()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if err := writer.WriteField("checkin_count", strconv.Itoa(len(checkinImages))); err != nil {
		return nil, fmt.Errorf("cv: write checkin_count: %w", err)
	}
	if err := writer.WriteField("checkout_count", strconv.Itoa(len(checkoutImages))); err != nil {
		return nil, fmt.Errorf("cv: write checkout_count: %w", err)
	}

	orientationsJSON, err := json.Marshal(orientations)
	if err != nil {
		return nil, fmt.Errorf("cv: marshal orientations: %w", err)
	}
	if err := writer.WriteField("orientations_json", string(orientationsJSON)); err != nil {
		return nil, fmt.Errorf("cv: write orientations: %w", err)
	}

	for i, img := range checkinImages {
		part, err := writer.CreateFormFile("checkin_images", fmt.Sprintf("checkin_%d.jpg", i))
		if err != nil {
			return nil, fmt.Errorf("cv: create checkin form file: %w", err)
		}
		if _, err := part.Write(img); err != nil {
			return nil, fmt.Errorf("cv: write checkin image: %w", err)
		}
	}
	for i, img := range checkoutImages {
		part, err := writer.CreateFormFile("checkout_images", fmt.Sprintf("checkout_%d.jpg", i))
		if err != nil {
			return nil, fmt.Errorf("cv: create checkout form file: %w", err)
		}
		if _, err := part.Write(img); err != nil {
			return nil, fmt.Errorf("cv: write checkout image: %w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("cv: close writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/preprocess", body)
	if err != nil {
		return nil, fmt.Errorf("cv: create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cv: preprocess request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("cv: preprocess returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result PreprocessResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("cv: decode preprocess response: %w", err)
	}
	return &result, nil
}

// CheckQuality checks a single image for blur, resolution, and item coverage.
func (c *Client) CheckQuality(ctx context.Context, imageBytes []byte) (*QualityResult, error) {
	ctx, cancel := context.WithTimeout(ctx, c.qualityTimeout)
	defer cancel()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("image", "image.jpg")
	if err != nil {
		return nil, fmt.Errorf("cv: create quality form file: %w", err)
	}
	if _, err := part.Write(imageBytes); err != nil {
		return nil, fmt.Errorf("cv: write quality image: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("cv: close quality writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/quality", body)
	if err != nil {
		return nil, fmt.Errorf("cv: create quality request: %w", err)
	}
	req.Method = http.MethodPost
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cv: quality request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("cv: quality returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result QualityResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("cv: decode quality response: %w", err)
	}
	return &result, nil
}

// HealthCheck verifies the cv-service is reachable.
func (c *Client) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("cv: create health request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("cv: health request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("cv: health check returned %d", resp.StatusCode)
	}
	return nil
}
