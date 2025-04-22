export namespace common {
	
	export class ServerConfig {
	    id: string;
	    workspace: string;
	    name: string;
	    type: string;
	    transport: string;
	    cmd: string;
	    env: string[];
	    url: string;
	    params: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new ServerConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.workspace = source["workspace"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.transport = source["transport"];
	        this.cmd = source["cmd"];
	        this.env = source["env"];
	        this.url = source["url"];
	        this.params = source["params"];
	    }
	}

}

export namespace data {
	
	export class ManagedClient {
	    config: string;
	
	    static createFrom(source: any = {}) {
	        return new ManagedClient(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.config = source["config"];
	    }
	}
	export class ServerInstance {
	    id: string;
	    config?: common.ServerConfig;
	    status: string;
	    error: string;
	    serverInfo?: mcp.Implementation;
	    endpoint: string;
	
	    static createFrom(source: any = {}) {
	        return new ServerInstance(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.config = this.convertValues(source["config"], common.ServerConfig);
	        this.status = source["status"];
	        this.error = source["error"];
	        this.serverInfo = this.convertValues(source["serverInfo"], mcp.Implementation);
	        this.endpoint = source["endpoint"];
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
	export class Settings {
	    language: string;
	    theme: string;
	    baseUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.language = source["language"];
	        this.theme = source["theme"];
	        this.baseUrl = source["baseUrl"];
	    }
	}
	export class Workspace {
	    id: string;
	    name: string;
	    status: string;
	    description: string;
	    enabled: boolean;
	    autoRun: boolean;
	    managedClients: Record<string, ManagedClient>;
	
	    static createFrom(source: any = {}) {
	        return new Workspace(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.status = source["status"];
	        this.description = source["description"];
	        this.enabled = source["enabled"];
	        this.autoRun = source["autoRun"];
	        this.managedClients = this.convertValues(source["managedClients"], ManagedClient, true);
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

export namespace mcp {
	
	export class Implementation {
	    name: string;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new Implementation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	    }
	}

}

