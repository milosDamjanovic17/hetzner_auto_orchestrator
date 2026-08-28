package secrets

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"filippo.io/age"
)

// AgeStore is a Store backed by a single age-encrypted file on disk, with the
// age identity sealed in the OS keychain.
//
// It deals in opaque bytes and does not know what they mean. Keeping it that
// way is what lets internal/config be tested and changed independently.
type AgeStore struct {
	path     string
	identity *age.X25519Identity
}

var _ Store = (*AgeStore)(nil)

// DefaultStorePath is where the encrypted store lives unless overridden.
func DefaultStorePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("secrets: locating user config dir: %w", err)
	}
	return filepath.Join(dir, "hetzner_auto_orchestrator", "contexts.age"), nil
}

// NewAgeStore opens the store at path, generating an age identity on first use.
func NewAgeStore(path string) (*AgeStore, error) {
	id, err := EnsureIdentity()
	if err != nil {
		return nil, err
	}
	return &AgeStore{path: path, identity: id}, nil
}

// NewAgeStoreWithIdentity opens the store using an explicit identity instead of
// the one in the OS keychain.
func NewAgeStoreWithIdentity(path string, id *age.X25519Identity) *AgeStore {
	return &AgeStore{path: path, identity: id}
}

// Path returns the file backing this store.
func (s *AgeStore) Path() string { return s.path }

// Load decrypts and returns the stored bytes. It returns ErrNotInitialized if
// the store does not exist yet.
func (s *AgeStore) Load() ([]byte, error) {
	f, err := os.Open(s.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrNotInitialized
		}
		return nil, fmt.Errorf("secrets: opening store: %w", err)
	}
	defer f.Close()

	r, err := age.Decrypt(f, s.identity)
	if err != nil {
		return nil, fmt.Errorf("secrets: decrypting store at %s: %w", s.path, err)
	}

	plaintext, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("secrets: reading store: %w", err)
	}
	return plaintext, nil
}

// Save encrypts plaintext and writes it to disk.
//
// The write goes to a temp file in the same directory and is then renamed over
// the target, so an interrupted save cannot leave a half-written store that no
// longer decrypts.
func (s *AgeStore) Save(plaintext []byte) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("secrets: creating store dir: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".contexts-*.age.tmp")
	if err != nil {
		return fmt.Errorf("secrets: creating temp file: %w", err)
	}
	tmpName := tmp.Name()

	committed := false
	defer func() {
		if !committed {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()

	w, err := age.Encrypt(tmp, s.identity.Recipient())
	if err != nil {
		return fmt.Errorf("secrets: starting encryption: %w", err)
	}
	if _, err := w.Write(plaintext); err != nil {
		return fmt.Errorf("secrets: writing ciphertext: %w", err)
	}
	// Closing the age writer finalizes the payload. Skipping this yields a
	// file that looks written but will not decrypt.
	if err := w.Close(); err != nil {
		return fmt.Errorf("secrets: finalizing ciphertext: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("secrets: closing temp file: %w", err)
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return fmt.Errorf("secrets: setting store permissions: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("secrets: replacing store: %w", err)
	}
	committed = true
	return nil
}
