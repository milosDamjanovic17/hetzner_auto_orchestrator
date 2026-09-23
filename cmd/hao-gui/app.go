package main

import (
	"context"

	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/config"
	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/service"
)

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
