export namespace model {
	
	export class BPMKey {
	    bpm: number;
	    key: string;
	    scale: string;
	    strength: number;
	
	    static createFrom(source: any = {}) {
	        return new BPMKey(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bpm = source["bpm"];
	        this.key = source["key"];
	        this.scale = source["scale"];
	        this.strength = source["strength"];
	    }
	}
	export class Track {
	    id: number;
	    path: string;
	    fileName: string;
	    extension: string;
	    size: number;
	    modifiedUnix: number;
	    title: string;
	    artist: string;
	    album: string;
	    albumArtist: string;
	    genre: string;
	    year: number;
	    trackNumber: number;
	    trackTotal: number;
	    discNumber: number;
	    discTotal: number;
	    composer: string;
	    comment: string;
	    durationMs: number;
	    codec: string;
	    sampleRate: number;
	    channels: number;
	    bitRate: number;
	    bpm: number;
	    key: string;
	    keyScale: string;
	    loudnessI: number;
	    truePeak: number;
	    lra: number;
	    threshold: number;
	    scanError: string;
	
	    static createFrom(source: any = {}) {
	        return new Track(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.fileName = source["fileName"];
	        this.extension = source["extension"];
	        this.size = source["size"];
	        this.modifiedUnix = source["modifiedUnix"];
	        this.title = source["title"];
	        this.artist = source["artist"];
	        this.album = source["album"];
	        this.albumArtist = source["albumArtist"];
	        this.genre = source["genre"];
	        this.year = source["year"];
	        this.trackNumber = source["trackNumber"];
	        this.trackTotal = source["trackTotal"];
	        this.discNumber = source["discNumber"];
	        this.discTotal = source["discTotal"];
	        this.composer = source["composer"];
	        this.comment = source["comment"];
	        this.durationMs = source["durationMs"];
	        this.codec = source["codec"];
	        this.sampleRate = source["sampleRate"];
	        this.channels = source["channels"];
	        this.bitRate = source["bitRate"];
	        this.bpm = source["bpm"];
	        this.key = source["key"];
	        this.keyScale = source["keyScale"];
	        this.loudnessI = source["loudnessI"];
	        this.truePeak = source["truePeak"];
	        this.lra = source["lra"];
	        this.threshold = source["threshold"];
	        this.scanError = source["scanError"];
	    }
	}
	export class DuplicateGroup {
	    artist: string;
	    title: string;
	    durationMs: number;
	    tracks: Track[];
	
	    static createFrom(source: any = {}) {
	        return new DuplicateGroup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.artist = source["artist"];
	        this.title = source["title"];
	        this.durationMs = source["durationMs"];
	        this.tracks = this.convertValues(source["tracks"], Track);
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
	export class FFmpegUpdateResult {
	    version: string;
	    changed: boolean;
	    ffmpegPath: string;
	    ffprobePath: string;
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new FFmpegUpdateResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.changed = source["changed"];
	        this.ffmpegPath = source["ffmpegPath"];
	        this.ffprobePath = source["ffprobePath"];
	        this.source = source["source"];
	    }
	}
	export class LibraryRoot {
	    path: string;
	    createdAt: string;
	    lastScanAt: string;
	
	    static createFrom(source: any = {}) {
	        return new LibraryRoot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.createdAt = source["createdAt"];
	        this.lastScanAt = source["lastScanAt"];
	    }
	}
	export class LibraryStats {
	    tracks: number;
	    artists: number;
	    albums: number;
	    durationMs: number;
	    sizeBytes: number;
	    duplicateGroups: number;
	
	    static createFrom(source: any = {}) {
	        return new LibraryStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tracks = source["tracks"];
	        this.artists = source["artists"];
	        this.albums = source["albums"];
	        this.durationMs = source["durationMs"];
	        this.sizeBytes = source["sizeBytes"];
	        this.duplicateGroups = source["duplicateGroups"];
	    }
	}
	export class Loudness {
	    inputI: number;
	    inputTP: number;
	    inputLRA: number;
	    inputThreshold: number;
	    targetOffset: number;
	
	    static createFrom(source: any = {}) {
	        return new Loudness(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.inputI = source["inputI"];
	        this.inputTP = source["inputTP"];
	        this.inputLRA = source["inputLRA"];
	        this.inputThreshold = source["inputThreshold"];
	        this.targetOffset = source["targetOffset"];
	    }
	}
	export class MetadataCandidate {
	    source: string;
	    externalId: string;
	    title: string;
	    artist: string;
	    album: string;
	    year: number;
	    genre: string;
	    artworkUrl: string;
	    durationMs: number;
	    confidence: number;
	
	    static createFrom(source: any = {}) {
	        return new MetadataCandidate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.externalId = source["externalId"];
	        this.title = source["title"];
	        this.artist = source["artist"];
	        this.album = source["album"];
	        this.year = source["year"];
	        this.genre = source["genre"];
	        this.artworkUrl = source["artworkUrl"];
	        this.durationMs = source["durationMs"];
	        this.confidence = source["confidence"];
	    }
	}
	export class MetadataLookupResult {
	    candidates: MetadataCandidate[];
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new MetadataLookupResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.candidates = this.convertValues(source["candidates"], MetadataCandidate);
	        this.warnings = source["warnings"];
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
	export class OrganizeRequest {
	    rootDir: string;
	    template: string;
	    regexPattern: string;
	    regexReplace: string;
	    move: boolean;
	
	    static createFrom(source: any = {}) {
	        return new OrganizeRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rootDir = source["rootDir"];
	        this.template = source["template"];
	        this.regexPattern = source["regexPattern"];
	        this.regexReplace = source["regexReplace"];
	        this.move = source["move"];
	    }
	}
	export class ProcessingOptions {
	    outputPath: string;
	    targetLUFS: number;
	    targetTruePeakDb: number;
	    targetLRA: number;
	    preGainDb: number;
	    repairClipping: boolean;
	    multibandCompress: boolean;
	    limit: boolean;
	    pitchSemitones: number;
	    keepOriginal: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProcessingOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.outputPath = source["outputPath"];
	        this.targetLUFS = source["targetLUFS"];
	        this.targetTruePeakDb = source["targetTruePeakDb"];
	        this.targetLRA = source["targetLRA"];
	        this.preGainDb = source["preGainDb"];
	        this.repairClipping = source["repairClipping"];
	        this.multibandCompress = source["multibandCompress"];
	        this.limit = source["limit"];
	        this.pitchSemitones = source["pitchSemitones"];
	        this.keepOriginal = source["keepOriginal"];
	    }
	}
	export class ProcessingResult {
	    inputPath: string;
	    outputPath: string;
	    measurement: Loudness;
	    filterGraph: string;
	
	    static createFrom(source: any = {}) {
	        return new ProcessingResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.inputPath = source["inputPath"];
	        this.outputPath = source["outputPath"];
	        this.measurement = this.convertValues(source["measurement"], Loudness);
	        this.filterGraph = source["filterGraph"];
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
	export class ScanResult {
	    root: string;
	    found: number;
	    indexed: number;
	    added: number;
	    updated: number;
	    skipped: number;
	    removed: number;
	    failed: number;
	    cancelled: boolean;
	    duration: number;
	    errors: string[];
	
	    static createFrom(source: any = {}) {
	        return new ScanResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.root = source["root"];
	        this.found = source["found"];
	        this.indexed = source["indexed"];
	        this.added = source["added"];
	        this.updated = source["updated"];
	        this.skipped = source["skipped"];
	        this.removed = source["removed"];
	        this.failed = source["failed"];
	        this.cancelled = source["cancelled"];
	        this.duration = source["duration"];
	        this.errors = source["errors"];
	    }
	}
	export class SystemStatus {
	    ffmpegPath: string;
	    ffprobePath: string;
	    ffmpegVersion: string;
	    ffmpegSource: string;
	    ffmpegReady: boolean;
	    ffmpegUpdating: boolean;
	    ffmpegUpdateError: string;
	    ffmpegAutoUpdateSupported: boolean;
	    essentiaPath: string;
	    essentiaReady: boolean;
	    metadataProviders: string[];
	
	    static createFrom(source: any = {}) {
	        return new SystemStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ffmpegPath = source["ffmpegPath"];
	        this.ffprobePath = source["ffprobePath"];
	        this.ffmpegVersion = source["ffmpegVersion"];
	        this.ffmpegSource = source["ffmpegSource"];
	        this.ffmpegReady = source["ffmpegReady"];
	        this.ffmpegUpdating = source["ffmpegUpdating"];
	        this.ffmpegUpdateError = source["ffmpegUpdateError"];
	        this.ffmpegAutoUpdateSupported = source["ffmpegAutoUpdateSupported"];
	        this.essentiaPath = source["essentiaPath"];
	        this.essentiaReady = source["essentiaReady"];
	        this.metadataProviders = source["metadataProviders"];
	    }
	}
	export class TagApplyResult {
	    changeSetId: number;
	    changed: number;
	    failed: number;
	    errors: string[];
	
	    static createFrom(source: any = {}) {
	        return new TagApplyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.changeSetId = source["changeSetId"];
	        this.changed = source["changed"];
	        this.failed = source["failed"];
	        this.errors = source["errors"];
	    }
	}
	export class TagHistory {
	    id: number;
	    createdAt: string;
	    label: string;
	    status: string;
	    affectedCount: number;
	
	    static createFrom(source: any = {}) {
	        return new TagHistory(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.createdAt = source["createdAt"];
	        this.label = source["label"];
	        this.status = source["status"];
	        this.affectedCount = source["affectedCount"];
	    }
	}
	export class TagPatch {
	    fields: string[];
	    title: string;
	    artist: string;
	    album: string;
	    albumArtist: string;
	    genre: string;
	    composer: string;
	    comment: string;
	    year: number;
	    trackNumber: number;
	    trackTotal: number;
	    discNumber: number;
	    discTotal: number;
	
	    static createFrom(source: any = {}) {
	        return new TagPatch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fields = source["fields"];
	        this.title = source["title"];
	        this.artist = source["artist"];
	        this.album = source["album"];
	        this.albumArtist = source["albumArtist"];
	        this.genre = source["genre"];
	        this.composer = source["composer"];
	        this.comment = source["comment"];
	        this.year = source["year"];
	        this.trackNumber = source["trackNumber"];
	        this.trackTotal = source["trackTotal"];
	        this.discNumber = source["discNumber"];
	        this.discTotal = source["discTotal"];
	    }
	}
	export class TagSnapshot {
	    title: string;
	    artist: string;
	    album: string;
	    albumArtist: string;
	    genre: string;
	    composer: string;
	    comment: string;
	    year: number;
	    trackNumber: number;
	    trackTotal: number;
	    discNumber: number;
	    discTotal: number;
	    coverMime: string;
	    coverSize: number;
	
	    static createFrom(source: any = {}) {
	        return new TagSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.artist = source["artist"];
	        this.album = source["album"];
	        this.albumArtist = source["albumArtist"];
	        this.genre = source["genre"];
	        this.composer = source["composer"];
	        this.comment = source["comment"];
	        this.year = source["year"];
	        this.trackNumber = source["trackNumber"];
	        this.trackTotal = source["trackTotal"];
	        this.discNumber = source["discNumber"];
	        this.discTotal = source["discTotal"];
	        this.coverMime = source["coverMime"];
	        this.coverSize = source["coverSize"];
	    }
	}
	export class TagPreview {
	    trackId: number;
	    path: string;
	    before: TagSnapshot;
	    after: TagSnapshot;
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new TagPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.trackId = source["trackId"];
	        this.path = source["path"];
	        this.before = this.convertValues(source["before"], TagSnapshot);
	        this.after = this.convertValues(source["after"], TagSnapshot);
	        this.warnings = source["warnings"];
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

