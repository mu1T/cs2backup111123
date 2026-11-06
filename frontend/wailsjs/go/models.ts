export namespace app {
	
	export class AppInfo {
	    name: string;
	    version: string;
	    description: string;
	    os: string;
	    arch: string;
	
	    static createFrom(source: any = {}) {
	        return new AppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.description = source["description"];
	        this.os = source["os"];
	        this.arch = source["arch"];
	    }
	}

}

export namespace backup {
	
	export class DiffLine {
	    type: string;
	    old_line_num: number;
	    new_line_num: number;
	    content: string;
	    old_content?: string;
	
	    static createFrom(source: any = {}) {
	        return new DiffLine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.old_line_num = source["old_line_num"];
	        this.new_line_num = source["new_line_num"];
	        this.content = source["content"];
	        this.old_content = source["old_content"];
	    }
	}
	export class DiffResult {
	    lines: DiffLine[];
	    add_count: number;
	    del_count: number;
	    mod_count: number;
	
	    static createFrom(source: any = {}) {
	        return new DiffResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.lines = this.convertValues(source["lines"], DiffLine);
	        this.add_count = source["add_count"];
	        this.del_count = source["del_count"];
	        this.mod_count = source["mod_count"];
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
	export class FileDiffItem {
	    rel_path: string;
	    status: string;
	    snapshot_hash?: string;
	    live_hash?: string;
	    diff?: DiffResult;
	
	    static createFrom(source: any = {}) {
	        return new FileDiffItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rel_path = source["rel_path"];
	        this.status = source["status"];
	        this.snapshot_hash = source["snapshot_hash"];
	        this.live_hash = source["live_hash"];
	        this.diff = this.convertValues(source["diff"], DiffResult);
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
	export class GameCFGFile {
	    name: string;
	    size: number;
	    modified: string;
	    ext: string;
	    source: string;
	    read_only: boolean;
	
	    static createFrom(source: any = {}) {
	        return new GameCFGFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.size = source["size"];
	        this.modified = source["modified"];
	        this.ext = source["ext"];
	        this.source = source["source"];
	        this.read_only = source["read_only"];
	    }
	}
	export class SnapshotInfo {
	    id: string;
	    account_id: string;
	    created_at: string;
	    note: string;
	    total_files: number;
	    total_bytes: number;
	    file_hashes: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new SnapshotInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.account_id = source["account_id"];
	        this.created_at = source["created_at"];
	        this.note = source["note"];
	        this.total_files = source["total_files"];
	        this.total_bytes = source["total_bytes"];
	        this.file_hashes = source["file_hashes"];
	    }
	}

}

export namespace crosshair {
	
	export class Convar {
	    key: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new Convar(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.value = source["value"];
	    }
	}
	export class CommandPreview {
	    lines: Convar[];
	    console: string;
	    share: string;
	
	    static createFrom(source: any = {}) {
	        return new CommandPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.lines = this.convertValues(source["lines"], Convar);
	        this.console = source["console"];
	        this.share = source["share"];
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
	
	export class RGBA {
	    r: number;
	    g: number;
	    b: number;
	    a: number;
	
	    static createFrom(source: any = {}) {
	        return new RGBA(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.r = source["r"];
	        this.g = source["g"];
	        this.b = source["b"];
	        this.a = source["a"];
	    }
	}
	export class Crosshair {
	    format: string;
	    style: number;
	    follow_recoil: boolean;
	    dot: boolean;
	    t_style: boolean;
	    outline_mode: number;
	    color: RGBA;
	    outline_color: RGBA;
	    gap: number;
	    length: number;
	    thickness: number;
	    spread_limit: number;
	    split_dist: number;
	    inner_alpha: number;
	    outer_alpha: number;
	    split_ratio: number;
	    screen_height: number;
	    scope_dot_scale: number;
	    scope_dot_use_color: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Crosshair(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.format = source["format"];
	        this.style = source["style"];
	        this.follow_recoil = source["follow_recoil"];
	        this.dot = source["dot"];
	        this.t_style = source["t_style"];
	        this.outline_mode = source["outline_mode"];
	        this.color = this.convertValues(source["color"], RGBA);
	        this.outline_color = this.convertValues(source["outline_color"], RGBA);
	        this.gap = source["gap"];
	        this.length = source["length"];
	        this.thickness = source["thickness"];
	        this.spread_limit = source["spread_limit"];
	        this.split_dist = source["split_dist"];
	        this.inner_alpha = source["inner_alpha"];
	        this.outer_alpha = source["outer_alpha"];
	        this.split_ratio = source["split_ratio"];
	        this.screen_height = source["screen_height"];
	        this.scope_dot_scale = source["scope_dot_scale"];
	        this.scope_dot_use_color = source["scope_dot_use_color"];
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
	export class ParsedCFG {
	    crosshair: Crosshair;
	    found_keys: string[];
	    missing_keys: string[];
	    display_toggle: string;
	    display_toggle_present: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ParsedCFG(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.crosshair = this.convertValues(source["crosshair"], Crosshair);
	        this.found_keys = source["found_keys"];
	        this.missing_keys = source["missing_keys"];
	        this.display_toggle = source["display_toggle"];
	        this.display_toggle_present = source["display_toggle_present"];
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
	export class Preset {
	    id: string;
	    name: string;
	    description: string;
	    builtin: boolean;
	    crosshair: Crosshair;
	
	    static createFrom(source: any = {}) {
	        return new Preset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.builtin = source["builtin"];
	        this.crosshair = this.convertValues(source["crosshair"], Crosshair);
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
	
	export class WriteReport {
	    replaced_keys: string[];
	    appended_keys: string[];
	    commented_keys: string[];
	    display_toggle: string;
	    preserved_toggle: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WriteReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.replaced_keys = source["replaced_keys"];
	        this.appended_keys = source["appended_keys"];
	        this.commented_keys = source["commented_keys"];
	        this.display_toggle = source["display_toggle"];
	        this.preserved_toggle = source["preserved_toggle"];
	    }
	}
	export class SaveResult {
	    path: string;
	    report: WriteReport;
	    backup_path: string;
	    share_code: string;
	    created: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SaveResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.report = this.convertValues(source["report"], WriteReport);
	        this.backup_path = source["backup_path"];
	        this.share_code = source["share_code"];
	        this.created = source["created"];
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
	export class State {
	    path: string;
	    exists: boolean;
	    parsed: ParsedCFG;
	    share_code: string;
	
	    static createFrom(source: any = {}) {
	        return new State(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.exists = source["exists"];
	        this.parsed = this.convertValues(source["parsed"], ParsedCFG);
	        this.share_code = source["share_code"];
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

export namespace lan {
	
	export class DiscoveredDevice {
	    ip: string;
	    port: number;
	    name: string;
	    version: string;
	    // Go type: time
	    last_seen: any;
	    is_manual: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DiscoveredDevice(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ip = source["ip"];
	        this.port = source["port"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.last_seen = this.convertValues(source["last_seen"], null);
	        this.is_manual = source["is_manual"];
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
	export class DownloadedBackupResult {
	    account_id: string;
	    dest_dir: string;
	    total_files: number;
	    total_bytes: number;
	    source_name: string;
	    download_time: string;
	
	    static createFrom(source: any = {}) {
	        return new DownloadedBackupResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.account_id = source["account_id"];
	        this.dest_dir = source["dest_dir"];
	        this.total_files = source["total_files"];
	        this.total_bytes = source["total_bytes"];
	        this.source_name = source["source_name"];
	        this.download_time = source["download_time"];
	    }
	}
	export class LANServerStatus {
	    running: boolean;
	    url: string;
	    all_urls: string[];
	    port: number;
	    share_name: string;
	    pin: string;
	    pin_enabled: boolean;
	    shared_count: number;
	    connections: number;
	    shared_local_ids: string[];
	    shared_zip_paths: string[];
	    rate_limit: number;
	    bind_address: string;
	    available_addresses: string[];
	
	    static createFrom(source: any = {}) {
	        return new LANServerStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.url = source["url"];
	        this.all_urls = source["all_urls"];
	        this.port = source["port"];
	        this.share_name = source["share_name"];
	        this.pin = source["pin"];
	        this.pin_enabled = source["pin_enabled"];
	        this.shared_count = source["shared_count"];
	        this.connections = source["connections"];
	        this.shared_local_ids = source["shared_local_ids"];
	        this.shared_zip_paths = source["shared_zip_paths"];
	        this.rate_limit = source["rate_limit"];
	        this.bind_address = source["bind_address"];
	        this.available_addresses = source["available_addresses"];
	    }
	}
	export class PackageInfo {
	    filename: string;
	    path: string;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new PackageInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filename = source["filename"];
	        this.path = source["path"];
	        this.size = source["size"];
	    }
	}
	export class RemoteBackupItem {
	    id: string;
	    filename: string;
	    size: number;
	    backup_name: string;
	    created_at: string;
	    contains_cfg: boolean;
	    contains_userdata: boolean;
	    is_local: boolean;
	    account_id?: string;
	
	    static createFrom(source: any = {}) {
	        return new RemoteBackupItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.filename = source["filename"];
	        this.size = source["size"];
	        this.backup_name = source["backup_name"];
	        this.created_at = source["created_at"];
	        this.contains_cfg = source["contains_cfg"];
	        this.contains_userdata = source["contains_userdata"];
	        this.is_local = source["is_local"];
	        this.account_id = source["account_id"];
	    }
	}

}

export namespace logger {
	
	export class LogEntry {
	    id: number;
	    timestamp: string;
	    level: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new LogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.timestamp = source["timestamp"];
	        this.level = source["level"];
	        this.message = source["message"];
	    }
	}

}

export namespace steam {
	
	export class UserCardInfo {
	    account_id: string;
	    steam64: string;
	    username: string;
	    account_name: string;
	    remember_password: boolean;
	    is_recent: boolean;
	    avatar_url: string;
	    notes: string;
	
	    static createFrom(source: any = {}) {
	        return new UserCardInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.account_id = source["account_id"];
	        this.steam64 = source["steam64"];
	        this.username = source["username"];
	        this.account_name = source["account_name"];
	        this.remember_password = source["remember_password"];
	        this.is_recent = source["is_recent"];
	        this.avatar_url = source["avatar_url"];
	        this.notes = source["notes"];
	    }
	}

}

export namespace video {
	
	export class DisplayMode {
	    width: number;
	    height: number;
	    refresh_rate: number;
	    bits_per_pel: number;
	
	    static createFrom(source: any = {}) {
	        return new DisplayMode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.width = source["width"];
	        this.height = source["height"];
	        this.refresh_rate = source["refresh_rate"];
	        this.bits_per_pel = source["bits_per_pel"];
	    }
	}
	export class VideoConfigData {
	    file_path: string;
	    read_only: boolean;
	    settings: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new VideoConfigData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file_path = source["file_path"];
	        this.read_only = source["read_only"];
	        this.settings = source["settings"];
	    }
	}
	export class VideoPreset {
	    id: string;
	    name: string;
	    created_at: string;
	    is_default: boolean;
	    file_path: string;
	    settings: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new VideoPreset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.created_at = source["created_at"];
	        this.is_default = source["is_default"];
	        this.file_path = source["file_path"];
	        this.settings = source["settings"];
	    }
	}

}

