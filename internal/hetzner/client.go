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
		// A structured API error means we reached Hetzner; it is not a
		// connectivity problem and must not be reported as one.
		return fmt.Errorf("hetzner: %s: %w", op, err)
	}
	return fmt.Errorf("%w: %s: %v", ErrUnreachable, op, err)
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
