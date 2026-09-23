// Command hao is the console driver for the orchestrator.
//
// It is deliberately thin: it formats and prints, and decides nothing. Every
// choice lives in internal/, so the Wails GUI can make the same calls later
// without reimplementing any of it. Resist adding flag libraries, table
// formatting or prompt frameworks here.
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/config"
	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/secrets"
	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/service"
	"golang.org/x/term"
)

// apiTimeout bounds every Hetzner call so the CLI cannot hang indefinitely on
// a stalled connection.
const apiTimeout = 30 * time.Second

const usage = `hao - Hetzner auto orchestrator

Usage:
  hao init                 create the encrypted store and its age identity
  hao import               adopt contexts from the hcloud CLI's cli.toml
  hao context list         list contexts, marking the active one
  hao context use <name>   set the active context
  hao context add <name>   add a context; prompts for its token and validates it
  hao context delete <name> remove a context from the store (asks first)

Listing (active context, read-only):
  hao server list
  hao load-balancer list
  hao network list
  hao firewall list
  hao floating-ip list
  hao volume list
  hao ssh-key list
  hao certificate list
  hao zone list            DNS zones

  hao preflight                     server type availability, every location
  hao preflight <type> <location>   is <type> available in <location>?
  hao preflight <loc>, <loc>, ...   server types available in these locations

  hao token | hao member   not in the API - points to the Hetzner Console
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", withHint(err))
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		fmt.Print(usage)
		return nil
	}

	// HCLOUD_TOKEN overrides everything in the hcloud CLI. We never honour it:
	// an ambient token makes "which project am I about to change" unanswerable
	// and bypasses the encrypted store entirely. Warn and carry on.
	if config.EnvTokenPresent() {
		fmt.Fprintf(os.Stderr,
			"warning: %s is set in the environment and is being ignored; hao uses its own encrypted store\n",
			config.EnvTokenVar)
	}

	switch args[0] {
	case "init":
		return cmdInit()
	case "import":
		return cmdImport()
	case "context":
		return cmdContext(args[1:])
	case "preflight":
		return cmdPreflight(args[1:])
	case "token", "member":
		return cmdConsoleOnly(args[0])
	case "help", "-h", "--help":
		fmt.Print(usage)
		return nil
	default:
		if r, ok := resources[args[0]]; ok {
			return cmdList(args[0], r, args[1:])
		}
		return fmt.Errorf("unknown command %q\n\n%s", args[0], usage)
	}
}

// openStore returns the encrypted store, creating the age identity if this is
// the first run.
func openStore() (*secrets.AgeStore, error) {
	path, err := secrets.DefaultStorePath()
	if err != nil {
		return nil, err
	}
	return secrets.NewAgeStore(path)
}

// withHint turns an error the user can fix into one that names the fixing
// command. internal/service writes messages for every driver, so it cannot
// mention `hao` commands itself; this is the CLI's half of that.
func withHint(err error) error {
	switch {
	case errors.Is(err, secrets.ErrNotInitialized):
		return errors.New("no store yet - run `hao init` first")
	case errors.Is(err, config.ErrNoActive):
		return fmt.Errorf("%w - run `hao context use <name>`", err)
	}
	return err
}

// hasContext reports whether name is in the store. The CLI asks before
// prompting so a clash or a typo does not cost a paste or a confirmation.
func hasContext(svc *service.Service, name string) (bool, error) {
	contexts, err := svc.Contexts()
	if err != nil {
		return false, err
	}
	for _, c := range contexts {
		if c.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func cmdInit() error {
	svc, err := service.Open()
	if err != nil {
		return err
	}
	if err := svc.Init(); err != nil {
		return err
	}
	fmt.Printf("initialized encrypted store at %s\n", svc.Path())
	fmt.Println("age identity stored in the OS keychain")
	fmt.Println("next: `hao import` to adopt your hcloud contexts")
	return nil
}

func cmdImport() error {
	st, err := openStore()
	if err != nil {
		return err
	}
	store, err := config.Load(st)
	if err != nil {
		return err
	}

	path, err := config.HcloudConfigPath()
	if err != nil {
		return err
	}
	found, hcloudActive, err := config.ImportHcloudContexts(path)
	if err != nil {
		return err
	}

	fmt.Printf("read %s\n", path)
	if len(found) == 0 {
		// Zero contexts from a file that exists usually means the cli.toml
		// shape differs from what we parse. Say so rather than reporting
		// "nothing to do".
		fmt.Println("no contexts found - if you do have hcloud contexts, the cli.toml format may differ from what hao parses")
		return nil
	}

	fmt.Printf("found %d context(s) (hcloud active: %q):\n", len(found), hcloudActive)
	for i, c := range found {
		_, exists := store.Get(c.Name)
		status := ""
		if exists == nil {
			status = "  [already imported]"
		}
		fmt.Printf("  %d) %s%s\n", i+1, c.Name, status)
	}

	fmt.Print("adopt which? (numbers separated by commas, `all`, or blank to cancel): ")
	selection, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading selection: %w", err)
	}

	chosen, err := parseSelection(strings.TrimSpace(selection), len(found))
	if err != nil {
		return err
	}
	if len(chosen) == 0 {
		fmt.Println("cancelled, nothing written")
		return nil
	}

	added := 0
	for _, i := range chosen {
		if err := store.Add(found[i]); err != nil {
			// A name already present is not fatal: report and keep going so
			// one collision does not abandon the rest of the import.
			fmt.Fprintf(os.Stderr, "  skipped %s: %v\n", found[i].Name, err)
			continue
		}
		added++
	}
	if added == 0 {
		fmt.Println("nothing new to import")
		return nil
	}

	if err := config.Save(st, store); err != nil {
		return err
	}
	fmt.Printf("imported %d context(s); cli.toml was not modified\n", added)
	return nil
}

// parseSelection turns "1,3" or "all" into zero-based indices.
func parseSelection(input string, max int) ([]int, error) {
	if input == "" {
		return nil, nil
	}
	if strings.EqualFold(input, "all") {
		all := make([]int, max)
		for i := range all {
			all[i] = i
		}
		return all, nil
	}

	var out []int
	seen := make(map[int]bool)
	for _, field := range strings.Split(input, ",") {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		n, err := strconv.Atoi(field)
		if err != nil {
			return nil, fmt.Errorf("not a number: %q", field)
		}
		if n < 1 || n > max {
			return nil, fmt.Errorf("selection %d out of range 1-%d", n, max)
		}
		if !seen[n-1] {
			seen[n-1] = true
			out = append(out, n-1)
		}
	}
	return out, nil
}

// readToken reads a token from stdin.
//
// It is never taken as an argument: arguments land in shell history, which is
// plaintext on disk. On an interactive console the input is not echoed, so the
// token stays out of scrollback and off a shared screen. Piped input is read as
// a plain line.
func readToken() (string, error) {
	fmt.Print("token: ")

	var raw string
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		b, err := term.ReadPassword(fd)
		fmt.Println() // ReadPassword swallows the newline the user typed
		if err != nil {
			return "", fmt.Errorf("reading token: %w", err)
		}
		raw = string(b)
	} else {
		sc := bufio.NewScanner(os.Stdin)
		if sc.Scan() {
			raw = sc.Text()
		} else if err := sc.Err(); err != nil {
			return "", fmt.Errorf("reading token: %w", err)
		}
	}

	token := strings.TrimSpace(raw)
	if token == "" {
		return "", errors.New("no token given; nothing saved")
	}
	return token, nil
}

func cmdContext(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("context: expected `list`, `use <name>`, `add <name>` or `delete <name>`")
	}

	svc, err := service.Open()
	if err != nil {
		return err
	}

	switch args[0] {
	case "list":
		contexts, err := svc.Contexts()
		if err != nil {
			return err
		}
		if len(contexts) == 0 {
			fmt.Println("no contexts - run `hao import` or `hao context add <name>`")
			return nil
		}
		for _, c := range contexts {
			marker := " "
			if c.Active {
				marker = "*"
			}
			fmt.Printf("%s %s\n", marker, c.Name)
		}
		return nil

	case "use":
		if len(args) < 2 {
			return errors.New("context use: expected a context name")
		}
		if err := svc.Use(args[1]); err != nil {
			return err
		}
		fmt.Printf("active context is now %s\n", args[1])
		return nil

	case "add":
		if len(args) < 2 {
			return errors.New("context add: expected a context name")
		}
		name := args[1]
		// Checked before prompting so a clashing name does not cost a paste.
		// svc.Add rejects it again; this is only the earlier, cheaper exit.
		if exists, err := hasContext(svc, name); err != nil {
			return err
		} else if exists {
			return fmt.Errorf("%w: %q", config.ErrDuplicate, name)
		}

		token, err := readToken()
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
		defer cancel()
		active, err := svc.Add(ctx, name, token)
		if err != nil {
			return err
		}
		fmt.Printf("added context %s (token validated)\n", name)
		if active {
			fmt.Println("it is the first context, so it is now active")
		}
		return nil

	case "delete":
		if len(args) < 2 {
			return errors.New("context delete: expected a context name")
		}
		name := args[1]

		// Fail on an unknown name before asking anything; asking
		// "delete foo?" about a foo that does not exist is misleading.
		if exists, err := hasContext(svc, name); err != nil {
			return err
		} else if !exists {
			return fmt.Errorf("%w: %q", config.ErrNotFound, name)
		}

		// The store may hold the only copy of this token, so deleting is
		// not something to do on a typo. Default is "no".
		fmt.Printf("delete context %s? its token is removed from hao's store (y/N): ", name)
		answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			return fmt.Errorf("reading answer: %w", err)
		}
		if !strings.EqualFold(strings.TrimSpace(answer), "y") {
			fmt.Println("cancelled, nothing deleted")
			return nil
		}

		wasActive, err := svc.Delete(name)
		if err != nil {
			return err
		}

		fmt.Printf("deleted context %s\n", name)
		fmt.Println("the token itself still works at Hetzner - revoke it in the Console if you no longer need it")
		if wasActive {
			fmt.Println("it was the active context; pick another with `hao context use <name>`")
		}
		return nil

	default:
		return fmt.Errorf("context: unknown subcommand %q", args[0])
	}
}
