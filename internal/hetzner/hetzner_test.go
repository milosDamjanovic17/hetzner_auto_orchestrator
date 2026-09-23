package hetzner

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/hetznercloud/hcloud-go/v2/hcloud/schema"
)

// Offline tests against a fake Hetzner API. Response bodies are built from the
// SDK's own schema types, so their shape is exactly what the SDK parses.

// apiError is a route answered with a Hetzner error body instead of data.
type apiError struct {
	status int
	code   string
}

// fakeAPI serves routes (URL path -> response body or apiError) and returns a
// Client pointed at it.
func fakeAPI(t *testing.T, routes map[string]any) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := routes[r.URL.Path]
		if !ok {
			t.Errorf("unexpected request %s", r.URL.Path)
			body = apiError{http.StatusNotFound, "not_found"}
		}
		w.Header().Set("Content-Type", "application/json")
		if e, ok := body.(apiError); ok {
			w.WriteHeader(e.status)
			body = schema.ErrorResponse{Error: schema.Error{Code: e.code, Message: e.code}}
		}
		if err := json.NewEncoder(w).Encode(body); err != nil {
			t.Errorf("encoding response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	return &Client{api: hcloud.NewClient(hcloud.WithToken("test"), hcloud.WithEndpoint(srv.URL))}, srv
}

func testCtx(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// Every caller picks its message from these categories: "fix your token" vs
// "check your connection" vs the API's own reason. A wrong category sends the
// user chasing the wrong problem, and Phase 2b writes fail through here too.
func TestClassifySortsErrorsIntoTheRightCategory(t *testing.T) {
	for _, tc := range []struct {
		name            string
		code            string
		status          int
		wantUnauth      bool
		wantUnreachable bool
	}{
		{"rejected token", "unauthorized", http.StatusUnauthorized, true, false},
		// A read-only token reached Hetzner fine: not a connection problem,
		// and not "token rejected" either, since reads with it still work.
		{"read-only token", "token_readonly", http.StatusForbidden, false, false},
		{"forbidden", "forbidden", http.StatusForbidden, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := fakeAPI(t, map[string]any{"/servers": apiError{tc.status, tc.code}})
			err := c.ValidateToken(testCtx(t))
			if err == nil {
				t.Fatal("ValidateToken succeeded, want an error")
			}
			if got := errors.Is(err, ErrUnauthorized); got != tc.wantUnauth {
				t.Errorf("errors.Is(ErrUnauthorized) = %v, want %v (err: %v)", got, tc.wantUnauth, err)
			}
			if got := errors.Is(err, ErrUnreachable); got != tc.wantUnreachable {
				t.Errorf("errors.Is(ErrUnreachable) = %v, want %v (err: %v)", got, tc.wantUnreachable, err)
			}
			// The SDK error stays wrapped, so a caller can still ask for the
			// exact code (a Phase 2b write will want token_readonly).
			if tc.code != "unauthorized" && !hcloud.IsError(err, hcloud.ErrorCode(tc.code)) {
				t.Errorf("hcloud code %q lost in %v", tc.code, err)
			}
		})
	}
}

func TestClassifyNoConnectionIsUnreachable(t *testing.T) {
	c, srv := fakeAPI(t, nil)
	srv.Close() // connection refused from here on
	err := c.ValidateToken(testCtx(t))
	if !errors.Is(err, ErrUnreachable) || errors.Is(err, ErrUnauthorized) {
		t.Errorf("err = %v, want ErrUnreachable only", err)
	}
}

// The API names the server a volume or floating IP is attached to by ID only.
// An attached one must never be shown as unattached ("-"): in Phase 2b that
// display drives detach and delete decisions.
func TestAttachedResourcesNameTheirServer(t *testing.T) {
	web, gone := int64(42), int64(99) // 99: deleted between the two requests
	c, _ := fakeAPI(t, map[string]any{
		"/servers": schema.ServerListResponse{Servers: []schema.Server{{ID: 42, Name: "web-1"}}},
		"/volumes": schema.VolumeListResponse{Volumes: []schema.Volume{
			{ID: 1, Name: "data", Server: &web},
			{ID: 2, Name: "spare"},
			{ID: 3, Name: "orphan", Server: &gone},
		}},
		"/floating_ips": schema.FloatingIPListResponse{FloatingIPs: []schema.FloatingIP{
			{ID: 1, Name: "ip-a", Type: "ipv4", IP: "203.0.113.10", Server: &web},
			{ID: 2, Name: "ip-b", Type: "ipv4", IP: "203.0.113.11"},
		}},
	})

	vols, err := c.Volumes(testCtx(t))
	if err != nil {
		t.Fatalf("Volumes: %v", err)
	}
	for i, want := range []string{"web-1", "", "server 99"} {
		if vols[i].AttachedTo != want {
			t.Errorf("volume %s attached to %q, want %q", vols[i].Name, vols[i].AttachedTo, want)
		}
	}

	ips, err := c.FloatingIPs(testCtx(t))
	if err != nil {
		t.Fatalf("FloatingIPs: %v", err)
	}
	for i, want := range []string{"web-1", ""} {
		if ips[i].AssignedTo != want {
			t.Errorf("floating IP %s assigned to %q, want %q", ips[i].Name, ips[i].AssignedTo, want)
		}
	}
}

// Nothing attached: no extra request. fakeAPI fails the test on any route it
// does not know, and /servers is deliberately missing here.
func TestUnattachedVolumesCostNoServerLookup(t *testing.T) {
	c, _ := fakeAPI(t, map[string]any{
		"/volumes": schema.VolumeListResponse{Volumes: []schema.Volume{{ID: 2, Name: "spare"}}},
	})
	if _, err := c.Volumes(testCtx(t)); err != nil {
		t.Fatalf("Volumes: %v", err)
	}
}
