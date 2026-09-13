import type {
  BPMKey,
  BackgroundJob,
  BackgroundJobItem,
  DuplicateGroup,
  FFmpegUpdateResult,
  LibraryRoot,
  LibraryStats,
  Loudness,
  MetadataEnrichmentOptions,
  MetadataEnrichmentResult,
  MetadataLookupResult,
  MetadataSettings,
  OrganizeRequest,
  ProcessingOptions,
  ProcessingResult,
  SpectrogramComparison,
  TrackMedia,
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
          PrepareTrackMedia(trackID: number): Promise<TrackMedia>
          PrepareTrackAudioPreview(trackID: number): Promise<string>
          GenerateSpectrograms(trackID: number, processedPath: string): Promise<SpectrogramComparison>
          AnalyzeLoudness(trackID: number): Promise<Loudness>
          NormalizeTrack(trackID: number, opts: ProcessingOptions): Promise<ProcessingResult>
          WriteReplayGain(trackID: number, targetLUFS: number): Promise<Loudness>
          AnalyzeBPMKey(trackID: number): Promise<BPMKey>
          LookupMetadata(trackID: number): Promise<MetadataLookupResult>
          RefreshMetadata(trackID: number): Promise<MetadataLookupResult>
          TestMetadataProviders(settings: MetadataSettings): Promise<import('./types').MetadataProviderReport[]>
          GetMetadataSettings(): Promise<MetadataSettings>
          SaveMetadataSettings(settings: MetadataSettings): Promise<MetadataSettings>
          OpenMetadataLink(key: string): Promise<void>
          EnrichMetadata(trackIDs: number[], opts: MetadataEnrichmentOptions): Promise<MetadataEnrichmentResult>
          CreateMetadataEnrichmentJob(trackIDs: number[], opts: MetadataEnrichmentOptions): Promise<BackgroundJob>
          CreateLibraryMetadataEnrichmentJob(opts: MetadataEnrichmentOptions): Promise<BackgroundJob>
          ListBackgroundJobs(limit: number): Promise<BackgroundJob[]>
          ListBackgroundJobItems(jobID: number, limit: number, offset: number): Promise<BackgroundJobItem[]>
          PauseBackgroundJob(jobID: number): Promise<BackgroundJob>
          ResumeBackgroundJob(jobID: number): Promise<BackgroundJob>
          CancelBackgroundJob(jobID: number): Promise<BackgroundJob>
          RetryFailedBackgroundJob(jobID: number): Promise<BackgroundJob>
          PreviewRename(trackID: number, req: OrganizeRequest): Promise<string>
          OrganizeTrack(trackID: number, req: OrganizeRequest): Promise<string>
        }
      }
    }
  }
}

export {}
