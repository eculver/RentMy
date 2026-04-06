package cv

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthCheck_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/health", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":       "ok",
			"model_loaded": false,
		})
	}))
	defer srv.Close()

	c := New(srv.URL)
	err := c.HealthCheck(context.Background())
	assert.NoError(t, err)
}

func TestHealthCheck_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := New(srv.URL)
	err := c.HealthCheck(context.Background())
	assert.Error(t, err)
}

func TestCheckQuality(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/quality", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		// Verify we received a multipart form with an image.
		err := r.ParseMultipartForm(10 << 20)
		require.NoError(t, err)
		file, _, err := r.FormFile("image")
		require.NoError(t, err)
		file.Close()

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(QualityResult{
			Passed:           true,
			BlurScore:        200.5,
			BlurPassed:       true,
			ResolutionPassed: true,
			Width:            1920,
			Height:           1080,
			Issues:           nil,
		})
	}))
	defer srv.Close()

	c := New(srv.URL)
	result, err := c.CheckQuality(context.Background(), []byte("fake-image-data"))
	require.NoError(t, err)
	assert.True(t, result.Passed)
	assert.InDelta(t, 200.5, result.BlurScore, 0.1)
	assert.Equal(t, 1920, result.Width)
}

func TestPreprocess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/preprocess", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		err := r.ParseMultipartForm(10 << 20)
		require.NoError(t, err)
		assert.Equal(t, "2", r.FormValue("checkin_count"))
		assert.Equal(t, "2", r.FormValue("checkout_count"))

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(PreprocessResult{
			Pairs: []ImagePair{
				{CheckinImage: "Y2hlY2tpbjE=", CheckoutImage: "Y2hlY2tvdXQx", CheckinIndex: 0, CheckoutIndex: 0},
				{CheckinImage: "Y2hlY2tpbjI=", CheckoutImage: "Y2hlY2tvdXQy", CheckinIndex: 1, CheckoutIndex: 1},
			},
			TotalCheckin:  2,
			TotalCheckout: 2,
			PairsMatched:  2,
		})
	}))
	defer srv.Close()

	c := New(srv.URL)
	result, err := c.Preprocess(
		context.Background(),
		[][]byte{[]byte("img1"), []byte("img2")},
		[][]byte{[]byte("img3"), []byte("img4")},
		[]OrientationMeta{
			{Type: "checkin", Roll: 0, Pitch: 45, Yaw: 90},
			{Type: "checkout", Roll: 0, Pitch: 44, Yaw: 91},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, 2, result.PairsMatched)
	assert.Len(t, result.Pairs, 2)
}
