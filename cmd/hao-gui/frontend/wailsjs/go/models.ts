export namespace main {
	
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

