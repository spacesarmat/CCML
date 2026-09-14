export type Track = {
  id: number
  path: string
  fileName: string
  extension: string
  size: number
  modifiedUnix: number
  title: string
  artist: string
  album: string
  albumArtist: string
  genre: string
  year: number
  trackNumber: number
  trackTotal: number
  discNumber: number
  discTotal: number
  composer: string
  comment: string
  label: string
  catalogNumber: string
  isrc: string
  releaseDate: string
  durationMs: number
  codec: string
  sampleRate: number
  channels: number
  bitRate: number
  bpm: number
  key: string
  keyScale: string
  loudnessI: number
  truePeak: number
  lra: number
  threshold: number
  scanError: string
  lastMetadataJobStatus: string
  lastMetadataJobUpdatedAt: string
}

export type ScanResult = {
  root: string
  found: number
  indexed: number
  added: number
  updated: number
  skipped: number
  removed: number
  failed: number
  cancelled: boolean
  duration: number
  errors: string[] | null
}

export type ScanProgress = {
  root: string
  currentFile: string
  found: number
  scanned: number
  added: number
  updated: number
  skipped: number
  removed: number
  failed: number
  finished: boolean
  cancelled: boolean
}

export type LibraryRoot = {
  path: string
  createdAt: string
  lastScanAt: string
}

export type LibraryStats = {
  tracks: number
  artists: number
  albums: number
  durationMs: number
  sizeBytes: number
  duplicateGroups: number
}

export type TagSnapshot = {
  title: string
  artist: string
  album: string
  albumArtist: string
  genre: string
  composer: string
  comment: string
  label: string
  catalogNumber: string
  isrc: string
  releaseDate: string
  year: number
  trackNumber: number
  trackTotal: number
  discNumber: number
  discTotal: number
  coverMime: string
  coverSize: number
}

export type TagPatch = {
  fields: string[]
  title: string
  artist: string
  album: string
  albumArtist: string
  genre: string
  composer: string
  comment: string
  label: string
  catalogNumber: string
  isrc: string
  releaseDate: string
  year: number
  trackNumber: number
  trackTotal: number
  discNumber: number
  discTotal: number
}

export type TagPreview = {
  trackId: number
  path: string
  before: TagSnapshot
  after: TagSnapshot
  warnings: string[] | null
}

export type TagApplyResult = {
  changeSetId: number
  changed: number
  failed: number
  errors: string[] | null
}

export type TagHistory = {
  id: number
  createdAt: string
  label: string
  status: string
  affectedCount: number
}

export type DuplicateGroup = {
  artist: string
  title: string
  durationMs: number
  tracks: Track[]
}

export type Loudness = {
  inputI: number
  inputTP: number
  inputLRA: number
  inputThreshold: number
  targetOffset: number
}

export type ProcessingOptions = {
  outputPath: string
  targetLUFS: number
  targetTruePeakDb: number
  targetLRA: number
  preGainDb: number
  repairClipping: boolean
  multibandCompress: boolean
  limit: boolean
  pitchSemitones: number
  keepOriginal: boolean
}

export type ProcessingResult = {
  inputPath: string
  outputPath: string
  measurement: Loudness
  filterGraph: string
}

export type TrackMedia = {
  audioUrl: string
  coverUrl: string
  durationMs: number
  isPreview: boolean
}

export type SpectrogramComparison = {
  beforeUrl: string
  afterUrl: string
  beforePath: string
  afterPath: string
}

export type BPMKey = {
  bpm: number
  key: string
  scale: string
  strength: number
}

export type MetadataScore = {
  title: number
  artist: number
  album: number
  version: number
  duration: number
  identifier: number
  completeness: number
  total: number
}

export type MetadataCandidate = {
  source: string
  externalId: string
  sourceUrl: string
  title: string
  artist: string
  album: string
  albumArtist: string
  releaseDate: string
  year: number
  genre: string
  label: string
  catalogNumber: string
  isrc: string
  trackNumber: number
  trackTotal: number
  discNumber: number
  discTotal: number
  artworkUrl: string
  artworkWidth: number
  artworkHeight: number
  artworkEmbeddable: boolean
  durationMs: number
  confidence: number
  matchClass: 'exact' | 'high' | 'medium' | 'low' | 'rejected' | string
  matchIssues: string[] | null
  score: MetadataScore
}

export type MetadataFieldOption = {
  field: string
  value: string
  number: number
  source: string
  externalId: string
  confidence: number
}

export type MetadataProviderReport = {
  name: string
  status: 'ok' | 'empty' | 'error' | string
  candidates: number
  durationMs: number
  error: string
  retryable: boolean
}

export type MetadataLookupResult = {
  candidates: MetadataCandidate[] | null
  suggested: MetadataCandidate
  fieldOptions: MetadataFieldOption[] | null
  providerReports: MetadataProviderReport[] | null
  warnings: string[] | null
  cached: boolean
  cacheAgeSeconds: number
}


export type MetadataSettings = {
  musicBrainzEnabled: boolean
  theAudioDBEnabled: boolean
  deezerEnabled: boolean
  iTunesEnabled: boolean
  discogsEnabled: boolean
  spotifyEnabled: boolean
  appleMusicEnabled: boolean
  youTubeEnabled: boolean
  soundCloudEnabled: boolean
  yandexMusicEnabled: boolean
  traxsourceEnabled: boolean
  traxsourceApiKey: string
  metadataEnrichmentConcurrency: number
  theAudioDBApiKey: string
  iTunesCountry: string
  discogsToken: string
  spotifyAccessToken: string
  spotifyClientId: string
  spotifyClientSecret: string
  spotifyMarket: string
  appleMusicDeveloperToken: string
  appleMusicStorefront: string
  youTubeApiKey: string
  soundCloudAccessToken: string
  yandexMusicToken: string
  yandexMusicLanguage: string
}

export type MetadataEnrichmentOptions = {
  minimumConfidence: number
  includeArtwork: boolean
  onlyMissing: boolean
}

export type MetadataEnrichmentItem = {
  trackId: number
  path: string
  source: string
  confidence: number
  applied: boolean
  skipped: boolean
  error: string
}

export type MetadataEnrichmentResult = {
  processed: number
  applied: number
  skipped: number
  failed: number
  items: MetadataEnrichmentItem[] | null
}

export type BackgroundJob = {
  id: number
  type: string
  title: string
  status: 'queued' | 'running' | 'paused' | 'completed' | 'failed' | 'cancelled' | string
  optionsJson: string
  createdAt: string
  startedAt: string
  finishedAt: string
  updatedAt: string
  totalItems: number
  completedItems: number
  skippedItems: number
  failedItems: number
  cancelledItems: number
  currentItem: string
  lastError: string
  progress: number
}

export type BackgroundJobItem = {
  id: number
  jobId: number
  trackId: number
  path: string
  status: 'queued' | 'running' | 'completed' | 'skipped' | 'failed' | 'cancelled' | string
  attempts: number
  error: string
  resultJson: string
  createdAt: string
  startedAt: string
  finishedAt: string
  updatedAt: string
}

export type OrganizeRequest = {
  rootDir: string
  template: string
  regexPattern: string
  regexReplace: string
  move: boolean
}

export type SystemStatus = {
  ffmpegPath: string
  ffprobePath: string
  ffmpegVersion: string
  ffmpegSource: string
  ffmpegReady: boolean
  ffmpegUpdating: boolean
  ffmpegUpdateError: string
  ffmpegAutoUpdateSupported: boolean
  essentiaPath: string
  essentiaReady: boolean
  metadataProviders: string[]
}

export type FFmpegUpdateResult = {
  version: string
  changed: boolean
  ffmpegPath: string
  ffprobePath: string
  source: string
}
