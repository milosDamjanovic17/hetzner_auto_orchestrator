package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/hetzner"
)

// fakeCloud stands in for Hetzner. Reads answer from servers; every write is
// recorded, so a test can prove a refused operation changed nothing.
type fakeCloud struct {
	servers   map[int64]hetzner.Server
	available bool
	writes    []string
	deadline  time.Duration // time left on the last write's context
}

func (f *fakeCloud) write(ctx context.Context, op string) {
	f.writes = append(f.writes, op)
	if d, ok := ctx.Deadline(); ok {
		f.deadline = time.Until(d)
	}
}

func (f *fakeCloud) Server(_ context.Context, id int64) (hetzner.Server, error) {
	if s, ok := f.servers[id]; ok {
		return s, nil
	}
	return hetzner.Server{}, fmt.Errorf("getting server %d: %w", id, hetzner.ErrNotFound)
}

func (f *fakeCloud) CreateServer(ctx context.Context, o hetzner.CreateServerOpts) (hetzner.Server, error) {
	f.write(ctx, "create "+o.Name)
	return hetzner.Server{ID: 99, Name: o.Name}, nil
}

func (f *fakeCloud) DeleteServer(ctx context.Context, id int64) error {
	f.write(ctx, fmt.Sprintf("delete %d", id))
	return nil
}

func (f *fakeCloud) PowerOn(ctx context.Context, id int64) error {
	f.write(ctx, fmt.Sprintf("poweron %d", id))
	return nil
}

func (f *fakeCloud) Shutdown(ctx context.Context, id int64) error {
	f.write(ctx, fmt.Sprintf("shutdown %d", id))
	return nil
}

func (f *fakeCloud) PowerOff(ctx context.Context, id int64) error {
	f.write(ctx, fmt.Sprintf("poweroff %d", id))
	return nil
}

func (f *fakeCloud) Reboot(ctx context.Context, id int64) error {
	f.write(ctx, fmt.Sprintf("reboot %d", id))
	return nil
}

func (f *fakeCloud) SetProtection(ctx context.Context, id int64, on bool) error {
	f.write(ctx, fmt.Sprintf("protect %d %v", id, on))
	return nil
}

func (f *fakeCloud) Backup(ctx context.Context, id int64) (int64, error) {
	f.write(ctx, fmt.Sprintf("backup %d", id))
	return 500, nil
}

func (f *fakeCloud) Snapshot(ctx context.Context, id int64, _ string) (int64, error) {
	f.write(ctx, fmt.Sprintf("snapshot %d", id))
	return 501, nil
}

func (f *fakeCloud) Prices(context.Context) (hetzner.Prices, error) { return fixedPrices, nil }

func (f *fakeCloud) IsAvailable(context.Context, string, string) (bool, error) {
	return f.available, nil
}

// Servers in the fake project "prod": ours (labelled by hao, backups on) and
// theirs (made elsewhere, backups off).
const (
	ours   int64 = 1
	theirs int64 = 2
)

// newWriteService returns a Service with contexts prod (active) and staging,
// talking to a fake Hetzner. clouds counts how often a client was built.
func newWriteService(t *testing.T) (s *Service, f *fakeCloud, clouds *int) {
	t.Helper()
	s, _, _ = newTestService(t)
	for _, n := range []string{"prod", "staging"} {
		if _, err := s.Add(context.Background(), n, "tok-good"); err != nil {
			t.Fatalf("Add %s: %v", n, err)
		}
	}
	f = &fakeCloud{
		available: true,
		servers: map[int64]hetzner.Server{
			ours:   {ID: ours, Name: "web-1", Managed: true, Backups: true},
			theirs: {ID: theirs, Name: "legacy"},
		},
	}
	clouds = new(int)
	s.cloudFor = func(string) cloud { *clouds++; return f }
	return s, f, clouds
}

var createOpts = hetzner.CreateServerOpts{Name: "web-2", Type: "cpx02", Location: "fsn1", Image: "ubuntu-24.04", SSHKeys: []string{"me"}}

// Every write, by name, with the inputs a driver would pass when the user
// confirmed correctly. Used to prove each one applies guard 1.
var writes = map[string]func(s *Service, expected string) error{
	"create": func(s *Service, e string) error {
		_, err := s.CreateServer(context.Background(), e, createOpts)
		return err
	},
	"delete":  func(s *Service, e string) error { return s.DeleteServer(context.Background(), e, ours, "web-1") },
	"power":   func(s *Service, e string) error { return s.Power(context.Background(), e, ours, "poweroff") },
	"protect": func(s *Service, e string) error { return s.Protect(context.Background(), e, ours) },
	"unprotect": func(s *Service, e string) error {
		return s.Unprotect(context.Background(), e, ours, "web-1")
	},
	"backup":   func(s *Service, e string) error { _, err := s.Backup(context.Background(), e, ours); return err },
	"snapshot": func(s *Service, e string) error { _, err := s.Snapshot(context.Background(), e, ours, ""); return err },
}

// Guard 1. The user confirmed a change looking at "staging", but the CLI (or
// another window) switched to "prod" meanwhile. Going ahead would change the
// wrong project. Refused before any call to Hetzner at all.
func TestEveryWriteRefusesWhenTheProjectChanged(t *testing.T) {
	for name, write := range writes {
		t.Run(name, func(t *testing.T) {
			s, f, clouds := newWriteService(t)
			err := write(s, "staging") // active is prod
			if !errors.Is(err, ErrWrongProject) {
				t.Errorf("err = %v, want ErrWrongProject", err)
			}
			if *clouds != 0 || len(f.writes) != 0 {
				t.Errorf("reached Hetzner (%d clients, writes %v), want nothing", *clouds, f.writes)
			}
		})
	}
}

// The same calls with the right project go through, each as exactly one
// write, and within the 75 s write deadline: a write must never hang a driver.
func TestEveryWriteGoesThroughWhenConfirmedCorrectly(t *testing.T) {
	for name, write := range writes {
		t.Run(name, func(t *testing.T) {
			s, f, _ := newWriteService(t)
			if err := write(s, "prod"); err != nil {
				t.Fatalf("err = %v", err)
			}
			if len(f.writes) != 1 {
				t.Errorf("writes = %v, want exactly one", f.writes)
			}
			if f.deadline <= 0 || f.deadline > writeTimeout {
				t.Errorf("write ran with %v left, want a deadline within %v", f.deadline, writeTimeout)
			}
		})
	}
}

// The other guards. Each refusal must leave Hetzner untouched.
func TestGuardsRefuseWithoutWriting(t *testing.T) {
	for _, tc := range []struct {
		name  string
		write func(s *Service) error
		want  error
	}{
		// Guard 2: a typo'd name is a cancel, never "close enough".
		{"delete, wrong name", func(s *Service) error {
			return s.DeleteServer(context.Background(), "prod", ours, "web-2")
		}, ErrNameMismatch},
		// Guard 3: hao deletes only what it created.
		{"delete, not created by hao", func(s *Service) error {
			return s.DeleteServer(context.Background(), "prod", theirs, "legacy")
		}, ErrNotManaged},
		// Turning protection off removes a guard: same typed name.
		{"unprotect, wrong name", func(s *Service) error {
			return s.Unprotect(context.Background(), "prod", ours, "web")
		}, ErrNameMismatch},
		// Hetzner refuses this with a generic error; ours names the fix.
		{"backup, backups off", func(s *Service) error {
			_, err := s.Backup(context.Background(), "prod", theirs)
			return err
		}, ErrBackupsOff},
		{"delete, no such server", func(s *Service) error {
			return s.DeleteServer(context.Background(), "prod", 404, "web-1")
		}, hetzner.ErrNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, f, _ := newWriteService(t)
			if err := tc.write(s); !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
			if len(f.writes) != 0 {
				t.Errorf("writes = %v, want none", f.writes)
			}
		})
	}
}

// Out of stock: the create is refused before it is sent, not left to fail
// at Hetzner after the user already agreed to the cost.
func TestCreateRefusedWhenUnavailable(t *testing.T) {
	s, f, _ := newWriteService(t)
	f.available = false
	if _, err := s.CreateServer(context.Background(), "prod", createOpts); !errors.Is(err, ErrUnavailable) {
		t.Errorf("err = %v, want ErrUnavailable", err)
	}
	if len(f.writes) != 0 {
		t.Errorf("writes = %v, want none", f.writes)
	}
}

// Power runs exactly the action named. A mix-up between shutdown (graceful)
// and poweroff (cuts power, may lose data) is not harmless.
func TestPowerRunsTheNamedAction(t *testing.T) {
	for _, verb := range []string{"poweron", "shutdown", "poweroff", "reboot"} {
		s, f, _ := newWriteService(t)
		if err := s.Power(context.Background(), "prod", theirs, verb); err != nil {
			t.Fatalf("Power %s: %v", verb, err)
		}
		if want := []string{fmt.Sprintf("%s %d", verb, theirs)}; !slices.Equal(f.writes, want) {
			t.Errorf("Power %s wrote %v, want %v", verb, f.writes, want)
		}
	}
	s, f, _ := newWriteService(t)
	if err := s.Power(context.Background(), "prod", ours, "rset"); err == nil || len(f.writes) != 0 {
		t.Errorf("unknown verb: err = %v, writes %v; want an error and no write", err, f.writes)
	}
}

// Protect and unprotect must not be swapped: one adds a guard, one removes it.
func TestProtectAndUnprotectSetTheRightState(t *testing.T) {
	s, f, _ := newWriteService(t)
	if err := s.Protect(context.Background(), "prod", ours); err != nil {
		t.Fatalf("Protect: %v", err)
	}
	if err := s.Unprotect(context.Background(), "prod", ours, "web-1"); err != nil {
		t.Fatalf("Unprotect: %v", err)
	}
	if want := []string{"protect 1 true", "protect 1 false"}; !slices.Equal(f.writes, want) {
		t.Errorf("writes = %v, want %v", f.writes, want)
	}
}

// A fixed price table. cpx02 matches what Hetzner returned in Step 0.
var fixedPrices = hetzner.Prices{
	Servers: []hetzner.ServerPrice{
		{Type: "cpx02", Location: "fsn1", Hourly: "0.0096000000000000", Monthly: "5.9900000000000000"},
		{Type: "cheap", Location: "fsn1", Hourly: "0.0050", Monthly: "5.9900"},
		{Type: "broken", Location: "fsn1", Hourly: "n/a", Monthly: "5.9900"},
		{Type: "cpx02", Location: "nowhere", Hourly: "0.0096", Monthly: "5.99"},
	},
	PrimaryIPv4: []hetzner.IPPrice{{Location: "fsn1", Monthly: "0.5000"}},
}

// The cost check the user agrees to before paying. 30 days 24/7 = 720 h, and
// Hetzner caps the hourly bill at the monthly price.
func TestEstimateThirtyDaysWithTheMonthlyCap(t *testing.T) {
	for _, tc := range []struct {
		typ, location string
		want          string // Total; "" = must be an error
	}{
		// 0.0096 x 720 = 6.912 > cap 5.99 -> 5.99 + IPv4 0.50
		{"cpx02", "fsn1", "6.4900"},
		// 0.005 x 720 = 3.60 < cap -> 3.60 + 0.50
		{"cheap", "fsn1", "4.1000"},
		// Never "free": an unreadable, missing price or IPv4 price is an error.
		{"broken", "fsn1", ""},
		{"cax99", "fsn1", ""},
		{"cpx02", "nowhere", ""},
	} {
		e, err := estimate(fixedPrices, tc.typ, tc.location)
		if tc.want == "" {
			if err == nil {
				t.Errorf("%s in %s: estimate %+v, want an error", tc.typ, tc.location, e)
			}
			continue
		}
		if err != nil || e.Total != tc.want {
			t.Errorf("%s in %s: total %q, err %v; want %s", tc.typ, tc.location, e.Total, err, tc.want)
		}
	}
}

// The driver shows the project with the price, then passes it back as
// expectedContext: the estimate must name the project it was fetched for.
func TestEstimateNamesTheProject(t *testing.T) {
	s, _, _ := newWriteService(t)
	e, err := s.Estimate(context.Background(), "cpx02", "fsn1")
	if err != nil {
		t.Fatalf("Estimate: %v", err)
	}
	if e.Context != "prod" || e.Hourly != "0.0096" || e.Monthly != "5.9900" || e.IPv4 != "0.5000" {
		t.Errorf("estimate = %+v, want context prod, 0.0096 / 5.9900 / 0.5000", e)
	}
}

// Shown before a snapshot: the most it can cost per month. A snapshot is at
// most the disk size, so per-GB x disk is a safe upper bound.
func TestImageCostBoundsSnapshotByDisk(t *testing.T) {
	// BackupPercent as the live API sends it (Step 3 gate, 2026-10-20).
	p := hetzner.Prices{ImagePerGBMonth: "0.0143", BackupPercent: "20.0000000000"}
	c, err := imageCost(p, 40)
	if err != nil || c.SnapshotMax != "0.5720" || c.PerGBMonth != "0.0143" || c.BackupPercent != "20.00" {
		t.Errorf("imageCost = %+v, %v; want max 0.5720, 0.0143/GB, backup 20.00", c, err)
	}
	// Never "free" on bad data.
	if _, err := imageCost(p, 0); err == nil {
		t.Error("disk 0 GB: want an error")
	}
	if _, err := imageCost(hetzner.Prices{ImagePerGBMonth: "?"}, 40); err == nil {
		t.Error("unreadable price: want an error")
	}
}
