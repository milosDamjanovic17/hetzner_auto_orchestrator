// Package preflight answers "can I deploy server type X in location Y" before
// a deployment is attempted, rather than after it fails.
//
// It answers AVAILABILITY ONLY. Hetzner exposes no endpoint for a project's
// remaining quota, so a type reported available here can still fail at create
// time with a limit error. Nothing in this package should be named or presented
// in a way that implies otherwise.
package preflight

import (
	"context"
	"fmt"
	"sort"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

// Availability is one server type in one location.
type Availability struct {
	ServerType   string  `json:"server_type"`
	Location     string  `json:"location"`
	Available    bool    `json:"available"`
	Recommended  bool    `json:"recommended"`
	Deprecated   bool    `json:"deprecated"`
	Cores        int     `json:"cores"`
	MemoryGB     float32 `json:"memory_gb"`
	DiskGB       int     `json:"disk_gb"`
	Architecture string  `json:"architecture"`
}

// Checker reports server type availability.
type Checker struct {
	api *hcloud.Client
}

// NewChecker returns a Checker for the given project token.
func NewChecker(token string) *Checker {
	return &Checker{api: hcloud.NewClient(hcloud.WithToken(token))}
}

// All returns every server type in every location it is offered in.
//
// Source is ServerType.Locations, NOT Datacenter.ServerTypes: the latter is
// deprecated and stops being returned by the API after 2026-10-01.
func (c *Checker) All(ctx context.Context) ([]Availability, error) {
	types, err := c.api.ServerType.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("preflight: listing server types: %w", err)
	}

	out := make([]Availability, 0, len(types))
	for _, st := range types {
		for _, loc := range st.Locations {
			if loc.Location == nil {
				continue
			}
			out = append(out, Availability{
				ServerType:   st.Name,
				Location:     loc.Location.Name,
				Available:    loc.Available,
				Recommended:  loc.Recommended,
				Deprecated:   loc.IsDeprecated(),
				Cores:        st.Cores,
				MemoryGB:     st.Memory,
				DiskGB:       st.Disk,
				Architecture: string(st.Architecture),
			})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].ServerType != out[j].ServerType {
			return out[i].ServerType < out[j].ServerType
		}
		return out[i].Location < out[j].Location
	})
	return out, nil
}

// IsAvailable reports whether serverType can be created in location right now.
//
// An unknown type or location returns false with an error rather than a bare
// false, so a typo is never mistaken for "out of stock".
func (c *Checker) IsAvailable(ctx context.Context, serverType, location string) (bool, error) {
	all, err := c.All(ctx)
	if err != nil {
		return false, err
	}

	typeExists := false
	for _, a := range all {
		if a.ServerType != serverType {
			continue
		}
		typeExists = true
		if a.Location == location {
			return a.Available, nil
		}
	}

	if !typeExists {
		return false, fmt.Errorf("preflight: unknown server type %q", serverType)
	}
	return false, fmt.Errorf("preflight: server type %q is not offered in location %q", serverType, location)
}
