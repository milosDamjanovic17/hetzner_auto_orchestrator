package secrets

import (
	"errors"
	"fmt"

	"filippo.io/age"
	"github.com/zalando/go-keyring"
)

// Where the age private key lives in the OS keychain: Credential Manager
// (DPAPI) on Windows, Keychain on macOS, libsecret on Linux.
const (
	keyringService = "hetzner_auto_orchestrator"
	keyringAccount = "age-identity"
)

// LoadIdentity returns the age identity held in the OS keychain.
//
// Returns ErrNoIdentity if none has been generated yet, and
// ErrKeyringUnavailable if the keychain could not be reached at all. Those two
// cases are deliberately distinct: the first is first-run, the second is a
// broken environment the user has to fix.
func LoadIdentity() (*age.X25519Identity, error) {
	raw, err := keyring.Get(keyringService, keyringAccount)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil, ErrNoIdentity
		}
		return nil, fmt.Errorf("%w: %v", ErrKeyringUnavailable, err)
	}

	id, err := age.ParseX25519Identity(raw)
	if err != nil {
		// The keychain held something, but not a usable key. Editing it by
		// hand or a partial write are the likely causes; either way we cannot
		// guess our way out.
		return nil, fmt.Errorf("secrets: stored age identity is unusable: %w", err)
	}
	return id, nil
}

// EnsureIdentity returns the stored age identity, generating and storing one on
// first use. Any error other than "not yet generated" is returned as-is: we
// never overwrite a key we merely failed to read, because that would destroy
// the only means of decrypting an existing store.
func EnsureIdentity() (*age.X25519Identity, error) {
	id, err := LoadIdentity()
	switch {
	case err == nil:
		return id, nil
	case errors.Is(err, ErrNoIdentity):
		// fall through to generation
	default:
		return nil, err
	}

	id, err = age.GenerateX25519Identity()
	if err != nil {
		return nil, fmt.Errorf("secrets: generating age identity: %w", err)
	}

	if err := keyring.Set(keyringService, keyringAccount, id.String()); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrKeyringUnavailable, err)
	}
	return id, nil
}

// DeleteIdentity removes the identity from the OS keychain.
//
// This makes any existing encrypted store permanently unreadable, so callers
// must confirm with the user first.
func DeleteIdentity() error {
	err := keyring.Delete(keyringService, keyringAccount)
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("%w: %v", ErrKeyringUnavailable, err)
	}
	return nil
}
