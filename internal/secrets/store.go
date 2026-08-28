package secrets

import (
	"errors"
)

// ErrNotInitialized is returned by Load when no store exists yet. The caller
// branches on this to trigger first-run setup rather than treating it as a
// failure.
var ErrNotInitialized = errors.New("secrets: store not initialized")

// ErrNoIdentity is returned when no age identity is held in the OS keychain.
var ErrNoIdentity = errors.New("secrets: no age identity in keychain")

// ErrKeyringUnavailable is returned when the OS keychain itself cannot be
// reached. This is never recovered from silently: falling back to a plaintext
// key file would defeat the point of encrypting the store at all.
var ErrKeyringUnavailable = errors.New("secrets: OS keychain unavailable")

type Store interface {
	// Load retrieves all secret values.
	Load() ([]byte, error)

	// Save persists all secret values.
	Save([]byte) error
}
