package config

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/secrets"
)

var (
	ErrNotFound  = errors.New("config: no such context")
	ErrDuplicate = errors.New("config: context already exists")
	ErrNoActive  = errors.New("config: no active context")
)

// Context is a Hetzner Cloud project token under a local name, matching what
// the hcloud CLI calls a context.
//
// The model is deliberately flat. Every resource Phase 1 lists -- including DNS
// zones, which moved into the Cloud API in May 2026 -- is reachable with this
// one token, so there is nothing for an account layer above this to hold.
type Context struct {
	Name  string `json:"name"`
	Token string `json:"token"`
}

// Store is the full set of contexts plus which one is active.
//
// The whole Store is the secret: it is serialized to JSON and handed to
// secrets.Store as a single encrypted blob. There is no separate plaintext
// config file, because every field here is either a token or tells an attacker
// which tokens exist.
type Store struct {
	Contexts []Context `json:"contexts"`
	Active   string    `json:"active"`
}

// List returns all contexts.
func (s *Store) List() []Context { return s.Contexts }

// Get returns the context with the given name.
func (s *Store) Get(name string) (Context, error) {
	for _, c := range s.Contexts {
		if c.Name == name {
			return c, nil
		}
	}
	return Context{}, fmt.Errorf("%w: %q", ErrNotFound, name)
}

// ActiveContext returns the currently active context.
func (s *Store) ActiveContext() (Context, error) {
	if s.Active == "" {
		return Context{}, ErrNoActive
	}
	return s.Get(s.Active)
}

// Add stores a new context. Adding a name that already exists is an error
// rather than a silent overwrite: quietly replacing a token is how someone
// ends up pointed at the wrong project without noticing.
func (s *Store) Add(c Context) error {
	if c.Name == "" {
		return errors.New("config: context name must not be empty")
	}
	if c.Token == "" {
		return fmt.Errorf("config: context %q has no token", c.Name)
	}
	if _, err := s.Get(c.Name); err == nil {
		return fmt.Errorf("%w: %q", ErrDuplicate, c.Name)
	}
	s.Contexts = append(s.Contexts, c)
	// First context added becomes active, so a fresh install is usable without
	// a second command.
	if s.Active == "" {
		s.Active = c.Name
	}
	return nil
}

// Remove deletes a context. Removing the active one clears the active marker
// rather than silently promoting another; which project you operate on is not
// a decision this package should make on the user's behalf.
func (s *Store) Remove(name string) error {
	for i, c := range s.Contexts {
		if c.Name == name {
			s.Contexts = append(s.Contexts[:i], s.Contexts[i+1:]...)
			if s.Active == name {
				s.Active = ""
			}
			return nil
		}
	}
	return fmt.Errorf("%w: %q", ErrNotFound, name)
}

// SetActive marks a context active. Unknown names are rejected so that the
// active marker can never point at nothing.
func (s *Store) SetActive(name string) error {
	if _, err := s.Get(name); err != nil {
		return err
	}
	s.Active = name
	return nil
}

// Load reads and decrypts the store.
//
// secrets.ErrNotInitialized is passed through untouched so the caller can offer
// first-run setup instead of reporting a failure.
func Load(st secrets.Store) (*Store, error) {
	raw, err := st.Load()
	if err != nil {
		return nil, err
	}

	var s Store
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("config: parsing store: %w", err)
	}
	return &s, nil
}

// Save serializes and encrypts the store.
func Save(st secrets.Store, s *Store) error {
	raw, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("config: serializing store: %w", err)
	}
	return st.Save(raw)
}
