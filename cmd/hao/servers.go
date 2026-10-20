package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/hetzner"
	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/service"
)

// `hao server <action>`: the Phase 2b writes. Every guard lives in
// internal/service; this file resolves names, asks and prints. The few checks
// repeated here only stop the CLI from asking a question whose answer is
// already "refused".

var stdin = bufio.NewReader(os.Stdin)

// prompt prints question and returns the line typed, trimmed. End of input
// reads as an empty answer, which every caller treats as "no".
func prompt(question string) string {
	fmt.Print(question)
	line, _ := stdin.ReadString('\n')
	return strings.TrimSpace(line)
}

func confirmed(question string) bool {
	return strings.EqualFold(prompt(question+" (y/N): "), "y")
}

// finish prints the outcome of a write. "Still running" is not a failure:
// Hetzner accepted the write and carries on, so it is said plainly, exit 0.
func finish(err error, done string) error {
	if errors.Is(err, hetzner.ErrStillRunning) {
		fmt.Println(err)
		fmt.Println("check with `hao server list`")
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Println(done)
	return nil
}

// resolveServer finds a server in the active project by name, or by ID when
// no name matches. It returns the project too: the write passes it back as
// expectedContext, so a context switch in between is refused.
func resolveServer(svc *service.Service, arg string) (string, hetzner.Server, error) {
	project, client, err := svc.ActiveClient()
	if err != nil {
		return "", hetzner.Server{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()
	servers, err := client.Servers(ctx)
	if err != nil {
		return "", hetzner.Server{}, err
	}
	for _, s := range servers {
		if s.Name == arg {
			return project, s, nil
		}
	}
	if id, err := strconv.ParseInt(arg, 10, 64); err == nil {
		for _, s := range servers {
			if s.ID == id {
				return project, s, nil
			}
		}
	}
	return "", hetzner.Server{}, fmt.Errorf("no server %q in project %s", arg, project)
}

func cmdServer(args []string) error {
	if len(args) == 0 || args[0] == "list" {
		return cmdList("server", resources["server"], args)
	}
	svc, err := service.Open()
	if err != nil {
		return err
	}
	verb, rest := args[0], args[1:]

	if verb == "create" {
		if len(rest) != 5 {
			return errors.New("server create: expected <name> <type> <location> <image> <key>[,<key>...]")
		}
		return serverCreate(svc, rest)
	}
	if len(rest) == 0 {
		return fmt.Errorf("server %s: expected a server name or ID", verb)
	}
	switch verb {
	case "delete", "poweron", "shutdown", "poweroff", "reboot", "backup", "protect", "unprotect":
		if len(rest) != 1 {
			return fmt.Errorf("server %s: expected one server name or ID", verb)
		}
	case "snapshot":
	default:
		return fmt.Errorf("server: unknown subcommand %q", verb)
	}

	project, srv, err := resolveServer(svc, rest[0])
	if err != nil {
		return err
	}
	fmt.Printf("project: %s\n\n", project)
	bg := context.Background() // the service bounds every write (75 s)

	switch verb {
	case "delete":
		if !srv.Managed {
			return fmt.Errorf("server %q: %w", srv.Name, service.ErrNotManaged)
		}
		fmt.Printf("Delete server %q (id %d) in project %s? This cannot be undone.\n", srv.Name, srv.ID, project)
		typed := prompt("Type the server name to confirm: ")
		return finish(svc.DeleteServer(bg, project, srv.ID, typed), "deleted")

	case "poweron":
		return finish(svc.Power(bg, project, srv.ID, verb), "powered on")

	case "shutdown", "poweroff", "reboot":
		question := map[string]string{
			"shutdown": "Shut down server %q in project %s (the OS shuts down cleanly)?",
			"poweroff": "Power off server %q in project %s? This cuts power like pulling the plug; unsaved data is lost.",
			"reboot":   "Reboot server %q in project %s?",
		}[verb]
		if !confirmed(fmt.Sprintf(question, srv.Name, project)) {
			fmt.Println("cancelled, nothing changed")
			return nil
		}
		return finish(svc.Power(bg, project, srv.ID, verb), "done")

	case "backup":
		if !srv.Backups {
			return fmt.Errorf("server %q: %w", srv.Name, service.ErrBackupsOff)
		}
		cost, err := imageCost(svc, srv)
		if err != nil {
			return err
		}
		fmt.Printf("Manual backup of %q. Backups are billed through the backup option:\n", srv.Name)
		fmt.Printf("%s%% of the server price (Hetzner's live price list).\n", orUnknown(cost.BackupPercent))
		fmt.Println("Hetzner keeps up to 7 backups; when all are taken, the oldest is deleted.")
		if !confirmed("Create backup?") {
			fmt.Println("cancelled, nothing changed")
			return nil
		}
		id, err := svc.Backup(bg, project, srv.ID)
		return finish(err, fmt.Sprintf("backup image %d created", id))

	case "snapshot":
		cost, err := imageCost(svc, srv)
		if err != nil {
			return err
		}
		fmt.Printf("Snapshot of %q. Snapshots cost € %s per GB per month: at most\n", srv.Name, cost.PerGBMonth)
		fmt.Printf("€ %s / month for this server's %d GB disk. Kept until deleted in the Console.\n", cost.SnapshotMax, srv.DiskGB)
		if !confirmed("Create snapshot?") {
			fmt.Println("cancelled, nothing changed")
			return nil
		}
		id, err := svc.Snapshot(bg, project, srv.ID, strings.Join(rest[1:], " "))
		return finish(err, fmt.Sprintf("snapshot image %d created", id))

	case "protect":
		return finish(svc.Protect(bg, project, srv.ID), "delete and rebuild protection on")

	case "unprotect":
		fmt.Printf("Turn off delete and rebuild protection for %q in project %s?\n", srv.Name, project)
		typed := prompt("Type the server name to confirm: ")
		return finish(svc.Unprotect(bg, project, srv.ID, typed), "protection off")
	}
	return nil
}

func serverCreate(svc *service.Service, args []string) error {
	o := hetzner.CreateServerOpts{Name: args[0], Type: args[1], Location: args[2], Image: args[3]}
	for _, k := range strings.Split(args[4], ",") {
		if k = strings.TrimSpace(k); k != "" {
			o.SSHKeys = append(o.SSHKeys, k)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()
	e, err := svc.Estimate(ctx, o.Type, o.Location)
	if err != nil {
		return err
	}
	fmt.Printf("project: %s\n\n", e.Context)
	fmt.Println("Creating this server costs money. Prices fetched from Hetzner just now")
	fmt.Printf("(%s); Hetzner may change them.\n", e.Fetched.Format("2006-01-02 15:04"))
	fmt.Printf("  %s in %-8s € %s / hour, capped at € %s / month\n", e.Type, e.Location, e.Hourly, e.Monthly)
	fmt.Printf("  primary IPv4        € %s / month\n", e.IPv4)
	fmt.Printf("  Estimate, 30 days 24/7 (720 h): € %s\n", e.Total)
	fmt.Println("Billed while the server EXISTS, even when powered off. Not included:")
	fmt.Println("traffic over the included amount, backups, snapshots.")
	fmt.Println()

	question := fmt.Sprintf("Create %q (%s, %s, %s, keys %s) in project %s? Type yes: ",
		o.Name, o.Type, o.Location, o.Image, strings.Join(o.SSHKeys, ","), e.Context)
	if prompt(question) != "yes" {
		fmt.Println("cancelled, nothing created")
		return nil
	}
	s, err := svc.CreateServer(context.Background(), e.Context, o)
	if err != nil && s.ID != 0 {
		fmt.Printf("server %d %q accepted by Hetzner\n", s.ID, s.Name)
	}
	return finish(err, fmt.Sprintf("server %d %q created, ipv4 %s", s.ID, s.Name, orDash(s.PublicIPv4)))
}

func imageCost(svc *service.Service, srv hetzner.Server) (service.ImageCost, error) {
	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()
	return svc.ImageCost(ctx, srv.DiskGB)
}

func orUnknown(s string) string {
	if s == "" {
		return "(unknown)"
	}
	return s
}
