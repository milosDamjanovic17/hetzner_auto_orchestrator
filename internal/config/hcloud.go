package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// ErrNoHcloudConfig means the hcloud CLI has no config file here -- normal if
// the CLI was never installed or never used.
var ErrNoHcloudConfig = errors.New("config: no hcloud cli.toml found")

// EnvTokenVar is the environment variable the hcloud CLI honours. This app
// deliberately does not.
const EnvTokenVar = "HCLOUD_TOKEN"

// hcloudConfig mirrors the parts of the hcloud CLI's cli.toml that we read.
//
// VERIFY against your own cli.toml before trusting this (build-order Step 0).
// If the shape has changed, ImportHcloudContexts returns zero contexts rather
// than failing, which is why the caller must report the count it got back.
type hcloudConfig struct {
	ActiveContext string `toml:"active_context"`
	Contexts      []struct {
		Name  string `toml:"name"`
		Token string `toml:"token"`
	} `toml:"contexts"`
}

// HcloudConfigPath returns the hcloud CLI's config location for this OS.
//
// Resolved through os.UserConfigDir rather than hardcoded: it is
// %AppData%\hcloud\cli.toml on Windows and ~/.config/hcloud/cli.toml on
// Unix, and this app targets both.
func HcloudConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config: locating user config dir: %w", err)
	}
	return filepath.Join(dir, "hcloud", "cli.toml"), nil
}

// ImportHcloudContexts reads contexts out of an hcloud cli.toml and returns
// what it found, along with the name hcloud considers active.
//
// It never writes -- not to cli.toml, not to our own store. The caller decides
// what to adopt, so the CLI and the future GUI can present that choice without
// duplicating the logic.
func ImportHcloudContexts(path string) ([]Context, string, error) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, "", fmt.Errorf("%w at %s", ErrNoHcloudConfig, path)
		}
		return nil, "", fmt.Errorf("config: reading %s: %w", path, err)
	}

	var raw hcloudConfig
	if _, err := toml.DecodeFile(path, &raw); err != nil {
		return nil, "", fmt.Errorf("config: parsing %s: %w", path, err)
	}

	contexts := make([]Context, 0, len(raw.Contexts))
	for _, c := range raw.Contexts {
		if c.Name == "" || c.Token == "" {
			// Skip rather than fail: one malformed entry should not block
			// importing the rest. The caller reports the count it received.
			continue
		}
		contexts = append(contexts, Context{Name: c.Name, Token: c.Token})
	}
	return contexts, raw.ActiveContext, nil
}

// EnvTokenPresent reports whether HCLOUD_TOKEN is set in the environment.
//
// The token itself is never returned. Honouring an ambient env var would make
// "which token am I using" unanswerable and would defeat the encrypted store,
// so callers use this only to warn.
func EnvTokenPresent() bool {
	return os.Getenv(EnvTokenVar) != ""
}
