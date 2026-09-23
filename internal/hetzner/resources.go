package hetzner

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

// The nine resource types Phase 1 lists. All reachable with the single project
// token, including Zones -- DNS moved into the Cloud API in May 2026.
//
// API Tokens and Members are deliberately absent: Hetzner exposes no API for
// them, so the UI links to the Console instead of pretending.

type Server struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	ServerType string    `json:"server_type"`
	Location   string    `json:"location"`
	PublicIPv4 string    `json:"public_ipv4"`
	Created    time.Time `json:"created"`
}

type LoadBalancer struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Location   string `json:"location"`
	PublicIPv4 string `json:"public_ipv4"`
	Services   int    `json:"services"`
	Targets    int    `json:"targets"`
}

type Network struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	IPRange string `json:"ip_range"`
	Subnets int    `json:"subnets"`
	Servers int    `json:"servers"`
}

type Firewall struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Rules     int    `json:"rules"`
	AppliedTo int    `json:"applied_to"`
}

type FloatingIP struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	IP           string `json:"ip"`
	HomeLocation string `json:"home_location"`
	AssignedTo   string `json:"assigned_to"`
}

type Volume struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	SizeGB     int    `json:"size_gb"`
	Location   string `json:"location"`
	AttachedTo string `json:"attached_to"`
}

type SSHKey struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Fingerprint string `json:"fingerprint"`
}

type Certificate struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Type          string    `json:"type"`
	DomainNames   []string  `json:"domain_names"`
	NotValidAfter time.Time `json:"not_valid_after"`
}

type Zone struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Mode        string `json:"mode"`
	RecordCount int    `json:"record_count"`
}

// Every List method returns an empty slice rather than nil when a project has
// none of that resource, so callers can render "no servers" without a nil check
// and the GUI receives [] instead of null.

func (c *Client) Servers(ctx context.Context) ([]Server, error) {
	raw, err := c.api.Server.All(ctx)
	if err != nil {
		return nil, classify(err, "listing servers")
	}
	out := make([]Server, 0, len(raw))
	for _, s := range raw {
		v := Server{ID: s.ID, Name: s.Name, Status: string(s.Status), Created: s.Created}
		if s.ServerType != nil {
			v.ServerType = s.ServerType.Name
		}
		if s.Location != nil {
			v.Location = s.Location.Name
		}
		if s.PublicNet.IPv4.IP != nil {
			v.PublicIPv4 = s.PublicNet.IPv4.IP.String()
		}
		out = append(out, v)
	}
	return out, nil
}

func (c *Client) LoadBalancers(ctx context.Context) ([]LoadBalancer, error) {
	raw, err := c.api.LoadBalancer.All(ctx)
	if err != nil {
		return nil, classify(err, "listing load balancers")
	}
	out := make([]LoadBalancer, 0, len(raw))
	for _, lb := range raw {
		v := LoadBalancer{
			ID:       lb.ID,
			Name:     lb.Name,
			Services: len(lb.Services),
			Targets:  len(lb.Targets),
		}
		if lb.LoadBalancerType != nil {
			v.Type = lb.LoadBalancerType.Name
		}
		if lb.Location != nil {
			v.Location = lb.Location.Name
		}
		if lb.PublicNet.IPv4.IP != nil {
			v.PublicIPv4 = lb.PublicNet.IPv4.IP.String()
		}
		out = append(out, v)
	}
	return out, nil
}

func (c *Client) Networks(ctx context.Context) ([]Network, error) {
	raw, err := c.api.Network.All(ctx)
	if err != nil {
		return nil, classify(err, "listing networks")
	}
	out := make([]Network, 0, len(raw))
	for _, n := range raw {
		v := Network{ID: n.ID, Name: n.Name, Subnets: len(n.Subnets), Servers: len(n.Servers)}
		if n.IPRange != nil {
			v.IPRange = n.IPRange.String()
		}
		out = append(out, v)
	}
	return out, nil
}

func (c *Client) Firewalls(ctx context.Context) ([]Firewall, error) {
	raw, err := c.api.Firewall.All(ctx)
	if err != nil {
		return nil, classify(err, "listing firewalls")
	}
	out := make([]Firewall, 0, len(raw))
	for _, f := range raw {
		out = append(out, Firewall{
			ID:        f.ID,
			Name:      f.Name,
			Rules:     len(f.Rules),
			AppliedTo: len(f.AppliedTo),
		})
	}
	return out, nil
}

func (c *Client) FloatingIPs(ctx context.Context) ([]FloatingIP, error) {
	raw, err := c.api.FloatingIP.All(ctx)
	if err != nil {
		return nil, classify(err, "listing floating IPs")
	}
	names, err := c.serverNames(ctx, slices.ContainsFunc(raw, func(ip *hcloud.FloatingIP) bool {
		return ip.Server != nil
	}), "listing floating IPs")
	if err != nil {
		return nil, err
	}
	out := make([]FloatingIP, 0, len(raw))
	for _, ip := range raw {
		v := FloatingIP{ID: ip.ID, Name: ip.Name, Type: string(ip.Type)}
		if ip.IP != nil {
			v.IP = ip.IP.String()
		}
		if ip.HomeLocation != nil {
			v.HomeLocation = ip.HomeLocation.Name
		}
		if ip.Server != nil {
			v.AssignedTo = serverName(names, ip.Server.ID)
		}
		out = append(out, v)
	}
	return out, nil
}

func (c *Client) Volumes(ctx context.Context) ([]Volume, error) {
	raw, err := c.api.Volume.All(ctx)
	if err != nil {
		return nil, classify(err, "listing volumes")
	}
	names, err := c.serverNames(ctx, slices.ContainsFunc(raw, func(vol *hcloud.Volume) bool {
		return vol.Server != nil
	}), "listing volumes")
	if err != nil {
		return nil, err
	}
	out := make([]Volume, 0, len(raw))
	for _, vol := range raw {
		v := Volume{ID: vol.ID, Name: vol.Name, Status: string(vol.Status), SizeGB: vol.Size}
		if vol.Location != nil {
			v.Location = vol.Location.Name
		}
		if vol.Server != nil {
			v.AttachedTo = serverName(names, vol.Server.ID)
		}
		out = append(out, v)
	}
	return out, nil
}

// serverNames maps server ID to name. The API reports the server a volume or
// floating IP is attached to by ID only, so the SDK's Server there has no
// name. Skipped when nothing is attached: the common case costs no request.
func (c *Client) serverNames(ctx context.Context, needed bool, op string) (map[int64]string, error) {
	if !needed {
		return nil, nil
	}
	servers, err := c.api.Server.All(ctx)
	if err != nil {
		return nil, classify(err, op)
	}
	names := make(map[int64]string, len(servers))
	for _, s := range servers {
		names[s.ID] = s.Name
	}
	return names, nil
}

// serverName names an attached server. One deleted between the two requests
// has no name any more, but "attached" must still never read as unattached.
func serverName(names map[int64]string, id int64) string {
	if name, ok := names[id]; ok {
		return name
	}
	return fmt.Sprintf("server %d", id)
}

func (c *Client) SSHKeys(ctx context.Context) ([]SSHKey, error) {
	raw, err := c.api.SSHKey.All(ctx)
	if err != nil {
		return nil, classify(err, "listing SSH keys")
	}
	out := make([]SSHKey, 0, len(raw))
	for _, k := range raw {
		out = append(out, SSHKey{ID: k.ID, Name: k.Name, Fingerprint: k.Fingerprint})
	}
	return out, nil
}

func (c *Client) Certificates(ctx context.Context) ([]Certificate, error) {
	raw, err := c.api.Certificate.All(ctx)
	if err != nil {
		return nil, classify(err, "listing certificates")
	}
	out := make([]Certificate, 0, len(raw))
	for _, cert := range raw {
		out = append(out, Certificate{
			ID:            cert.ID,
			Name:          cert.Name,
			Type:          string(cert.Type),
			DomainNames:   cert.DomainNames,
			NotValidAfter: cert.NotValidAfter,
		})
	}
	return out, nil
}

// Zones lists DNS zones. Project-scoped, same as every other resource here, so
// switching context correctly changes what comes back.
func (c *Client) Zones(ctx context.Context) ([]Zone, error) {
	raw, err := c.api.Zone.All(ctx)
	if err != nil {
		return nil, classify(err, "listing zones")
	}
	out := make([]Zone, 0, len(raw))
	for _, z := range raw {
		out = append(out, Zone{
			ID:          z.ID,
			Name:        z.Name,
			Status:      string(z.Status),
			Mode:        string(z.Mode),
			RecordCount: z.RecordCount,
		})
	}
	return out, nil
}
