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
	export class BackgroundJob {
	    id: number;
	    type: string;
	    title: string;
	    status: string;
	    optionsJson: string;
	    createdAt: string;
	    startedAt: string;
	    finishedAt: string;
	    updatedAt: string;
	    totalItems: number;
	    completedItems: number;
	    skippedItems: number;
	    failedItems: number;
	    cancelledItems: number;
	    currentItem: string;
	    lastError: string;
	    progress: number;
	
	    static createFrom(source: any = {}) {
	        return new BackgroundJob(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.title = source["title"];
	        this.status = source["status"];
	        this.optionsJson = source["optionsJson"];
	        this.createdAt = source["createdAt"];
	        this.startedAt = source["startedAt"];
	        this.finishedAt = source["finishedAt"];
	        this.updatedAt = source["updatedAt"];
	        this.totalItems = source["totalItems"];
	        this.completedItems = source["completedItems"];
	        this.skippedItems = source["skippedItems"];
	        this.failedItems = source["failedItems"];
	        this.cancelledItems = source["cancelledItems"];
	        this.currentItem = source["currentItem"];
	        this.lastError = source["lastError"];
	        this.progress = source["progress"];
	    }
	}
	export class BackgroundJobItem {
	    id: number;
	    jobId: number;
	    trackId: number;
	    path: string;
	    status: string;
	    attempts: number;
	    error: string;
	    resultJson: string;
	    createdAt: string;
	    startedAt: string;
	    finishedAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new BackgroundJobItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.jobId = source["jobId"];
	        this.trackId = source["trackId"];
	        this.path = source["path"];
	        this.status = source["status"];
	        this.attempts = source["attempts"];
	        this.error = source["error"];
	        this.resultJson = source["resultJson"];
	        this.createdAt = source["createdAt"];
	        this.startedAt = source["startedAt"];
	        this.finishedAt = source["finishedAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class DuplicateActionResult {
	    action: string;
	    requested: number;
	    completed: number;
	    failed: number;
	    destination: string;
	    paths: string[];
	    errors: string[];
	
	    static createFrom(source: any = {}) {
	        return new DuplicateActionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.action = source["action"];
	        this.requested = source["requested"];
	        this.completed = source["completed"];
	        this.failed = source["failed"];
	        this.destination = source["destination"];
	        this.paths = source["paths"];
	        this.errors = source["errors"];
	    }
	}
	export class DuplicateAudioComparison {
	    trackId: number;
	    similarity: number;
	    offsetMs: number;
	    durationDeltaMs: number;
	    status: string;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new DuplicateAudioComparison(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.trackId = source["trackId"];
	        this.similarity = source["similarity"];
	        this.offsetMs = source["offsetMs"];
	        this.durationDeltaMs = source["durationDeltaMs"];
	        this.status = source["status"];
	        this.error = source["error"];
	    }
	}
	export class DuplicateAudioVerification {
	    groupKey: string;
	    referenceTrackId: number;
	    comparisons: DuplicateAudioComparison[];
	    sameCount: number;
	    similarCount: number;
	    differentCount: number;
	    errorCount: number;
	
	    static createFrom(source: any = {}) {
	        return new DuplicateAudioVerification(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.groupKey = source["groupKey"];
	        this.referenceTrackId = source["referenceTrackId"];
	        this.comparisons = this.convertValues(source["comparisons"], DuplicateAudioComparison);
	        this.sameCount = source["sameCount"];
	        this.similarCount = source["similarCount"];
	        this.differentCount = source["differentCount"];
	        this.errorCount = source["errorCount"];
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
	    label: string;
	    catalogNumber: string;
	    isrc: string;
	    releaseDate: string;
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
	    hasCover: boolean;
	    coverIndexed: boolean;
	    lastMetadataJobStatus: string;
	    lastMetadataJobUpdatedAt: string;
	
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
	        this.label = source["label"];
	        this.catalogNumber = source["catalogNumber"];
	        this.isrc = source["isrc"];
	        this.releaseDate = source["releaseDate"];
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
	        this.hasCover = source["hasCover"];
	        this.coverIndexed = source["coverIndexed"];
	        this.lastMetadataJobStatus = source["lastMetadataJobStatus"];
	        this.lastMetadataJobUpdatedAt = source["lastMetadataJobUpdatedAt"];
	    }
	}
	export class DuplicateTrackQuality {
	    trackId: number;
	    score: number;
	    audioScore: number;
	    metadataScore: number;
	    formatClass: string;
	    reasons: string[];
	
	    static createFrom(source: any = {}) {
	        return new DuplicateTrackQuality(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.trackId = source["trackId"];
	        this.score = source["score"];
	        this.audioScore = source["audioScore"];
	        this.metadataScore = source["metadataScore"];
	        this.formatClass = source["formatClass"];
	        this.reasons = source["reasons"];
	    }
	}
	export class DuplicateGroup {
	    key: string;
	    artist: string;
	    title: string;
	    durationMs: number;
	    durationSpreadMs: number;
	    matchClass: string;
	    confidence: number;
	    reasons: string[];
	    sharedIsrc: string;
	    recommendedTrackId: number;
	    quality: DuplicateTrackQuality[];
	    tracks: Track[];
	
	    static createFrom(source: any = {}) {
	        return new DuplicateGroup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.artist = source["artist"];
	        this.title = source["title"];
	        this.durationMs = source["durationMs"];
	        this.durationSpreadMs = source["durationSpreadMs"];
	        this.matchClass = source["matchClass"];
	        this.confidence = source["confidence"];
	        this.reasons = source["reasons"];
	        this.sharedIsrc = source["sharedIsrc"];
	        this.recommendedTrackId = source["recommendedTrackId"];
	        this.quality = this.convertValues(source["quality"], DuplicateTrackQuality);
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
	export class MetadataScore {
	    title: number;
	    artist: number;
	    album: number;
	    version: number;
	    duration: number;
	    identifier: number;
	    completeness: number;
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new MetadataScore(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.artist = source["artist"];
	        this.album = source["album"];
	        this.version = source["version"];
	        this.duration = source["duration"];
	        this.identifier = source["identifier"];
	        this.completeness = source["completeness"];
	        this.total = source["total"];
	    }
	}
	export class MetadataCandidate {
	    source: string;
	    sourceKind: string;
	    externalId: string;
	    sourceUrl: string;
	    title: string;
	    artist: string;
	    album: string;
	    albumArtist: string;
	    releaseDate: string;
	    year: number;
	    genre: string;
	    stage: string;
	    label: string;
	    catalogNumber: string;
	    isrc: string;
	    trackNumber: number;
	    trackTotal: number;
	    discNumber: number;
	    discTotal: number;
	    bpm: number;
	    key: string;
	    keyScale: string;
	    artworkUrl: string;
	    artworkWidth: number;
	    artworkHeight: number;
	    artworkEmbeddable: boolean;
	    durationMs: number;
	    confidence: number;
	    matchClass: string;
	    matchIssues: string[];
	    score: MetadataScore;
	
	    static createFrom(source: any = {}) {
	        return new MetadataCandidate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.sourceKind = source["sourceKind"];
	        this.externalId = source["externalId"];
	        this.sourceUrl = source["sourceUrl"];
	        this.title = source["title"];
	        this.artist = source["artist"];
	        this.album = source["album"];
	        this.albumArtist = source["albumArtist"];
	        this.releaseDate = source["releaseDate"];
	        this.year = source["year"];
	        this.genre = source["genre"];
	        this.stage = source["stage"];
	        this.label = source["label"];
	        this.catalogNumber = source["catalogNumber"];
	        this.isrc = source["isrc"];
	        this.trackNumber = source["trackNumber"];
	        this.trackTotal = source["trackTotal"];
	        this.discNumber = source["discNumber"];
	        this.discTotal = source["discTotal"];
	        this.bpm = source["bpm"];
	        this.key = source["key"];
	        this.keyScale = source["keyScale"];
	        this.artworkUrl = source["artworkUrl"];
	        this.artworkWidth = source["artworkWidth"];
	        this.artworkHeight = source["artworkHeight"];
	        this.artworkEmbeddable = source["artworkEmbeddable"];
	        this.durationMs = source["durationMs"];
	        this.confidence = source["confidence"];
	        this.matchClass = source["matchClass"];
	        this.matchIssues = source["matchIssues"];
	        this.score = this.convertValues(source["score"], MetadataScore);
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
	export class MetadataEnrichmentItem {
	    trackId: number;
	    path: string;
	    source: string;
	    confidence: number;
	    applied: boolean;
	    skipped: boolean;
	    warning: string;
	    searchMode: string;
	    searchDurationMs: number;
	    providersResponded: number;
	    providersSkipped: number;
	    earlyStopped: boolean;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new MetadataEnrichmentItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.trackId = source["trackId"];
	        this.path = source["path"];
	        this.source = source["source"];
	        this.confidence = source["confidence"];
	        this.applied = source["applied"];
	        this.skipped = source["skipped"];
	        this.warning = source["warning"];
	        this.searchMode = source["searchMode"];
	        this.searchDurationMs = source["searchDurationMs"];
	        this.providersResponded = source["providersResponded"];
	        this.providersSkipped = source["providersSkipped"];
	        this.earlyStopped = source["earlyStopped"];
	        this.error = source["error"];
	    }
	}
	export class MetadataEnrichmentOptions {
	    minimumConfidence: number;
	    includeArtwork: boolean;
	    onlyMissing: boolean;
	    searchMode: string;
	
	    static createFrom(source: any = {}) {
	        return new MetadataEnrichmentOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.minimumConfidence = source["minimumConfidence"];
	        this.includeArtwork = source["includeArtwork"];
	        this.onlyMissing = source["onlyMissing"];
	        this.searchMode = source["searchMode"];
	    }
	}
	export class MetadataEnrichmentResult {
	    processed: number;
	    applied: number;
	    skipped: number;
	    failed: number;
	    items: MetadataEnrichmentItem[];
	
	    static createFrom(source: any = {}) {
	        return new MetadataEnrichmentResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.processed = source["processed"];
	        this.applied = source["applied"];
	        this.skipped = source["skipped"];
	        this.failed = source["failed"];
	        this.items = this.convertValues(source["items"], MetadataEnrichmentItem);
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
	export class MetadataFieldOption {
	    field: string;
	    value: string;
	    number: number;
	    decimal: number;
	    source: string;
	    externalId: string;
	    confidence: number;
	    support: number;
	    sources: string[];
	    quality: number;
	
	    static createFrom(source: any = {}) {
	        return new MetadataFieldOption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.field = source["field"];
	        this.value = source["value"];
	        this.number = source["number"];
	        this.decimal = source["decimal"];
	        this.source = source["source"];
	        this.externalId = source["externalId"];
	        this.confidence = source["confidence"];
	        this.support = source["support"];
	        this.sources = source["sources"];
	        this.quality = source["quality"];
	    }
	}
	export class MetadataProviderReport {
	    name: string;
	    kind: string;
	    status: string;
	    candidates: number;
	    durationMs: number;
	    error: string;
	    retryable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MetadataProviderReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.status = source["status"];
	        this.candidates = source["candidates"];
	        this.durationMs = source["durationMs"];
	        this.error = source["error"];
	        this.retryable = source["retryable"];
	    }
	}
	export class MetadataLookupResult {
	    candidates: MetadataCandidate[];
	    suggested: MetadataCandidate;
	    fieldOptions: MetadataFieldOption[];
	    providerReports: MetadataProviderReport[];
	    warnings: string[];
	    cached: boolean;
	    cacheAgeSeconds: number;
	
	    static createFrom(source: any = {}) {
	        return new MetadataLookupResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.candidates = this.convertValues(source["candidates"], MetadataCandidate);
	        this.suggested = this.convertValues(source["suggested"], MetadataCandidate);
	        this.fieldOptions = this.convertValues(source["fieldOptions"], MetadataFieldOption);
	        this.providerReports = this.convertValues(source["providerReports"], MetadataProviderReport);
	        this.warnings = source["warnings"];
	        this.cached = source["cached"];
	        this.cacheAgeSeconds = source["cacheAgeSeconds"];
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
	
	
	export class MetadataSettings {
	    musicBrainzEnabled: boolean;
	    theAudioDBEnabled: boolean;
	    deezerEnabled: boolean;
	    iTunesEnabled: boolean;
	    discogsEnabled: boolean;
	    spotifyEnabled: boolean;
	    appleMusicEnabled: boolean;
	    youtubeEnabled: boolean;
	    soundCloudEnabled: boolean;
	    yandexMusicEnabled: boolean;
	    traxsourceEnabled: boolean;
	    muzvizorEnabled: boolean;
	    remixPoolEnabled: boolean;
	    bananaStreetEnabled: boolean;
	    mixcloudEnabled: boolean;
	    jesteiEnabled: boolean;
	    traxsourceApiKey: string;
	    metadataEnrichmentConcurrency: number;
	    theAudioDBApiKey: string;
	    iTunesCountry: string;
	    discogsToken: string;
	    spotifyAccessToken: string;
	    spotifyClientId: string;
	    spotifyClientSecret: string;
	    spotifyMarket: string;
	    appleMusicDeveloperToken: string;
	    appleMusicStorefront: string;
	    youTubeApiKey: string;
	    soundCloudAccessToken: string;
	    yandexMusicToken: string;
	    yandexMusicLanguage: string;
	
	    static createFrom(source: any = {}) {
	        return new MetadataSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.musicBrainzEnabled = source["musicBrainzEnabled"];
	        this.theAudioDBEnabled = source["theAudioDBEnabled"];
	        this.deezerEnabled = source["deezerEnabled"];
	        this.iTunesEnabled = source["iTunesEnabled"];
	        this.discogsEnabled = source["discogsEnabled"];
	        this.spotifyEnabled = source["spotifyEnabled"];
	        this.appleMusicEnabled = source["appleMusicEnabled"];
	        this.youtubeEnabled = source["youtubeEnabled"];
	        this.soundCloudEnabled = source["soundCloudEnabled"];
	        this.yandexMusicEnabled = source["yandexMusicEnabled"];
	        this.traxsourceEnabled = source["traxsourceEnabled"];
	        this.muzvizorEnabled = source["muzvizorEnabled"];
	        this.remixPoolEnabled = source["remixPoolEnabled"];
	        this.bananaStreetEnabled = source["bananaStreetEnabled"];
	        this.mixcloudEnabled = source["mixcloudEnabled"];
	        this.jesteiEnabled = source["jesteiEnabled"];
	        this.traxsourceApiKey = source["traxsourceApiKey"];
	        this.metadataEnrichmentConcurrency = source["metadataEnrichmentConcurrency"];
	        this.theAudioDBApiKey = source["theAudioDBApiKey"];
	        this.iTunesCountry = source["iTunesCountry"];
	        this.discogsToken = source["discogsToken"];
	        this.spotifyAccessToken = source["spotifyAccessToken"];
	        this.spotifyClientId = source["spotifyClientId"];
	        this.spotifyClientSecret = source["spotifyClientSecret"];
	        this.spotifyMarket = source["spotifyMarket"];
	        this.appleMusicDeveloperToken = source["appleMusicDeveloperToken"];
	        this.appleMusicStorefront = source["appleMusicStorefront"];
	        this.youTubeApiKey = source["youTubeApiKey"];
	        this.soundCloudAccessToken = source["soundCloudAccessToken"];
	        this.yandexMusicToken = source["yandexMusicToken"];
	        this.yandexMusicLanguage = source["yandexMusicLanguage"];
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
	export class SpectrogramComparison {
	    beforeUrl: string;
	    afterUrl: string;
	    beforePath: string;
	    afterPath: string;
	
	    static createFrom(source: any = {}) {
	        return new SpectrogramComparison(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.beforeUrl = source["beforeUrl"];
	        this.afterUrl = source["afterUrl"];
	        this.beforePath = source["beforePath"];
	        this.afterPath = source["afterPath"];
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
	    essentiaSource: string;
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
	        this.essentiaSource = source["essentiaSource"];
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
	    label: string;
	    catalogNumber: string;
	    isrc: string;
	    releaseDate: string;
	    year: number;
	    trackNumber: number;
	    trackTotal: number;
	    discNumber: number;
	    discTotal: number;
	    bpm: number;
	    key: string;
	    keyScale: string;
	
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
	        this.label = source["label"];
	        this.catalogNumber = source["catalogNumber"];
	        this.isrc = source["isrc"];
	        this.releaseDate = source["releaseDate"];
	        this.year = source["year"];
	        this.trackNumber = source["trackNumber"];
	        this.trackTotal = source["trackTotal"];
	        this.discNumber = source["discNumber"];
	        this.discTotal = source["discTotal"];
	        this.bpm = source["bpm"];
	        this.key = source["key"];
	        this.keyScale = source["keyScale"];
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
	    label: string;
	    catalogNumber: string;
	    isrc: string;
	    releaseDate: string;
	    year: number;
	    trackNumber: number;
	    trackTotal: number;
	    discNumber: number;
	    discTotal: number;
	    bpm: number;
	    key: string;
	    keyScale: string;
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
	        this.label = source["label"];
	        this.catalogNumber = source["catalogNumber"];
	        this.isrc = source["isrc"];
	        this.releaseDate = source["releaseDate"];
	        this.year = source["year"];
	        this.trackNumber = source["trackNumber"];
	        this.trackTotal = source["trackTotal"];
	        this.discNumber = source["discNumber"];
	        this.discTotal = source["discTotal"];
	        this.bpm = source["bpm"];
	        this.key = source["key"];
	        this.keyScale = source["keyScale"];
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
	
	export class TagTransformRequest {
	    operation: string;
	    fields: string[];
	    search: string;
	    replace: string;
	    prefix: string;
	    suffix: string;
	    sourceField: string;
	    targetField: string;
	    caseSensitive: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TagTransformRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.operation = source["operation"];
	        this.fields = source["fields"];
	        this.search = source["search"];
	        this.replace = source["replace"];
	        this.prefix = source["prefix"];
	        this.suffix = source["suffix"];
	        this.sourceField = source["sourceField"];
	        this.targetField = source["targetField"];
	        this.caseSensitive = source["caseSensitive"];
	    }
	}
	
	export class TrackMedia {
	    audioUrl: string;
	    coverUrl: string;
	    durationMs: number;
	    isPreview: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TrackMedia(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.audioUrl = source["audioUrl"];
	        this.coverUrl = source["coverUrl"];
	        this.durationMs = source["durationMs"];
	        this.isPreview = source["isPreview"];
	    }
	}

}

