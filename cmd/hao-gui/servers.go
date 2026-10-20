package main

import (
	"context"
	"errors"

	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/hetzner"
	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/preflight"
	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/service"
)

// Server writes (phase-2b Step 4). Every guard runs in internal/service; these
// methods only pass on what the page showed. expectedContext is always the
// project named by the response the page rendered (Listing.Context or
// CreateOptions.Context), so a project switched in the CLI meanwhile is
// refused in Go.
//
// Writes run on a.ctx, not apiTimeout: the service bounds each one (75 s).
// They return "" when done, or Go's note when Hetzner is still at it; the
// page writes its own success notice, as for contexts.

// CreateOptions is what the create form offers, all from one project.
type CreateOptions struct {
	Context string                   `json:"context"`
	Types   []preflight.Availability `json:"types"` // type + location pairs available right now
	Images  []hetzner.Image          `json:"images"`
	SSHKeys []hetzner.SSHKey         `json:"ssh_keys"`
}

// CreateOptions fetches the form's choices for the active project.
func (a *App) CreateOptions() (CreateOptions, error) {
	svc, err := a.service()
	if err != nil {
		return CreateOptions{}, err
	}
	name, client, err := svc.ActiveClient()
	if err != nil {
		return CreateOptions{}, err
	}
	checkerName, checker, err := svc.ActiveChecker()
	if err != nil {
		return CreateOptions{}, err
	}
	// Each call reloads the store: a switch in between would mix two projects.
	if checkerName != name {
		return CreateOptions{}, errors.New("the active project changed while loading; refresh")
	}
	ctx, cancel := context.WithTimeout(a.ctx, apiTimeout)
	defer cancel()
	all, err := checker.All(ctx)
	if err != nil {
		return CreateOptions{}, err
	}
	images, err := client.SystemImages(ctx)
	if err != nil {
		return CreateOptions{}, err
	}
	keys, err := client.SSHKeys(ctx)
	if err != nil {
		return CreateOptions{}, err
	}
	return createOptions(name, all, images, keys), nil
}

// createOptions keeps only the type + location pairs that can be created
// now: offering an out-of-stock pair would only fail at the create.
func createOptions(name string, all []preflight.Availability, images []hetzner.Image, keys []hetzner.SSHKey) CreateOptions {
	o := CreateOptions{Context: name, Types: []preflight.Availability{}, Images: images, SSHKeys: keys}
	for _, a := range all {
		if a.Available {
			o.Types = append(o.Types, a)
		}
	}
	// JSON null would break the page's lists; no keys is a valid answer
	// (the form then says to add one in the Console).
	if o.Images == nil {
		o.Images = []hetzner.Image{}
	}
	if o.SSHKeys == nil {
		o.SSHKeys = []hetzner.SSHKey{}
	}
	return o
}

// Estimate is the cost box: live prices, 30 days 24/7.
func (a *App) Estimate(serverType, location string) (service.Estimate, error) {
	svc, err := a.service()
	if err != nil {
		return service.Estimate{}, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, apiTimeout)
	defer cancel()
	return svc.Estimate(ctx, serverType, location)
}

// ImageCost is the cost line shown before a backup or a snapshot.
func (a *App) ImageCost(diskGB int) (service.ImageCost, error) {
	svc, err := a.service()
	if err != nil {
		return service.ImageCost{}, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, apiTimeout)
	defer cancel()
	return svc.ImageCost(ctx, diskGB)
}

// finish is the CLI's rule: "still running" is not a failure. Hetzner
// accepted the write and carries on, so the page shows it as a notice.
func finish(err error) (string, error) {
	if errors.Is(err, hetzner.ErrStillRunning) {
		return err.Error(), nil
	}
	return "", err
}

func (a *App) CreateServer(expectedContext, name, serverType, location, image string, sshKeys []string) (string, error) {
	svc, err := a.service()
	if err != nil {
		return "", err
	}
	_, err = svc.CreateServer(a.ctx, expectedContext, hetzner.CreateServerOpts{
		Name: name, Type: serverType, Location: location, Image: image, SSHKeys: sshKeys,
	})
	return finish(err)
}

func (a *App) DeleteServer(expectedContext string, id int64, typedName string) (string, error) {
	svc, err := a.service()
	if err != nil {
		return "", err
	}
	return finish(svc.DeleteServer(a.ctx, expectedContext, id, typedName))
}

// PowerServer takes the CLI's verbs: poweron, shutdown, poweroff, reboot.
func (a *App) PowerServer(expectedContext string, id int64, verb string) (string, error) {
	svc, err := a.service()
	if err != nil {
		return "", err
	}
	return finish(svc.Power(a.ctx, expectedContext, id, verb))
}

func (a *App) ProtectServer(expectedContext string, id int64) (string, error) {
	svc, err := a.service()
	if err != nil {
		return "", err
	}
	return finish(svc.Protect(a.ctx, expectedContext, id))
}

func (a *App) UnprotectServer(expectedContext string, id int64, typedName string) (string, error) {
	svc, err := a.service()
	if err != nil {
		return "", err
	}
	return finish(svc.Unprotect(a.ctx, expectedContext, id, typedName))
}

func (a *App) BackupServer(expectedContext string, id int64) (string, error) {
	svc, err := a.service()
	if err != nil {
		return "", err
	}
	_, err = svc.Backup(a.ctx, expectedContext, id)
	return finish(err)
}

func (a *App) SnapshotServer(expectedContext string, id int64, description string) (string, error) {
	svc, err := a.service()
	if err != nil {
		return "", err
	}
	_, err = svc.Snapshot(a.ctx, expectedContext, id, description)
	return finish(err)
}
