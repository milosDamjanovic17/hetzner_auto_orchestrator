// Command hao is the console driver for the orchestrator.
//
// It is deliberately thin: it formats and prints, and decides nothing. Every
// choice lives in internal/, so the Lorca GUI can make the same calls later
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
	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/hetzner"
	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/secrets"
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
		fmt.Fprintln(os.Stderr, "error:", err)
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

// loadContexts loads the context store, translating first-run into an
// actionable message instead of a bare error.
func loadContexts(st secrets.Store) (*config.Store, error) {
	s, err := config.Load(st)
	if errors.Is(err, secrets.ErrNotInitialized) {
		return nil, errors.New("no store yet - run `hao init` first")
	}
	return s, err
}

// activeToken returns the active context's token. It is only ever handed to a
// client constructor, never printed.
func activeToken() (string, error) {
	st, err := openStore()
	if err != nil {
		return "", err
	}
	store, err := loadContexts(st)
	if err != nil {
		return "", err
	}
	active, err := store.ActiveContext()
	if err != nil {
		return "", fmt.Errorf("%w - run `hao context use <name>`", err)
	}
	return active.Token, nil
}

// activeClient returns a Hetzner client for the active context.
//
// Every resource listing goes through here: the local store answers "which
// token", and Hetzner answers "what exists". The two are separate questions and
// conflating them is what made `zone list` print project names.
func activeClient() (*hetzner.Client, error) {
	token, err := activeToken()
	if err != nil {
		return nil, err
	}
	return hetzner.NewClient(token), nil
}

func cmdInit() error {
	st, err := openStore()
	if err != nil {
		return err
	}

	// Never clobber an existing store: the tokens in it may be the only copy.
	if _, err := st.Load(); err == nil {
		return fmt.Errorf("store already exists at %s", st.Path())
	} else if !errors.Is(err, secrets.ErrNotInitialized) {
		return err
	}

	if err := config.Save(st, &config.Store{}); err != nil {
		return err
	}
	fmt.Printf("initialized encrypted store at %s\n", st.Path())
	fmt.Println("age identity stored in the OS keychain")
	fmt.Println("next: `hao import` to adopt your hcloud contexts")
	return nil
}

func cmdImport() error {
	st, err := openStore()
	if err != nil {
		return err
	}
	store, err := loadContexts(st)
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

	st, err := openStore()
	if err != nil {
		return err
	}
	store, err := loadContexts(st)
	if err != nil {
		return err
	}

	switch args[0] {
	case "list":
		contexts := store.List()
		if len(contexts) == 0 {
			fmt.Println("no contexts - run `hao import` or `hao context add <name>`")
			return nil
		}
		for _, c := range contexts {
			marker := " "
			if c.Name == store.Active {
				marker = "*"
			}
			fmt.Printf("%s %s\n", marker, c.Name)
		}
		return nil

	case "use":
		if len(args) < 2 {
			return errors.New("context use: expected a context name")
		}
		if err := store.SetActive(args[1]); err != nil {
			return err
		}
		if err := config.Save(st, store); err != nil {
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
		// Store.Add rejects it again; this is only the earlier, cheaper exit.
		if _, err := store.Get(name); err == nil {
			return fmt.Errorf("%w: %q", config.ErrDuplicate, name)
		}

		token, err := readToken()
		if err != nil {
			return err
		}

		// Never store a token Hetzner has not accepted: a bad one would
		// otherwise surface later as a confusing failure on some listing.
		ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
		defer cancel()
		if err := hetzner.NewClient(token).ValidateToken(ctx); err != nil {
			switch {
			case errors.Is(err, hetzner.ErrUnauthorized):
				return errors.New("token rejected by Hetzner - check it was copied whole and has not been revoked; nothing saved")
			case errors.Is(err, hetzner.ErrUnreachable):
				return fmt.Errorf("could not reach Hetzner to validate the token; nothing saved: %w", err)
			default:
				return fmt.Errorf("%w; nothing saved", err)
			}
		}

		if err := store.Add(config.Context{Name: name, Token: token}); err != nil {
			return err
		}
		if err := config.Save(st, store); err != nil {
			return err
		}
		fmt.Printf("added context %s (token validated)\n", name)
		if store.Active == name {
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
		if _, err := store.Get(name); err != nil {
			return err
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

		// Remove clears the active marker if this was the active context,
		// so remember that first to tell the user afterwards.
		wasActive := store.Active == name
		if err := store.Remove(name); err != nil {
			return err
		}
		if err := config.Save(st, store); err != nil {
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
