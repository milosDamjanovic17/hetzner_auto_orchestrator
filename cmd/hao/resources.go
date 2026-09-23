package main

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/hetzner"
	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/preflight"
)

// consoleURL is where API tokens and project members are managed. Checked
// 2026-09-23: the old console.hetzner.cloud redirects here.
const consoleURL = "https://console.hetzner.com/"

// resource is one listable Hetzner resource type as the CLI exposes it.
type resource struct {
	plural string // for the empty case: "no servers in this project"
	// list fetches the resource and formats one line per item.
	list func(context.Context, *hetzner.Client) ([]string, error)
}

// resources maps each CLI noun to its lister. Nouns match the hcloud CLI
// (`hcloud server list`, `hcloud load-balancer list`, ...) so habits carry over.
var resources = map[string]resource{
	"server": {"servers", func(ctx context.Context, c *hetzner.Client) ([]string, error) {
		items, err := c.Servers(ctx)
		return formatEach(items, err, func(s hetzner.Server) string {
			return fmt.Sprintf("%-32s %-10s %-10s %-6s %s",
				s.Name, s.Status, s.ServerType, s.Location, orDash(s.PublicIPv4))
		})
	}},
	"load-balancer": {"load balancers", func(ctx context.Context, c *hetzner.Client) ([]string, error) {
		items, err := c.LoadBalancers(ctx)
		return formatEach(items, err, func(lb hetzner.LoadBalancer) string {
			return fmt.Sprintf("%-32s %-8s %-6s %-15s %d services, %d targets",
				lb.Name, lb.Type, lb.Location, orDash(lb.PublicIPv4), lb.Services, lb.Targets)
		})
	}},
	"network": {"networks", func(ctx context.Context, c *hetzner.Client) ([]string, error) {
		items, err := c.Networks(ctx)
		return formatEach(items, err, func(n hetzner.Network) string {
			return fmt.Sprintf("%-32s %-18s %d subnets, %d servers",
				n.Name, orDash(n.IPRange), n.Subnets, n.Servers)
		})
	}},
	"firewall": {"firewalls", func(ctx context.Context, c *hetzner.Client) ([]string, error) {
		items, err := c.Firewalls(ctx)
		return formatEach(items, err, func(f hetzner.Firewall) string {
			return fmt.Sprintf("%-32s %d rules, applied to %d", f.Name, f.Rules, f.AppliedTo)
		})
	}},
	"floating-ip": {"floating IPs", func(ctx context.Context, c *hetzner.Client) ([]string, error) {
		items, err := c.FloatingIPs(ctx)
		return formatEach(items, err, func(ip hetzner.FloatingIP) string {
			return fmt.Sprintf("%-32s %-4s %-39s %-6s assigned to %s",
				ip.Name, ip.Type, orDash(ip.IP), ip.HomeLocation, orDash(ip.AssignedTo))
		})
	}},
	"volume": {"volumes", func(ctx context.Context, c *hetzner.Client) ([]string, error) {
		items, err := c.Volumes(ctx)
		return formatEach(items, err, func(v hetzner.Volume) string {
			return fmt.Sprintf("%-32s %-10s %5d GB %-6s attached to %s",
				v.Name, v.Status, v.SizeGB, v.Location, orDash(v.AttachedTo))
		})
	}},
	"ssh-key": {"SSH keys", func(ctx context.Context, c *hetzner.Client) ([]string, error) {
		items, err := c.SSHKeys(ctx)
		return formatEach(items, err, func(k hetzner.SSHKey) string {
			return fmt.Sprintf("%-32s %s", k.Name, k.Fingerprint)
		})
	}},
	"certificate": {"certificates", func(ctx context.Context, c *hetzner.Client) ([]string, error) {
		items, err := c.Certificates(ctx)
		return formatEach(items, err, func(cert hetzner.Certificate) string {
			expires := "-" // a managed certificate still being issued has no expiry yet
			if !cert.NotValidAfter.IsZero() {
				expires = cert.NotValidAfter.Format("2006-01-02")
			}
			return fmt.Sprintf("%-32s %-10s expires %-10s %s",
				cert.Name, cert.Type, expires, strings.Join(cert.DomainNames, ","))
		})
	}},
	"zone": {"zones", func(ctx context.Context, c *hetzner.Client) ([]string, error) {
		items, err := c.Zones(ctx)
		return formatEach(items, err, func(z hetzner.Zone) string {
			return fmt.Sprintf("%-32s %-10s %-10s %d records", z.Name, z.Status, z.Mode, z.RecordCount)
		})
	}},
}

// formatEach turns a list result into printable lines, passing any error
// through. Taking err as a parameter lets each lister stay a two-liner.
func formatEach[T any](items []T, err error, format func(T) string) ([]string, error) {
	if err != nil {
		return nil, err
	}
	lines := make([]string, 0, len(items))
	for _, it := range items {
		lines = append(lines, format(it))
	}
	return lines, nil
}

// orDash keeps columns readable when a field is legitimately empty, such as an
// unattached volume or a server without a public IPv4.
func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// cmdList runs `hao <noun> list` for any entry in resources.
func cmdList(noun string, r resource, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("%s: expected `list`", noun)
	}
	if args[0] != "list" {
		return fmt.Errorf("%s: unknown subcommand %q", noun, args[0])
	}

	client, err := activeClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()

	lines, err := r.list(ctx, client)
	if err != nil {
		return err
	}
	if len(lines) == 0 {
		// An empty project is a valid answer, not a failure.
		fmt.Printf("no %s in this project\n", r.plural)
		return nil
	}
	for _, l := range lines {
		fmt.Println(l)
	}
	return nil
}

// cmdPreflight answers server type availability for the active context.
//
// Every answer ends with the quota disclaimer: "available" here means Hetzner
// offers the type in that location, not that this project may create one more.
//
// Arguments may be separated by commas, spaces or both, so `fsn1, nbg1`,
// `fsn1,nbg1` and `fsn1 nbg1` are the same request.
func cmdPreflight(args []string) error {
	words := strings.FieldsFunc(strings.ToLower(strings.Join(args, " ")), func(r rune) bool {
		return r == ',' || unicode.IsSpace(r)
	})

	token, err := activeToken()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()

	// Fetched once; every mode below answers from this list.
	all, err := preflight.NewChecker(token).All(ctx)
	if err != nil {
		return err
	}

	switch {
	case len(words) == 0:
		for _, a := range all {
			printAvailability(a)
		}

	// Two words where the first is not a location: `<type> <location>`.
	// Otherwise `fsn1 nbg1` would be read as server type "fsn1".
	case len(words) == 2 && !preflight.IsLocation(all, words[0]):
		ok, err := preflight.Lookup(all, words[0], words[1])
		if err != nil {
			return err
		}
		if ok {
			fmt.Printf("%s in %s: available\n", words[0], words[1])
		} else {
			fmt.Printf("%s in %s: not available right now\n", words[0], words[1])
		}

	default:
		groups, err := preflight.AvailableIn(all, words)
		if err != nil {
			return err
		}
		for i, g := range groups {
			if i > 0 {
				fmt.Println()
			}
			fmt.Printf("%s (%s): %d server types available\n", g.Location, g.City, len(g.Available))
			for _, a := range g.Available {
				printAvailability(a)
			}
		}
	}

	fmt.Println("(availability only - not a quota check; project limits can still fail a create)")
	return nil
}

// printAvailability prints one server type in one location.
func printAvailability(a preflight.Availability) {
	state := "unavailable"
	if a.Available {
		state = "available"
	}
	var notes []string
	if a.Recommended {
		notes = append(notes, "recommended")
	}
	if a.Deprecated {
		notes = append(notes, "deprecated")
	}
	fmt.Printf("%-12s %-6s %-11s %3d cores %6.1f GB RAM %5d GB disk %-5s %s\n",
		a.ServerType, a.Location, state, a.Cores, a.MemoryGB, a.DiskGB,
		a.Architecture, strings.Join(notes, ","))
}

// cmdConsoleOnly answers `hao token ...` and `hao member ...`. Hetzner has no
// API for either, so the CLI says where to go instead of looking broken.
func cmdConsoleOnly(noun string) error {
	what := "API tokens"
	if noun == "member" {
		what = "project members"
	}
	fmt.Printf("%s cannot be listed or managed through the Hetzner API.\n", what)
	fmt.Printf("manage them in the Hetzner Console: %s\n", consoleURL)
	return nil
}
