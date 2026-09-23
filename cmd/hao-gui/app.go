package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/config"
	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/service"
)

// apiTimeout bounds every Hetzner call, as in the CLI, so the window cannot
// wait forever on an unreachable Hetzner.
const apiTimeout = 30 * time.Second

// App is the object Wails binds to the frontend: every exported method is
// callable from JavaScript.
//
// Boundary rule: no exported method may return a token, directly or inside a
// struct. The service's ContextInfo exists for exactly that reason.
type App struct {
	ctx     context.Context
	svc     *service.Service
	openErr error // why the store could not be opened; returned by every call
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// Opened once: the service reloads the store on every call, so a context
	// switched in the CLI still shows up here.
	a.svc, a.openErr = service.Open()
}

func (a *App) service() (*service.Service, error) {
	if a.openErr != nil {
		return nil, a.openErr
	}
	return a.svc, nil
}

// Status is what the header needs on every refresh.
type Status struct {
	Initialized     bool   `json:"initialized"`
	Active          string `json:"active"` // "" when no context is active
	EnvTokenIgnored bool   `json:"envTokenIgnored"`
}

// Status reports whether the store exists and which project is active.
func (a *App) Status() (Status, error) {
	st := Status{EnvTokenIgnored: config.EnvTokenPresent()}
	svc, err := a.service()
	if err != nil {
		return st, err
	}
	if st.Initialized, err = svc.Initialized(); err != nil || !st.Initialized {
		return st, err
	}
	contexts, err := svc.Contexts()
	if err != nil {
		return st, err
	}
	for _, c := range contexts {
		if c.Active {
			st.Active = c.Name
		}
	}
	return st, nil
}

// Contexts lists every context with the active one marked. Names only.
func (a *App) Contexts() ([]service.ContextInfo, error) {
	svc, err := a.service()
	if err != nil {
		return nil, err
	}
	return svc.Contexts()
}

// Init creates the encrypted store. The service refuses if one already exists.
func (a *App) Init() error {
	svc, err := a.service()
	if err != nil {
		return err
	}
	return svc.Init()
}

// UseContext makes name the active project, for the CLI too.
func (a *App) UseContext(name string) error {
	svc, err := a.service()
	if err != nil {
		return err
	}
	return svc.Use(name)
}

// AddContext validates the token with Hetzner and stores it under name.
//
// This is the one place a token crosses JS -> Go. It is never returned,
// echoed in a message, or logged.
func (a *App) AddContext(name, token string) error {
	// Trimmed as the CLI trims a pasted token: stray whitespace from a
	// copy-paste would otherwise be stored and rejected by Hetzner later.
	// Checked before anything else so an empty field never costs a network call.
	name, token = strings.TrimSpace(name), strings.TrimSpace(token)
	if name == "" {
		return errors.New("no name given; nothing saved")
	}
	if token == "" {
		return errors.New("no token given; nothing saved")
	}
	svc, err := a.service()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(a.ctx, apiTimeout)
	defer cancel()
	_, err = svc.Add(ctx, name, token)
	return err
}

// DeleteContext removes name from the store. The page confirms first; the
// token itself is not revoked at Hetzner.
func (a *App) DeleteContext(name string) error {
	svc, err := a.service()
	if err != nil {
		return err
	}
	_, err = svc.Delete(name)
	return err
}
