export namespace main {
	
	export class LogEntry {
	    timestamp: string;
	    message: string;
	    level: string;
	
	    static createFrom(source: any = {}) {
	        return new LogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.message = source["message"];
	        this.level = source["level"];
	    }
	}
	export class ServerStatus {
	    mdnsActive: boolean;
	    httpActive: boolean;
	    httpAddress: string;
	    printerCount: number;
	
	    static createFrom(source: any = {}) {
	        return new ServerStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mdnsActive = source["mdnsActive"];
	        this.httpActive = source["httpActive"];
	        this.httpAddress = source["httpAddress"];
	        this.printerCount = source["printerCount"];
	    }
	}
	export class UpdateCheckResult {
	    updateAvailable: boolean;
	    latestVersion: string;
	    downloadUrl: string;
	    releaseNotes: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateCheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.updateAvailable = source["updateAvailable"];
	        this.latestVersion = source["latestVersion"];
	        this.downloadUrl = source["downloadUrl"];
	        this.releaseNotes = source["releaseNotes"];
	    }
	}

}

export namespace mdns {
	
	export class DiscoveredDevice {
	    name: string;
	    ips: string[];
	    port: number;
	    token: string;
	
	    static createFrom(source: any = {}) {
	        return new DiscoveredDevice(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.ips = source["ips"];
	        this.port = source["port"];
	        this.token = source["token"];
	    }
	}

}

export namespace printer {
	
	export class SupplyInfo {
	    name: string;
	    type: string;
	    percent: number;
	
	    static createFrom(source: any = {}) {
	        return new SupplyInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.percent = source["percent"];
	    }
	}
	export class PrinterInfo {
	    name: string;
	    status: string;
	    isDefault: boolean;
	    shared: boolean;
	    supplies: SupplyInfo[];
	
	    static createFrom(source: any = {}) {
	        return new PrinterInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.status = source["status"];
	        this.isDefault = source["isDefault"];
	        this.shared = source["shared"];
	        this.supplies = this.convertValues(source["supplies"], SupplyInfo);
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

export namespace remote {
	
	export class ConnectedDevice {
	    name: string;
	    ips: string[];
	    port: number;
	    status: string;
	    activeIP: string;
	    token: string;
	
	    static createFrom(source: any = {}) {
	        return new ConnectedDevice(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.ips = source["ips"];
	        this.port = source["port"];
	        this.status = source["status"];
	        this.activeIP = source["activeIP"];
	        this.token = source["token"];
	    }
	}
	export class RemotePrinter {
	    name: string;
	    status: string;
	    isDefault: boolean;
	    deviceName: string;
	    deviceIP: string;
	    devicePort: number;
	
	    static createFrom(source: any = {}) {
	        return new RemotePrinter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.status = source["status"];
	        this.isDefault = source["isDefault"];
	        this.deviceName = source["deviceName"];
	        this.deviceIP = source["deviceIP"];
	        this.devicePort = source["devicePort"];
	    }
	}

}

export namespace server {
	
	export class PrintJob {
	    id: string;
	    filename: string;
	    printer: string;
	    status: string;
	    submittedAt: string;
	    pages: string;
	    color: string;
	    copies: number;
	    orientation?: string;
	    paperSize?: string;
	    duplex?: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new PrintJob(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.filename = source["filename"];
	        this.printer = source["printer"];
	        this.status = source["status"];
	        this.submittedAt = source["submittedAt"];
	        this.pages = source["pages"];
	        this.color = source["color"];
	        this.copies = source["copies"];
	        this.orientation = source["orientation"];
	        this.paperSize = source["paperSize"];
	        this.duplex = source["duplex"];
	        this.error = source["error"];
	    }
	}

}

