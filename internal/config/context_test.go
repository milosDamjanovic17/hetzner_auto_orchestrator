package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"filippo.io/age"
	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/secrets"
)

func newTestSecrets(t *testing.T) secrets.Store {
	t.Helper()
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generating test identity: %v", err)
	}
	return secrets.NewAgeStoreWithIdentity(filepath.Join(t.TempDir(), "contexts.age"), id)
}

func TestStoreRoundTripPreservesActive(t *testing.T) {
	st := newTestSecrets(t)

	s := &Store{}
	if err := s.Add(Context{Name: "prod", Token: "tok-prod"}); err != nil {
		t.Fatalf("Add prod: %v", err)
	}
	if err := s.Add(Context{Name: "staging", Token: "tok-staging"}); err != nil {
		t.Fatalf("Add staging: %v", err)
	}
	if err := s.SetActive("staging"); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	if err := Save(st, s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(st)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.List()) != 2 {
		t.Errorf("got %d contexts, want 2", len(got.List()))
	}
	active, err := got.ActiveContext()
	if err != nil {
		t.Fatalf("ActiveContext: %v", err)
	}
	if active.Name != "staging" || active.Token != "tok-staging" {
		t.Errorf("active = %+v, want staging/tok-staging", active)
	}
}

// First run must reach the caller as ErrNotInitialized so the CLI can offer
// setup. Flattening it into a generic error would make `hao init` unreachable.
func TestLoadUninitializedPassesThroughSentinel(t *testing.T) {
	if _, err := Load(newTestSecrets(t)); !errors.Is(err, secrets.ErrNotInitialized) {
		t.Fatalf("Load returned %v, want secrets.ErrNotInitialized", err)
	}
}

// Silently overwriting a token would repoint the user at a different Hetzner
// project without any signal -- the exact class of mistake this app exists to
// prevent.
func TestAddDuplicateNameRejected(t *testing.T) {
	s := &Store{}
	if err := s.Add(Context{Name: "prod", Token: "a"}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := s.Add(Context{Name: "prod", Token: "b"}); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("duplicate Add returned %v, want ErrDuplicate", err)
	}
	if c, _ := s.Get("prod"); c.Token != "a" {
		t.Errorf("token was overwritten to %q", c.Token)
	}
}

func TestAddRejectsEmptyFields(t *testing.T) {
	s := &Store{}
	if err := s.Add(Context{Name: "", Token: "tok"}); err == nil {
		t.Error("Add with empty name succeeded, want error")
	}
	if err := s.Add(Context{Name: "prod", Token: ""}); err == nil {
		t.Error("Add with empty token succeeded, want error")
	}
}

// The active marker must never name a context that is not there, or every
// later lookup fails somewhere far from the cause.
func TestSetActiveUnknownRejected(t *testing.T) {
	s := &Store{}
	if err := s.SetActive("ghost"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("SetActive returned %v, want ErrNotFound", err)
	}
	if s.Active != "" {
		t.Errorf("active was set to %q despite the error", s.Active)
	}
}

func TestFirstAddBecomesActive(t *testing.T) {
	s := &Store{}
	if err := s.Add(Context{Name: "prod", Token: "tok"}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if s.Active != "prod" {
		t.Errorf("active = %q, want prod", s.Active)
	}
}

// Removing the active context clears the marker instead of promoting a
// neighbour: picking which project you now operate on is the user's call.
func TestRemoveActiveClearsMarker(t *testing.T) {
	s := &Store{}
	_ = s.Add(Context{Name: "prod", Token: "a"})
	_ = s.Add(Context{Name: "staging", Token: "b"})

	if err := s.Remove("prod"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if s.Active != "" {
		t.Errorf("active = %q, want empty", s.Active)
	}
	if _, err := s.ActiveContext(); !errors.Is(err, ErrNoActive) {
		t.Fatalf("ActiveContext returned %v, want ErrNoActive", err)
	}
}

func TestRemoveUnknownRejected(t *testing.T) {
	s := &Store{}
	if err := s.Remove("ghost"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Remove returned %v, want ErrNotFound", err)
	}
}

// An empty store is a legitimate state (fresh `hao init`), not a failure.
func TestEmptyStoreIsValid(t *testing.T) {
	st := newTestSecrets(t)
	if err := Save(st, &Store{}); err != nil {
		t.Fatalf("Save empty: %v", err)
	}
	s, err := Load(st)
	if err != nil {
		t.Fatalf("Load empty: %v", err)
	}
	if len(s.List()) != 0 {
		t.Errorf("got %d contexts, want 0", len(s.List()))
	}
	if _, err := s.ActiveContext(); !errors.Is(err, ErrNoActive) {
		t.Fatalf("ActiveContext returned %v, want ErrNoActive", err)
	}
}

const fixtureCliToml = `active_context = "staging"

[[contexts]]
  name = "prod"
  token = "tok-prod"

[[contexts]]
  name = "staging"
  token = "tok-staging"
`

func writeFixture(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cli.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	return path
}

func TestImportHcloudContexts(t *testing.T) {
	path := writeFixture(t, fixtureCliToml)

	got, active, err := ImportHcloudContexts(path)
	if err != nil {
		t.Fatalf("ImportHcloudContexts: %v", err)
	}
	if active != "staging" {
		t.Errorf("active = %q, want staging", active)
	}
	want := []Context{{Name: "prod", Token: "tok-prod"}, {Name: "staging", Token: "tok-staging"}}
	if len(got) != len(want) {
		t.Fatalf("got %d contexts, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("context %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// Import must leave the hcloud CLI's own config untouched. Writing back to it
// would put plaintext tokens somewhere this app claims not to manage.
func TestImportDoesNotModifyCliToml(t *testing.T) {
	path := writeFixture(t, fixtureCliToml)

	if _, _, err := ImportHcloudContexts(path); err != nil {
		t.Fatalf("ImportHcloudContexts: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("re-reading fixture: %v", err)
	}
	if string(after) != fixtureCliToml {
		t.Error("cli.toml was modified by import")
	}
}

// One malformed entry should not block importing the rest.
func TestImportSkipsIncompleteEntries(t *testing.T) {
	path := writeFixture(t, `active_context = "prod"

[[contexts]]
  name = "prod"
  token = "tok-prod"

[[contexts]]
  name = "broken"
`)

	got, _, err := ImportHcloudContexts(path)
	if err != nil {
		t.Fatalf("ImportHcloudContexts: %v", err)
	}
	if len(got) != 1 || got[0].Name != "prod" {
		t.Errorf("got %+v, want only prod", got)
	}
}

func TestImportMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.toml")
	if _, _, err := ImportHcloudContexts(path); !errors.Is(err, ErrNoHcloudConfig) {
		t.Fatalf("ImportHcloudContexts returned %v, want ErrNoHcloudConfig", err)
	}
}

func TestEnvTokenPresent(t *testing.T) {
	t.Setenv(EnvTokenVar, "")
	if EnvTokenPresent() {
		t.Error("EnvTokenPresent true with empty env var")
	}
	t.Setenv(EnvTokenVar, "some-token")
	if !EnvTokenPresent() {
		t.Error("EnvTokenPresent false with env var set")
	}
}
