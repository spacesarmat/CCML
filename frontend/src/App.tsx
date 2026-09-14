import { useEffect, useMemo, useRef, useState } from 'react'
import { EventsOn } from '../wailsjs/runtime/runtime'
import TagEditor from './TagEditor'
import MetadataMerge from './MetadataMerge'
import SettingsModal from './SettingsModal'
import JobsPanel from './JobsPanel'
import HelpModal from './HelpModal'
import DuplicateGroupsView from './DuplicateGroupsView'
import {
  detectInitialLanguage,
  localeFor,
  saveLanguage,
  translate,
  type AppLanguage,
  type TranslateParams,
  type TranslationKey,
} from './i18n'
import type {
  DuplicateAudioVerification,
  DuplicateGroup,
  LibraryRoot,
  LibraryStats,
  MetadataCandidate,
  MetadataEnrichmentOptions,
  MetadataLookupResult,
  OrganizeRequest,
  ProcessingOptions,
  ScanProgress,
  SpectrogramComparison,
  SystemStatus,
  Track,
  TrackMedia,
} from './types'

import ConfigurableTrackTable from './ConfigurableTrackTable'
import LibraryFilterPanel from './LibraryFilterPanel'
import {
  applyLibraryFilters,
  countActiveLibraryFilters,
  createEmptyLibraryFilters,
  loadLibraryFilters,
  saveLibraryFilters,
  type LibraryFilters,
} from './libraryFilters'
import {loadTableSort, saveTableSort, sortTracks, type TableSort} from './tableSort'
import {applyTheme, loadTheme, saveTheme, type AppTheme} from './theme'
import {applyUIScale, loadUIScale, saveUIScale, type AppUIScale} from './uiScale'

function compactProviderMessage(value: string | undefined): string {
  if (!value) return ''
  const htmlAt = value.search(/<!doctype\s+html|<html(?:\s|>)/i)
  const compact = htmlAt >= 0 ? value.slice(0, htmlAt).replace(/:\s*$/, '') : value
  return compact.length > 600 ? `${compact.slice(0, 600)}…` : compact
}

const defaultProcessing: ProcessingOptions = {
  outputPath: '',
  targetLUFS: -14,
  targetTruePeakDb: -1,
  targetLRA: 11,
  preGainDb: 0,
  repairClipping: false,
  multibandCompress: false,
  limit: true,
  pitchSemitones: 0,
  keepOriginal: true,
}

const defaultOrganize: OrganizeRequest = {
  rootDir: '',
  template: '%artist%/%album%/%track% - %title%',
  regexPattern: '',
  regexReplace: '',
  move: true,
}

function App() {
  const [language, setLanguage] = useState<AppLanguage>(() => detectInitialLanguage())
  const [theme, setTheme] = useState<AppTheme>(() => {
    const initial = loadTheme()
    applyTheme(initial)
    return initial
  })
  const [uiScale, setUIScale] = useState<AppUIScale>(() => {
    const initial = loadUIScale()
    applyUIScale(initial)
    return initial
  })
  const [status, setStatus] = useState<SystemStatus | null>(null)
  const [folder, setFolder] = useState('')
  const [search, setSearch] = useState('')
  const [searchLoading, setSearchLoading] = useState(false)
  const [tracks, setTracks] = useState<Track[]>([])
  const [selectedIDs, setSelectedIDs] = useState<number[]>([])
  const [activeTrackID, setActiveTrackID] = useState<number | null>(null)
  const [tagRevision, setTagRevision] = useState(0)
  const [message, setMessage] = useState(() => translate(detectInitialLanguage(), 'message.ready'))
  const [busy, setBusy] = useState(false)
  const [processing, setProcessing] = useState(defaultProcessing)
  const [organize, setOrganize] = useState(defaultOrganize)
  const [previewPath, setPreviewPath] = useState('')
  const [metadata, setMetadata] = useState<MetadataCandidate[]>([])
  const [metadataWarnings, setMetadataWarnings] = useState<string[]>([])
  const [metadataLookup, setMetadataLookup] = useState<MetadataLookupResult | null>(null)
  const [enrichment, setEnrichment] = useState<MetadataEnrichmentOptions>({minimumConfidence: 0.86, includeArtwork: true, onlyMissing: true, searchMode: 'same'})
  const [duplicates, setDuplicates] = useState<DuplicateGroup[]>([])
  const [roots, setRoots] = useState<LibraryRoot[]>([])
  const [stats, setStats] = useState<LibraryStats | null>(null)
  const [scanProgress, setScanProgress] = useState<ScanProgress | null>(null)
  const [scanning, setScanning] = useState(false)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [jobsOpen, setJobsOpen] = useState(false)
  const [helpOpen, setHelpOpen] = useState(false)
  const [metadataFilter, setMetadataFilter] = useState<'all' | 'skipped' | 'failed'>('all')
  const [libraryFilters, setLibraryFilters] = useState<LibraryFilters>(() => loadLibraryFilters())
  const [tableSort, setTableSort] = useState<TableSort[]>(() => loadTableSort())
  const [mainView, setMainView] = useState<'library' | 'duplicates'>('library')
  const [inspectorTab, setInspectorTab] = useState<'tags' | 'metadata' | 'analysis' | 'organize'>('tags')
  const [trackMedia, setTrackMedia] = useState<TrackMedia | null>(null)
  const [mediaLoading, setMediaLoading] = useState(false)
  const [mediaFallbackLoading, setMediaFallbackLoading] = useState(false)
  const [mediaError, setMediaError] = useState('')
  const [mediaPlaying, setMediaPlaying] = useState(false)
  const [mediaCurrentTime, setMediaCurrentTime] = useState(0)
  const [mediaDuration, setMediaDuration] = useState(0)
  const [mediaVolume, setMediaVolume] = useState(1)
  const [spectrograms, setSpectrograms] = useState<SpectrogramComparison | null>(null)
  const [spectrogramLoading, setSpectrogramLoading] = useState(false)
  const [spectrogramError, setSpectrogramError] = useState('')
  const [processedPath, setProcessedPath] = useState('')
  const mediaRequest = useRef(0)
  const mediaFallbackTried = useRef(false)
  const audioRef = useRef<HTMLAudioElement | null>(null)
  const searchInputRef = useRef<HTMLInputElement | null>(null)
  const trackLoadRequestRef = useRef(0)
  const liveSearchTimerRef = useRef<number | null>(null)
  const liveSearchReadyRef = useRef(false)
  const skipNextLiveSearchRef = useRef(false)
  const selectionAnchor = useRef<number | null>(null)
  const spectrogramRequest = useRef(0)

  const locale = localeFor(language)
  const t = (key: TranslationKey, params?: TranslateParams) => translate(language, key, params)

  const filteredTracks = useMemo(() => {
    const byColumns = applyLibraryFilters(tracks, libraryFilters)
    return metadataFilter === 'all'
      ? byColumns
      : byColumns.filter((track) => track.lastMetadataJobStatus === metadataFilter)
  }, [tracks, libraryFilters, metadataFilter])

  const activeLibraryFilterCount = useMemo(
    () => countActiveLibraryFilters(libraryFilters),
    [libraryFilters],
  )

  const visibleTracks = useMemo(
    () => sortTracks(filteredTracks, tableSort, locale),
    [filteredTracks, tableSort, locale],
  )

  const selectedIDSet = useMemo(
    () => new Set(selectedIDs),
    [selectedIDs],
  )

  const selectedTracks = useMemo(
    () => tracks.filter((track) => selectedIDSet.has(track.id)),
    [tracks, selectedIDSet],
  )

  const selected = selectedTracks.length === 1 ? selectedTracks[0] : null
  const unchangedAfterEnrichment = tracks.filter((track) => track.lastMetadataJobStatus === 'skipped').length
  const failedAfterEnrichment = tracks.filter((track) => track.lastMetadataJobStatus === 'failed').length

  function metadataRowClass(track: Track): string {
    const classes: string[] = []
    if (track.lastMetadataJobStatus === 'skipped') classes.push('metadata-unchanged')
    if (track.lastMetadataJobStatus === 'failed') classes.push('metadata-failed')
    if (selectedIDSet.has(track.id)) classes.push('selected')
    if (activeTrackID === track.id) classes.push('active-row')
    return classes.join(' ')
  }

  function metadataRowTitle(track: Track): string | undefined {
    if (track.lastMetadataJobStatus === 'skipped') return t('table.metadataUnchanged')
    if (track.lastMetadataJobStatus === 'failed') return t('table.metadataFailed')
    return undefined
  }

  function changeTableSort(next: TableSort[]) {
    setTableSort(next)
    saveTableSort(next)
  }

  function backend() {
    if (!window.go?.main?.App) {
      throw new Error(t('message.backendUnavailable'))
    }
    return window.go.main.App
  }

  useEffect(() => {
    document.documentElement.lang = language
    saveLanguage(language)
  }, [language])

  useEffect(() => {
    const onHelpKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'F1') return
      event.preventDefault()
      setHelpOpen(true)
    }
    window.addEventListener('keydown', onHelpKeyDown)
    return () => window.removeEventListener('keydown', onHelpKeyDown)
  }, [])

  useEffect(() => {
    if (!liveSearchReadyRef.current) {
      liveSearchReadyRef.current = true
      return
    }

    if (skipNextLiveSearchRef.current) {
      skipNextLiveSearchRef.current = false
      return
    }

    if (liveSearchTimerRef.current !== null) {
      window.clearTimeout(liveSearchTimerRef.current)
    }

    liveSearchTimerRef.current = window.setTimeout(() => {
      liveSearchTimerRef.current = null
      void refreshTracksLive(search)
    }, 280)

    return () => {
      if (liveSearchTimerRef.current !== null) {
        window.clearTimeout(liveSearchTimerRef.current)
        liveSearchTimerRef.current = null
      }
    }
  }, [search])

  useEffect(() => {
    if (activeTrackID === null || mainView !== 'library') return
    const frame = window.requestAnimationFrame(() => {
      const row = document.querySelector<HTMLTableRowElement>(`tr[data-track-id="${activeTrackID}"]`)
      row?.scrollIntoView({block: 'nearest'})
    })
    return () => window.cancelAnimationFrame(frame)
  }, [activeTrackID, mainView])

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const command = event.ctrlKey || event.metaKey
      const key = event.key
      const target = event.target as HTMLElement | null

      if (command && key.toLowerCase() === 'f') {
        if (settingsOpen || jobsOpen || helpOpen || mainView !== 'library') return
        event.preventDefault()
        searchInputRef.current?.focus()
        searchInputRef.current?.select()
        return
      }

      if (settingsOpen || jobsOpen || helpOpen || mainView !== 'library') return
      if (target?.closest('input, textarea, select, button, a, [contenteditable="true"], [role="slider"]')) return

      if (command && key.toLowerCase() === 'a') {
        event.preventDefault()
        selectAllVisibleTracks()
        return
      }

      switch (key) {
        case 'ArrowDown':
          event.preventDefault()
          moveActiveTrack(1, event.shiftKey, command)
          break
        case 'ArrowUp':
          event.preventDefault()
          moveActiveTrack(-1, event.shiftKey, command)
          break
        case 'PageDown':
          event.preventDefault()
          moveActiveTrack(10, event.shiftKey, command)
          break
        case 'PageUp':
          event.preventDefault()
          moveActiveTrack(-10, event.shiftKey, command)
          break
        case 'Home':
          event.preventDefault()
          moveActiveTrackTo(0, event.shiftKey, command)
          break
        case 'End':
          event.preventDefault()
          moveActiveTrackTo(visibleTracks.length - 1, event.shiftKey, command)
          break
        case 'Enter':
          if (activeTrackID !== null && visibleTracks.some((track) => track.id === activeTrackID)) {
            event.preventDefault()
            selectOnlyTrack(activeTrackID)
            setInspectorTab('tags')
          }
          break
        case ' ':
          if (selectedIDs.length === 1 && trackMedia?.audioUrl && !mediaLoading && !mediaFallbackLoading) {
            event.preventDefault()
            void toggleAudioPlayback()
          }
          break
        case 'Escape':
          if (selectedIDs.length > 0 || activeTrackID !== null) {
            event.preventDefault()
            clearTrackSelection()
          }
          break
      }
    }

    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [
    activeTrackID,
    visibleTracks,
    jobsOpen,
    helpOpen,
    mainView,
    mediaFallbackLoading,
    mediaLoading,
    selectedIDs,
    settingsOpen,
    trackMedia?.audioUrl,
  ])

  useEffect(() => {
    setMetadata([])
    setMetadataWarnings([])
    setMetadataLookup(null)
  }, [selected?.id])

  useEffect(() => {
    if (audioRef.current) {
      audioRef.current.pause()
      audioRef.current.currentTime = 0
    }
    mediaFallbackTried.current = false
    setTrackMedia(null)
    setMediaLoading(false)
    setMediaFallbackLoading(false)
    setMediaError('')
    setMediaPlaying(false)
    setMediaCurrentTime(0)
    setMediaDuration(0)
    setProcessedPath('')
    setSpectrograms(null)
    setSpectrogramError('')
  }, [selected?.id])

  useEffect(() => {
    const request = ++mediaRequest.current
    if (!selected) {
      setMediaLoading(false)
      return
    }
    setMediaLoading(true)
    setMediaError('')
    void backend().PrepareTrackMedia(selected.id)
      .then((result) => {
        if (mediaRequest.current === request) setTrackMedia(result)
      })
      .catch((error) => {
        if (mediaRequest.current === request) setMediaError(error instanceof Error ? error.message : String(error))
      })
      .finally(() => {
        if (mediaRequest.current === request) setMediaLoading(false)
      })
  }, [selected?.id, status?.ffmpegReady, tagRevision])

  useEffect(() => {
    setMediaPlaying(false)
    setMediaCurrentTime(0)
    setMediaDuration(trackMedia?.durationMs ? trackMedia.durationMs / 1000 : 0)
  }, [trackMedia?.audioUrl, trackMedia?.durationMs])

  useEffect(() => {
    if (audioRef.current) audioRef.current.volume = mediaVolume
  }, [mediaVolume, trackMedia?.audioUrl])

  useEffect(() => {
    if (inspectorTab !== 'analysis' || !selected) return
    void loadSpectrograms()
  }, [inspectorTab, selected?.id, processedPath, status?.ffmpegReady])

  useEffect(() => {
    const offProgress = EventsOn('library:scan:progress', (progress: ScanProgress) => {
      setScanProgress(progress)
      setScanning(!progress.finished)
    })
    const offStarted = EventsOn('library:scan:started', (progress: ScanProgress) => {
      setScanProgress(progress)
      setScanning(true)
    })
    const offFFmpegStarted = EventsOn('tools:ffmpeg:update-started', () => {
      setStatus((current) => current ? {...current, ffmpegUpdating: true, ffmpegUpdateError: ''} : current)
      setMessage(t('message.updatingBundledFFmpeg'))
    })
    const offFFmpegFinished = EventsOn('tools:ffmpeg:update-finished', () => {
      void refreshStatus()
    })
    const offFFmpegError = EventsOn('tools:ffmpeg:update-error', (error: string) => {
      setStatus((current) => current ? {...current, ffmpegUpdating: false, ffmpegUpdateError: error} : current)
      setMessage(t('message.ffmpegUpdateFailed', {error}))
    })
    const offJobsUpdated = EventsOn('jobs:updated', (job: {status?: string}) => {
      if (job?.status === 'completed' || job?.status === 'failed' || job?.status === 'cancelled') {
        void refreshTracks(search)
        void refreshStats()
      }
    })

    return () => {
      offProgress()
      offStarted()
      offFFmpegStarted()
      offFFmpegFinished()
      offFFmpegError()
      offJobsUpdated()
    }
  }, [language, search])

  useEffect(() => {
    void refreshStatus()
    void refreshTracks('')
    void refreshRoots()
    void refreshStats()
  }, [])

  function changeLanguage(nextLanguage: AppLanguage) {
    setLanguage(nextLanguage)
    setMessage(translate(nextLanguage, 'message.ready'))
  }

  function changeTheme(nextTheme: AppTheme) {
    setTheme(nextTheme)
    applyTheme(nextTheme)
    saveTheme(nextTheme)
  }

  function changeUIScale(nextScale: AppUIScale) {
    setUIScale(nextScale)
    applyUIScale(nextScale)
    saveUIScale(nextScale)
  }

  async function run<T>(label: string, work: () => Promise<T>): Promise<T | undefined> {
    setBusy(true)
    setMessage(label)
    try {
      const result = await work()
      return result
    } catch (error) {
      setMessage(error instanceof Error ? error.message : String(error))
      return undefined
    } finally {
      setBusy(false)
    }
  }

  async function refreshStatus() {
    const result = await run(t('message.checkingAudioTools'), () => backend().SystemStatus())
    if (result) {
      setStatus(result)
      if (result.ffmpegUpdating) {
        setMessage(t('message.updatingBundledFFmpeg'))
      } else if (result.ffmpegReady) {
        setMessage(t('message.ffmpegDetected', {
          version: result.ffmpegVersion || t('status.ffmpeg.ready'),
          source: result.ffmpegSource || 'detected',
        }))
      } else {
        setMessage(result.ffmpegUpdateError || t('message.ffmpegNotFound'))
      }
    }
  }

  async function updateFFmpeg() {
    const result = await run(t('message.checkingFFmpegUpdates'), () => backend().UpdateFFmpeg())
    if (result) {
      await refreshStatus()
      setMessage(result.changed
        ? t('message.ffmpegUpdated', {version: result.version})
        : t('message.ffmpegUpToDate', {version: result.version || t('status.ffmpeg.ready')}))
    }
  }

  async function loadTrackRows(query: string): Promise<Track[]> {
    const pageSize = 1000
    const allTracks: Track[] = []

    for (let offset = 0; ; offset += pageSize) {
      const page = await backend().ListTracks(query, pageSize, offset)
      const rows = page ?? []
      allTracks.push(...rows)
      if (rows.length < pageSize) break
    }

    return allTracks
  }

  function applyTrackRows(result: Track[]) {
    setTracks(result)
    const visibleIDs = new Set(result
      .filter((track) => metadataFilter === 'all' || track.lastMetadataJobStatus === metadataFilter)
      .map((track) => track.id))
    setSelectedIDs((current) => current.filter((id) => visibleIDs.has(id)))
    setActiveTrackID((current) => current !== null && visibleIDs.has(current) ? current : null)
    setMessage(t('message.tracksLoaded', {count: result.length}))
  }

  async function refreshTracks(query = search) {
    const requestID = ++trackLoadRequestRef.current
    const result = await run(t('message.loadingLibrary'), () => loadTrackRows(query))

    if (result && requestID === trackLoadRequestRef.current) {
      applyTrackRows(result)
    }
  }

  async function refreshTracksLive(query: string) {
    const requestID = ++trackLoadRequestRef.current
    setSearchLoading(true)

    try {
      const result = await loadTrackRows(query)
      if (requestID !== trackLoadRequestRef.current) return
      applyTrackRows(result)
    } catch (error) {
      if (requestID === trackLoadRequestRef.current) {
        setMessage(error instanceof Error ? error.message : String(error))
      }
    } finally {
      if (requestID === trackLoadRequestRef.current) {
        setSearchLoading(false)
      }
    }
  }

  function runSearchImmediately() {
    if (liveSearchTimerRef.current !== null) {
      window.clearTimeout(liveSearchTimerRef.current)
      liveSearchTimerRef.current = null
    }
    void refreshTracks(search)
  }

  function clearLibrarySearch() {
    if (liveSearchTimerRef.current !== null) {
      window.clearTimeout(liveSearchTimerRef.current)
      liveSearchTimerRef.current = null
    }

    skipNextLiveSearchRef.current = true
    setSearch('')
    void refreshTracks('')
    searchInputRef.current?.focus()
  }

  async function refreshRoots() {
    try {
      const result = await backend().ListLibraryRoots()
      setRoots(result ?? [])
    } catch (error) {
      setMessage(error instanceof Error ? error.message : String(error))
    }
  }

  async function refreshStats() {
    try {
      const result = await backend().LibraryStatistics()
      setStats(result)
    } catch (error) {
      setMessage(error instanceof Error ? error.message : String(error))
    }
  }

  async function chooseFolder() {
    const result = await run(t('message.chooseMusicFolder'), () => backend().SelectMusicFolder())
    if (result !== undefined && result !== '') {
      setFolder(result)
      setMessage(result)
    }
  }

  async function scanFolder() {
    if (!folder.trim()) {
      setMessage(t('message.chooseFolderFirst'))
      return
    }

    setBusy(true)
    setScanning(true)
    setScanProgress({
      root: folder, currentFile: '', found: 0, scanned: 0, added: 0, updated: 0,
      skipped: 0, removed: 0, failed: 0, finished: false, cancelled: false,
    })
    setMessage(t('message.scanningMusic'))
    try {
      const result = await backend().ScanFolder(folder)
      if (result.cancelled) {
        setMessage(t('message.scanCancelled', {count: result.found}))
      } else {
        setMessage(t('message.scanComplete', {
          added: result.added,
          updated: result.updated,
          skipped: result.skipped,
          removed: result.removed,
          failed: result.failed,
        }))
      }
      await refreshTracks(search)
      await Promise.all([refreshRoots(), refreshStats()])
    } catch (error) {
      setMessage(error instanceof Error ? error.message : String(error))
    } finally {
      setBusy(false)
      setScanning(false)
    }
  }

  async function cancelScan() {
    try {
      const cancelled = await backend().CancelScan()
      if (cancelled) setMessage(t('message.cancellingScan'))
    } catch (error) {
      setMessage(error instanceof Error ? error.message : String(error))
    }
  }

  async function useLibraryRoot(path: string) {
    setFolder(path)
    setMessage(path)
  }

  async function removeLibraryRoot(path: string) {
    const removeTracks = window.confirm(t('roots.removeConfirm'))
    try {
      await backend().RemoveLibraryRoot(path, removeTracks)
      if (folder === path) setFolder('')
      await Promise.all([refreshRoots(), refreshTracks(search), refreshStats()])
      setMessage(t('message.libraryFolderRemoved'))
    } catch (error) {
      setMessage(error instanceof Error ? error.message : String(error))
    }
  }

  function clearTrackSelection() {
    setSelectedIDs([])
    setActiveTrackID(null)
    selectionAnchor.current = null
  }

  function toggleTrackSelection(trackID: number) {
    setActiveTrackID(trackID)
    selectionAnchor.current = trackID
    setSelectedIDs((current) => current.includes(trackID)
      ? current.filter((id) => id !== trackID)
      : [...current, trackID])
  }

  function selectOnlyTrack(trackID: number) {
    setActiveTrackID(trackID)
    selectionAnchor.current = trackID
    setSelectedIDs([trackID])
  }

  function selectRangeTo(trackID: number) {
    const targetIndex = visibleTracks.findIndex((track) => track.id === trackID)
    if (targetIndex < 0) return

    const anchorID = selectionAnchor.current ?? activeTrackID ?? selectedIDs[0] ?? trackID
    let anchorIndex = visibleTracks.findIndex((track) => track.id === anchorID)
    if (anchorIndex < 0) anchorIndex = targetIndex

    const start = Math.min(anchorIndex, targetIndex)
    const end = Math.max(anchorIndex, targetIndex)
    setSelectedIDs(visibleTracks.slice(start, end + 1).map((track) => track.id))
    setActiveTrackID(trackID)
    if (selectionAnchor.current === null) selectionAnchor.current = anchorID
  }

  function handleTrackRowClick(trackID: number, extendSelection: boolean, toggleSelection: boolean) {
    if (extendSelection) {
      selectRangeTo(trackID)
      return
    }
    if (toggleSelection) {
      toggleTrackSelection(trackID)
      return
    }
    selectOnlyTrack(trackID)
  }

  function moveActiveTrack(delta: number, extendSelection: boolean, preserveSelection: boolean) {
    if (visibleTracks.length === 0) return
    let index = visibleTracks.findIndex((track) => track.id === activeTrackID)
    if (index < 0) index = visibleTracks.findIndex((track) => selectedIDSet.has(track.id))
    if (index < 0) index = delta > 0 ? -1 : 0
    moveActiveTrackTo(index + delta, extendSelection, preserveSelection)
  }

  function moveActiveTrackTo(index: number, extendSelection: boolean, preserveSelection: boolean) {
    if (visibleTracks.length === 0) return
    const clamped = Math.max(0, Math.min(index, visibleTracks.length - 1))
    const trackID = visibleTracks[clamped].id

    if (extendSelection) {
      selectRangeTo(trackID)
      return
    }

    setActiveTrackID(trackID)
    if (!preserveSelection) {
      selectionAnchor.current = trackID
      setSelectedIDs([trackID])
    }
  }

  function selectAllVisibleTracks() {
    if (visibleTracks.length === 0) return
    const ids = visibleTracks.map((track) => track.id)
    setSelectedIDs(ids)
    const active = activeTrackID !== null && ids.includes(activeTrackID) ? activeTrackID : ids[0]
    setActiveTrackID(active)
    selectionAnchor.current = active
  }

  function changeMetadataFilter(next: 'all' | 'skipped' | 'failed') {
    setMainView('library')
    setMetadataFilter(next)
    clearTrackSelection()
  }

  function changeLibraryFilters(next: LibraryFilters) {
    setMainView('library')
    setLibraryFilters(next)
    saveLibraryFilters(next)
    clearTrackSelection()
  }

  function clearLibraryFilters() {
    const empty = createEmptyLibraryFilters()
    setMainView('library')
    setLibraryFilters(empty)
    saveLibraryFilters(empty)
    clearTrackSelection()
  }

  async function revealJobTrack(trackID: number) {
    setJobsOpen(false)
    setMainView('library')
    setMetadataFilter('all')
    if (activeLibraryFilterCount > 0) {
      const empty = createEmptyLibraryFilters()
      setLibraryFilters(empty)
      saveLibraryFilters(empty)
    }

    // A search query can hide the requested track even though it exists in the
    // library. Clear it and reload the complete library before selecting.
    if (search.trim() !== '' || !tracks.some((track) => track.id === trackID)) {
      setSearch('')
      await refreshTracks('')
    }

    selectionAnchor.current = trackID
    setSelectedIDs([trackID])
    setActiveTrackID(trackID)
    setMessage(t('jobs.trackRevealed'))
  }

  function toggleAllVisible() {
    if (visibleTracks.length > 0 && visibleTracks.every((track) => selectedIDSet.has(track.id))) {
      clearTrackSelection()
      return
    }
    selectAllVisibleTracks()
  }

  async function tagDataChanged() {
    setTagRevision((value) => value + 1)
    await Promise.all([refreshTracks(search), refreshStats()])
  }

  async function applyMetadataCandidate(item: MetadataCandidate, includeArtwork: boolean) {
    if (!selected) return
    const result = await run(t('message.applyingMetadata'), () => backend().ApplyMetadataCandidate(selected.id, item, includeArtwork))
    if (result) {
      await tagDataChanged()
      setMessage(t('message.metadataApplied', {changed: result.changed, failed: result.failed}))
    }
  }


  async function toggleAudioPlayback() {
    const audio = audioRef.current
    if (!audio || mediaLoading || mediaFallbackLoading) return
    if (!audio.paused) {
      audio.pause()
      return
    }

    setMediaError('')
    try {
      await audio.play()
    } catch {
      await handleAudioPlaybackError()
    }
  }

  function updateAudioMetadata() {
    const audio = audioRef.current
    if (!audio) return
    const duration = Number.isFinite(audio.duration) && audio.duration > 0
      ? audio.duration
      : (trackMedia?.durationMs || 0) / 1000
    setMediaDuration(duration)
    setMediaCurrentTime(Number.isFinite(audio.currentTime) ? audio.currentTime : 0)
    audio.volume = mediaVolume
  }

  function seekAudio(value: number) {
    const audio = audioRef.current
    if (!audio || !Number.isFinite(value)) return
    const nativeDuration = Number.isFinite(audio.duration) ? audio.duration : 0
    const duration = mediaDuration > 0 ? mediaDuration : Math.max(0, nativeDuration)
    const next = Math.max(0, Math.min(value, duration || value))
    audio.currentTime = next
    setMediaCurrentTime(next)
  }

  function changeAudioVolume(value: number) {
    const next = Math.max(0, Math.min(1, value))
    setMediaVolume(next)
    if (audioRef.current) audioRef.current.volume = next
  }

  async function handleAudioPlaybackError() {
    setMediaPlaying(false)
    if (!selected || !trackMedia || mediaFallbackLoading) return
    if (trackMedia.isPreview || mediaFallbackTried.current) {
      setMediaError(t('media.playbackFailed'))
      return
    }

    mediaFallbackTried.current = true
    const request = mediaRequest.current
    setMediaFallbackLoading(true)
    setMediaError('')
    try {
      const url = await backend().PrepareTrackAudioPreview(selected.id)
      if (mediaRequest.current !== request) return
      setTrackMedia((current) => current ? {...current, audioUrl: url, isPreview: true} : current)
    } catch (error) {
      if (mediaRequest.current === request) {
        setMediaError(error instanceof Error ? error.message : String(error))
      }
    } finally {
      if (mediaRequest.current === request) setMediaFallbackLoading(false)
    }
  }

  async function loadSpectrograms() {
    if (!selected || !status?.ffmpegReady) return
    const request = ++spectrogramRequest.current
    setSpectrogramLoading(true)
    setSpectrogramError('')
    try {
      const result = await backend().GenerateSpectrograms(selected.id, processedPath)
      if (spectrogramRequest.current === request) setSpectrograms(result)
    } catch (error) {
      if (spectrogramRequest.current === request) {
        setSpectrogramError(error instanceof Error ? error.message : String(error))
      }
    } finally {
      if (spectrogramRequest.current === request) setSpectrogramLoading(false)
    }
  }

  async function analyzeLoudness() {
    if (!selected) return
    const result = await run(t('message.analyzingLoudness'), () => backend().AnalyzeLoudness(selected.id))
    if (result) {
      setMessage(t('message.loudnessResult', {
        lufs: result.inputI.toFixed(1),
        peak: result.inputTP.toFixed(1),
      }))
      await refreshTracks(search)
    }
  }

  async function writeReplayGain() {
    if (!selected) return
    const result = await run(t('message.writingReplayGain'), () => backend().WriteReplayGain(selected.id, processing.targetLUFS))
    if (result) {
      setMessage(t('message.replayGainWritten', {lufs: result.inputI.toFixed(1)}))
      await refreshTracks(search)
    }
  }

  async function normalize() {
    if (!selected) return
    const result = await run(t('message.renderingProcessed'), () => backend().NormalizeTrack(selected.id, processing))
    if (result) {
      setProcessedPath(result.outputPath)
      setMessage(t('message.processed', {path: result.outputPath}))
    }
  }

  async function analyzeBPMKey() {
    if (!selected) return
    const result = await run(t('message.analyzingBpmKey'), () => backend().AnalyzeBPMKey(selected.id))
    if (result) {
      setMessage(t('message.bpmKeyResult', {
        bpm: result.bpm.toFixed(1),
        key: result.key,
        scale: result.scale,
      }))
      await refreshTracks(search)
    }
  }

  function showMetadataResult(result: MetadataLookupResult) {
    setMetadataLookup(result)
    setMetadata(result.candidates ?? [])
    setMetadataWarnings(result.warnings ?? [])
    setMessage(t('message.metadataCandidatesFound', {count: result.candidates?.length ?? 0}))
  }

  async function lookupMetadata() {
    if (!selected) return
    const result = await run(t('message.searchingMetadata'), () => backend().LookupMetadata(selected.id))
    if (result) showMetadataResult(result)
  }

  async function refreshMetadata() {
    if (!selected) return
    const result = await run(t('message.searchingMetadata'), () => backend().RefreshMetadata(selected.id))
    if (result) showMetadataResult(result)
  }

  async function enrichSelected() {
    if (selectedIDs.length === 0) return
    if (selectedIDs.length > 1 && !window.confirm(t('metadata.enrichConfirm', {count: selectedIDs.length}))) return
    const result = await run(t('message.enrichingMetadata'), () => backend().CreateMetadataEnrichmentJob(selectedIDs, enrichment))
    if (result) {
      setMessage(t('jobs.queued', {count: selectedIDs.length}))
      setJobsOpen(true)
    }
  }

  async function enrichLibrary() {
    const count = stats?.tracks ?? 0
    if (count <= 0) return
    if (!window.confirm(t('metadata.enrichLibraryConfirm', {count}))) return
    const result = await run(t('message.enrichingMetadata'), () => backend().CreateLibraryMetadataEnrichmentJob(enrichment))
    if (result) {
      setMessage(t('jobs.libraryQueued', {count}))
      setJobsOpen(true)
    }
  }

  async function previewRename() {
    if (!selected) return
    const result = await run(t('message.buildingTargetPath'), () => backend().PreviewRename(selected.id, organize))
    if (result !== undefined) {
      setPreviewPath(result)
      setMessage(t('message.renamePreviewUpdated'))
    }
  }

  async function applyOrganize() {
    if (!selected) return
    const result = await run(t('message.movingTrack'), () => backend().OrganizeTrack(selected.id, organize))
    if (result) {
      setPreviewPath(result)
      setMessage(t('message.moved', {path: result}))
      await refreshTracks(search)
    }
  }

  async function findDuplicates() {
    const result = await run(t('message.searchingDuplicates'), () => backend().FindDuplicates())
    if (result) {
      clearTrackSelection()
      setDuplicates(result)
      setMainView('duplicates')
      setMessage(t('message.duplicateGroupsFound', {count: result.length}))
    }
  }

  async function reloadAfterDuplicateAction() {
    const [rows, nextDuplicates, nextStats] = await Promise.all([
      loadTrackRows(search),
      backend().FindDuplicates(),
      backend().LibraryStatistics(),
    ])
    return {rows, nextDuplicates, nextStats}
  }

  async function verifyDuplicateAudio(trackIDs: number[]): Promise<DuplicateAudioVerification | null> {
    const result = await run(
      language === 'ru' ? 'Сравнение декодированного аудио…' : 'Comparing decoded audio…',
      () => backend().VerifyDuplicateAudio(trackIDs),
    )
    if (!result) return null

    setMessage(language === 'ru'
      ? `Проверка аудио: совпадает ${result.sameCount}, похоже ${result.similarCount}, отличается ${result.differentCount}, ошибок ${result.errorCount}`
      : `Audio check: same ${result.sameCount}, similar ${result.similarCount}, different ${result.differentCount}, errors ${result.errorCount}`)
    return result
  }

  async function quarantineDuplicateTracks(trackIDs: number[]): Promise<boolean> {
    let destination = ''
    try {
      destination = await backend().SelectDuplicateQuarantineFolder()
    } catch (error) {
      setMessage(error instanceof Error ? error.message : String(error))
      return false
    }
    if (!destination) return false

    const result = await run(
      language === 'ru' ? 'Перемещение дублей в карантин…' : 'Moving duplicates to quarantine…',
      async () => {
        const action = await backend().QuarantineDuplicateTracks(trackIDs, destination)
        const refreshed = await reloadAfterDuplicateAction()
        return {action, ...refreshed}
      },
    )
    if (!result) return false

    applyTrackRows(result.rows)
    setDuplicates(result.nextDuplicates)
    setStats(result.nextStats)
    setMessage(language === 'ru'
      ? `Карантин: перемещено ${result.action.completed}. Папка: ${result.action.destination}`
      : `Quarantine: moved ${result.action.completed}. Folder: ${result.action.destination}`)
    return true
  }

  async function deleteDuplicateTracks(trackIDs: number[]): Promise<boolean> {
    const result = await run(
      language === 'ru' ? 'Удаление выбранных дублей…' : 'Deleting selected duplicates…',
      async () => {
        const action = await backend().DeleteDuplicateTracks(trackIDs, 'DELETE')
        const refreshed = await reloadAfterDuplicateAction()
        return {action, ...refreshed}
      },
    )
    if (!result) return false

    applyTrackRows(result.rows)
    setDuplicates(result.nextDuplicates)
    setStats(result.nextStats)

    if (result.action.failed > 0) {
      setMessage(language === 'ru'
        ? `Удалено: ${result.action.completed}. Не удалось физически удалить: ${result.action.failed}. Временные файлы: ${result.action.paths.join(', ')}`
        : `Deleted: ${result.action.completed}. Physical cleanup failed: ${result.action.failed}. Temporary files: ${result.action.paths.join(', ')}`)
    } else {
      setMessage(language === 'ru'
        ? `Удалено дублей: ${result.action.completed}`
        : `Duplicates deleted: ${result.action.completed}`)
    }
    return true
  }

  async function openMetadataInspector() {
    setInspectorTab('metadata')
    if (selected && metadataLookup === null) await lookupMetadata()
  }

  return (
    <div className="app-shell workspace-app">
      <header className="workspace-topbar">
        <div className="workspace-brand">
          <div className="workspace-logo">C</div>
          <div>
            <h1>CCML</h1>
            <p>{t('app.subtitle')}</p>
          </div>
        </div>
        <div className="workspace-top-actions">
          <div className="workspace-health" aria-label={t('workspace.systemStatus')}>
            <span className={status?.ffmpegReady ? 'health-dot ok' : 'health-dot bad'} title={`FFmpeg · ${status?.ffmpegVersion || t('status.ffmpeg.missing')}`} />
            <span className={status?.essentiaReady ? 'health-dot ok' : 'health-dot warn'} title={`Essentia · ${status?.essentiaReady ? t('status.essentia.ready') : t('status.essentia.optional')}`} />
            <span className="health-label">{status?.metadataProviders?.length ?? 0} {t('status.metadata')}</span>
          </div>
          <button className="top-action" onClick={() => setJobsOpen(true)}>{t('jobs.button')}</button>
          <button className="top-action" onClick={() => setSettingsOpen(true)} disabled={busy || scanning}>{t('settings.button')}</button>
          <button className="top-action" onClick={() => setHelpOpen(true)} title={language === 'ru' ? 'Справка (F1)' : 'Help (F1)'}>{language === 'ru' ? 'Справка' : 'Help'}</button>
          <div className="language-switcher compact" aria-label={t('language.label')}>
            <button type="button" className={language === 'ru' ? 'active' : ''} onClick={() => changeLanguage('ru')} aria-pressed={language === 'ru'}>RU</button>
            <button type="button" className={language === 'en' ? 'active' : ''} onClick={() => changeLanguage('en')} aria-pressed={language === 'en'}>EN</button>
          </div>
        </div>
      </header>

      {status?.ffmpegUpdateError && status.ffmpegReady && (
        <div className="workspace-banner warning-banner">
          <strong>{t('warning.ffmpegUpdateFailed')}</strong>
          <span>{status.ffmpegUpdateError}</span>
        </div>
      )}

      <div className={`workspace-grid${mainView === 'duplicates' ? ' duplicates-layout' : ''}`}>
        <aside className="workspace-sidebar">
          <nav className="sidebar-section">
            <div className="sidebar-heading">{t('workspace.library')}</div>
            <button className={`sidebar-nav ${mainView === 'library' && metadataFilter === 'all' ? 'active' : ''}`} onClick={() => { setMainView('library'); changeMetadataFilter('all') }}>
              <span>{t('workspace.allTracks')}</span><b>{formatNumber(tracks.length, locale)}</b>
            </button>
            <button className={`sidebar-nav status-warn ${mainView === 'library' && metadataFilter === 'skipped' ? 'active' : ''}`} onClick={() => changeMetadataFilter('skipped')} disabled={unchangedAfterEnrichment === 0}>
              <span>{t('library.filterUnchanged')}</span><b>{formatNumber(unchangedAfterEnrichment, locale)}</b>
            </button>
            <button className={`sidebar-nav status-bad ${mainView === 'library' && metadataFilter === 'failed' ? 'active' : ''}`} onClick={() => changeMetadataFilter('failed')} disabled={failedAfterEnrichment === 0}>
              <span>{t('library.filterFailed')}</span><b>{formatNumber(failedAfterEnrichment, locale)}</b>
            </button>
          </nav>

          <section className="sidebar-section">
            <div className="sidebar-heading row-between"><span>{t('workspace.folders')}</span><button className="icon-button" onClick={chooseFolder} disabled={scanning} title={t('toolbar.chooseFolder')}>＋</button></div>
            <div className="sidebar-folder-list">
              {roots.map((root) => (
                <div className={`sidebar-folder ${folder === root.path ? 'active' : ''}`} key={root.path}>
                  <button className="sidebar-folder-path" onClick={() => void useLibraryRoot(root.path)} title={root.path}>{root.path}</button>
                  <button className="sidebar-folder-remove" onClick={() => void removeLibraryRoot(root.path)} disabled={scanning} title={t('roots.remove')}>×</button>
                </div>
              ))}
              {roots.length === 0 && <span className="sidebar-empty">{t('workspace.noFolders')}</span>}
            </div>
            <div className="sidebar-scan">
              <div className="sidebar-scan-path" title={folder}>{folder || t('toolbar.folderPlaceholder')}</div>
              <div className="sidebar-scan-actions">
                <button className="primary" onClick={scanFolder} disabled={scanning || status?.ffmpegUpdating || !status?.ffmpegReady}>{scanning ? t('scan.scanning') : t('toolbar.scan')}</button>
                {scanning && <button onClick={cancelScan}>{t('toolbar.cancel')}</button>}
              </div>
              {scanProgress && (scanning || scanProgress.finished) && (
                <div className="sidebar-scan-progress">
                  <strong>{formatNumber(scanProgress.scanned, locale)}</strong>
                  <span>{t('scan.added')} {scanProgress.added} · {t('scan.failed')} {scanProgress.failed}</span>
                </div>
              )}
            </div>
          </section>

          <nav className="sidebar-section">
            <div className="sidebar-heading">{t('workspace.tools')}</div>
            <button className={`sidebar-nav ${mainView === 'duplicates' ? 'active' : ''}`} onClick={() => void findDuplicates()} disabled={busy}>
              <span>{t('library.duplicates')}</span><b>{stats?.duplicateGroups ?? 0}</b>
            </button>
            <button className="sidebar-nav" onClick={() => void enrichLibrary()} disabled={busy || scanning || (stats?.tracks ?? 0) === 0}>
              <span>{t('metadata.enrichLibrary')}</span><b>↗</b>
            </button>
          </nav>

          {stats && (
            <section className="sidebar-section sidebar-stats">
              <div className="sidebar-heading">{t('workspace.summary')}</div>
              <div><span>{t('stats.tracks')}</span><strong>{formatNumber(stats.tracks, locale)}</strong></div>
              <div><span>{t('stats.artists')}</span><strong>{formatNumber(stats.artists, locale)}</strong></div>
              <div><span>{t('stats.albums')}</span><strong>{formatNumber(stats.albums, locale)}</strong></div>
              <div><span>{t('stats.duration')}</span><strong>{formatLongDuration(stats.durationMs, language)}</strong></div>
            </section>
          )}

          {status?.ffmpegAutoUpdateSupported && (
            <button className="sidebar-update" onClick={updateFFmpeg} disabled={scanning || busy || status.ffmpegUpdating}>
              {status.ffmpegUpdating ? t('toolbar.updatingFFmpeg') : t('toolbar.updateFFmpeg')}
            </button>
          )}
        </aside>

        <main className={`workspace-main${mainView === 'duplicates' ? ' duplicates-main' : ''}`}>
          {mainView === 'library' ? (
            <>
              <div className="workspace-commandbar">
                <div className="workspace-title-block">
                  <h2>{t('library.title')}</h2>
                  <span>{metadataFilter === 'all' && activeLibraryFilterCount === 0
                    ? t('library.shown', {count: formatNumber(tracks.length, locale)})
                    : t('library.shownFiltered', {count: formatNumber(filteredTracks.length, locale), total: formatNumber(tracks.length, locale)})}</span>
                </div>
                <div className={`workspace-search${searchLoading ? ' is-searching' : ''}`}>
                  <input
                    ref={searchInputRef}
                    value={search}
                    onChange={(event) => setSearch(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key !== 'Enter') return
                      event.preventDefault()
                      runSearchImmediately()
                    }}
                    placeholder={t('library.searchPlaceholder')}
                    aria-label={t('library.search')}
                  />
                  {search.length > 0 && (
                    <button
                      type="button"
                      className="search-clear"
                      onClick={clearLibrarySearch}
                      title={t('library.clearSearch')}
                    >
                      {t('library.clearSearch')}
                    </button>
                  )}
                  <button
                    type="button"
                    className="search-submit"
                    onClick={runSearchImmediately}
                    disabled={busy || searchLoading}
                  >
                    {searchLoading ? t('library.searching') : t('library.search')}
                  </button>
                </div>
              </div>

              <div className="library-filterbar">
                <div className="metadata-filter" role="group" aria-label={t('library.enrichmentFilterLabel')}>
                  <button className={metadataFilter === 'all' ? 'active' : ''} onClick={() => changeMetadataFilter('all')}>{t('library.filterAll')} · {formatNumber(tracks.length, locale)}</button>
                  <button className={`unchanged ${metadataFilter === 'skipped' ? 'active' : ''}`} onClick={() => changeMetadataFilter('skipped')} disabled={unchangedAfterEnrichment === 0}>{t('library.filterUnchanged')} · {formatNumber(unchangedAfterEnrichment, locale)}</button>
                  <button className={`failed ${metadataFilter === 'failed' ? 'active' : ''}`} onClick={() => changeMetadataFilter('failed')} disabled={failedAfterEnrichment === 0}>{t('library.filterFailed')} · {formatNumber(failedAfterEnrichment, locale)}</button>
                </div>
                <LibraryFilterPanel
                  language={language}
                  locale={locale}
                  tracks={tracks}
                  filters={libraryFilters}
                  onChange={changeLibraryFilters}
                  onClear={clearLibraryFilters}
                />
                <span className="filter-legend">
                  {unchangedAfterEnrichment > 0 && <i className="legend-dot warn" title={t('library.enrichmentLegendUnchanged')} />}
                  {failedAfterEnrichment > 0 && <i className="legend-dot bad" title={t('library.enrichmentLegendFailed')} />}
                </span>
              </div>

              {selectedIDs.length > 0 && (
                <div className="selection-toolbar">
                  <strong>{t('workspace.selectedCount', {count: selectedIDs.length})}</strong>
                  {selectedIDs.length === 1 && <button onClick={() => setInspectorTab('tags')}>{t('workspace.editTags')}</button>}
                  <button onClick={() => void openMetadataInspector()} disabled={selectedIDs.length !== 1 || busy}>{t('actions.findMetadata')}</button>
                  <button className="primary" onClick={() => void enrichSelected()} disabled={busy || scanning}>{t('metadata.enrichSelected')}</button>
                  <button onClick={clearTrackSelection}>{t('workspace.clearSelection')}</button>
                </div>
              )}

              <ConfigurableTrackTable
                language={language}
                tracks={visibleTracks}
                coverRevision={tagRevision}
                selectedIDs={selectedIDs}
                activeTrackID={activeTrackID}
                selectedTracks={selectedTracks}
                batchToolsRevision={tagRevision}
                batchToolsDisabled={busy || scanning}
                onBatchBusyChange={setBusy}
                onBatchMessage={setMessage}
                onBatchChanged={tagDataChanged}
                sort={tableSort}
                onSortChange={changeTableSort}
                onToggleAllVisible={toggleAllVisible}
                onRowClick={handleTrackRowClick}
                onToggleTrackSelection={toggleTrackSelection}
                rowClass={metadataRowClass}
                rowTitle={metadataRowTitle}
                selectAllLabel={t('library.selectAll')}
                selectTrackLabel={(track) => t('library.selectTrack', {title: track.title || track.fileName})}
                unknownArtistLabel={t('library.unknownArtist')}
                emptyLabel={t('library.filterEmpty')}
              />
            </>
          ) : (
            <DuplicateGroupsView
              language={language}
              groups={duplicates}
              busy={busy}
              onBack={() => setMainView('library')}
              onRefresh={() => void findDuplicates()}
              onOpenTrack={(track) => {
                setMainView('library')
                selectOnlyTrack(track.id)
                setInspectorTab('tags')
              }}
              canVerifyAudio={Boolean(status?.ffmpegReady)}
              onVerifyAudio={verifyDuplicateAudio}
              onQuarantine={quarantineDuplicateTracks}
              onDelete={deleteDuplicateTracks}
            />
          )}
        </main>

        {mainView === 'library' && (
        <aside className="workspace-inspector">
          <div className="inspector-head">
            <div className="inspector-summary">
              <div className="inspector-artwork" aria-hidden={!trackMedia?.coverUrl}>
                {selected && trackMedia?.coverUrl
                  ? <img src={trackMedia.coverUrl} alt={t('media.coverAlt')} />
                  : <span>♪</span>}
              </div>
              <div className="inspector-summary-text">
                <span className="inspector-kicker">{t('workspace.inspector')}</span>
                <h2 title={selected?.title || undefined}>{selected?.title || (selectedTracks.length > 1 ? t('details.multipleTracks', {count: selectedTracks.length}) : t('details.selectTrack'))}</h2>
                <p title={selected?.artist || undefined}>{selected?.artist || (selectedTracks.length > 1 ? t('details.batchEditing') : t('workspace.selectHint'))}</p>
                {selected && <span className="inspector-duration">{formatDuration(selected.durationMs)} · {selected.codec || selected.extension.replace('.', '').toUpperCase()}</span>}
              </div>
            </div>
            {selected && (
              <div className="inspector-player">
                {(mediaLoading || mediaFallbackLoading) && <span>{mediaFallbackLoading ? t('media.compatibilityPreview') : t('media.preparing')}</span>}
                {!mediaLoading && !mediaFallbackLoading && mediaError && <span className="media-error" title={mediaError}>{t('media.unavailable')}</span>}
                {!mediaLoading && !mediaFallbackLoading && trackMedia?.audioUrl && (
                  <div className={`inspector-player-ready${trackMedia.isPreview ? ' is-preview' : ''}`}>
                    <audio
                      ref={audioRef}
                      key={trackMedia.audioUrl}
                      preload="metadata"
                      src={trackMedia.audioUrl}
                      onLoadedMetadata={updateAudioMetadata}
                      onDurationChange={updateAudioMetadata}
                      onTimeUpdate={() => setMediaCurrentTime(audioRef.current?.currentTime || 0)}
                      onPlay={() => setMediaPlaying(true)}
                      onPause={() => setMediaPlaying(false)}
                      onEnded={() => setMediaPlaying(false)}
                      onError={() => void handleAudioPlaybackError()}
                    />
                    <div className="inspector-player-controls">
                      <button
                        type="button"
                        className="player-toggle"
                        onClick={() => void toggleAudioPlayback()}
                        aria-label={mediaPlaying ? t('media.pause') : t('media.play')}
                        title={mediaPlaying ? t('media.pause') : t('media.play')}
                      >
                        {mediaPlaying ? 'Ⅱ' : '▶'}
                      </button>
                      <span className="player-time">{formatPlayerTime(mediaCurrentTime)}</span>
                      <input
                        className="player-seek"
                        type="range"
                        min="0"
                        max={Math.max(mediaDuration, 0.1)}
                        step="0.1"
                        value={Math.min(mediaCurrentTime, mediaDuration || mediaCurrentTime)}
                        onChange={(event) => seekAudio(Number(event.target.value))}
                        aria-label={t('media.seek')}
                        disabled={mediaDuration <= 0}
                      />
                      <span className="player-time">{formatPlayerTime(mediaDuration)}</span>
                      <input
                        className="player-volume"
                        type="range"
                        min="0"
                        max="1"
                        step="0.05"
                        value={mediaVolume}
                        onChange={(event) => changeAudioVolume(Number(event.target.value))}
                        aria-label={t('media.volume')}
                        title={t('media.volume')}
                      />
                      {trackMedia.isPreview && <span className="player-preview-badge">{t('media.preview')}</span>}
                    </div>
                  </div>
                )}
              </div>
            )}
          </div>
          <div className="inspector-tabs" role="tablist">
            <button className={inspectorTab === 'tags' ? 'active' : ''} onClick={() => setInspectorTab('tags')}>{t('workspace.tags')}</button>
            <button className={inspectorTab === 'metadata' ? 'active' : ''} onClick={() => setInspectorTab('metadata')}>{t('workspace.metadata')}</button>
            <button className={inspectorTab === 'analysis' ? 'active' : ''} onClick={() => setInspectorTab('analysis')}>{t('workspace.analysis')}</button>
            <button className={inspectorTab === 'organize' ? 'active' : ''} onClick={() => setInspectorTab('organize')}>{t('workspace.organize')}</button>
          </div>
          <div className="inspector-body">
            {inspectorTab === 'tags' && (
              <div className="inspector-section">
                <TagEditor language={language} tracks={selectedTracks} revision={tagRevision} disabled={busy || scanning} onBusyChange={setBusy} onMessage={setMessage} onChanged={tagDataChanged} />
              </div>
            )}

            {inspectorTab === 'metadata' && (
              <div className="inspector-section metadata-inspector">
                {selectedTracks.length === 0 ? <div className="workspace-empty-state">{t('workspace.metadataHint')}</div> : (
                  <>
                    <div className="inspector-action-row">
                      {selected && <button className="primary" onClick={lookupMetadata} disabled={busy}>{t('actions.findMetadata')}</button>}
                      {selected && metadataLookup && <button onClick={() => void refreshMetadata()} disabled={busy}>{t('metadata.refresh')}</button>}
                    </div>
                    <div className="compact-enrichment">
                      <label>{t('metadata.minimumConfidence')}<input type="number" min="0.5" max="1" step="0.01" value={enrichment.minimumConfidence} onChange={(e) => setEnrichment({...enrichment, minimumConfidence: Number(e.target.value)})} /></label>
                      <label className="enrichment-mode-label">{t('metadata.searchMode')}<strong>{t('metadata.searchMode.same')}</strong></label>
                      <small className="enrichment-mode-hint">{t('metadata.searchMode.same.hint')}</small>
                      <label className="inline-check"><input type="checkbox" checked={enrichment.onlyMissing} onChange={(e) => setEnrichment({...enrichment, onlyMissing: e.target.checked})} /> {t('metadata.onlyMissing')}</label>
                      <label className="inline-check"><input type="checkbox" checked={enrichment.includeArtwork} onChange={(e) => setEnrichment({...enrichment, includeArtwork: e.target.checked})} /> {t('metadata.includeArtwork')}</label>
                      <button onClick={() => void enrichSelected()} disabled={busy || scanning}>{t('metadata.enrichSelected')}</button>
                    </div>
                    {metadataLookup?.cached && <span className="metadata-cache-badge">{t('metadata.cached', {age: formatCacheAge(metadataLookup.cacheAgeSeconds, language)})}</span>}
                    {metadataLookup?.providerReports && metadataLookup.providerReports.length > 0 && (
                      <div className="inspector-provider-list">
                        {metadataLookup.providerReports.map((report) => <div className={`inspector-provider ${report.status}`} key={report.name} title={compactProviderMessage(report.error) || undefined}><strong>{report.name}{report.kind === 'dj_pool' && <em className="metadata-source-kind">{t('metadata.djPool')}</em>}</strong><span>{report.status === 'ok' ? t('metadata.providerOk', {count: report.candidates}) : report.status === 'empty' ? t('metadata.providerEmpty') : t(report.retryable ? 'metadata.providerTemporaryError' : 'metadata.providerError')}</span><small>{(report.durationMs / 1000).toFixed(1)}s</small></div>)}
                      </div>
                    )}
                    {!metadataLookup?.providerReports?.length && metadataWarnings.map((warning) => <p className="warning compact-warning" key={warning}>{compactProviderMessage(warning)}</p>)}
                    {metadataLookup && <MetadataMerge language={language} lookup={metadataLookup} current={selected} disabled={busy} onApply={applyMetadataCandidate} />}
                    <div className="inspector-candidates">
                      {metadata.map((item, index) => (
                        <article className="inspector-candidate" key={`${item.source}-${item.externalId}-${index}`}>
                          <div className="candidate-head">
                            {item.artworkUrl && <img src={item.artworkUrl} alt={t('metadata.artworkAlt')} />}
                            <div><strong>{item.artist} — {item.title}</strong><span>{item.album || t('metadata.unknownAlbum')}</span><small>{item.source}{item.sourceKind === 'dj_pool' && <em className="metadata-source-kind">{t('metadata.djPool')}</em>}</small></div>
                            <div className={`metadata-match-badge ${item.matchClass || 'low'}`}>{Math.round(item.confidence * 100)}%</div>
                          </div>
                          <div className="candidate-facts">
                            <MetadataDetail label={t('metadata.year')} value={item.year ? String(item.year) : '–'} />
                            <MetadataDetail label={t('metadata.genre')} value={item.genre || '–'} />
                            <MetadataDetail label={t('metadata.duration')} value={item.durationMs ? formatDuration(item.durationMs) : '–'} />
                            <MetadataDetail label={t('metadata.label')} value={item.label || '–'} />
                            <MetadataDetail label={t('metadata.isrc')} value={item.isrc || '–'} />
                            <MetadataDetail label={t('metadata.catalogNumber')} value={item.catalogNumber || '–'} />
                            <MetadataDetail label={t('tags.field.bpm')} value={item.bpm ? item.bpm.toFixed(item.bpm % 1 === 0 ? 0 : 1) : '–'} />
                            <MetadataDetail label={t('tags.field.key')} value={item.key || '–'} />
                          </div>
                          {(item.matchIssues?.length ?? 0) > 0 && <div className="metadata-issues">{(item.matchIssues ?? []).map((issue) => <span key={issue}>{t((`metadata.issue.${issue}`) as TranslationKey)}</span>)}</div>}
                          {selected && <div className="candidate-actions"><button onClick={() => void applyMetadataCandidate(item, false)} disabled={busy || item.matchClass === 'rejected'}>{t('metadata.applyTags')}</button>{item.artworkUrl && item.artworkEmbeddable && <button onClick={() => void applyMetadataCandidate(item, true)} disabled={busy || item.matchClass === 'rejected'}>{t('metadata.applyWithArtwork')}</button>}</div>}
                        </article>
                      ))}
                    </div>
                  </>
                )}
              </div>
            )}

            {inspectorTab === 'analysis' && (
              <div className="inspector-section">
                {!selected ? <div className="workspace-empty-state">{t('workspace.analysisHint')}</div> : <>
                  <dl className="facts inspector-facts"><dt>{t('details.path')}</dt><dd title={selected.path}>{selected.path}</dd><dt>{t('details.format')}</dt><dd>{selected.codec} · {selected.sampleRate || '–'} Hz · {selected.channels || '–'} {t('details.channelsShort')}</dd><dt>{t('details.loudness')}</dt><dd>{selected.loudnessI ? `${selected.loudnessI.toFixed(1)} LUFS / ${selected.truePeak.toFixed(1)} dBTP` : t('details.notAnalyzed')}</dd></dl>
                  <div className="inspector-action-grid"><button onClick={analyzeLoudness} disabled={busy}>{t('actions.loudnessAnalysis')}</button><button onClick={analyzeBPMKey} disabled={busy || !status?.essentiaReady}>{t('actions.bpmKey')}</button><button onClick={writeReplayGain} disabled={busy}>{t('actions.replayGain')}</button></div>
                  <div className="spectrogram-section">
                    <div className="spectrogram-heading"><h3>{t('spectrogram.title')}</h3><button onClick={() => void loadSpectrograms()} disabled={spectrogramLoading || !status?.ffmpegReady}>{spectrogramLoading ? t('spectrogram.generating') : t('spectrogram.refresh')}</button></div>
                    {spectrogramError && <p className="warning compact-warning">{spectrogramError}</p>}
                    <div className="spectrogram-grid">
                      <figure className="spectrogram-card">
                        <figcaption><strong>{t('spectrogram.before')}</strong><span>{t('spectrogram.original')}</span></figcaption>
                        {spectrograms?.beforeUrl ? <img src={spectrograms.beforeUrl} alt={t('spectrogram.beforeAlt')} /> : <div className="spectrogram-placeholder">{spectrogramLoading ? t('spectrogram.generating') : t('spectrogram.notReady')}</div>}
                      </figure>
                      <figure className="spectrogram-card">
                        <figcaption><strong>{t('spectrogram.after')}</strong><span>{processedPath ? t('spectrogram.processed') : t('spectrogram.afterHint')}</span></figcaption>
                        {spectrograms?.afterUrl ? <img src={spectrograms.afterUrl} alt={t('spectrogram.afterAlt')} /> : <div className="spectrogram-placeholder">{processedPath ? (spectrogramLoading ? t('spectrogram.generating') : t('spectrogram.notReady')) : t('spectrogram.afterHint')}</div>}
                      </figure>
                    </div>
                  </div>
                  <h3>{t('audio.title')}</h3>
                  <div className="form-grid"><label>{t('audio.targetLUFS')}<input type="number" step="0.5" value={processing.targetLUFS} onChange={(e) => setProcessing({...processing, targetLUFS: Number(e.target.value)})} /></label><label>{t('audio.truePeak')}<input type="number" step="0.1" value={processing.targetTruePeakDb} onChange={(e) => setProcessing({...processing, targetTruePeakDb: Number(e.target.value)})} /></label><label>{t('audio.preGain')}<input type="number" step="0.5" value={processing.preGainDb} onChange={(e) => setProcessing({...processing, preGainDb: Number(e.target.value)})} /></label><label>{t('audio.pitch')}<input type="number" step="0.1" value={processing.pitchSemitones} onChange={(e) => setProcessing({...processing, pitchSemitones: Number(e.target.value)})} /></label></div>
                  <div className="checks"><label><input type="checkbox" checked={processing.repairClipping} onChange={(e) => setProcessing({...processing, repairClipping: e.target.checked})} /> {t('audio.clippingRepair')}</label><label><input type="checkbox" checked={processing.multibandCompress} onChange={(e) => setProcessing({...processing, multibandCompress: e.target.checked})} /> {t('audio.multiband')}</label><label><input type="checkbox" checked={processing.limit} onChange={(e) => setProcessing({...processing, limit: e.target.checked})} /> {t('audio.limiter')}</label><label><input type="checkbox" checked={processing.keepOriginal} onChange={(e) => setProcessing({...processing, keepOriginal: e.target.checked})} /> {t('audio.keepOriginal')}</label></div>
                  <button className="primary full" onClick={normalize} disabled={busy}>{t('audio.createProcessedCopy')}</button>
                </>}
              </div>
            )}

            {inspectorTab === 'organize' && (
              <div className="inspector-section">
                {!selected ? <div className="workspace-empty-state">{t('workspace.organizeHint')}</div> : <>
                  <h3>{t('organize.title')}</h3>
                  <label>{t('organize.rootFolder')}<input value={organize.rootDir} onChange={(e) => setOrganize({...organize, rootDir: e.target.value})} placeholder={t('organize.rootPlaceholder')} /></label>
                  <label>{t('organize.template')}<input value={organize.template} onChange={(e) => setOrganize({...organize, template: e.target.value})} /></label>
                  <label>{t('organize.regex')}<input value={organize.regexPattern} onChange={(e) => setOrganize({...organize, regexPattern: e.target.value})} placeholder={t('organize.regexPlaceholder')} /></label>
                  <label>{t('organize.replace')}<input value={organize.regexReplace} onChange={(e) => setOrganize({...organize, regexReplace: e.target.value})} placeholder={t('organize.replacePlaceholder')} /></label>
                  <div className="inspector-action-row"><button onClick={previewRename} disabled={busy}>{t('organize.preview')}</button><button className="primary" onClick={applyOrganize} disabled={busy}>{t('organize.apply')}</button></div>
                  {previewPath && <p className="path-preview">{previewPath}</p>}
                </>}
              </div>
            )}
          </div>
        </aside>
        )}
      </div>

      <HelpModal language={language} open={helpOpen} version={t('footer.version')} onClose={() => setHelpOpen(false)} />
      <JobsPanel language={language} open={jobsOpen} onClose={() => setJobsOpen(false)} onMessage={setMessage} onRevealTrack={revealJobTrack} />
      <SettingsModal language={language} theme={theme} uiScale={uiScale} open={settingsOpen} status={status} onClose={() => setSettingsOpen(false)} onSaved={async () => { await refreshStatus() }} onMessage={setMessage} onThemeChange={changeTheme} onUIScaleChange={changeUIScale} />

      <footer className="workspace-statusbar">
        <div><span className={scanning ? 'status-busy-dot' : busy ? 'status-busy-dot' : 'status-ready-dot'} />{scanning ? t('footer.scanning') : busy ? t('footer.working') : message}</div>
        <div className="statusbar-meta"><span>{t('workspace.selectedCount', {count: selectedIDs.length})}</span><span>{formatNumber(filteredTracks.length, locale)} / {formatNumber(tracks.length, locale)}</span><span>{t('footer.version')}</span></div>
      </footer>
    </div>
  )
}


function MetadataDetail({label, value}: {label: string; value: string}) {
  return <div className="metadata-detail"><span>{label}</span><strong title={value}>{value}</strong></div>
}

function MetadataScore({label, value}: {label: string; value: number}) {
  return <span>{label}: {Math.round(value * 100)}%</span>
}

function formatNumberPair(value: number, total: number, empty: string): string {
  if (!value && !total) return empty
  if (value && total) return `${value}/${total}`
  return String(value || total)
}

function formatCacheAge(seconds: number, language: AppLanguage): string {
  if (seconds < 60) return translate(language, 'metadata.cacheSeconds', {count: Math.max(0, Math.round(seconds))})
  if (seconds < 3600) return translate(language, 'metadata.cacheMinutes', {count: Math.round(seconds / 60)})
  return translate(language, 'metadata.cacheHours', {count: Math.round(seconds / 3600)})
}

function formatDuration(ms: number): string {
  if (!ms) return '–'
  const total = Math.round(ms / 1000)
  const minutes = Math.floor(total / 60)
  const seconds = total % 60
  return `${minutes}:${seconds.toString().padStart(2, '0')}`
}

function formatPlayerTime(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return '0:00'
  const total = Math.floor(seconds)
  const minutes = Math.floor(total / 60)
  const rest = total % 60
  return `${minutes}:${rest.toString().padStart(2, '0')}`
}

function formatLongDuration(ms: number, language: AppLanguage): string {
  const hour = translate(language, 'duration.hourShort')
  const day = translate(language, 'duration.dayShort')
  if (!ms) return `0 ${hour}`
  const totalHours = Math.round(ms / 3_600_000)
  const days = Math.floor(totalHours / 24)
  const hours = totalHours % 24
  return days > 0 ? `${days} ${day} ${hours} ${hour}` : `${hours} ${hour}`
}

function formatBytes(bytes: number, locale: string): string {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let value = bytes
  let index = 0
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024
    index++
  }
  const formatted = new Intl.NumberFormat(locale, {
    minimumFractionDigits: 0,
    maximumFractionDigits: index === 0 ? 0 : 2,
  }).format(value)
  return `${formatted} ${units[index]}`
}

function formatDate(value: string, locale: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString(locale)
}

function formatNumber(value: number, locale: string): string {
  return new Intl.NumberFormat(locale).format(value)
}

function StatCard({ label, value }: { label: string; value: string }) {
  return <div className="panel stat-card"><span>{label}</span><strong>{value}</strong></div>
}

export default App
