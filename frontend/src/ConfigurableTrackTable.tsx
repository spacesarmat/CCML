import { useMemo, useState, type ReactNode } from 'react'
import type { AppLanguage } from './i18n'
import type { Track } from './types'

type TableColumnID =
  | 'trackNumber' | 'artist' | 'title' | 'album' | 'albumArtist'
  | 'year' | 'genre' | 'label' | 'catalogNumber' | 'releaseDate'
  | 'duration' | 'codec' | 'sampleRate' | 'bitRate' | 'channels'
  | 'lufs' | 'truePeak' | 'bpmKey' | 'isrc' | 'fileName' | 'path'

type TableLayout = {
  order: TableColumnID[]
  visible: TableColumnID[]
}

type Props = {
  language: AppLanguage
  tracks: Track[]
  selectedIDs: number[]
  onToggleAllVisible: () => void
  onRowClick: (trackID: number, extendSelection: boolean, toggleSelection: boolean) => void
  onToggleTrackSelection: (trackID: number) => void
  rowClass: (track: Track) => string
  rowTitle: (track: Track) => string | undefined
  selectAllLabel: string
  selectTrackLabel: (track: Track) => string
  unknownArtistLabel: string
  emptyLabel: string
}

const ALL_COLUMNS: TableColumnID[] = [
  'trackNumber', 'artist', 'title', 'album', 'albumArtist', 'year', 'genre',
  'label', 'catalogNumber', 'releaseDate', 'duration', 'codec', 'sampleRate',
  'bitRate', 'channels', 'lufs', 'truePeak', 'bpmKey', 'isrc', 'fileName', 'path',
]

const DEFAULT_LAYOUT: TableLayout = {
  order: [...ALL_COLUMNS],
  visible: ['trackNumber', 'artist', 'title', 'album', 'duration', 'codec', 'lufs', 'bpmKey'],
}

const STORAGE_KEY = 'ccml.table-layout.v1'

const LABELS: Record<AppLanguage, Record<TableColumnID, string>> = {
  ru: {
    trackNumber: '#',
    artist: 'Исполнитель',
    title: 'Название',
    album: 'Альбом',
    albumArtist: 'Исполнитель альбома',
    year: 'Год',
    genre: 'Жанр',
    label: 'Лейбл',
    catalogNumber: 'Каталожный №',
    releaseDate: 'Дата релиза',
    duration: 'Время',
    codec: 'Кодек',
    sampleRate: 'Sample Rate',
    bitRate: 'Битрейт',
    channels: 'Каналы',
    lufs: 'LUFS',
    truePeak: 'True Peak',
    bpmKey: 'BPM / Key',
    isrc: 'ISRC',
    fileName: 'Имя файла',
    path: 'Путь',
  },
  en: {
    trackNumber: '#',
    artist: 'Artist',
    title: 'Title',
    album: 'Album',
    albumArtist: 'Album Artist',
    year: 'Year',
    genre: 'Genre',
    label: 'Label',
    catalogNumber: 'Catalog #',
    releaseDate: 'Release Date',
    duration: 'Time',
    codec: 'Codec',
    sampleRate: 'Sample Rate',
    bitRate: 'Bitrate',
    channels: 'Channels',
    lufs: 'LUFS',
    truePeak: 'True Peak',
    bpmKey: 'BPM / Key',
    isrc: 'ISRC',
    fileName: 'File Name',
    path: 'Path',
  },
}

function ConfigurableTrackTable({
  language,
  tracks,
  selectedIDs,
  onToggleAllVisible,
  onRowClick,
  onToggleTrackSelection,
  rowClass,
  rowTitle,
  selectAllLabel,
  selectTrackLabel,
  unknownArtistLabel,
  emptyLabel,
}: Props) {
  const [layout, setLayout] = useState<TableLayout>(() => loadLayout())
  const [dragging, setDragging] = useState<TableColumnID | null>(null)
  const visibleColumns = useMemo(
    () => layout.order.filter((id) => layout.visible.includes(id)),
    [layout],
  )

  const copy = language === 'ru'
    ? {
        columns: 'Колонки',
        panelTitle: 'Столбцы таблицы',
        panelHint: 'Отметьте нужные поля. Перетаскивайте строки списка или заголовки таблицы, чтобы менять порядок.',
        reset: 'Сбросить',
        shown: 'Показано',
        drag: 'Перетащите для изменения порядка',
      }
    : {
        columns: 'Columns',
        panelTitle: 'Table columns',
        panelHint: 'Choose visible fields. Drag list rows or table headers to reorder them.',
        reset: 'Reset',
        shown: 'Shown',
        drag: 'Drag to reorder',
      }

  function commit(next: TableLayout) {
    const normalized = normalizeLayout(next)
    setLayout(normalized)
    saveLayout(normalized)
  }

  function toggleColumn(id: TableColumnID) {
    const shown = layout.visible.includes(id)
    if (shown && layout.visible.length === 1) return
    commit({
      ...layout,
      visible: shown
        ? layout.visible.filter((column) => column !== id)
        : [...layout.visible, id],
    })
  }

  function moveColumn(source: TableColumnID, target: TableColumnID) {
    if (source === target) return
    const order = [...layout.order]
    const from = order.indexOf(source)
    const to = order.indexOf(target)
    if (from < 0 || to < 0) return
    order.splice(from, 1)
    order.splice(to, 0, source)
    commit({...layout, order})
  }

  function readDragSource(event: React.DragEvent): TableColumnID | null {
    const value = event.dataTransfer.getData('text/plain') || dragging || ''
    return isColumnID(value) ? value : null
  }

  const allVisibleSelected = tracks.length > 0 && tracks.every((track) => selectedIDs.includes(track.id))

  return (
    <div className="workspace-table-wrap configurable-track-table">
      <div className="table-columns-toolbar">
        <details className="table-column-picker">
          <summary>
            <span aria-hidden="true">☷</span>
            <strong>{copy.columns}</strong>
            <b>{layout.visible.length}</b>
          </summary>

          <div className="table-column-panel">
            <div className="table-column-panel-head">
              <div>
                <strong>{copy.panelTitle}</strong>
                <span>{copy.panelHint}</span>
              </div>
              <button type="button" onClick={() => commit(DEFAULT_LAYOUT)}>{copy.reset}</button>
            </div>

            <div className="table-column-list">
              {layout.order.map((id) => {
                const checked = layout.visible.includes(id)
                return (
                  <div
                    key={id}
                    className={`table-column-option${dragging === id ? ' dragging' : ''}`}
                    draggable
                    onDragStart={(event) => {
                      setDragging(id)
                      event.dataTransfer.effectAllowed = 'move'
                      event.dataTransfer.setData('text/plain', id)
                    }}
                    onDragEnd={() => setDragging(null)}
                    onDragOver={(event) => {
                      event.preventDefault()
                      event.dataTransfer.dropEffect = 'move'
                    }}
                    onDrop={(event) => {
                      event.preventDefault()
                      const source = readDragSource(event)
                      if (source) moveColumn(source, id)
                      setDragging(null)
                    }}
                  >
                    <span className="column-grip" aria-hidden="true">⋮⋮</span>
                    <label>
                      <input
                        type="checkbox"
                        checked={checked}
                        disabled={checked && layout.visible.length === 1}
                        onChange={() => toggleColumn(id)}
                      />
                      <span>{LABELS[language][id]}</span>
                    </label>
                    <small>{checked ? copy.shown : ''}</small>
                  </div>
                )
              })}
            </div>
          </div>
        </details>
      </div>

      <table className="workspace-table configurable-workspace-table">
        <thead>
          <tr>
            <th className="selection-col">
              <input
                type="checkbox"
                aria-label={selectAllLabel}
                checked={allVisibleSelected}
                onChange={onToggleAllVisible}
              />
            </th>
            {visibleColumns.map((column) => (
              <th
                key={column}
                className={`table-column-header${dragging === column ? ' dragging' : ''}`}
                draggable
                data-column-id={column}
                title={copy.drag}
                onDragStart={(event) => {
                  setDragging(column)
                  event.dataTransfer.effectAllowed = 'move'
                  event.dataTransfer.setData('text/plain', column)
                }}
                onDragEnd={() => setDragging(null)}
                onDragOver={(event) => {
                  event.preventDefault()
                  event.dataTransfer.dropEffect = 'move'
                }}
                onDrop={(event) => {
                  event.preventDefault()
                  const source = readDragSource(event)
                  if (source) moveColumn(source, column)
                  setDragging(null)
                }}
              >
                <span>{LABELS[language][column]}</span>
                <i className="column-drag-handle" aria-hidden="true">⋮⋮</i>
              </th>
            ))}
          </tr>
        </thead>

        <tbody>
          {tracks.map((track) => (
            <tr
              key={track.id}
              data-track-id={track.id}
              className={rowClass(track)}
              title={rowTitle(track)}
              onClick={(event) => onRowClick(track.id, event.shiftKey, event.ctrlKey || event.metaKey)}
            >
              <td className="selection-col" onClick={(event) => event.stopPropagation()}>
                <input
                  type="checkbox"
                  checked={selectedIDs.includes(track.id)}
                  aria-label={selectTrackLabel(track)}
                  onChange={() => onToggleTrackSelection(track.id)}
                />
              </td>
              {visibleColumns.map((column) => (
                <td
                  key={column}
                  className={cellClass(column)}
                  title={column === 'path' ? track.path : undefined}
                >
                  {renderColumn(column, track, unknownArtistLabel)}
                </td>
              ))}
            </tr>
          ))}

          {tracks.length === 0 && (
            <tr className="empty-row">
              <td colSpan={visibleColumns.length + 1}>{emptyLabel}</td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  )
}

export default ConfigurableTrackTable

function loadLayout(): TableLayout {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    if (!raw) return cloneDefault()
    return normalizeLayout(JSON.parse(raw) as Partial<TableLayout>)
  } catch {
    return cloneDefault()
  }
}

function saveLayout(layout: TableLayout) {
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(layout))
  } catch {
    // localStorage can be disabled by a locked-down WebView; current-session
    // customization still works.
  }
}

function normalizeLayout(value: Partial<TableLayout>): TableLayout {
  const seen = new Set<TableColumnID>()
  const order: TableColumnID[] = []

  for (const candidate of value.order ?? []) {
    if (typeof candidate === 'string' && isColumnID(candidate) && !seen.has(candidate)) {
      seen.add(candidate)
      order.push(candidate)
    }
  }
  for (const id of ALL_COLUMNS) {
    if (!seen.has(id)) order.push(id)
  }

  const visible = (value.visible ?? [])
    .filter((candidate): candidate is TableColumnID => typeof candidate === 'string' && isColumnID(candidate))
    .filter((candidate, index, values) => values.indexOf(candidate) === index)

  return {
    order,
    visible: visible.length > 0 ? visible : [...DEFAULT_LAYOUT.visible],
  }
}

function cloneDefault(): TableLayout {
  return {
    order: [...DEFAULT_LAYOUT.order],
    visible: [...DEFAULT_LAYOUT.visible],
  }
}

function isColumnID(value: string): value is TableColumnID {
  return (ALL_COLUMNS as string[]).includes(value)
}

function renderColumn(id: TableColumnID, track: Track, unknownArtistLabel: string): ReactNode {
  switch (id) {
    case 'trackNumber':
      return track.trackNumber || '–'
    case 'artist':
      return track.artist || unknownArtistLabel
    case 'title':
      return track.title || track.fileName
    case 'album':
      return track.album || '–'
    case 'albumArtist':
      return track.albumArtist || '–'
    case 'year':
      return track.year || '–'
    case 'genre':
      return track.genre || '–'
    case 'label':
      return track.label || '–'
    case 'catalogNumber':
      return track.catalogNumber || '–'
    case 'releaseDate':
      return track.releaseDate || '–'
    case 'duration':
      return formatDuration(track.durationMs)
    case 'codec':
      return track.codec || track.extension.replace('.', '').toUpperCase() || '–'
    case 'sampleRate':
      return track.sampleRate > 0 ? formatSampleRate(track.sampleRate) : '–'
    case 'bitRate':
      return track.bitRate > 0 ? `${Math.round(track.bitRate / 1000)} kbps` : '–'
    case 'channels':
      return track.channels || '–'
    case 'lufs':
      return track.loudnessI ? track.loudnessI.toFixed(1) : '–'
    case 'truePeak':
      return track.truePeak ? `${track.truePeak.toFixed(1)} dBTP` : '–'
    case 'bpmKey':
      return track.bpm
        ? `${track.bpm.toFixed(1)} · ${track.key || '–'}${track.keyScale ? ` ${track.keyScale}` : ''}`
        : '–'
    case 'isrc':
      return track.isrc || '–'
    case 'fileName':
      return track.fileName || '–'
    case 'path':
      return track.path || '–'
  }
}

function cellClass(id: TableColumnID): string {
  if (id === 'title') return 'title-cell'
  if (id === 'path' || id === 'fileName') return 'file-cell'
  if (['trackNumber', 'year', 'duration', 'sampleRate', 'bitRate', 'channels', 'lufs', 'truePeak'].includes(id)) {
    return 'numeric-cell'
  }
  return ''
}

function formatDuration(ms: number): string {
  if (!ms) return '–'
  const seconds = Math.round(ms / 1000)
  return `${Math.floor(seconds / 60)}:${(seconds % 60).toString().padStart(2, '0')}`
}

function formatSampleRate(value: number): string {
  return value % 1000 === 0 ? `${value / 1000} kHz` : `${(value / 1000).toFixed(1)} kHz`
}
