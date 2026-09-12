import type {
  BPMKey,
  DuplicateGroup,
  FFmpegUpdateResult,
  LibraryRoot,
  LibraryStats,
  Loudness,
  MetadataEnrichmentOptions,
  MetadataEnrichmentResult,
  MetadataLookupResult,
  OrganizeRequest,
  ProcessingOptions,
  ProcessingResult,
  ScanResult,
  SystemStatus,
  TagApplyResult,
  TagHistory,
  TagPatch,
  TagPreview,
  TagSnapshot,
  Track,
} from './types'

declare global {
  interface Window {
    go: {
      main: {
        App: {
          SystemStatus(): Promise<SystemStatus>
          UpdateFFmpeg(): Promise<FFmpegUpdateResult>
          SelectMusicFolder(): Promise<string>
          ScanFolder(root: string): Promise<ScanResult>
          CancelScan(): Promise<boolean>
          ListLibraryRoots(): Promise<LibraryRoot[]>
          RemoveLibraryRoot(root: string, deleteTracks: boolean): Promise<void>
          LibraryStatistics(): Promise<LibraryStats>
          ListTracks(search: string, limit: number, offset: number): Promise<Track[]>
          FindDuplicates(): Promise<DuplicateGroup[]>
          ReadTrackTags(trackID: number): Promise<TagSnapshot>
          PreviewTagEdits(trackIDs: number[], patch: TagPatch): Promise<TagPreview[]>
          ApplyTagEdits(trackIDs: number[], patch: TagPatch): Promise<TagApplyResult>
          SelectCoverArt(): Promise<string>
          SetCoverArt(trackIDs: number[], imagePath: string): Promise<TagApplyResult>
          RemoveCoverArt(trackIDs: number[]): Promise<TagApplyResult>
          ApplyMetadataCandidate(trackID: number, candidate: import('./types').MetadataCandidate, includeArtwork: boolean): Promise<TagApplyResult>
          ListTagHistory(limit: number): Promise<TagHistory[]>
          UndoTagChange(changeSetID: number): Promise<TagApplyResult>
          AnalyzeLoudness(trackID: number): Promise<Loudness>
          NormalizeTrack(trackID: number, opts: ProcessingOptions): Promise<ProcessingResult>
          WriteReplayGain(trackID: number, targetLUFS: number): Promise<Loudness>
          AnalyzeBPMKey(trackID: number): Promise<BPMKey>
          LookupMetadata(trackID: number): Promise<MetadataLookupResult>
          EnrichMetadata(trackIDs: number[], opts: MetadataEnrichmentOptions): Promise<MetadataEnrichmentResult>
          PreviewRename(trackID: number, req: OrganizeRequest): Promise<string>
          OrganizeTrack(trackID: number, req: OrganizeRequest): Promise<string>
        }
      }
    }
  }
}

export {}
