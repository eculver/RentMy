package proximity

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"
)

// GeneratePIN returns a cryptographically random 4-digit PIN, zero-padded to
// exactly 4 characters (e.g. "0042", "9876").
func GeneratePIN() (string, error) {
	// 10_000 gives range [0, 9999] inclusive.
	n, err := rand.Int(rand.Reader, big.NewInt(10_000))
	if err != nil {
		return "", fmt.Errorf("generate PIN: %w", err)
	}
	return fmt.Sprintf("%04d", n.Int64()), nil
}

// ValidatePIN checks that the entered PIN matches the stored PIN and has not
// expired. Returns ErrPINInvalid or ErrPINExpired on failure.
func ValidatePIN(stored, entered string, expiresAt *time.Time) error {
	if expiresAt != nil && time.Now().After(*expiresAt) {
		return ErrPINExpired
	}
	if stored != entered {
		return ErrPINInvalid
	}
	return nil
}
