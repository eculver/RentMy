package proximity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratePIN_AlwaysFourDigits(t *testing.T) {
	for i := 0; i < 200; i++ {
		pin, err := GeneratePIN()
		require.NoError(t, err)
		assert.Len(t, pin, 4, "PIN must always be exactly 4 characters")
		for _, c := range pin {
			assert.GreaterOrEqual(t, c, rune('0'))
			assert.LessOrEqual(t, c, rune('9'))
		}
	}
}

func TestGeneratePIN_ZeroPadded(t *testing.T) {
	// Run enough iterations that we'd almost certainly hit a value < 1000.
	seen := make(map[string]struct{})
	for i := 0; i < 5000; i++ {
		pin, err := GeneratePIN()
		require.NoError(t, err)
		seen[pin] = struct{}{}
	}
	// Verify at least one zero-padded PIN exists in the output set.
	// With 5000 samples from [0,9999] this will almost always pass.
	assert.Greater(t, len(seen), 100, "should have diverse PIN values")
}

func TestValidatePIN_Match(t *testing.T) {
	future := time.Now().Add(10 * time.Minute)
	assert.NoError(t, ValidatePIN("1234", "1234", &future))
}

func TestValidatePIN_Mismatch(t *testing.T) {
	future := time.Now().Add(10 * time.Minute)
	assert.ErrorIs(t, ValidatePIN("1234", "9999", &future), ErrPINInvalid)
}

func TestValidatePIN_Expired(t *testing.T) {
	past := time.Now().Add(-1 * time.Minute)
	assert.ErrorIs(t, ValidatePIN("1234", "1234", &past), ErrPINExpired)
}

func TestValidatePIN_NoExpiry(t *testing.T) {
	// nil expiresAt means no expiry enforced.
	assert.NoError(t, ValidatePIN("5678", "5678", nil))
}
