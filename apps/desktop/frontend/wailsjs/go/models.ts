export namespace agent {
	
	export class LogEntry {
	    time: string;
	    type: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new LogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.type = source["type"];
	        this.message = source["message"];
	    }
	}
	export class StatusInfo {
	    connected: boolean;
	    uptime: string;
	    handles: string[];
	    device_name: string;
	
	    static createFrom(source: any = {}) {
	        return new StatusInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connected = source["connected"];
	        this.uptime = source["uptime"];
	        this.handles = source["handles"];
	        this.device_name = source["device_name"];
	    }
	}

}

export namespace main {
	
	export class UpdateStatus {
	    available: boolean;
	    version: string;
	    notes: string;
	    required: boolean;
	    downloading: boolean;
	    progress: number;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.version = source["version"];
	        this.notes = source["notes"];
	        this.required = source["required"];
	        this.downloading = source["downloading"];
	        this.progress = source["progress"];
	        this.error = source["error"];
	    }
	}

}

