import { useEffect, useMemo, useState } from 'react'
import type {
  DuplicateGroup,
  MetadataCandidate,
  OrganizeRequest,
  ProcessingOptions,
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

function api() {
  if (!window.go?.main?.App) {
    throw new Error('Wails backend is not available. Run the app with "wails dev".')
  }
  return window.go.main.App
}

function App() {
  const [status, setStatus] = useState<SystemStatus | null>(null)
  const [folder, setFolder] = useState('')
  const [search, setSearch] = useState('')
  const [tracks, setTracks] = useState<Track[]>([])
  const [selectedID, setSelectedID] = useState<number | null>(null)
  const [message, setMessage] = useState('Ready')
  const [busy, setBusy] = useState(false)
  const [processing, setProcessing] = useState(defaultProcessing)
  const [organize, setOrganize] = useState(defaultOrganize)
  const [previewPath, setPreviewPath] = useState('')
  const [metadata, setMetadata] = useState<MetadataCandidate[]>([])
  const [metadataWarnings, setMetadataWarnings] = useState<string[]>([])
  const [duplicates, setDuplicates] = useState<DuplicateGroup[]>([])

  const selected = useMemo(
    () => tracks.find((track) => track.id === selectedID) ?? null,
    [tracks, selectedID],
  )

  useEffect(() => {
    void refreshStatus()
    void refreshTracks('')
  }, [])

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
    const result = await run('Checking audio tools…', () => api().SystemStatus())
    if (result) {
      setStatus(result)
      setMessage(result.ffmpegReady ? 'Audio toolchain ready' : 'FFmpeg/ffprobe not found')
    }
  }

  async function refreshTracks(query = search) {
    const result = await run('Loading library…', () => api().ListTracks(query, 500, 0))
    if (result) {
      setTracks(result)
      setMessage(`${result.length} tracks loaded`)
    }
  }

  async function chooseFolder() {
    const result = await run('Choose a music folder…', () => api().SelectMusicFolder())
    if (result !== undefined && result !== '') {
      setFolder(result)
      setMessage(result)
    }
  }

  async function scanFolder() {
    if (!folder.trim()) {
      setMessage('Choose a music folder first')
      return
    }
    const result = await run('Scanning music…', () => api().ScanFolder(folder))
    if (result) {
      setMessage(`Scan complete: ${result.indexed}/${result.found} indexed, ${result.failed} failed`)
      await refreshTracks(search)
    }
  }

  async function analyzeLoudness() {
    if (!selected) return
    const result = await run('Analyzing loudness…', () => api().AnalyzeLoudness(selected.id))
    if (result) {
      setMessage(`Loudness ${result.inputI.toFixed(1)} LUFS, true peak ${result.inputTP.toFixed(1)} dBTP`)
      await refreshTracks(search)
    }
  }

  async function writeReplayGain() {
    if (!selected) return
    const result = await run('Writing ReplayGain metadata…', () => api().WriteReplayGain(selected.id, processing.targetLUFS))
    if (result) {
      setMessage(`ReplayGain written from ${result.inputI.toFixed(1)} LUFS analysis`)
      await refreshTracks(search)
    }
  }

  async function normalize() {
    if (!selected) return
    const result = await run('Rendering processed copy…', () => api().NormalizeTrack(selected.id, processing))
    if (result) {
      setMessage(`Processed: ${result.outputPath}`)
    }
  }

  async function analyzeBPMKey() {
    if (!selected) return
    const result = await run('Analyzing BPM and key with Essentia…', () => api().AnalyzeBPMKey(selected.id))
    if (result) {
      setMessage(`${result.bpm.toFixed(1)} BPM · ${result.key} ${result.scale}`)
      await refreshTracks(search)
    }
  }

  async function lookupMetadata() {
    if (!selected) return
    const result = await run('Searching metadata providers…', () => api().LookupMetadata(selected.id))
    if (result) {
      setMetadata(result.candidates ?? [])
      setMetadataWarnings(result.warnings ?? [])
      setMessage(`${result.candidates?.length ?? 0} metadata candidates found`)
    }
  }

  async function previewRename() {
    if (!selected) return
    const result = await run('Building target path…', () => api().PreviewRename(selected.id, organize))
    if (result !== undefined) {
      setPreviewPath(result)
      setMessage('Rename preview updated')
    }
  }

  async function applyOrganize() {
    if (!selected) return
    const result = await run('Moving/renaming track…', () => api().OrganizeTrack(selected.id, organize))
    if (result) {
      setPreviewPath(result)
      setMessage(`Moved: ${result}`)
      await refreshTracks(search)
    }
  }

  async function findDuplicates() {
    const result = await run('Searching probable duplicates…', () => api().FindDuplicates())
    if (result) {
      setDuplicates(result)
      setMessage(`${result.length} duplicate groups found`)
    }
  }

  return (
    <div className="app-shell">
      <header className="topbar">
        <div>
          <h1>CCML</h1>
          <p>Cross-platform music library & mastering workspace</p>
        </div>
        <div className="status-pills">
          <span className={status?.ffmpegReady ? 'pill ok' : 'pill bad'}>
            FFmpeg {status?.ffmpegReady ? 'ready' : 'missing'}
          </span>
          <span className={status?.essentiaReady ? 'pill ok' : 'pill warn'}>
            Essentia {status?.essentiaReady ? 'ready' : 'optional'}
          </span>
          <span className="pill">
            Metadata {status?.metadataProviders?.length ?? 0}
          </span>
        </div>
      </header>

      <section className="toolbar panel">
        <button onClick={chooseFolder} disabled={busy}>Choose folder</button>
        <input value={folder} onChange={(event) => setFolder(event.target.value)} placeholder="Music folder" />
        <button className="primary" onClick={scanFolder} disabled={busy || !status?.ffmpegReady}>Scan</button>
      </section>

      <section className="library-layout">
        <div className="panel library-panel">
          <div className="panel-title row-between">
            <div>
              <h2>Library</h2>
              <span>{tracks.length} shown</span>
            </div>
            <div className="search-row">
              <input
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                onKeyDown={(event) => event.key === 'Enter' && void refreshTracks()}
                placeholder="Search artist, title, album…"
              />
              <button onClick={() => void refreshTracks()} disabled={busy}>Search</button>
              <button onClick={findDuplicates} disabled={busy}>Duplicates</button>
            </div>
          </div>

          <div className="table-wrap">
            <table>
              <thead>
                <tr><th>#</th><th>Artist</th><th>Title</th><th>Album</th><th>Time</th><th>Codec</th><th>LUFS</th><th>BPM / Key</th></tr>
              </thead>
              <tbody>
                {tracks.map((track) => (
                  <tr
                    key={track.id}
                    className={selectedID === track.id ? 'selected' : ''}
                    onClick={() => setSelectedID(track.id)}
                  >
                    <td>{track.trackNumber || '–'}</td>
                    <td>{track.artist || 'Unknown artist'}</td>
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
            <h2>{selected?.title || 'Select a track'}</h2>
            <span>{selected?.artist || 'Track tools appear here'}</span>
          </div>

          {selected && (
            <>
              <dl className="facts">
                <dt>Path</dt><dd title={selected.path}>{selected.path}</dd>
                <dt>Format</dt><dd>{selected.codec} · {selected.sampleRate || '–'} Hz · {selected.channels || '–'} ch</dd>
                <dt>Loudness</dt><dd>{selected.loudnessI ? `${selected.loudnessI.toFixed(1)} LUFS / ${selected.truePeak.toFixed(1)} dBTP` : 'Not analyzed'}</dd>
              </dl>

              <div className="button-grid">
                <button onClick={analyzeLoudness} disabled={busy}>Loudness analysis</button>
                <button onClick={lookupMetadata} disabled={busy}>Find metadata</button>
                <button onClick={analyzeBPMKey} disabled={busy || !status?.essentiaReady}>BPM & Key</button>
                <button onClick={writeReplayGain} disabled={busy}>ReplayGain tags</button>
              </div>

              <h3>Audio processing</h3>
              <div className="form-grid">
                <label>Target LUFS<input type="number" step="0.5" value={processing.targetLUFS} onChange={(e) => setProcessing({...processing, targetLUFS: Number(e.target.value)})} /></label>
                <label>True peak dBTP<input type="number" step="0.1" value={processing.targetTruePeakDb} onChange={(e) => setProcessing({...processing, targetTruePeakDb: Number(e.target.value)})} /></label>
                <label>Pre-gain dB<input type="number" step="0.5" value={processing.preGainDb} onChange={(e) => setProcessing({...processing, preGainDb: Number(e.target.value)})} /></label>
                <label>Pitch semitones<input type="number" step="0.1" value={processing.pitchSemitones} onChange={(e) => setProcessing({...processing, pitchSemitones: Number(e.target.value)})} /></label>
              </div>
              <div className="checks">
                <label><input type="checkbox" checked={processing.repairClipping} onChange={(e) => setProcessing({...processing, repairClipping: e.target.checked})} /> Clipping repair</label>
                <label><input type="checkbox" checked={processing.multibandCompress} onChange={(e) => setProcessing({...processing, multibandCompress: e.target.checked})} /> Multiband compression</label>
                <label><input type="checkbox" checked={processing.limit} onChange={(e) => setProcessing({...processing, limit: e.target.checked})} /> Limiter</label>
                <label><input type="checkbox" checked={processing.keepOriginal} onChange={(e) => setProcessing({...processing, keepOriginal: e.target.checked})} /> Keep original if replacing</label>
              </div>
              <button className="primary full" onClick={normalize} disabled={busy}>Create processed copy</button>

              <h3>Organize / rename</h3>
              <label>Root folder<input value={organize.rootDir} onChange={(e) => setOrganize({...organize, rootDir: e.target.value})} placeholder="Empty = current folder" /></label>
              <label>Template<input value={organize.template} onChange={(e) => setOrganize({...organize, template: e.target.value})} /></label>
              <div className="form-grid">
                <label>Regex<input value={organize.regexPattern} onChange={(e) => setOrganize({...organize, regexPattern: e.target.value})} placeholder="Optional pattern" /></label>
                <label>Replace<input value={organize.regexReplace} onChange={(e) => setOrganize({...organize, regexReplace: e.target.value})} placeholder="$1 etc." /></label>
              </div>
              <div className="button-grid">
                <button onClick={previewRename} disabled={busy}>Preview</button>
                <button onClick={applyOrganize} disabled={busy}>Apply move/rename</button>
              </div>
              {previewPath && <p className="path-preview">{previewPath}</p>}
            </>
          )}
        </aside>
      </section>

      {(metadata.length > 0 || metadataWarnings.length > 0) && (
        <section className="panel lower-panel">
          <div className="panel-title"><h2>Metadata candidates</h2></div>
          {metadataWarnings.map((warning) => <p className="warning" key={warning}>{warning}</p>)}
          <div className="cards">
            {metadata.map((item, index) => (
              <article className="meta-card" key={`${item.source}-${item.externalId}-${index}`}>
                {item.artworkUrl && <img src={item.artworkUrl} alt="Artwork" />}
                <div><strong>{item.artist} — {item.title}</strong><span>{item.album || 'Unknown album'}</span><small>{item.source} · {Math.round(item.confidence * 100)}% match</small></div>
              </article>
            ))}
          </div>
        </section>
      )}

      {duplicates.length > 0 && (
        <section className="panel lower-panel">
          <div className="panel-title"><h2>Probable duplicates</h2></div>
          {duplicates.map((group, index) => (
            <details key={`${group.artist}-${group.title}-${index}`}>
              <summary>{group.artist} — {group.title} ({group.tracks.length} files)</summary>
              {group.tracks.map((track) => <p className="dup-path" key={track.id}>{formatDuration(track.durationMs)} · {track.path}</p>)}
            </details>
          ))}
        </section>
      )}

      <footer><span>{busy ? 'Working…' : message}</span><span>CCML v0.1 MVP</span></footer>
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

export default App
