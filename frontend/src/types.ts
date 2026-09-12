export type Track = {
  id: number
  path: string
  fileName: string
  extension: string
  size: number
  title: string
  artist: string
  album: string
  albumArtist: string
  genre: string
  year: number
  trackNumber: number
  discNumber: number
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
}

export type ScanResult = {
  root: string
  found: number
  indexed: number
  failed: number
  duration: number
  errors: string[] | null
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

export type BPMKey = {
  bpm: number
  key: string
  scale: string
  strength: number
}

export type MetadataCandidate = {
  source: string
  externalId: string
  title: string
  artist: string
  album: string
  year: number
  genre: string
  artworkUrl: string
  durationMs: number
  confidence: number
}

export type MetadataLookupResult = {
  candidates: MetadataCandidate[] | null
  warnings: string[] | null
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
  essentiaPath: string
  ffmpegReady: boolean
  essentiaReady: boolean
  metadataProviders: string[]
}
