package secrets

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"filippo.io/age"
	"github.com/zalando/go-keyring"
)

// The real OS keychain is never touched by tests. go-keyring's mock provider
// keeps runs hermetic and stops test runs from littering the developer's
// Credential Manager.
func TestMain(m *testing.M) {
	keyring.MockInit()
	os.Exit(m.Run())
}

func newTestStore(t *testing.T) *AgeStore {
	t.Helper()
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generating test identity: %v", err)
	}
	return NewAgeStoreWithIdentity(filepath.Join(t.TempDir(), "contexts.age"), id)
}

func TestRoundTrip(t *testing.T) {
	s := newTestStore(t)
	want := []byte(`{"contexts":[{"name":"prod","token":"abc"}],"active":"prod"}`)

	if err := s.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("Load returned %q, want %q", got, want)
	}
}

// The whole reason this package exists is that a Hetzner token must never sit
// on disk in the clear. If this test ever fails, the encryption has been
// bypassed somewhere and the security property of the app is gone -- so it
// asserts on the raw file, not on the API.
func TestSaveLeaksNoPlaintext(t *testing.T) {
	s := newTestStore(t)
	const sentinel = "SENTINEL-TOKEN-DO-NOT-LEAK"

	if err := s.Save([]byte(`{"token":"` + sentinel + `"}`)); err != nil {
		t.Fatalf("Save: %v", err)
	}

	raw, err := os.ReadFile(s.Path())
	if err != nil {
		t.Fatalf("reading store file: %v", err)
	}
	if bytes.Contains(raw, []byte(sentinel)) {
		t.Fatalf("store file contains plaintext secret %q", sentinel)
	}
}

// Save must not leave a readable temp file behind next to the store. An
// interrupted or sloppy write is the likeliest way plaintext escapes.
func TestSaveLeavesNoTempFiles(t *testing.T) {
	s := newTestStore(t)
	if err := s.Save([]byte("payload")); err != nil {
		t.Fatalf("Save: %v", err)
	}

	entries, err := os.ReadDir(filepath.Dir(s.Path()))
	if err != nil {
		t.Fatalf("reading store dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(s.Path()) {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("store dir holds %v, want only %s", names, filepath.Base(s.Path()))
	}
}

// First run must be distinguishable from failure: the CLI branches on this to
// offer setup instead of printing an error.
func TestLoadMissingStoreIsNotInitialized(t *testing.T) {
	s := newTestStore(t)
	_, err := s.Load()
	if !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("Load on missing store returned %v, want ErrNotInitialized", err)
	}
}

// A store encrypted to someone else's key must fail loudly rather than return
// empty or partial data that callers would treat as "no contexts yet".
func TestLoadWithWrongIdentityFails(t *testing.T) {
	s := newTestStore(t)
	if err := s.Save([]byte("payload")); err != nil {
		t.Fatalf("Save: %v", err)
	}

	other, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generating other identity: %v", err)
	}
	wrong := NewAgeStoreWithIdentity(s.Path(), other)

	if _, err := wrong.Load(); err == nil {
		t.Fatal("Load with wrong identity succeeded, want error")
	}
}

func TestSaveOverwritesExistingStore(t *testing.T) {
	s := newTestStore(t)
	if err := s.Save([]byte("first")); err != nil {
		t.Fatalf("first Save: %v", err)
	}
	if err := s.Save([]byte("second")); err != nil {
		t.Fatalf("second Save: %v", err)
	}

	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if string(got) != "second" {
		t.Errorf("Load returned %q, want %q", got, "second")
	}
}

// EnsureIdentity must be idempotent. Generating a fresh key on a later run
// would silently orphan every context already stored.
func TestEnsureIdentityIsStable(t *testing.T) {
	t.Cleanup(func() { _ = DeleteIdentity() })
	if err := DeleteIdentity(); err != nil {
		t.Fatalf("clearing identity: %v", err)
	}

	first, err := EnsureIdentity()
	if err != nil {
		t.Fatalf("first EnsureIdentity: %v", err)
	}
	second, err := EnsureIdentity()
	if err != nil {
		t.Fatalf("second EnsureIdentity: %v", err)
	}
	if first.String() != second.String() {
		t.Error("EnsureIdentity generated a new key on the second call")
	}
}

func TestLoadIdentityMissing(t *testing.T) {
	t.Cleanup(func() { _ = DeleteIdentity() })
	if err := DeleteIdentity(); err != nil {
		t.Fatalf("clearing identity: %v", err)
	}

	if _, err := LoadIdentity(); !errors.Is(err, ErrNoIdentity) {
		t.Fatalf("LoadIdentity returned %v, want ErrNoIdentity", err)
	}
}
