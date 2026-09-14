import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import type { AppLanguage } from './i18n'
import type { Track } from './types'
import {nextTableSort, type TableColumnID, type TableSort} from './tableSort'
import {
  addCustomTablePreset,
  loadCustomTablePresets,
  removeCustomTablePreset,
  saveCustomTablePresets,
  tablePreset,
  TABLE_PRESET_IDS,
  updateCustomTablePreset,
  type CustomTablePreset,
  type TablePreset,
} from './tablePresets'
import TrackCoverCell from './TrackCoverCell'

type TableLayout = {
  order: TableColumnID[]
  visible: TableColumnID[]
  widths: Partial<Record<TableColumnID, number>>
}

type Props = {
  language: AppLanguage
  tracks: Track[]
  coverRevision: number
  selectedIDs: number[]
  sort: TableSort[]
  onSortChange: (sort: TableSort[]) => void
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
  'cover', 'trackNumber', 'artist', 'title', 'album', 'albumArtist', 'year', 'genre',
  'label', 'catalogNumber', 'releaseDate', 'duration', 'codec', 'sampleRate',
  'bitRate', 'channels', 'lufs', 'truePeak', 'bpmKey', 'isrc', 'fileName', 'path',
]

const DEFAULT_WIDTHS: Record<TableColumnID, number> = {
  cover: 58,
  trackNumber: 58,
  artist: 170,
  title: 220,
  album: 180,
  albumArtist: 180,
  year: 68,
  genre: 130,
  label: 150,
  catalogNumber: 125,
  releaseDate: 112,
  duration: 78,
  codec: 86,
  sampleRate: 104,
  bitRate: 96,
  channels: 82,
  lufs: 72,
  truePeak: 92,
  bpmKey: 112,
  isrc: 132,
  fileName: 280,
  path: 420,
}

const MIN_COLUMN_WIDTH = 54
const MAX_COLUMN_WIDTH = 640
const SELECTION_COLUMN_WIDTH = 34

const DEFAULT_LAYOUT: TableLayout = {
  order: [...ALL_COLUMNS],
  visible: ['cover', 'trackNumber', 'artist', 'title', 'album', 'duration', 'codec', 'lufs', 'bpmKey'],
  widths: {},
}

const STORAGE_KEY = 'ccml.table-layout.v1'

const LABELS: Record<AppLanguage, Record<TableColumnID, string>> = {
  ru: {
    cover: 'Обложка',
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
    cover: 'Cover',
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
  coverRevision,
  selectedIDs,
  sort,
  onSortChange,
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
  const [customPresets, setCustomPresets] = useState<CustomTablePreset[]>(() => loadCustomTablePresets())
  const [presetName, setPresetName] = useState('')
  const [presetOpen, setPresetOpen] = useState(false)
  const columnPickerRef = useRef<HTMLDetailsElement | null>(null)
  const overlayCloseTimer = useRef<number | null>(null)
  const [dragging, setDragging] = useState<TableColumnID | null>(null)
  const [resizingColumn, setResizingColumn] = useState<TableColumnID | null>(null)
  const visibleColumns = useMemo(
    () => layout.order.filter((id) => layout.visible.includes(id)),
    [layout],
  )
  const tableWidth = useMemo(
    () => SELECTION_COLUMN_WIDTH + visibleColumns.reduce((total, id) => total + columnWidth(layout, id), 0),
    [layout, visibleColumns],
  )

  const copy = language === 'ru'
    ? {
        columns: 'Колонки',
        panelTitle: 'Столбцы таблицы',
        panelHint: 'Отметьте нужные поля. Маркер ⋮⋮ меняет порядок; правую границу заголовка можно тянуть для изменения ширины.',
        reset: 'Сбросить',
        shown: 'Показано',
        width: 'Ширина',
        sort: 'Клик — сортировка; Shift+клик — добавить уровень сортировки',
        drag: 'Перетащите маркер для изменения порядка колонок',
        resize: 'Тяните для изменения ширины; двойной клик — автоширина',
        presets: 'Пресеты',
        presetHint: 'Пресет меняет только колонки, ширины и сортировку. Поиск и фильтры сохраняются.',
        builtInPresets: 'Готовые пресеты',
        customPresets: 'Мои пресеты',
        presetName: 'Название пресета…',
        savePreset: 'Сохранить текущий',
        updatePreset: 'Обновить',
        deletePreset: 'Удалить',
        noCustomPresets: 'Пользовательских пресетов пока нет',
        updatePresetConfirm: 'Перезаписать этот пресет текущими колонками и сортировкой?',
        deletePresetConfirm: 'Удалить этот пресет?',
        dj: 'DJ',
        metadata: 'Метаданные',
        technical: 'Технический',
        compact: 'Компактный',
      }
    : {
        columns: 'Columns',
        panelTitle: 'Table columns',
        panelHint: 'Choose visible fields. Drag ⋮⋮ to reorder; drag a header’s right edge to resize.',
        reset: 'Reset',
        shown: 'Shown',
        width: 'Width',
        sort: 'Click to sort; Shift+click adds another sort level',
        drag: 'Drag the handle to reorder columns',
        resize: 'Drag to resize; double-click to auto-fit',
        presets: 'Presets',
        presetHint: 'A preset changes only columns, widths and sorting. Search and filters stay unchanged.',
        builtInPresets: 'Built-in presets',
        customPresets: 'My presets',
        presetName: 'Preset name…',
        savePreset: 'Save current',
        updatePreset: 'Update',
        deletePreset: 'Delete',
        noCustomPresets: 'No custom presets yet',
        updatePresetConfirm: 'Replace this preset with the current columns and sorting?',
        deletePresetConfirm: 'Delete this preset?',
        dj: 'DJ',
        metadata: 'Metadata',
        technical: 'Technical',
        compact: 'Compact',
      }

  function commit(next: TableLayout) {
    const normalized = normalizeLayout(next)
    setLayout(normalized)
    saveLayout(normalized)
  }

  function cancelOverlayClose() {
    if (overlayCloseTimer.current !== null) {
      window.clearTimeout(overlayCloseTimer.current)
      overlayCloseTimer.current = null
    }
  }

  function schedulePresetClose() {
    cancelOverlayClose()
    overlayCloseTimer.current = window.setTimeout(() => {
      setPresetOpen(false)
      overlayCloseTimer.current = null
    }, 420)
  }

  function scheduleColumnClose() {
    cancelOverlayClose()
    overlayCloseTimer.current = window.setTimeout(() => {
      if (columnPickerRef.current) columnPickerRef.current.open = false
      overlayCloseTimer.current = null
    }, 420)
  }

  function currentPreset(): TablePreset {
    return {
      layout: {
        order: [...layout.order],
        visible: [...layout.visible],
        widths: {...layout.widths},
      },
      sort: sort.map((rule) => ({...rule})),
    }
  }

  function applyPreset(preset: TablePreset) {
    commit(preset.layout)
    onSortChange(preset.sort)
    setPresetOpen(false)
  }

  function persistCustomPresets(next: CustomTablePreset[]) {
    setCustomPresets(next)
    saveCustomTablePresets(next)
  }

  function saveCurrentPreset() {
    const name = presetName.trim()
    if (!name) return

    persistCustomPresets(addCustomTablePreset(customPresets, name, currentPreset()))
    setPresetName('')
  }

  function updatePreset(item: CustomTablePreset) {
    if (!window.confirm(copy.updatePresetConfirm)) return
    persistCustomPresets(updateCustomTablePreset(customPresets, item.id, currentPreset()))
  }

  function deletePreset(item: CustomTablePreset) {
    if (!window.confirm(copy.deletePresetConfirm)) return
    persistCustomPresets(removeCustomTablePreset(customPresets, item.id))
  }

  function togglePresetPopup() {
    cancelOverlayClose()
    const next = !presetOpen
    if (next && columnPickerRef.current) columnPickerRef.current.open = false
    setPresetOpen(next)
  }

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (event.key !== 'Escape') return
      setPresetOpen(false)
      if (columnPickerRef.current) columnPickerRef.current.open = false
    }

    window.addEventListener('keydown', onKeyDown)
    return () => {
      window.removeEventListener('keydown', onKeyDown)
      cancelOverlayClose()
    }
  }, [])

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

  function startColumnResize(event: React.PointerEvent<HTMLSpanElement>, id: TableColumnID) {
    event.preventDefault()
    event.stopPropagation()

    const startX = event.clientX
    const startWidth = columnWidth(layout, id)
    setResizingColumn(id)
    document.body.classList.add('table-column-resizing')

    const widthAt = (clientX: number) => clampColumnWidth(startWidth + clientX - startX)

    const onMove = (moveEvent: PointerEvent) => {
      const width = widthAt(moveEvent.clientX)
      setLayout((current) => normalizeLayout({
        ...current,
        widths: {...current.widths, [id]: width},
      }))
    }

    const cleanup = () => {
      window.removeEventListener('pointermove', onMove)
      window.removeEventListener('pointerup', onFinish)
      window.removeEventListener('pointercancel', onFinish)
      document.body.classList.remove('table-column-resizing')
      setResizingColumn(null)
    }

    const onFinish = (finishEvent: PointerEvent) => {
      const width = widthAt(finishEvent.clientX)
      setLayout((current) => {
        const next = normalizeLayout({
          ...current,
          widths: {...current.widths, [id]: width},
        })
        saveLayout(next)
        return next
      })
      cleanup()
    }

    window.addEventListener('pointermove', onMove)
    window.addEventListener('pointerup', onFinish)
    window.addEventListener('pointercancel', onFinish)
  }

  function autoFitColumn(id: TableColumnID) {
    const selector = `[data-column-id="${id}"]`
    const header = document.querySelector<HTMLElement>(`th${selector}`)
    const cells = Array.from(document.querySelectorAll<HTMLElement>(`td${selector}`)).slice(0, 500)
    const elements = header ? [header, ...cells] : cells
    const canvas = document.createElement('canvas')
    const context = canvas.getContext('2d')
    let width = DEFAULT_WIDTHS[id]

    if (context) {
      for (const element of elements) {
        const text = (element.textContent ?? '').trim()
        if (!text) continue
        const style = window.getComputedStyle(element)
        context.font = style.font
        const padding = parseFloat(style.paddingLeft || '0') + parseFloat(style.paddingRight || '0')
        width = Math.max(width, Math.ceil(context.measureText(text).width + padding + 28))
      }
    }

    commit({
      ...layout,
      widths: {...layout.widths, [id]: clampColumnWidth(width)},
    })
  }

  const allVisibleSelected = tracks.length > 0 && tracks.every((track) => selectedIDs.includes(track.id))

  return (
    <div className="workspace-table-wrap configurable-track-table">
      <div className="table-columns-toolbar">
        <div
          className="table-preset-picker"
          onMouseEnter={cancelOverlayClose}
          onMouseLeave={schedulePresetClose}
        >
          <button
            type="button"
            className={`table-preset-trigger${presetOpen ? ' active' : ''}`}
            title={copy.presetHint}
            aria-expanded={presetOpen}
            onClick={togglePresetPopup}
          >
            <span aria-hidden="true">▦</span>
            <strong>{copy.presets}</strong>
            {customPresets.length > 0 && <b>{customPresets.length}</b>}
          </button>

          {presetOpen && (
            <div className="table-preset-popover">
              <section className="table-preset-section">
                <strong>{copy.builtInPresets}</strong>
                <div className="table-preset-builtins">
                  {TABLE_PRESET_IDS.map((id) => (
                    <button type="button" key={id} onClick={() => applyPreset(tablePreset(id))}>
                      {copy[id]}
                    </button>
                  ))}
                </div>
              </section>

              <section className="table-preset-section">
                <strong>{copy.customPresets}</strong>

                <div className="table-preset-save">
                  <input
                    value={presetName}
                    maxLength={48}
                    placeholder={copy.presetName}
                    onChange={(event) => setPresetName(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key !== 'Enter') return
                      event.preventDefault()
                      saveCurrentPreset()
                    }}
                  />
                  <button type="button" onClick={saveCurrentPreset} disabled={!presetName.trim()}>
                    {copy.savePreset}
                  </button>
                </div>

                <div className="table-preset-custom-list">
                  {customPresets.length === 0 && (
                    <span className="table-preset-empty">{copy.noCustomPresets}</span>
                  )}
                  {customPresets.map((item) => (
                    <div className="table-preset-custom-row" key={item.id}>
                      <button
                        type="button"
                        className="table-preset-custom-name"
                        title={item.name}
                        onClick={() => applyPreset(item.preset)}
                      >
                        {item.name}
                      </button>
                      <button type="button" onClick={() => updatePreset(item)}>{copy.updatePreset}</button>
                      <button type="button" className="danger-lite" onClick={() => deletePreset(item)}>
                        {copy.deletePreset}
                      </button>
                    </div>
                  ))}
                </div>
              </section>
            </div>
          )}
        </div>

        <details
          ref={columnPickerRef}
          className="table-column-picker"
          onMouseEnter={cancelOverlayClose}
          onMouseLeave={scheduleColumnClose}
          onToggle={(event) => {
            if (event.currentTarget.open) setPresetOpen(false)
          }}
        >
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
              <button type="button" onClick={() => { commit(DEFAULT_LAYOUT); onSortChange([]) }}>{copy.reset}</button>
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
                    <small title={checked ? `${copy.width}: ${columnWidth(layout, id)} px` : undefined}>
                      {checked ? `${columnWidth(layout, id)} px` : ''}
                    </small>
                  </div>
                )
              })}
            </div>
          </div>
        </details>
      </div>

      <table
        className="workspace-table configurable-workspace-table"
        style={{width: `${tableWidth}px`, minWidth: '100%'}}
      >
        <colgroup>
          <col style={{width: `${SELECTION_COLUMN_WIDTH}px`}} />
          {visibleColumns.map((column) => (
            <col key={column} style={{width: `${columnWidth(layout, column)}px`}} />
          ))}
        </colgroup>
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
            {visibleColumns.map((column) => {
              const sortIndex = sort.findIndex((item) => item.column === column)
              const sortRule = sortIndex >= 0 ? sort[sortIndex] : null
              return (
                <th
                  key={column}
                  className={`table-column-header${dragging === column ? ' dragging' : ''}${resizingColumn === column ? ' resizing' : ''}${sortRule ? ' sorted' : ''}`}
                  data-column-id={column}
                  style={{width: `${columnWidth(layout, column)}px`}}
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
                  {column === 'cover' ? (
                    <span className="column-static-label">{LABELS[language][column]}</span>
                  ) : (
                    <button
                      type="button"
                      className="column-sort-button"
                      title={copy.sort}
                      onClick={(event) => onSortChange(nextTableSort(sort, column, event.shiftKey))}
                    >
                      <span>{LABELS[language][column]}</span>
                      {sortRule && (
                        <b className="column-sort-indicator">
                          {sortRule.direction === 'asc' ? '↑' : '↓'}
                          {sort.length > 1 ? sortIndex + 1 : ''}
                        </b>
                      )}
                    </button>
                  )}
                  <i
                    className="column-drag-handle"
                    aria-hidden="true"
                    draggable
                    title={copy.drag}
                    onDragStart={(event) => {
                      setDragging(column)
                      event.dataTransfer.effectAllowed = 'move'
                      event.dataTransfer.setData('text/plain', column)
                    }}
                    onDragEnd={() => setDragging(null)}
                  >⋮⋮</i>
                  <span
                    className="column-resize-handle"
                    role="separator"
                    aria-orientation="vertical"
                    aria-label={`${copy.width}: ${LABELS[language][column]}`}
                    title={copy.resize}
                    onClick={(event) => event.stopPropagation()}
                    onDoubleClick={(event) => {
                      event.preventDefault()
                      event.stopPropagation()
                      autoFitColumn(column)
                    }}
                    onPointerDown={(event) => startColumnResize(event, column)}
                  />
                </th>
              )
            })}
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
                  data-column-id={column}
                  className={cellClass(column)}
                  title={column === 'path' ? track.path : undefined}
                  style={{width: `${columnWidth(layout, column)}px`}}
                >
                  {renderColumn(column, track, unknownArtistLabel, coverRevision)}
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
  const hadCoverColumn = (value.order ?? []).some((candidate) => candidate === 'cover')
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

  if ((value.order?.length ?? 0) > 0 && !hadCoverColumn) {
    const coverIndex = order.indexOf('cover')
    if (coverIndex >= 0) order.splice(coverIndex, 1)
    order.unshift('cover')
  }

  const visible = (value.visible ?? [])
    .filter((candidate): candidate is TableColumnID => typeof candidate === 'string' && isColumnID(candidate))
    .filter((candidate, index, values) => values.indexOf(candidate) === index)

  const widths: Partial<Record<TableColumnID, number>> = {}
  if (value.widths && typeof value.widths === 'object') {
    for (const [key, rawWidth] of Object.entries(value.widths)) {
      if (!isColumnID(key) || typeof rawWidth !== 'number' || !Number.isFinite(rawWidth)) continue
      widths[key] = clampColumnWidth(rawWidth)
    }
  }

  let normalizedVisible = visible.length > 0 ? visible : [...DEFAULT_LAYOUT.visible]
  if ((value.order?.length ?? 0) > 0 && !hadCoverColumn && !normalizedVisible.includes('cover')) {
    normalizedVisible = ['cover', ...normalizedVisible]
  }

  return {
    order,
    visible: normalizedVisible,
    widths,
  }
}

function cloneDefault(): TableLayout {
  return {
    order: [...DEFAULT_LAYOUT.order],
    visible: [...DEFAULT_LAYOUT.visible],
    widths: {...DEFAULT_LAYOUT.widths},
  }
}

function columnWidth(layout: TableLayout, id: TableColumnID): number {
  return clampColumnWidth(layout.widths[id] ?? DEFAULT_WIDTHS[id])
}

function clampColumnWidth(value: number): number {
  return Math.max(MIN_COLUMN_WIDTH, Math.min(MAX_COLUMN_WIDTH, Math.round(value)))
}

function isColumnID(value: string): value is TableColumnID {
  return (ALL_COLUMNS as string[]).includes(value)
}

function renderColumn(id: TableColumnID, track: Track, unknownArtistLabel: string, coverRevision: number): ReactNode {
  switch (id) {
    case 'cover':
      return <TrackCoverCell track={track} revision={coverRevision} />
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
  if (id === 'cover') return 'cover-cell'
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
