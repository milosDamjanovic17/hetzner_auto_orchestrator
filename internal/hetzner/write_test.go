package hetzner

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/hetznercloud/hcloud-go/v2/hcloud/schema"
)

func action(id int64, status string) schema.Action {
	a := schema.Action{ID: id, Status: status, Command: "test", Started: time.Now()}
	if status == "error" {
		a.Error = &schema.ActionError{Code: "action_failed", Message: "failed at Hetzner"}
	}
	return a
}

// decode reads a request body into v, failing the test on bad JSON.
func decode(t *testing.T, r *http.Request, v any) {
	t.Helper()
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		t.Errorf("decoding %s body: %v", r.URL.Path, err)
	}
}

// Every server hao creates must carry managed-by=hao: delete is limited to
// labelled servers, so an unlabelled one could never be deleted by hao. And it
// must carry the chosen SSH key: without one Hetzner sets a root password.
func TestCreateServerIsLabelledAndUsesTheSSHKey(t *testing.T) {
	var sent schema.ServerCreateRequest
	c, _ := fakeAPI(t, map[string]any{
		"/server_types": schema.ServerTypeListResponse{ServerTypes: []schema.ServerType{
			{ID: 1, Name: "cpx02", Architecture: "x86"},
		}},
		"/images": func(r *http.Request) any {
			// The image must be matched to the type's architecture: the same
			// name exists once per architecture.
			if got := r.URL.Query().Get("architecture"); got != "x86" {
				t.Errorf("image looked up for architecture %q, want x86", got)
			}
			name := "ubuntu-24.04"
			return schema.ImageListResponse{Images: []schema.Image{{ID: 5, Name: &name, Architecture: "x86"}}}
		},
		"/ssh_keys": schema.SSHKeyListResponse{SSHKeys: []schema.SSHKey{{ID: 7, Name: "moose@windesktop"}}},
		"/servers": func(r *http.Request) any {
			if r.Method != http.MethodPost {
				t.Errorf("%s /servers, want POST", r.Method)
			}
			decode(t, r, &sent)
			password := "s3cret-root"
			return schema.ServerCreateResponse{
				Server:       schema.Server{ID: 42, Name: "web-1", Status: "initializing"},
				Action:       action(1, "success"),
				RootPassword: &password,
			}
		},
	})

	s, err := c.CreateServer(testCtx(t), CreateServerOpts{
		Name: "web-1", Type: "cpx02", Location: "fsn1", Image: "ubuntu-24.04",
		SSHKeys: []string{"moose@windesktop"},
	})
	if err != nil {
		t.Fatalf("CreateServer: %v", err)
	}
	if sent.Labels == nil || (*sent.Labels)[managedKey] != managedValue {
		t.Errorf("create sent labels %v, want %s=%s", sent.Labels, managedKey, managedValue)
	}
	if !slices.Equal(sent.SSHKeys, []int64{7}) {
		t.Errorf("create sent SSH keys %v, want [7]", sent.SSHKeys)
	}
	if sent.Location != "fsn1" || sent.Image.ID != 5 {
		t.Errorf("create sent location %q image %+v, want fsn1 and image 5", sent.Location, sent.Image)
	}
	if s.ID != 42 {
		t.Errorf("returned server %d, want 42", s.ID)
	}
	// The result goes to the GUI as JSON. A root password must never ride along.
	out, _ := json.Marshal(s)
	if strings.Contains(string(out), "s3cret-root") {
		t.Errorf("root password leaked into the result: %s", out)
	}
}

// No key, no request: hao has no root-password path (phase-2b §5.2).
// fakeAPI with no routes fails the test on any request.
func TestCreateServerWithoutSSHKeySendsNothing(t *testing.T) {
	c, _ := fakeAPI(t, nil)
	_, err := c.CreateServer(testCtx(t), CreateServerOpts{Name: "web-1", Type: "cpx02", Location: "fsn1", Image: "ubuntu-24.04"})
	if err == nil {
		t.Fatal("CreateServer without SSH keys succeeded")
	}
}

// Both drivers show these messages. A wrong mapping tells the user the wrong
// thing to do, e.g. "check your connection" when the server is protected.
func TestWriteErrorsGetOurMessage(t *testing.T) {
	for _, tc := range []struct {
		code   string
		status int
		want   error
	}{
		{"protected", http.StatusLocked, ErrProtected},
		{"locked", http.StatusLocked, ErrLocked},
		{"resource_limit_exceeded", http.StatusForbidden, ErrLimit},
		{"uniqueness_error", http.StatusConflict, ErrNameTaken},
		{"token_readonly", http.StatusForbidden, ErrReadOnly},
		{"not_found", http.StatusNotFound, ErrNotFound},
	} {
		t.Run(tc.code, func(t *testing.T) {
			c, _ := fakeAPI(t, map[string]any{"/servers/1": apiError{tc.status, tc.code}})
			err := c.DeleteServer(testCtx(t), 1)
			if !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
			// Hetzner answered, so the outcome is known: nothing happened.
			if errors.Is(err, ErrOutcomeUnknown) || errors.Is(err, ErrUnreachable) {
				t.Errorf("err = %v, reported as a connection problem", err)
			}
		})
	}
}

// The API rejects delete and rebuild protection with different values
// (Step 0, phase-2b §11), so both flags always go together.
func TestProtectionSetsBothFlagsTogether(t *testing.T) {
	for _, on := range []bool{true, false} {
		var sent schema.ServerActionChangeProtectionRequest
		c, _ := fakeAPI(t, map[string]any{
			"/servers/1/actions/change_protection": func(r *http.Request) any {
				decode(t, r, &sent)
				return schema.ServerActionChangeProtectionResponse{Action: action(1, "success")}
			},
		})
		if err := c.SetProtection(testCtx(t), 1, on); err != nil {
			t.Fatalf("SetProtection(%v): %v", on, err)
		}
		if sent.Delete == nil || sent.Rebuild == nil || *sent.Delete != on || *sent.Rebuild != on {
			t.Errorf("SetProtection(%v) sent delete=%v rebuild=%v, want both %v",
				on, deref(sent.Delete), deref(sent.Rebuild), on)
		}
	}
}

func deref(b *bool) any {
	if b == nil {
		return "unset"
	}
	return *b
}

// Once Hetzner has accepted a write, how waiting ends decides what the user
// is told. "Failed" for something still running invites doing it twice.
func TestWaitReportsHowTheActionEnded(t *testing.T) {
	for _, tc := range []struct {
		name      string
		polled    string // status /actions keeps returning
		want      error
		wantError bool
	}{
		{"finishes", "success", nil, false},
		{"outlives the wait", "running", ErrStillRunning, true},
		{"fails at Hetzner", "error", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := fakeAPI(t, map[string]any{
				"/servers/1/actions/poweron": schema.ServerActionPoweronResponse{Action: action(9, "running")},
				"/actions":                   schema.ActionListResponse{Actions: []schema.Action{action(9, tc.polled)}},
			})
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			err := c.PowerOn(ctx, 1)
			if (err != nil) != tc.wantError {
				t.Fatalf("err = %v, want error: %v", err, tc.wantError)
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
			if tc.polled == "error" {
				var failed hcloud.ActionError
				if !errors.As(err, &failed) || errors.Is(err, ErrStillRunning) || errors.Is(err, ErrOutcomeUnknown) {
					t.Errorf("err = %v, want Hetzner's action error, not still running / unknown", err)
				}
			}
		})
	}
}

// The connection drops during a write: it may have reached Hetzner. Saying
// "failed" could make the user retry a create or delete that already happened.
func TestLostConnectionDuringWriteIsOutcomeUnknown(t *testing.T) {
	c, srv := fakeAPI(t, nil)
	srv.Close()
	if err := c.PowerOff(testCtx(t), 1); !errors.Is(err, ErrOutcomeUnknown) {
		t.Errorf("err = %v, want ErrOutcomeUnknown", err)
	}
}

// The service's guards read these: delete needs Managed, backup needs Backups.
// A server the API does not know must be ErrNotFound, not an empty Server.
func TestServerStateForTheGuards(t *testing.T) {
	window := "22-02"
	c, _ := fakeAPI(t, map[string]any{
		"/servers/1": schema.ServerGetResponse{Server: schema.Server{
			ID: 1, Name: "ours", Labels: map[string]string{"managed-by": "hao"},
			BackupWindow: &window, Protection: schema.ServerProtection{Delete: true, Rebuild: true},
		}},
		"/servers/2": schema.ServerGetResponse{Server: schema.Server{
			ID: 2, Name: "theirs", Labels: map[string]string{"managed-by": "terraform"},
		}},
		"/servers/3": apiError{http.StatusNotFound, "not_found"},
	})

	ours, err := c.Server(testCtx(t), 1)
	if err != nil {
		t.Fatalf("Server(1): %v", err)
	}
	if !ours.Managed || !ours.Backups || !ours.DeleteProtected || !ours.RebuildProtected {
		t.Errorf("Server(1) = %+v, want managed, backups, both protections", ours)
	}
	theirs, err := c.Server(testCtx(t), 2)
	if err != nil {
		t.Fatalf("Server(2): %v", err)
	}
	if theirs.Managed || theirs.Backups || theirs.DeleteProtected {
		t.Errorf("Server(2) = %+v, want unmanaged, no backups, unprotected", theirs)
	}
	if _, err := c.Server(testCtx(t), 3); !errors.Is(err, ErrNotFound) {
		t.Errorf("Server(3) err = %v, want ErrNotFound", err)
	}
}

// The cost check is money: amounts must come through exactly as Hetzner sent
// them (no float rounding), gross not net, and IPv4 not IPv6.
func TestPricesKeepHetznersExactGrossAmounts(t *testing.T) {
	c, _ := fakeAPI(t, map[string]any{
		"/pricing": schema.PricingGetResponse{Pricing: schema.Pricing{
			VATRate: "19.00",
			Image:   schema.PricingImage{PricePerGBMonth: schema.Price{Net: "0.0120", Gross: "0.0143"}},
			ServerTypes: []schema.PricingServerType{{ID: 1, Name: "cpx02", Prices: []schema.PricingServerTypePrice{{
				Location:     "fsn1",
				PriceHourly:  schema.Price{Net: "0.0081", Gross: "0.0096000000000000"},
				PriceMonthly: schema.Price{Net: "5.0336", Gross: "5.9900000000000000"},
			}}}},
			// hcloud-go v2.47.0 sizes the primary IP prices by the number
			// of floating IP prices (schema_gen.go primaryIPPricingFromSchema)
			// and panics if there are fewer. The real API sends both types
			// for both, so mirror that.
			FloatingIPs: []schema.PricingFloatingIPType{{Type: "ipv4"}, {Type: "ipv6"}},
			PrimaryIPs: []schema.PricingPrimaryIP{
				{Type: "ipv4", Prices: []schema.PricingPrimaryIPTypePrice{{Location: "fsn1", PriceMonthly: schema.Price{Net: "0.42", Gross: "0.5000"}}}},
				{Type: "ipv6", Prices: []schema.PricingPrimaryIPTypePrice{{Location: "fsn1", PriceMonthly: schema.Price{Net: "0", Gross: "0.0000"}}}},
			},
		}},
	})
	p, err := c.Prices(testCtx(t))
	if err != nil {
		t.Fatalf("Prices: %v", err)
	}
	wantServer := ServerPrice{Type: "cpx02", Location: "fsn1", Hourly: "0.0096000000000000", Monthly: "5.9900000000000000"}
	if len(p.Servers) != 1 || p.Servers[0] != wantServer {
		t.Errorf("servers = %+v, want [%+v]", p.Servers, wantServer)
	}
	if want := []IPPrice{{Location: "fsn1", Monthly: "0.5000"}}; !slices.Equal(p.PrimaryIPv4, want) {
		t.Errorf("primary IPv4 = %+v, want %+v", p.PrimaryIPv4, want)
	}
	if p.ImagePerGBMonth != "0.0143" || p.VATRate != "19.00" {
		t.Errorf("image %q VAT %q, want 0.0143 and 19.00", p.ImagePerGBMonth, p.VATRate)
	}
}
