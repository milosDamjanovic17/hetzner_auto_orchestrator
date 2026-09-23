// Package service holds the context flows every driver needs: open the store,
// load it, act, save it.
//
// The CLI and the GUI both call this package so the flows exist once. Drivers
// format and prompt; they do not decide. Messages written here are shared by
// both drivers, so they never name a CLI command -- the CLI adds its own hints.
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/config"
	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/hetzner"
	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/preflight"
	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/secrets"
)

// ContextInfo is what a driver may know about a context. It has no token
// field on purpose: this is the type that crosses into the GUI's JavaScript.
type ContextInfo struct {
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

// Service runs the context flows against one encrypted store.
//
// Every method reloads the store from disk. The CLI and the GUI share the
// file, so a `hao context use` in a terminal is seen by an open GUI window on
// its next call.
type Service struct {
	store secrets.Store
	path  string // store location, for messages only

	// validate checks a token against Hetzner. A field rather than a direct
	// call so tests can prove Add's guarantees without a network.
	validate func(ctx context.Context, token string) error
}

// Open returns a Service over the default store, creating the age identity if
// this is the first run.
func Open() (*Service, error) {
	path, err := secrets.DefaultStorePath()
	if err != nil {
		return nil, err
	}
	st, err := secrets.NewAgeStore(path)
	if err != nil {
		return nil, err
	}
	return &Service{
		store: st,
		path:  path,
		validate: func(ctx context.Context, token string) error {
			return hetzner.NewClient(token).ValidateToken(ctx)
		},
	}, nil
}

// Path returns where the store lives on disk.
func (s *Service) Path() string { return s.path }

// load reads the store. secrets.ErrNotInitialized passes through so drivers
// can offer first-run setup.
func (s *Service) load() (*config.Store, error) {
	return config.Load(s.store)
}

// Initialized reports whether the store exists yet.
func (s *Service) Initialized() (bool, error) {
	_, err := s.load()
	if errors.Is(err, secrets.ErrNotInitialized) {
		return false, nil
	}
	return err == nil, err
}

// Init creates an empty store.
func (s *Service) Init() error {
	// Never clobber an existing store: the tokens in it may be the only copy.
	if _, err := s.store.Load(); err == nil {
		return fmt.Errorf("store already exists at %s", s.path)
	} else if !errors.Is(err, secrets.ErrNotInitialized) {
		return err
	}
	return config.Save(s.store, &config.Store{})
}

// Contexts lists every context, marking the active one.
func (s *Service) Contexts() ([]ContextInfo, error) {
	store, err := s.load()
	if err != nil {
		return nil, err
	}
	out := make([]ContextInfo, 0, len(store.Contexts))
	for _, c := range store.List() {
		out = append(out, ContextInfo{Name: c.Name, Active: c.Name == store.Active})
	}
	return out, nil
}

// Use sets the active context.
func (s *Service) Use(name string) error {
	store, err := s.load()
	if err != nil {
		return err
	}
	if err := store.SetActive(name); err != nil {
		return err
	}
	return config.Save(s.store, store)
}

// Add validates the token with Hetzner and stores it under name. It reports
// whether the new context became active (it does when it is the first one).
//
// Nothing is written unless Hetzner accepted the token: a bad one would
// otherwise surface later as a confusing failure on some listing.
func (s *Service) Add(ctx context.Context, name, token string) (active bool, err error) {
	store, err := s.load()
	if err != nil {
		return false, err
	}
	// Checked before the network call; Store.Add rejects it again.
	if _, err := store.Get(name); err == nil {
		return false, fmt.Errorf("%w: %q", config.ErrDuplicate, name)
	}

	if err := s.validate(ctx, token); err != nil {
		switch {
		case errors.Is(err, hetzner.ErrUnauthorized):
			return false, errors.New("token rejected by Hetzner - check it was copied whole and has not been revoked; nothing saved")
		case errors.Is(err, hetzner.ErrUnreachable):
			return false, fmt.Errorf("could not reach Hetzner to validate the token; nothing saved: %w", err)
		default:
			return false, fmt.Errorf("%w; nothing saved", err)
		}
	}

	if err := store.Add(config.Context{Name: name, Token: token}); err != nil {
		return false, err
	}
	if err := config.Save(s.store, store); err != nil {
		return false, err
	}
	return store.Active == name, nil
}

// Delete removes a context from the store and reports whether it was the
// active one. The token itself is not revoked at Hetzner.
func (s *Service) Delete(name string) (wasActive bool, err error) {
	store, err := s.load()
	if err != nil {
		return false, err
	}
	// Remove clears the active marker, so read it first.
	wasActive = store.Active == name
	if err := store.Remove(name); err != nil {
		return false, err
	}
	if err := config.Save(s.store, store); err != nil {
		return false, err
	}
	return wasActive, nil
}

// activeToken returns the active context's name and token. The token is only
// ever handed to a client constructor, never returned to a driver.
func (s *Service) activeToken() (name, token string, err error) {
	store, err := s.load()
	if err != nil {
		return "", "", err
	}
	active, err := store.ActiveContext()
	if err != nil {
		return "", "", err
	}
	return active.Name, active.Token, nil
}

// ActiveClient returns a Hetzner client for the active context, and the
// context's name so a driver can say which project the data came from.
func (s *Service) ActiveClient() (string, *hetzner.Client, error) {
	name, token, err := s.activeToken()
	if err != nil {
		return "", nil, err
	}
	return name, hetzner.NewClient(token), nil
}

// ActiveChecker returns a preflight checker for the active context, and the
// context's name.
func (s *Service) ActiveChecker() (string, *preflight.Checker, error) {
	name, token, err := s.activeToken()
	if err != nil {
		return "", nil, err
	}
	return name, preflight.NewChecker(token), nil
}
