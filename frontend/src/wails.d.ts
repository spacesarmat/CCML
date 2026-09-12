import type {
  BPMKey,
  DuplicateGroup,
  FFmpegUpdateResult,
  LibraryRoot,
  LibraryStats,
  Loudness,
  MetadataLookupResult,
  OrganizeRequest,
  ProcessingOptions,
  ProcessingResult,
  ScanResult,
  SystemStatus,
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
          AnalyzeLoudness(trackID: number): Promise<Loudness>
          NormalizeTrack(trackID: number, opts: ProcessingOptions): Promise<ProcessingResult>
          WriteReplayGain(trackID: number, targetLUFS: number): Promise<Loudness>
          AnalyzeBPMKey(trackID: number): Promise<BPMKey>
          LookupMetadata(trackID: number): Promise<MetadataLookupResult>
          PreviewRename(trackID: number, req: OrganizeRequest): Promise<string>
          OrganizeTrack(trackID: number, req: OrganizeRequest): Promise<string>
        }
      }
    }
  }
}

export {}
