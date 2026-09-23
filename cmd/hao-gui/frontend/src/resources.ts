import {
    Certificates, Firewalls, FloatingIPs, LoadBalancers, Networks, Servers, SSHKeys, Volumes, Zones,
} from '../wailsjs/go/main/App';
import {hetzner} from '../wailsjs/go/models';

// One tab per listable resource type, in the CLI's order. Columns match what
// `hao <noun> list` prints, so the two drivers show the same facts.

type Column<T> = {title: string; value: (item: T) => string | number};

// The generated Listing_<pkgpath>_<Type>_ classes, by shape.
type Listing<T> = {context: string; items: T[]};

export type ResourceView<T> = {
    title: string;
    plural: string; // for the empty state: "no servers in this project"
    fetch: () => Promise<Listing<T>>;
    columns: Column<T>[];
};

// view type-checks one definition against its item type, then erases it so
// all nine fit in one array.
function view<T>(v: ResourceView<T>): ResourceView<any> {
    return v;
}

// orDash keeps a cell readable when a field is legitimately empty, such as an
// unattached volume or a server without a public IPv4.
function orDash(s: string): string {
    return s || '-';
}

// Go's zero time.Time: a managed certificate still being issued has no expiry.
function date(t: string): string {
    return !t || t.startsWith('0001-') ? '-' : t.slice(0, 10);
}

export const resourceViews: ResourceView<any>[] = [
    view<hetzner.Server>({
        title: 'Servers', plural: 'servers', fetch: Servers, columns: [
            {title: 'Name', value: s => s.name},
            {title: 'Status', value: s => s.status},
            {title: 'Type', value: s => s.server_type},
            {title: 'Location', value: s => s.location},
            {title: 'Public IPv4', value: s => orDash(s.public_ipv4)},
        ],
    }),
    view<hetzner.LoadBalancer>({
        title: 'Load balancers', plural: 'load balancers', fetch: LoadBalancers, columns: [
            {title: 'Name', value: lb => lb.name},
            {title: 'Type', value: lb => lb.type},
            {title: 'Location', value: lb => lb.location},
            {title: 'Public IPv4', value: lb => orDash(lb.public_ipv4)},
            {title: 'Services', value: lb => lb.services},
            {title: 'Targets', value: lb => lb.targets},
        ],
    }),
    view<hetzner.Network>({
        title: 'Networks', plural: 'networks', fetch: Networks, columns: [
            {title: 'Name', value: n => n.name},
            {title: 'IP range', value: n => orDash(n.ip_range)},
            {title: 'Subnets', value: n => n.subnets},
            {title: 'Servers', value: n => n.servers},
        ],
    }),
    view<hetzner.Firewall>({
        title: 'Firewalls', plural: 'firewalls', fetch: Firewalls, columns: [
            {title: 'Name', value: f => f.name},
            {title: 'Rules', value: f => f.rules},
            {title: 'Applied to', value: f => f.applied_to},
        ],
    }),
    view<hetzner.FloatingIP>({
        title: 'Floating IPs', plural: 'floating IPs', fetch: FloatingIPs, columns: [
            {title: 'Name', value: ip => ip.name},
            {title: 'Type', value: ip => ip.type},
            {title: 'IP', value: ip => orDash(ip.ip)},
            {title: 'Home location', value: ip => ip.home_location},
            {title: 'Assigned to', value: ip => orDash(ip.assigned_to)},
        ],
    }),
    view<hetzner.Volume>({
        title: 'Volumes', plural: 'volumes', fetch: Volumes, columns: [
            {title: 'Name', value: v => v.name},
            {title: 'Status', value: v => v.status},
            {title: 'Size (GB)', value: v => v.size_gb},
            {title: 'Location', value: v => v.location},
            {title: 'Attached to', value: v => orDash(v.attached_to)},
        ],
    }),
    view<hetzner.SSHKey>({
        title: 'SSH keys', plural: 'SSH keys', fetch: SSHKeys, columns: [
            {title: 'Name', value: k => k.name},
            {title: 'Fingerprint', value: k => k.fingerprint},
        ],
    }),
    view<hetzner.Certificate>({
        title: 'Certificates', plural: 'certificates', fetch: Certificates, columns: [
            {title: 'Name', value: c => c.name},
            {title: 'Type', value: c => c.type},
            {title: 'Expires', value: c => date(c.not_valid_after)},
            {title: 'Domains', value: c => (c.domain_names ?? []).join(', ')},
        ],
    }),
    view<hetzner.Zone>({
        title: 'Zones', plural: 'zones', fetch: Zones, columns: [
            {title: 'Name', value: z => z.name},
            {title: 'Status', value: z => z.status},
            {title: 'Mode', value: z => z.mode},
            {title: 'Records', value: z => z.record_count},
        ],
    }),
];
