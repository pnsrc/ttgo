export namespace client {
	
	export class Config {
	    Endpoint: string;
	    Hostname: string;
	    Username: string;
	    Password: string;
	    Insecure: boolean;
	    PinnedCertPEM: number[];
	    Exclusions: string[];
	    EnableAdBlock: boolean;
	    UpstreamDNS: string;
	    RoutingMode: string;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Endpoint = source["Endpoint"];
	        this.Hostname = source["Hostname"];
	        this.Username = source["Username"];
	        this.Password = source["Password"];
	        this.Insecure = source["Insecure"];
	        this.PinnedCertPEM = source["PinnedCertPEM"];
	        this.Exclusions = source["Exclusions"];
	        this.EnableAdBlock = source["EnableAdBlock"];
	        this.UpstreamDNS = source["UpstreamDNS"];
	        this.RoutingMode = source["RoutingMode"];
	    }
	}
	export class ConnEntry {
	    id: number;
	    target: string;
	    domain?: string;
	    started_at: number;
	    ended_at?: number;
	    bytes_up: number;
	    bytes_down: number;
	    active: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ConnEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.target = source["target"];
	        this.domain = source["domain"];
	        this.started_at = source["started_at"];
	        this.ended_at = source["ended_at"];
	        this.bytes_up = source["bytes_up"];
	        this.bytes_down = source["bytes_down"];
	        this.active = source["active"];
	    }
	}
	export class EndpointTOML {
	    Hostname: string;
	    Addresses: string[];
	    Username: string;
	    Password: string;
	    ClientRandom: string;
	    CustomSNI: string;
	    HasIPv6: boolean;
	    SkipVerification: boolean;
	    UpstreamProtocol: string;
	    UpstreamFallbackProtocol: string;
	    AntiDPI: boolean;
	    Certificate: string;
	
	    static createFrom(source: any = {}) {
	        return new EndpointTOML(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Hostname = source["Hostname"];
	        this.Addresses = source["Addresses"];
	        this.Username = source["Username"];
	        this.Password = source["Password"];
	        this.ClientRandom = source["ClientRandom"];
	        this.CustomSNI = source["CustomSNI"];
	        this.HasIPv6 = source["HasIPv6"];
	        this.SkipVerification = source["SkipVerification"];
	        this.UpstreamProtocol = source["UpstreamProtocol"];
	        this.UpstreamFallbackProtocol = source["UpstreamFallbackProtocol"];
	        this.AntiDPI = source["AntiDPI"];
	        this.Certificate = source["Certificate"];
	    }
	}
	export class EnrollResult {
	    ok: boolean;
	    profile_id?: string;
	    error?: string;
	    message?: string;
	    revoked?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new EnrollResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.profile_id = source["profile_id"];
	        this.error = source["error"];
	        this.message = source["message"];
	        this.revoked = source["revoked"];
	    }
	}
	export class EnrollState {
	    url: string;
	    fingerprint: string;
	    profile_id?: string;
	
	    static createFrom(source: any = {}) {
	        return new EnrollState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.fingerprint = source["fingerprint"];
	        this.profile_id = source["profile_id"];
	    }
	}
	export class GlobalSettings {
	    last_profile_id: string;
	    bypass_domains: boolean;
	    global_exclusions: string[];
	    language: string;
	    auto_connect: boolean;
	    enable_adblock: boolean;
	    theme: string;
	    upstream_dns: string;
	    routing_mode: string;
	
	    static createFrom(source: any = {}) {
	        return new GlobalSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.last_profile_id = source["last_profile_id"];
	        this.bypass_domains = source["bypass_domains"];
	        this.global_exclusions = source["global_exclusions"];
	        this.language = source["language"];
	        this.auto_connect = source["auto_connect"];
	        this.enable_adblock = source["enable_adblock"];
	        this.theme = source["theme"];
	        this.upstream_dns = source["upstream_dns"];
	        this.routing_mode = source["routing_mode"];
	    }
	}
	export class SocksTOML {
	    Address: string;
	    Username: string;
	    Password: string;
	
	    static createFrom(source: any = {}) {
	        return new SocksTOML(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Address = source["Address"];
	        this.Username = source["Username"];
	        this.Password = source["Password"];
	    }
	}
	export class TunTOML {
	    BoundIf: string;
	    MTU: number;
	    ChangeSystemDNS: boolean;
	    IncludedRoutes: string[];
	    ExcludedRoutes: string[];
	
	    static createFrom(source: any = {}) {
	        return new TunTOML(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.BoundIf = source["BoundIf"];
	        this.MTU = source["MTU"];
	        this.ChangeSystemDNS = source["ChangeSystemDNS"];
	        this.IncludedRoutes = source["IncludedRoutes"];
	        this.ExcludedRoutes = source["ExcludedRoutes"];
	    }
	}
	export class ListenerTOML {
	    TUN?: TunTOML;
	    SOCKS?: SocksTOML;
	
	    static createFrom(source: any = {}) {
	        return new ListenerTOML(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.TUN = this.convertValues(source["TUN"], TunTOML);
	        this.SOCKS = this.convertValues(source["SOCKS"], SocksTOML);
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
	export class ProfileTOML {
	    LogLevel: string;
	    VPNMode: string;
	    KillswitchEnabled: boolean;
	    PostQuantumEnabled: boolean;
	    Exclusions: string[];
	    DNSUpstreams: string[];
	    Endpoint: EndpointTOML;
	    Listener: ListenerTOML;
	
	    static createFrom(source: any = {}) {
	        return new ProfileTOML(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.LogLevel = source["LogLevel"];
	        this.VPNMode = source["VPNMode"];
	        this.KillswitchEnabled = source["KillswitchEnabled"];
	        this.PostQuantumEnabled = source["PostQuantumEnabled"];
	        this.Exclusions = source["Exclusions"];
	        this.DNSUpstreams = source["DNSUpstreams"];
	        this.Endpoint = this.convertValues(source["Endpoint"], EndpointTOML);
	        this.Listener = this.convertValues(source["Listener"], ListenerTOML);
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
	export class Profile {
	    id: string;
	    path: string;
	    name: string;
	    endpoint: string;
	    username: string;
	    toml: ProfileTOML;
	
	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.name = source["name"];
	        this.endpoint = source["endpoint"];
	        this.username = source["username"];
	        this.toml = this.convertValues(source["toml"], ProfileTOML);
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

export namespace main {
	
	export class BuildInfo {
	    version: string;
	    build_date: string;
	    git_commit: string;
	    git_branch: string;
	    go_version: string;
	    os: string;
	    arch: string;
	
	    static createFrom(source: any = {}) {
	        return new BuildInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.build_date = source["build_date"];
	        this.git_commit = source["git_commit"];
	        this.git_branch = source["git_branch"];
	        this.go_version = source["go_version"];
	        this.os = source["os"];
	        this.arch = source["arch"];
	    }
	}
	export class ConflictAdapter {
	    name: string;
	    description: string;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new ConflictAdapter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.status = source["status"];
	    }
	}
	export class UpdateInfo {
	    available: boolean;
	    version: string;
	    url: string;
	    release_notes: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.version = source["version"];
	        this.url = source["url"];
	        this.release_notes = source["release_notes"];
	    }
	}

}

