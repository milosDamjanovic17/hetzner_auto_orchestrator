package main

import (
	"context"

	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/hetzner"
)

// Listing is one resource list plus the context it was fetched with.
//
// The page shows Context from the response, not its own idea of the active
// project: if the CLI switched context while the window was open, the list
// still says which project the data really came from.
type Listing[T any] struct {
	Context string `json:"context"`
	Items   []T    `json:"items"`
}

// list fetches one resource type for the active context. A function, not a
// method: Go methods cannot have type parameters.
func list[T any](a *App, fetch func(*hetzner.Client, context.Context) ([]T, error)) (Listing[T], error) {
	svc, err := a.service()
	if err != nil {
		return Listing[T]{}, err
	}
	name, client, err := svc.ActiveClient()
	if err != nil {
		return Listing[T]{}, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, apiTimeout)
	defer cancel()
	items, err := fetch(client, ctx)
	if err != nil {
		return Listing[T]{}, err
	}
	// A nil slice encodes as JSON null; the page expects an array, and an
	// empty project is a valid answer, not a failure.
	if items == nil {
		items = []T{}
	}
	return Listing[T]{Context: name, Items: items}, nil
}

func (a *App) Servers() (Listing[hetzner.Server], error) {
	return list(a, (*hetzner.Client).Servers)
}

func (a *App) LoadBalancers() (Listing[hetzner.LoadBalancer], error) {
	return list(a, (*hetzner.Client).LoadBalancers)
}

func (a *App) Networks() (Listing[hetzner.Network], error) {
	return list(a, (*hetzner.Client).Networks)
}

func (a *App) Firewalls() (Listing[hetzner.Firewall], error) {
	return list(a, (*hetzner.Client).Firewalls)
}

func (a *App) FloatingIPs() (Listing[hetzner.FloatingIP], error) {
	return list(a, (*hetzner.Client).FloatingIPs)
}

func (a *App) Volumes() (Listing[hetzner.Volume], error) {
	return list(a, (*hetzner.Client).Volumes)
}

func (a *App) SSHKeys() (Listing[hetzner.SSHKey], error) {
	return list(a, (*hetzner.Client).SSHKeys)
}

func (a *App) Certificates() (Listing[hetzner.Certificate], error) {
	return list(a, (*hetzner.Client).Certificates)
}

func (a *App) Zones() (Listing[hetzner.Zone], error) {
	return list(a, (*hetzner.Client).Zones)
}
