// Package ulid provides thread-safe ULID generation using monotonic entropy.
package ulid

import (
	"crypto/rand"
	"sync"
	"time"

	oklogulid "github.com/oklog/ulid/v2"
)

var (
	mu      sync.Mutex
	entropy = oklogulid.Monotonic(rand.Reader, 0)
)

// New generates a new ULID string. It is safe for concurrent use.
func New() string {
	mu.Lock()
	defer mu.Unlock()
	return oklogulid.MustNew(oklogulid.Timestamp(time.Now()), entropy).String()
}

// MustParse parses a ULID string and panics on error. Intended for use in tests.
func MustParse(s string) oklogulid.ULID {
	return oklogulid.MustParse(s)
}
