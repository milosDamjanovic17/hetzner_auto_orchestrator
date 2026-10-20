package service

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/hetzner"
	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/preflight"
)

// Server writes (phase-2b). Every guard runs here, before any request that
// changes something, so the CLI and the GUI cannot differ on what is allowed.

// writeTimeout bounds a whole write, guards included (user, 2026-10-20).
// A write Hetzner accepted but has not finished by then is reported as
// hetzner.ErrStillRunning, not as a failure.
const writeTimeout = 75 * time.Second

var (
	// ErrWrongProject: the active project changed after the driver showed
	// the user what they were about to change (guard 1).
	ErrWrongProject = errors.New("the active project changed; nothing changed")
	// ErrNameMismatch: the typed name is not the server's (guard 2).
	ErrNameMismatch = errors.New("typed name does not match the server; nothing changed")
	// ErrNotManaged: hao deletes only servers it created (guard 3).
	ErrNotManaged = errors.New("not created by hao (no managed-by=hao label); delete it in the Console")
	// ErrBackupsOff: Hetzner refuses a manual backup then, with a generic
	// error, so it is checked here first.
	ErrBackupsOff  = errors.New("backups are off for this server; enable them in the Console")
	ErrUnavailable = errors.New("this server type is not available in that location right now")
)

// cloud is what server writes need from Hetzner. An interface so tests can
// prove a refused write sends nothing, without a network.
type cloud interface {
	Server(ctx context.Context, id int64) (hetzner.Server, error)
	CreateServer(ctx context.Context, o hetzner.CreateServerOpts) (hetzner.Server, error)
	DeleteServer(ctx context.Context, id int64) error
	PowerOn(ctx context.Context, id int64) error
	Shutdown(ctx context.Context, id int64) error
	PowerOff(ctx context.Context, id int64) error
	Reboot(ctx context.Context, id int64) error
	SetProtection(ctx context.Context, id int64, on bool) error
	Backup(ctx context.Context, id int64) (int64, error)
	Snapshot(ctx context.Context, id int64, description string) (int64, error)
	Prices(ctx context.Context) (hetzner.Prices, error)
	IsAvailable(ctx context.Context, serverType, location string) (bool, error)
}

type liveCloud struct {
	*hetzner.Client
	*preflight.Checker
}

func newLiveCloud(token string) cloud {
	return liveCloud{hetzner.NewClient(token), preflight.NewChecker(token)}
}

// writeCloud applies guard 1: the store is reloaded and the write refused
// unless the active project is still the one the user was shown.
func (s *Service) writeCloud(expectedContext string) (cloud, error) {
	name, token, err := s.activeToken()
	if err != nil {
		return nil, err
	}
	if name != expectedContext {
		return nil, fmt.Errorf("%w (you were shown %q, active is now %q)", ErrWrongProject, expectedContext, name)
	}
	return s.cloudFor(token), nil
}

// confirmName applies guard 2: target by ID, confirm by the name typed.
func confirmName(ctx context.Context, c cloud, id int64, typedName string) (hetzner.Server, error) {
	srv, err := c.Server(ctx, id)
	if err != nil {
		return hetzner.Server{}, err
	}
	if typedName != srv.Name {
		return hetzner.Server{}, fmt.Errorf("%w (typed %q, server %d is %q)", ErrNameMismatch, typedName, id, srv.Name)
	}
	return srv, nil
}

// CreateServer creates a server after checking the project and that the type
// is available in the location. The driver shows Estimate first.
func (s *Service) CreateServer(ctx context.Context, expectedContext string, o hetzner.CreateServerOpts) (hetzner.Server, error) {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	c, err := s.writeCloud(expectedContext)
	if err != nil {
		return hetzner.Server{}, err
	}
	ok, err := c.IsAvailable(ctx, o.Type, o.Location)
	if err != nil {
		return hetzner.Server{}, err
	}
	if !ok {
		return hetzner.Server{}, fmt.Errorf("%w (%s in %s)", ErrUnavailable, o.Type, o.Location)
	}
	return c.CreateServer(ctx, o)
}

// DeleteServer deletes a server hao created, once its name was typed.
// Hetzner's delete protection still applies on top (hetzner.ErrProtected).
func (s *Service) DeleteServer(ctx context.Context, expectedContext string, id int64, typedName string) error {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	c, err := s.writeCloud(expectedContext)
	if err != nil {
		return err
	}
	srv, err := confirmName(ctx, c, id, typedName)
	if err != nil {
		return err
	}
	if !srv.Managed {
		return fmt.Errorf("server %q: %w", srv.Name, ErrNotManaged)
	}
	return c.DeleteServer(ctx, id)
}

// Power runs one of the CLI's power verbs: poweron, shutdown (graceful),
// poweroff (hard) or reboot. Allowed on any server, labelled or not.
func (s *Service) Power(ctx context.Context, expectedContext string, id int64, verb string) error {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	c, err := s.writeCloud(expectedContext)
	if err != nil {
		return err
	}
	switch verb {
	case "poweron":
		return c.PowerOn(ctx, id)
	case "shutdown":
		return c.Shutdown(ctx, id)
	case "poweroff":
		return c.PowerOff(ctx, id)
	case "reboot":
		return c.Reboot(ctx, id)
	}
	return fmt.Errorf("unknown power action %q", verb)
}

// Protect turns delete and rebuild protection on. Adds a guard, so no typed name.
func (s *Service) Protect(ctx context.Context, expectedContext string, id int64) error {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	c, err := s.writeCloud(expectedContext)
	if err != nil {
		return err
	}
	return c.SetProtection(ctx, id, true)
}

// Unprotect turns protection off. It removes a guard, so the name is typed.
func (s *Service) Unprotect(ctx context.Context, expectedContext string, id int64, typedName string) error {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	c, err := s.writeCloud(expectedContext)
	if err != nil {
		return err
	}
	if _, err := confirmName(ctx, c, id, typedName); err != nil {
		return err
	}
	return c.SetProtection(ctx, id, false)
}

// Backup creates a manual backup now and returns the image ID. hao never
// turns backups on or off; that is the Console's job.
func (s *Service) Backup(ctx context.Context, expectedContext string, id int64) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	c, err := s.writeCloud(expectedContext)
	if err != nil {
		return 0, err
	}
	srv, err := c.Server(ctx, id)
	if err != nil {
		return 0, err
	}
	if !srv.Backups {
		return 0, fmt.Errorf("server %q: %w", srv.Name, ErrBackupsOff)
	}
	return c.Backup(ctx, id)
}

// Snapshot creates a snapshot and returns the image ID.
func (s *Service) Snapshot(ctx context.Context, expectedContext string, id int64, description string) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	c, err := s.writeCloud(expectedContext)
	if err != nil {
		return 0, err
	}
	return c.Snapshot(ctx, id, description)
}

// Estimate is the cost check shown before every create (phase-2b §5.1).
// Amounts are gross, in euro, 4 decimals.
type Estimate struct {
	Context  string    `json:"context"`
	Fetched  time.Time `json:"fetched"`
	Type     string    `json:"type"`
	Location string    `json:"location"`
	Hourly   string    `json:"hourly"`
	Monthly  string    `json:"monthly"` // Hetzner caps the hourly bill here
	IPv4     string    `json:"ipv4"`    // primary IPv4, per month
	Total    string    `json:"total"`   // 30 days 24/7: min(Hourly × 720, Monthly) + IPv4
}

// Estimate fetches live prices and works out the 30-day cost of a server.
// Never cached: it shows what Hetzner says at this moment.
func (s *Service) Estimate(ctx context.Context, serverType, location string) (Estimate, error) {
	name, token, err := s.activeToken()
	if err != nil {
		return Estimate{}, err
	}
	p, err := s.cloudFor(token).Prices(ctx)
	if err != nil {
		return Estimate{}, err
	}
	e, err := estimate(p, serverType, location)
	if err != nil {
		return Estimate{}, err
	}
	e.Context, e.Fetched = name, time.Now()
	return e, nil
}

// estimate assumes the server runs 24/7 for 30 days (user, 2026-10-20).
// A price that is missing or unreadable is an error, never zero: the user
// must not be told a server is free.
func estimate(p hetzner.Prices, serverType, location string) (Estimate, error) {
	e := Estimate{Type: serverType, Location: location}
	var hourly, monthly, ipv4 *big.Rat
	for _, sp := range p.Servers {
		if sp.Type == serverType && sp.Location == location {
			hourly, monthly = parsePrice(sp.Hourly), parsePrice(sp.Monthly)
			if hourly == nil || monthly == nil {
				return Estimate{}, fmt.Errorf("unreadable price for %s in %s (%q, %q)", serverType, location, sp.Hourly, sp.Monthly)
			}
		}
	}
	if hourly == nil {
		return Estimate{}, fmt.Errorf("Hetzner lists no price for %s in %s", serverType, location)
	}
	for _, ip := range p.PrimaryIPv4 {
		if ip.Location == location {
			if ipv4 = parsePrice(ip.Monthly); ipv4 == nil {
				return Estimate{}, fmt.Errorf("unreadable IPv4 price for %s (%q)", location, ip.Monthly)
			}
		}
	}
	if ipv4 == nil {
		return Estimate{}, fmt.Errorf("Hetzner lists no primary IPv4 price for %s", location)
	}

	server := new(big.Rat).Mul(hourly, big.NewRat(720, 1))
	if server.Cmp(monthly) > 0 {
		server = monthly
	}
	total := new(big.Rat).Add(server, ipv4)
	e.Hourly, e.Monthly, e.IPv4, e.Total = euro(hourly), euro(monthly), euro(ipv4), euro(total)
	return e, nil
}

// parsePrice reads one of the API's decimal strings exactly. big.Rat, not
// float64: money must not pick up binary rounding. nil if unreadable.
func parsePrice(s string) *big.Rat {
	r, ok := new(big.Rat).SetString(s)
	if !ok || r.Sign() < 0 {
		return nil
	}
	return r
}

func euro(r *big.Rat) string { return r.FloatString(4) }

// ImageCost is the cost line shown before a manual backup or a snapshot.
type ImageCost struct {
	Context       string `json:"context"`
	PerGBMonth    string `json:"per_gb_month"`   // snapshots, gross
	SnapshotMax   string `json:"snapshot_max"`   // PerGBMonth × server disk: the most a snapshot can cost
	BackupPercent string `json:"backup_percent"` // the backup option, % of the server price
}

// ImageCost fetches live prices for a server with diskGB of disk.
func (s *Service) ImageCost(ctx context.Context, diskGB int) (ImageCost, error) {
	name, token, err := s.activeToken()
	if err != nil {
		return ImageCost{}, err
	}
	p, err := s.cloudFor(token).Prices(ctx)
	if err != nil {
		return ImageCost{}, err
	}
	c, err := imageCost(p, diskGB)
	c.Context = name
	return c, err
}

// imageCost bounds a snapshot's monthly cost by the disk size: a snapshot
// is never bigger than the disk. As with Estimate, never "free" on bad data.
func imageCost(p hetzner.Prices, diskGB int) (ImageCost, error) {
	perGB := parsePrice(p.ImagePerGBMonth)
	if perGB == nil {
		return ImageCost{}, fmt.Errorf("unreadable snapshot price %q", p.ImagePerGBMonth)
	}
	if diskGB <= 0 {
		return ImageCost{}, fmt.Errorf("server disk size unknown (%d GB)", diskGB)
	}
	upper := new(big.Rat).Mul(perGB, big.NewRat(int64(diskGB), 1))
	return ImageCost{PerGBMonth: euro(perGB), SnapshotMax: euro(upper), BackupPercent: p.BackupPercent}, nil
}
