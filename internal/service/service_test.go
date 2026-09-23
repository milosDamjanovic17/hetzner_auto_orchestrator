package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"filippo.io/age"
	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/config"
	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/hetzner"
	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/secrets"
)

// fakeValidator accepts only the tokens it is given and counts calls.
type fakeValidator struct {
	good  map[string]bool
	err   error // returned for any other token
	calls int
}

func (f *fakeValidator) validate(_ context.Context, token string) error {
	f.calls++
	if f.good[token] {
		return nil
	}
	return f.err
}

// newTestService returns a Service over a temp store that accepts "tok-good",
// plus the store's path so tests can open a second Service on the same file.
func newTestService(t *testing.T) (*Service, *fakeValidator, string) {
	t.Helper()
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generating test identity: %v", err)
	}
	path := filepath.Join(t.TempDir(), "contexts.age")
	v := &fakeValidator{
		good: map[string]bool{"tok-good": true, "tok-good-2": true},
		err:  fmt.Errorf("%w (validating token)", hetzner.ErrUnauthorized),
	}
	s := &Service{store: secrets.NewAgeStoreWithIdentity(path, id), path: path, validate: v.validate}
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return s, v, path
}

// sameStore opens a second Service on the same file, as the GUI and the CLI do.
func sameStore(s *Service) *Service {
	return &Service{store: s.store, path: s.path, validate: s.validate}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading store file: %v", err)
	}
	return b
}

// A token Hetzner rejects must never be stored: it would fail later on some
// unrelated listing, far from where the mistake was made. age encryption is
// randomized, so identical file bytes prove no Save happened at all.
func TestAddRejectedTokenLeavesStoreUnchanged(t *testing.T) {
	for _, tc := range []struct {
		name    string
		err     error
		wantMsg string
	}{
		{"unauthorized", fmt.Errorf("%w (x)", hetzner.ErrUnauthorized), "token rejected"},
		{"unreachable", fmt.Errorf("%w: x", hetzner.ErrUnreachable), "could not reach Hetzner"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, v, path := newTestService(t)
			v.err = tc.err
			before := readFile(t, path)

			_, err := s.Add(context.Background(), "prod", "tok-bad")
			if err == nil || !strings.Contains(err.Error(), tc.wantMsg) || !strings.Contains(err.Error(), "nothing saved") {
				t.Fatalf("Add error = %v, want %q and \"nothing saved\"", err, tc.wantMsg)
			}
			if !bytes.Equal(before, readFile(t, path)) {
				t.Fatal("store file changed after a rejected token")
			}
		})
	}
}

// A duplicate name must fail before the token is sent to Hetzner: silently
// replacing a token is how you end up on the wrong project, and validating
// first would make the user wait for a request that cannot succeed.
func TestAddDuplicateFailsBeforeValidating(t *testing.T) {
	s, v, path := newTestService(t)
	if _, err := s.Add(context.Background(), "prod", "tok-good"); err != nil {
		t.Fatalf("first Add: %v", err)
	}
	v.calls = 0
	before := readFile(t, path)

	_, err := s.Add(context.Background(), "prod", "tok-good-2")
	if !errors.Is(err, config.ErrDuplicate) {
		t.Fatalf("Add duplicate = %v, want ErrDuplicate", err)
	}
	if v.calls != 0 {
		t.Errorf("validator called %d times for a duplicate name, want 0", v.calls)
	}
	if !bytes.Equal(before, readFile(t, path)) {
		t.Error("store file changed after a duplicate Add")
	}
}

// The first context becomes active so a fresh install is usable at once; a
// later one must not steal the active marker, or adding a project would
// silently change which project every command acts on.
func TestAddOnlyFirstContextBecomesActive(t *testing.T) {
	s, _, _ := newTestService(t)
	if active, err := s.Add(context.Background(), "prod", "tok-good"); err != nil || !active {
		t.Fatalf("first Add = (%v, %v), want (true, nil)", active, err)
	}
	if active, err := s.Add(context.Background(), "staging", "tok-good-2"); err != nil || active {
		t.Fatalf("second Add = (%v, %v), want (false, nil)", active, err)
	}
	assertActive(t, s, "prod")
}

// Use must persist to disk, and every call must reload: the GUI and the CLI
// are separate processes on one file, and the GUI must show the project the
// CLI switched to, not a stale copy.
func TestUsePersistsAndIsSeenByAnotherService(t *testing.T) {
	cli, _, _ := newTestService(t)
	gui := sameStore(cli)
	for _, n := range []string{"prod", "staging"} {
		if _, err := cli.Add(context.Background(), n, "tok-good"); err != nil {
			t.Fatalf("Add %s: %v", n, err)
		}
	}
	assertActive(t, gui, "prod") // GUI reads once...

	if err := cli.Use("staging"); err != nil {
		t.Fatalf("Use: %v", err)
	}
	assertActive(t, gui, "staging") // ...and sees the CLI's switch on the next call
}

// The active marker must never point at a context that does not exist.
func TestUseUnknownIsRejected(t *testing.T) {
	s, _, _ := newTestService(t)
	if _, err := s.Add(context.Background(), "prod", "tok-good"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := s.Use("prdo"); !errors.Is(err, config.ErrNotFound) {
		t.Fatalf("Use unknown = %v, want ErrNotFound", err)
	}
	assertActive(t, s, "prod")
}

// Deleting a name that is not there is an error, not a quiet no-op: a typo
// must not look like a successful delete.
func TestDeleteUnknownErrors(t *testing.T) {
	s, _, path := newTestService(t)
	before := readFile(t, path)
	if _, err := s.Delete("nope"); !errors.Is(err, config.ErrNotFound) {
		t.Fatalf("Delete unknown = %v, want ErrNotFound", err)
	}
	if !bytes.Equal(before, readFile(t, path)) {
		t.Error("store file changed after deleting an unknown context")
	}
}

// Deleting the active context leaves NO active context rather than promoting
// another: which project to act on is the user's choice, never a side effect.
func TestDeleteActiveReportsItAndPromotesNothing(t *testing.T) {
	s, _, _ := newTestService(t)
	for _, n := range []string{"prod", "staging"} {
		if _, err := s.Add(context.Background(), n, "tok-good"); err != nil {
			t.Fatalf("Add %s: %v", n, err)
		}
	}
	wasActive, err := s.Delete("prod")
	if err != nil || !wasActive {
		t.Fatalf("Delete active = (%v, %v), want (true, nil)", wasActive, err)
	}
	assertActive(t, s, "")
	if _, _, err := s.ActiveClient(); !errors.Is(err, config.ErrNoActive) {
		t.Errorf("ActiveClient after deleting active = %v, want ErrNoActive", err)
	}
}

// Contexts() is what the GUI hands to JavaScript. No token may cross that
// boundary, neither as a field nor hidden in the JSON.
func TestContextsCarryNoToken(t *testing.T) {
	s, _, _ := newTestService(t)
	if _, err := s.Add(context.Background(), "prod", "tok-good"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	got, err := s.Contexts()
	if err != nil {
		t.Fatalf("Contexts: %v", err)
	}

	typ := reflect.TypeOf(ContextInfo{})
	for i := 0; i < typ.NumField(); i++ {
		if strings.Contains(strings.ToLower(typ.Field(i).Name), "token") {
			t.Errorf("ContextInfo has a token-like field %q", typ.Field(i).Name)
		}
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if bytes.Contains(raw, []byte("tok-good")) {
		t.Errorf("Contexts JSON contains the token: %s", raw)
	}
}

// Init must never overwrite an existing store: it may hold the only copy of
// the tokens.
func TestInitRefusesToClobber(t *testing.T) {
	s, _, path := newTestService(t)
	if _, err := s.Add(context.Background(), "prod", "tok-good"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	before := readFile(t, path)
	if err := s.Init(); err == nil {
		t.Fatal("second Init succeeded, want an error")
	}
	if !bytes.Equal(before, readFile(t, path)) {
		t.Error("store file changed after a refused Init")
	}
}

// Before Init there is no store; that must read as "not initialized", not as
// a failure, so drivers can offer first-run setup.
func TestInitializedBeforeAndAfterInit(t *testing.T) {
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generating test identity: %v", err)
	}
	path := filepath.Join(t.TempDir(), "contexts.age")
	s := &Service{store: secrets.NewAgeStoreWithIdentity(path, id), path: path}

	if ok, err := s.Initialized(); ok || err != nil {
		t.Fatalf("Initialized before Init = (%v, %v), want (false, nil)", ok, err)
	}
	if _, err := s.Contexts(); !errors.Is(err, secrets.ErrNotInitialized) {
		t.Errorf("Contexts before Init = %v, want ErrNotInitialized", err)
	}
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if ok, err := s.Initialized(); !ok || err != nil {
		t.Fatalf("Initialized after Init = (%v, %v), want (true, nil)", ok, err)
	}
}

// assertActive checks which context Contexts() marks active ("" for none).
func assertActive(t *testing.T, s *Service, want string) {
	t.Helper()
	got, err := s.Contexts()
	if err != nil {
		t.Fatalf("Contexts: %v", err)
	}
	active := ""
	for _, c := range got {
		if c.Active {
			if active != "" {
				t.Fatalf("more than one active context: %+v", got)
			}
			active = c.Name
		}
	}
	if active != want {
		t.Errorf("active = %q, want %q", active, want)
	}
}
