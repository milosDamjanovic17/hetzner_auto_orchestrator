package hetzner

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

// Server writes (phase-2b). No policy here: the project check, typed names,
// the hao label on delete and the backups-on check all live in
// internal/service, which calls these only after its guards pass.

// The label hao puts on every server and image it creates. Delete is limited
// to servers carrying it (phase-2b §3). Misclick protection, not security:
// the label can be edited in the Console.
const (
	managedKey   = "managed-by"
	managedValue = "hao"
)

// How long a write waits for Hetzner to finish it (phase-2b §4). Step 0
// measured create at 12-22 s and a 0.6 GB snapshot at 80 s; images take
// longer the bigger the disk.
const (
	createTimeout = 5 * time.Minute
	imageTimeout  = 30 * time.Minute
	actionTimeout = 2 * time.Minute
)

// A fresh map per call: the SDK keeps a reference to the one it is given.
func managedLabels() map[string]string {
	return map[string]string{managedKey: managedValue}
}

// Server fetches one server by ID, for the service's guards.
func (c *Client) Server(ctx context.Context, id int64) (Server, error) {
	op := fmt.Sprintf("getting server %d", id)
	s, _, err := c.api.Server.GetByID(ctx, id)
	if err != nil {
		return Server{}, classify(err, op)
	}
	if s == nil {
		return Server{}, fmt.Errorf("%s: %w", op, ErrNotFound)
	}
	return serverFromSDK(s), nil
}

// CreateServerOpts names everything as the user typed or picked it.
type CreateServerOpts struct {
	Name     string
	Type     string
	Location string
	Image    string   // matched to the server type's architecture
	SSHKeys  []string // existing keys, by name or ID; at least one
}

// CreateServer creates a server labelled managed-by=hao and waits for it.
//
// At least one existing SSH key is required: without one Hetzner sets a root
// password, a path hao does not offer (phase-2b §5.2). A root password the API
// returns anyway is dropped here, never stored or passed on.
//
// The returned Server is the state Hetzner reported on accepting the create,
// also when waiting ends in ErrStillRunning.
func (c *Client) CreateServer(ctx context.Context, o CreateServerOpts) (Server, error) {
	op := fmt.Sprintf("creating server %q", o.Name)
	if len(o.SSHKeys) == 0 {
		return Server{}, fmt.Errorf("%s: at least one SSH key is required (add keys in the Console)", op)
	}
	typ, _, err := c.api.ServerType.Get(ctx, o.Type)
	if err != nil {
		return Server{}, classify(err, op)
	}
	if typ == nil {
		return Server{}, fmt.Errorf("%s: no server type %q", op, o.Type)
	}
	img, _, err := c.api.Image.GetForArchitecture(ctx, o.Image, typ.Architecture)
	if err != nil {
		return Server{}, classify(err, op)
	}
	if img == nil {
		return Server{}, fmt.Errorf("%s: no image %q for %s", op, o.Image, typ.Architecture)
	}
	// The API takes SSH keys by ID only.
	keys := make([]*hcloud.SSHKey, 0, len(o.SSHKeys))
	for _, k := range o.SSHKeys {
		key, _, err := c.api.SSHKey.Get(ctx, k)
		if err != nil {
			return Server{}, classify(err, op)
		}
		if key == nil {
			return Server{}, fmt.Errorf("%s: no SSH key %q (add it in the Console)", op, k)
		}
		keys = append(keys, key)
	}

	res, _, err := c.api.Server.Create(ctx, hcloud.ServerCreateOpts{
		Name:       o.Name,
		ServerType: typ,
		Image:      img,
		Location:   &hcloud.Location{Name: o.Location},
		SSHKeys:    keys,
		Labels:     managedLabels(),
	})
	if err != nil {
		return Server{}, classifyWrite(err, op)
	}
	s := serverFromSDK(res.Server)
	return s, c.wait(ctx, createTimeout, op, append([]*hcloud.Action{res.Action}, res.NextActions...)...)
}

// DeleteServer deletes a server and waits for it to be gone.
func (c *Client) DeleteServer(ctx context.Context, id int64) error {
	op := fmt.Sprintf("deleting server %d", id)
	res, _, err := c.api.Server.DeleteWithResult(ctx, &hcloud.Server{ID: id})
	if err != nil {
		return classifyWrite(err, op)
	}
	return c.wait(ctx, actionTimeout, op, res.Action)
}

func (c *Client) PowerOn(ctx context.Context, id int64) error {
	return c.serverAction(ctx, id, "powering on", c.api.Server.Poweron)
}

// Shutdown asks the OS to shut down (ACPI); PowerOff cuts power.
func (c *Client) Shutdown(ctx context.Context, id int64) error {
	return c.serverAction(ctx, id, "shutting down", c.api.Server.Shutdown)
}

func (c *Client) PowerOff(ctx context.Context, id int64) error {
	return c.serverAction(ctx, id, "powering off", c.api.Server.Poweroff)
}

func (c *Client) Reboot(ctx context.Context, id int64) error {
	return c.serverAction(ctx, id, "rebooting", c.api.Server.Reboot)
}

// SetProtection turns delete and rebuild protection on or off together: the
// API rejects the two flags with different values (phase-2b §11).
func (c *Client) SetProtection(ctx context.Context, id int64, on bool) error {
	verb := "protecting"
	if !on {
		verb = "unprotecting"
	}
	return c.serverAction(ctx, id, verb, func(ctx context.Context, s *hcloud.Server) (*hcloud.Action, *hcloud.Response, error) {
		return c.api.Server.ChangeProtection(ctx, s, hcloud.ServerChangeProtectionOpts{Delete: &on, Rebuild: &on})
	})
}

func (c *Client) serverAction(ctx context.Context, id int64, verb string,
	call func(context.Context, *hcloud.Server) (*hcloud.Action, *hcloud.Response, error),
) error {
	op := fmt.Sprintf("%s server %d", verb, id)
	action, _, err := call(ctx, &hcloud.Server{ID: id})
	if err != nil {
		return classifyWrite(err, op)
	}
	return c.wait(ctx, actionTimeout, op, action)
}

// Backup creates a manual backup now and returns the image ID. Hetzner refuses
// it with a generic invalid_input when backups are off for the server, so the
// service checks Server.Backups first (phase-2b §4).
func (c *Client) Backup(ctx context.Context, id int64) (int64, error) {
	return c.createImage(ctx, id, hcloud.ImageTypeBackup, "")
}

// Snapshot creates a snapshot and returns the image ID.
func (c *Client) Snapshot(ctx context.Context, id int64, description string) (int64, error) {
	return c.createImage(ctx, id, hcloud.ImageTypeSnapshot, description)
}

func (c *Client) createImage(ctx context.Context, id int64, typ hcloud.ImageType, description string) (int64, error) {
	op := fmt.Sprintf("creating %s of server %d", typ, id)
	opts := &hcloud.ServerCreateImageOpts{Type: typ, Labels: managedLabels()}
	if description != "" {
		opts.Description = &description
	}
	res, _, err := c.api.Server.CreateImage(ctx, &hcloud.Server{ID: id}, opts)
	if err != nil {
		return 0, classifyWrite(err, op)
	}
	var imageID int64
	if res.Image != nil {
		imageID = res.Image.ID
	}
	return imageID, c.wait(ctx, imageTimeout, op, res.Action)
}

// wait waits for actions Hetzner has accepted. The write is already under way,
// so running out of time (ours or the caller's) is not a failure.
func (c *Client) wait(ctx context.Context, timeout time.Duration, op string, actions ...*hcloud.Action) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	err := c.api.Action.WaitFor(ctx, actions...)
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return fmt.Errorf("%s: %w", op, ErrStillRunning)
	}
	var failed hcloud.ActionError
	if errors.As(err, &failed) {
		return fmt.Errorf("hetzner: %s failed: %w", op, err)
	}
	// Polling lost the connection after Hetzner accepted the write.
	return classifyWrite(err, op)
}
