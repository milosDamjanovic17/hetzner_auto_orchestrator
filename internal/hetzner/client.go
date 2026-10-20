// Package hetzner wraps the Hetzner Cloud SDK.
//
// Every type returned from here is defined in this package, not borrowed from
// the SDK. Two reasons: the Wails GUI will JSON-marshal these across the Go to
// JS boundary, so they must stay small and serializable; and no result type may
// ever carry a token field, so the credential cannot leak into the web layer.
package hetzner

import (
	"context"
	"errors"
	"fmt"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

// ConsoleURL is where API tokens and project members are managed: the API has
// no endpoints for either. Checked 2026-09-23: the old console.hetzner.cloud
// redirects here.
const ConsoleURL = "https://console.hetzner.com/"

// ErrUnauthorized means the token was rejected. Kept distinct from transport
// failures because the two need completely different messages: one is "fix your
// token", the other is "check your connection".
var ErrUnauthorized = errors.New("hetzner: token rejected")

// ErrUnreachable means the API could not be contacted at all.
var ErrUnreachable = errors.New("hetzner: API unreachable")

// ErrNotFound means the resource does not exist (any more).
var ErrNotFound = errors.New("not found")

// Errors the API reports with a clean code, each with our own message so the
// CLI and the GUI say the same thing (phase-2b §4). The API's message text is
// never parsed.
var (
	ErrProtected = errors.New("delete protection is on; turn it off first")
	ErrLocked    = errors.New("another action is running on it; try again shortly")
	ErrLimit     = errors.New("project limit reached (preflight cannot check quotas)")
	ErrNameTaken = errors.New("that name is already taken in this project")
	ErrReadOnly  = errors.New("token is read-only; this needs a Read & Write token")
)

var codeErrors = map[hcloud.ErrorCode]error{
	hcloud.ErrorCodeNotFound:              ErrNotFound,
	hcloud.ErrorCodeProtected:             ErrProtected,
	hcloud.ErrorCodeLocked:                ErrLocked,
	hcloud.ErrorCodeResourceLimitExceeded: ErrLimit,
	hcloud.ErrorCodeUniquenessError:       ErrNameTaken,
	hcloud.ErrorCodeTokenReadonly:         ErrReadOnly,
}

// ErrOutcomeUnknown means the connection failed while a write may already
// have reached Hetzner. Never report it as "failed": it may have happened.
var ErrOutcomeUnknown = errors.New("outcome unknown; refresh to see whether it happened")

// ErrStillRunning means Hetzner accepted a write but it did not finish in
// time. Not a failure: it carries on at Hetzner.
var ErrStillRunning = errors.New("still running at Hetzner; refresh later")

// Client talks to one Hetzner Cloud project.
//
// It does not read configuration. The caller resolves the active context and
// passes the token in, which keeps this package free of any dependency on how
// credentials are stored.
type Client struct {
	api *hcloud.Client
}

// NewClient returns a client for the given project token.
func NewClient(token string) *Client {
	return &Client{api: hcloud.NewClient(hcloud.WithToken(token))}
}

// classify turns an SDK error into one of our sentinels so callers never have
// to reach into the SDK's error types.
func classify(err error, op string) error {
	if err == nil {
		return nil
	}
	if hcloud.IsError(err, hcloud.ErrorCodeUnauthorized) {
		return fmt.Errorf("%w (%s)", ErrUnauthorized, op)
	}
	var apiErr hcloud.Error
	if errors.As(err, &apiErr) {
		if sentinel, ok := codeErrors[apiErr.Code]; ok {
			return fmt.Errorf("%s: %w", op, sentinel)
		}
		// A structured API error means we reached Hetzner; it is not a
		// connectivity problem and must not be reported as one.
		return fmt.Errorf("hetzner: %s: %w", op, err)
	}
	return fmt.Errorf("%w: %s: %v", ErrUnreachable, op, err)
}

// classifyWrite is classify for a request that changes something: losing the
// connection there means the change may or may not have happened.
func classifyWrite(err error, op string) error {
	err = classify(err, op)
	if errors.Is(err, ErrUnreachable) {
		return fmt.Errorf("%w: %w", ErrOutcomeUnknown, err)
	}
	return err
}

// ValidateToken checks the token against the live API.
//
// Uses a single-item page rather than a full listing so that validating a token
// on a large project stays cheap.
func (c *Client) ValidateToken(ctx context.Context) error {
	_, _, err := c.api.Server.List(ctx, hcloud.ServerListOpts{
		ListOpts: hcloud.ListOpts{PerPage: 1},
	})
	return classify(err, "validating token")
}
