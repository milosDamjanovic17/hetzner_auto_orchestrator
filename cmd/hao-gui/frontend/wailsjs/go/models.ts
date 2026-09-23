export namespace hetzner {
	
	export class Certificate {
	    id: number;
	    name: string;
	    type: string;
	    domain_names: string[];
	    // Go type: time
	    not_valid_after: any;
	
	    static createFrom(source: any = {}) {
	        return new Certificate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.domain_names = source["domain_names"];
	        this.not_valid_after = this.convertValues(source["not_valid_after"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Firewall {
	    id: number;
	    name: string;
	    rules: number;
	    applied_to: number;
	
	    static createFrom(source: any = {}) {
	        return new Firewall(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.rules = source["rules"];
	        this.applied_to = source["applied_to"];
	    }
	}
	export class FloatingIP {
	    id: number;
	    name: string;
	    type: string;
	    ip: string;
	    home_location: string;
	    assigned_to: string;
	
	    static createFrom(source: any = {}) {
	        return new FloatingIP(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.ip = source["ip"];
	        this.home_location = source["home_location"];
	        this.assigned_to = source["assigned_to"];
	    }
	}
	export class LoadBalancer {
	    id: number;
	    name: string;
	    type: string;
	    location: string;
	    public_ipv4: string;
	    services: number;
	    targets: number;
	
	    static createFrom(source: any = {}) {
	        return new LoadBalancer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.location = source["location"];
	        this.public_ipv4 = source["public_ipv4"];
	        this.services = source["services"];
	        this.targets = source["targets"];
	    }
	}
	export class Network {
	    id: number;
	    name: string;
	    ip_range: string;
	    subnets: number;
	    servers: number;
	
	    static createFrom(source: any = {}) {
	        return new Network(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.ip_range = source["ip_range"];
	        this.subnets = source["subnets"];
	        this.servers = source["servers"];
	    }
	}
	export class SSHKey {
	    id: number;
	    name: string;
	    fingerprint: string;
	
	    static createFrom(source: any = {}) {
	        return new SSHKey(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.fingerprint = source["fingerprint"];
	    }
	}
	export class Server {
	    id: number;
	    name: string;
	    status: string;
	    server_type: string;
	    location: string;
	    public_ipv4: string;
	    // Go type: time
	    created: any;
	
	    static createFrom(source: any = {}) {
	        return new Server(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.status = source["status"];
	        this.server_type = source["server_type"];
	        this.location = source["location"];
	        this.public_ipv4 = source["public_ipv4"];
	        this.created = this.convertValues(source["created"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Volume {
	    id: number;
	    name: string;
	    status: string;
	    size_gb: number;
	    location: string;
	    attached_to: string;
	
	    static createFrom(source: any = {}) {
	        return new Volume(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.status = source["status"];
	        this.size_gb = source["size_gb"];
	        this.location = source["location"];
	        this.attached_to = source["attached_to"];
	    }
	}
	export class Zone {
	    id: number;
	    name: string;
	    status: string;
	    mode: string;
	    record_count: number;
	
	    static createFrom(source: any = {}) {
	        return new Zone(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.status = source["status"];
	        this.mode = source["mode"];
	        this.record_count = source["record_count"];
	    }
	}

}

export namespace main {
	
	export class Listing_github_com_milosDamjanovic17_hetzner_auto_orchestrator_internal_hetzner_Certificate_ {
	    context: string;
	    items: hetzner.Certificate[];
	
	    static createFrom(source: any = {}) {
	        return new Listing_github_com_milosDamjanovic17_hetzner_auto_orchestrator_internal_hetzner_Certificate_(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.context = source["context"];
	        this.items = this.convertValues(source["items"], hetzner.Certificate);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Listing_github_com_milosDamjanovic17_hetzner_auto_orchestrator_internal_hetzner_Firewall_ {
	    context: string;
	    items: hetzner.Firewall[];
	
	    static createFrom(source: any = {}) {
	        return new Listing_github_com_milosDamjanovic17_hetzner_auto_orchestrator_internal_hetzner_Firewall_(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.context = source["context"];
	        this.items = this.convertValues(source["items"], hetzner.Firewall);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Listing_github_com_milosDamjanovic17_hetzner_auto_orchestrator_internal_hetzner_FloatingIP_ {
	    context: string;
	    items: hetzner.FloatingIP[];
	
	    static createFrom(source: any = {}) {
	        return new Listing_github_com_milosDamjanovic17_hetzner_auto_orchestrator_internal_hetzner_FloatingIP_(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.context = source["context"];
	        this.items = this.convertValues(source["items"], hetzner.FloatingIP);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Listing_github_com_milosDamjanovic17_hetzner_auto_orchestrator_internal_hetzner_LoadBalancer_ {
	    context: string;
	    items: hetzner.LoadBalancer[];
	
	    static createFrom(source: any = {}) {
	        return new Listing_github_com_milosDamjanovic17_hetzner_auto_orchestrator_internal_hetzner_LoadBalancer_(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.context = source["context"];
	        this.items = this.convertValues(source["items"], hetzner.LoadBalancer);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Listing_github_com_milosDamjanovic17_hetzner_auto_orchestrator_internal_hetzner_Network_ {
	    context: string;
	    items: hetzner.Network[];
	
	    static createFrom(source: any = {}) {
	        return new Listing_github_com_milosDamjanovic17_hetzner_auto_orchestrator_internal_hetzner_Network_(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.context = source["context"];
	        this.items = this.convertValues(source["items"], hetzner.Network);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Listing_github_com_milosDamjanovic17_hetzner_auto_orchestrator_internal_hetzner_SSHKey_ {
	    context: string;
	    items: hetzner.SSHKey[];
	
	    static createFrom(source: any = {}) {
	        return new Listing_github_com_milosDamjanovic17_hetzner_auto_orchestrator_internal_hetzner_SSHKey_(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.context = source["context"];
	        this.items = this.convertValues(source["items"], hetzner.SSHKey);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Listing_github_com_milosDamjanovic17_hetzner_auto_orchestrator_internal_hetzner_Server_ {
	    context: string;
	    items: hetzner.Server[];
	
	    static createFrom(source: any = {}) {
	        return new Listing_github_com_milosDamjanovic17_hetzner_auto_orchestrator_internal_hetzner_Server_(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.context = source["context"];
	        this.items = this.convertValues(source["items"], hetzner.Server);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Listing_github_com_milosDamjanovic17_hetzner_auto_orchestrator_internal_hetzner_Volume_ {
	    context: string;
	    items: hetzner.Volume[];
	
	    static createFrom(source: any = {}) {
	        return new Listing_github_com_milosDamjanovic17_hetzner_auto_orchestrator_internal_hetzner_Volume_(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.context = source["context"];
	        this.items = this.convertValues(source["items"], hetzner.Volume);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Listing_github_com_milosDamjanovic17_hetzner_auto_orchestrator_internal_hetzner_Zone_ {
	    context: string;
	    items: hetzner.Zone[];
	
	    static createFrom(source: any = {}) {
	        return new Listing_github_com_milosDamjanovic17_hetzner_auto_orchestrator_internal_hetzner_Zone_(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.context = source["context"];
	        this.items = this.convertValues(source["items"], hetzner.Zone);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PreflightResult {
	    context: string;
	    answer: preflight.Answer;
	
	    static createFrom(source: any = {}) {
	        return new PreflightResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.context = source["context"];
	        this.answer = this.convertValues(source["answer"], preflight.Answer);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Status {
	    initialized: boolean;
	    active: string;
	    envTokenIgnored: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.initialized = source["initialized"];
	        this.active = source["active"];
	        this.envTokenIgnored = source["envTokenIgnored"];
	    }
	}

}

export namespace preflight {
	
	export class LocationAvailability {
	    location: string;
	    city: string;
	    available: Availability[];
	
	    static createFrom(source: any = {}) {
	        return new LocationAvailability(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.location = source["location"];
	        this.city = source["city"];
	        this.available = this.convertValues(source["available"], Availability);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Availability {
	    server_type: string;
	    location: string;
	    city: string;
	    available: boolean;
	    recommended: boolean;
	    deprecated: boolean;
	    cores: number;
	    memory_gb: number;
	    disk_gb: number;
	    architecture: string;
	
	    static createFrom(source: any = {}) {
	        return new Availability(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_type = source["server_type"];
	        this.location = source["location"];
	        this.city = source["city"];
	        this.available = source["available"];
	        this.recommended = source["recommended"];
	        this.deprecated = source["deprecated"];
	        this.cores = source["cores"];
	        this.memory_gb = source["memory_gb"];
	        this.disk_gb = source["disk_gb"];
	        this.architecture = source["architecture"];
	    }
	}
	export class Answer {
	    mode: string;
	    all: Availability[];
	    serverType: string;
	    location: string;
	    available: boolean;
	    groups: LocationAvailability[];
	
	    static createFrom(source: any = {}) {
	        return new Answer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.all = this.convertValues(source["all"], Availability);
	        this.serverType = source["serverType"];
	        this.location = source["location"];
	        this.available = source["available"];
	        this.groups = this.convertValues(source["groups"], LocationAvailability);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	

}

export namespace service {
	
	export class ContextInfo {
	    name: string;
	    active: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ContextInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.active = source["active"];
	    }
	}

}

