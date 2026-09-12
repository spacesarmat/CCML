import { useEffect, useMemo, useState } from 'react'
import { EventsOn } from '../wailsjs/runtime/runtime'
import TagEditor from './TagEditor'
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
  DuplicateGroup,
  LibraryRoot,
  LibraryStats,
  MetadataCandidate,
  OrganizeRequest,
  ProcessingOptions,
  ScanProgress,
  SystemStatus,
  Track,
} from './types'

const defaultProcessing: ProcessingOptions = {
  outputPath: '',
  targetLUFS: -14,
  targetTruePeakDb: -1,
  targetLRA: 11,
  preGainDb: 0,
  repairClipping: true,
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
  const [status, setStatus] = useState<SystemStatus | null>(null)
  const [folder, setFolder] = useState('')
  const [search, setSearch] = useState('')
  const [tracks, setTracks] = useState<Track[]>([])
  const [selectedIDs, setSelectedIDs] = useState<number[]>([])
  const [tagRevision, setTagRevision] = useState(0)
  const [message, setMessage] = useState(() => translate(detectInitialLanguage(), 'message.ready'))
  const [busy, setBusy] = useState(false)
  const [processing, setProcessing] = useState(defaultProcessing)
  const [organize, setOrganize] = useState(defaultOrganize)
  const [previewPath, setPreviewPath] = useState('')
  const [metadata, setMetadata] = useState<MetadataCandidate[]>([])
  const [metadataWarnings, setMetadataWarnings] = useState<string[]>([])
  const [duplicates, setDuplicates] = useState<DuplicateGroup[]>([])
  const [roots, setRoots] = useState<LibraryRoot[]>([])
  const [stats, setStats] = useState<LibraryStats | null>(null)
  const [scanProgress, setScanProgress] = useState<ScanProgress | null>(null)
  const [scanning, setScanning] = useState(false)

  const locale = localeFor(language)
  const t = (key: TranslationKey, params?: TranslateParams) => translate(language, key, params)

  const selectedTracks = useMemo(
    () => tracks.filter((track) => selectedIDs.includes(track.id)),
    [tracks, selectedIDs],
  )

  const selected = selectedTracks.length === 1 ? selectedTracks[0] : null

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

    return () => {
      offProgress()
      offStarted()
      offFFmpegStarted()
      offFFmpegFinished()
      offFFmpegError()
    }
  }, [language])

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

  async function refreshTracks(query = search) {
    const result = await run(t('message.loadingLibrary'), () => backend().ListTracks(query, 500, 0))
    if (result) {
      setTracks(result)
      const visibleIDs = new Set(result.map((track) => track.id))
      setSelectedIDs((current) => current.filter((id) => visibleIDs.has(id)))
      setMessage(t('message.tracksLoaded', {count: result.length}))
    }
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

  function toggleTrackSelection(trackID: number) {
    setSelectedIDs((current) => current.includes(trackID)
      ? current.filter((id) => id !== trackID)
      : [...current, trackID])
  }

  function selectOnlyTrack(trackID: number) {
    setSelectedIDs([trackID])
  }

  function toggleAllVisible() {
    if (tracks.length > 0 && tracks.every((track) => selectedIDs.includes(track.id))) {
      setSelectedIDs([])
      return
    }
    setSelectedIDs(tracks.map((track) => track.id))
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

  async function lookupMetadata() {
    if (!selected) return
    const result = await run(t('message.searchingMetadata'), () => backend().LookupMetadata(selected.id))
    if (result) {
      setMetadata(result.candidates ?? [])
      setMetadataWarnings(result.warnings ?? [])
      setMessage(t('message.metadataCandidatesFound', {count: result.candidates?.length ?? 0}))
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
      setDuplicates(result)
      setMessage(t('message.duplicateGroupsFound', {count: result.length}))
    }
  }

  return (
    <div className="app-shell">
      <header className="topbar">
        <div>
          <h1>CCML</h1>
          <p>{t('app.subtitle')}</p>
        </div>
        <div className="topbar-actions">
          <div className="language-switcher" aria-label={t('language.label')} title={t('language.label')}>
            <button
              type="button"
              className={language === 'ru' ? 'active' : ''}
              onClick={() => changeLanguage('ru')}
              aria-pressed={language === 'ru'}
              title={t('language.russian')}
            >RU</button>
            <button
              type="button"
              className={language === 'en' ? 'active' : ''}
              onClick={() => changeLanguage('en')}
              aria-pressed={language === 'en'}
              title={t('language.english')}
            >EN</button>
          </div>
          <div className="status-pills">
            <span
              className={status?.ffmpegUpdating ? 'pill warn' : status?.ffmpegReady ? 'pill ok' : 'pill bad'}
              title={[status?.ffmpegSource, status?.ffmpegVersion, status?.ffmpegPath].filter(Boolean).join(' · ')}
            >
              FFmpeg {status?.ffmpegUpdating
                ? t('status.ffmpeg.updating')
                : status?.ffmpegReady
                  ? (status.ffmpegVersion || t('status.ffmpeg.ready'))
                  : t('status.ffmpeg.missing')}
            </span>
            <span className={status?.essentiaReady ? 'pill ok' : 'pill warn'}>
              Essentia {status?.essentiaReady ? t('status.essentia.ready') : t('status.essentia.optional')}
            </span>
            <span className="pill">
              {t('status.metadata')} {status?.metadataProviders?.length ?? 0}
            </span>
          </div>
        </div>
      </header>

      <section className="toolbar panel">
        <button onClick={chooseFolder} disabled={scanning}>{t('toolbar.chooseFolder')}</button>
        <input value={folder} onChange={(event) => setFolder(event.target.value)} placeholder={t('toolbar.folderPlaceholder')} disabled={scanning} />
        <button className="primary" onClick={scanFolder} disabled={scanning || status?.ffmpegUpdating || !status?.ffmpegReady}>{t('toolbar.scan')}</button>
        <button onClick={cancelScan} disabled={!scanning}>{t('toolbar.cancel')}</button>
        {status?.ffmpegAutoUpdateSupported && (
          <button onClick={updateFFmpeg} disabled={scanning || busy || status.ffmpegUpdating}>
            {status.ffmpegUpdating ? t('toolbar.updatingFFmpeg') : t('toolbar.updateFFmpeg')}
          </button>
        )}
      </section>

      {status?.ffmpegUpdateError && status.ffmpegReady && (
        <section className="panel tool-warning">
          <span>{t('warning.ffmpegUpdateFailed')}</span>
          <small>{status.ffmpegUpdateError}</small>
        </section>
      )}

      {roots.length > 0 && (
        <section className="panel roots-panel">
          <div className="panel-title"><h2>{t('roots.title')}</h2><span>{t('roots.managed', {count: roots.length})}</span></div>
          <div className="root-list">
            {roots.map((root) => (
              <div className="root-row" key={root.path}>
                <button className="root-path" onClick={() => void useLibraryRoot(root.path)} disabled={scanning}>{root.path}</button>
                <span>{root.lastScanAt ? t('roots.lastScan', {date: formatDate(root.lastScanAt, locale)}) : t('roots.notScanned')}</span>
                <button onClick={() => void removeLibraryRoot(root.path)} disabled={scanning}>{t('roots.remove')}</button>
              </div>
            ))}
          </div>
        </section>
      )}

      {stats && (
        <section className="stats-grid">
          <StatCard label={t('stats.tracks')} value={formatNumber(stats.tracks, locale)} />
          <StatCard label={t('stats.artists')} value={formatNumber(stats.artists, locale)} />
          <StatCard label={t('stats.albums')} value={formatNumber(stats.albums, locale)} />
          <StatCard label={t('stats.duration')} value={formatLongDuration(stats.durationMs, language)} />
          <StatCard label={t('stats.size')} value={formatBytes(stats.sizeBytes, locale)} />
          <StatCard label={t('stats.duplicateGroups')} value={formatNumber(stats.duplicateGroups, locale)} />
        </section>
      )}

      {scanProgress && (scanning || scanProgress.finished) && (
        <section className="panel scan-panel">
          <div className="panel-title row-between">
            <div>
              <h2>{scanning ? t('scan.scanning') : scanProgress.cancelled ? t('scan.cancelled') : t('scan.finished')}</h2>
              <span>{scanProgress.root}</span>
            </div>
            <strong>{t('scan.processed', {count: formatNumber(scanProgress.scanned, locale)})}</strong>
          </div>
          <div className="scan-counters">
            <span>{t('scan.added')} <strong>{formatNumber(scanProgress.added, locale)}</strong></span>
            <span>{t('scan.updated')} <strong>{formatNumber(scanProgress.updated, locale)}</strong></span>
            <span>{t('scan.skipped')} <strong>{formatNumber(scanProgress.skipped, locale)}</strong></span>
            <span>{t('scan.removed')} <strong>{formatNumber(scanProgress.removed, locale)}</strong></span>
            <span>{t('scan.failed')} <strong>{formatNumber(scanProgress.failed, locale)}</strong></span>
          </div>
          {scanProgress.currentFile && <p className="scan-current" title={scanProgress.currentFile}>{scanProgress.currentFile}</p>}
        </section>
      )}

      <section className="library-layout">
        <div className="panel library-panel">
          <div className="panel-title row-between">
            <div>
              <h2>{t('library.title')}</h2>
              <span>{t('library.shown', {count: formatNumber(tracks.length, locale)})}</span>
            </div>
            <div className="search-row">
              <input
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                onKeyDown={(event) => event.key === 'Enter' && void refreshTracks()}
                placeholder={t('library.searchPlaceholder')}
              />
              <button onClick={() => void refreshTracks()} disabled={busy}>{t('library.search')}</button>
              <button onClick={findDuplicates} disabled={busy}>{t('library.duplicates')}</button>
            </div>
          </div>

          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th className="selection-col">
                    <input
                      type="checkbox"
                      aria-label={t('library.selectAll')}
                      checked={tracks.length > 0 && tracks.every((track) => selectedIDs.includes(track.id))}
                      onChange={toggleAllVisible}
                    />
                  </th>
                  <th>#</th>
                  <th>{t('table.artist')}</th>
                  <th>{t('table.title')}</th>
                  <th>{t('table.album')}</th>
                  <th>{t('table.time')}</th>
                  <th>{t('table.codec')}</th>
                  <th>LUFS</th>
                  <th>{t('table.bpmKey')}</th>
                </tr>
              </thead>
              <tbody>
                {tracks.map((track) => (
                  <tr
                    key={track.id}
                    className={selectedIDs.includes(track.id) ? 'selected' : ''}
                    onClick={() => selectOnlyTrack(track.id)}
                  >
                    <td className="selection-col" onClick={(event: { stopPropagation(): void }) => event.stopPropagation()}>
                      <input
                        type="checkbox"
                        checked={selectedIDs.includes(track.id)}
                        aria-label={t('library.selectTrack', {title: track.title || track.fileName})}
                        onChange={() => toggleTrackSelection(track.id)}
                      />
                    </td>
                    <td>{track.trackNumber || '–'}</td>
                    <td>{track.artist || t('library.unknownArtist')}</td>
                    <td>{track.title || track.fileName}</td>
                    <td>{track.album || '–'}</td>
                    <td>{formatDuration(track.durationMs)}</td>
                    <td>{track.codec}</td>
                    <td>{track.loudnessI ? track.loudnessI.toFixed(1) : '–'}</td>
                    <td>{track.bpm ? `${track.bpm.toFixed(1)} · ${track.key} ${track.keyScale}` : '–'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>

        <aside className="panel detail-panel">
          <div className="panel-title">
            <h2>{selected?.title || (selectedTracks.length > 1 ? t('details.multipleTracks', {count: selectedTracks.length}) : t('details.selectTrack'))}</h2>
            <span>{selected?.artist || (selectedTracks.length > 1 ? t('details.batchEditing') : t('details.toolsHere'))}</span>
          </div>

          <TagEditor
            language={language}
            tracks={selectedTracks}
            revision={tagRevision}
            disabled={busy || scanning}
            onBusyChange={setBusy}
            onMessage={setMessage}
            onChanged={tagDataChanged}
          />

          {selected && (
            <>
              <dl className="facts">
                <dt>{t('details.path')}</dt><dd title={selected.path}>{selected.path}</dd>
                <dt>{t('details.format')}</dt><dd>{selected.codec} · {selected.sampleRate || '–'} Hz · {selected.channels || '–'} {t('details.channelsShort')}</dd>
                <dt>{t('details.loudness')}</dt><dd>{selected.loudnessI ? `${selected.loudnessI.toFixed(1)} LUFS / ${selected.truePeak.toFixed(1)} dBTP` : t('details.notAnalyzed')}</dd>
              </dl>

              <div className="button-grid">
                <button onClick={analyzeLoudness} disabled={busy}>{t('actions.loudnessAnalysis')}</button>
                <button onClick={lookupMetadata} disabled={busy}>{t('actions.findMetadata')}</button>
                <button onClick={analyzeBPMKey} disabled={busy || !status?.essentiaReady}>{t('actions.bpmKey')}</button>
                <button onClick={writeReplayGain} disabled={busy}>{t('actions.replayGain')}</button>
              </div>

              <h3>{t('audio.title')}</h3>
              <div className="form-grid">
                <label>{t('audio.targetLUFS')}<input type="number" step="0.5" value={processing.targetLUFS} onChange={(e) => setProcessing({...processing, targetLUFS: Number(e.target.value)})} /></label>
                <label>{t('audio.truePeak')}<input type="number" step="0.1" value={processing.targetTruePeakDb} onChange={(e) => setProcessing({...processing, targetTruePeakDb: Number(e.target.value)})} /></label>
                <label>{t('audio.preGain')}<input type="number" step="0.5" value={processing.preGainDb} onChange={(e) => setProcessing({...processing, preGainDb: Number(e.target.value)})} /></label>
                <label>{t('audio.pitch')}<input type="number" step="0.1" value={processing.pitchSemitones} onChange={(e) => setProcessing({...processing, pitchSemitones: Number(e.target.value)})} /></label>
              </div>
              <div className="checks">
                <label><input type="checkbox" checked={processing.repairClipping} onChange={(e) => setProcessing({...processing, repairClipping: e.target.checked})} /> {t('audio.clippingRepair')}</label>
                <label><input type="checkbox" checked={processing.multibandCompress} onChange={(e) => setProcessing({...processing, multibandCompress: e.target.checked})} /> {t('audio.multiband')}</label>
                <label><input type="checkbox" checked={processing.limit} onChange={(e) => setProcessing({...processing, limit: e.target.checked})} /> {t('audio.limiter')}</label>
                <label><input type="checkbox" checked={processing.keepOriginal} onChange={(e) => setProcessing({...processing, keepOriginal: e.target.checked})} /> {t('audio.keepOriginal')}</label>
              </div>
              <button className="primary full" onClick={normalize} disabled={busy}>{t('audio.createProcessedCopy')}</button>

              <h3>{t('organize.title')}</h3>
              <label>{t('organize.rootFolder')}<input value={organize.rootDir} onChange={(e) => setOrganize({...organize, rootDir: e.target.value})} placeholder={t('organize.rootPlaceholder')} /></label>
              <label>{t('organize.template')}<input value={organize.template} onChange={(e) => setOrganize({...organize, template: e.target.value})} /></label>
              <div className="form-grid">
                <label>{t('organize.regex')}<input value={organize.regexPattern} onChange={(e) => setOrganize({...organize, regexPattern: e.target.value})} placeholder={t('organize.regexPlaceholder')} /></label>
                <label>{t('organize.replace')}<input value={organize.regexReplace} onChange={(e) => setOrganize({...organize, regexReplace: e.target.value})} placeholder={t('organize.replacePlaceholder')} /></label>
              </div>
              <div className="button-grid">
                <button onClick={previewRename} disabled={busy}>{t('organize.preview')}</button>
                <button onClick={applyOrganize} disabled={busy}>{t('organize.apply')}</button>
              </div>
              {previewPath && <p className="path-preview">{previewPath}</p>}
            </>
          )}
        </aside>
      </section>

      {(metadata.length > 0 || metadataWarnings.length > 0) && (
        <section className="panel lower-panel">
          <div className="panel-title"><h2>{t('metadata.title')}</h2></div>
          {metadataWarnings.map((warning) => <p className="warning" key={warning}>{warning}</p>)}
          <div className="cards">
            {metadata.map((item, index) => (
              <article className="meta-card" key={`${item.source}-${item.externalId}-${index}`}>
                {item.artworkUrl && <img src={item.artworkUrl} alt={t('metadata.artworkAlt')} />}
                <div>
                  <strong>{item.artist} — {item.title}</strong>
                  <span>{item.album || t('metadata.unknownAlbum')}</span>
                  <small>{item.source} · {t('metadata.match', {percent: Math.round(item.confidence * 100)})}</small>
                  {selected && (
                    <div className="metadata-actions">
                      <button onClick={() => void applyMetadataCandidate(item, false)} disabled={busy}>{t('metadata.applyTags')}</button>
                      {item.artworkUrl && <button onClick={() => void applyMetadataCandidate(item, true)} disabled={busy}>{t('metadata.applyWithArtwork')}</button>}
                    </div>
                  )}
                </div>
              </article>
            ))}
          </div>
        </section>
      )}

      {duplicates.length > 0 && (
        <section className="panel lower-panel">
          <div className="panel-title"><h2>{t('duplicates.title')}</h2></div>
          {duplicates.map((group, index) => (
            <details key={`${group.artist}-${group.title}-${index}`}>
              <summary>{group.artist} — {group.title} ({t('duplicates.files', {count: group.tracks.length})})</summary>
              {group.tracks.map((track) => <p className="dup-path" key={track.id}>{formatDuration(track.durationMs)} · {track.path}</p>)}
            </details>
          ))}
        </section>
      )}

      <footer>
        <span>{scanning ? t('footer.scanning') : busy ? t('footer.working') : message}</span>
        <span>{t('footer.version')}</span>
      </footer>
    </div>
  )
}

function formatDuration(ms: number): string {
  if (!ms) return '–'
  const total = Math.round(ms / 1000)
  const minutes = Math.floor(total / 60)
  const seconds = total % 60
  return `${minutes}:${seconds.toString().padStart(2, '0')}`
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
