import {createEmptyLibraryFilters, normalizeLibraryFilters, type LibraryFilters} from './libraryFilters'
import type {TableColumnID, TableSort} from './tableSort'

export type MetadataStatusFilter = 'all' | 'skipped' | 'failed'

export type SavedTableLayout = {
  order: TableColumnID[]
  visible: TableColumnID[]
  widths: Partial<Record<TableColumnID, number>>
}

export type TableViewSnapshot = {
  layout: SavedTableLayout
  sort: TableSort[]
  filters: LibraryFilters
  metadataFilter: MetadataStatusFilter
}

export type CustomTableView = {
  id: string
  name: string
  snapshot: TableViewSnapshot
  updatedAt: string
}

export type BuiltInTableViewID = 'dj' | 'metadata' | 'technical' | 'compact'

export const BUILTIN_TABLE_VIEW_IDS: BuiltInTableViewID[] = [
  'dj',
  'metadata',
  'technical',
  'compact',
]

const STORAGE_KEY = 'ccml.table-views.v1'

const ALL_COLUMNS: TableColumnID[] = [
  'cover', 'trackNumber', 'artist', 'title', 'album', 'albumArtist',
  'year', 'genre', 'label', 'catalogNumber', 'releaseDate',
  'duration', 'codec', 'sampleRate', 'bitRate', 'channels',
  'lufs', 'truePeak', 'bpmKey', 'isrc', 'fileName', 'path',
]

export function builtInTableView(id: BuiltInTableViewID): TableViewSnapshot {
  const emptyFilters = createEmptyLibraryFilters()

  switch (id) {
    case 'dj':
      return snapshot(
        ['cover', 'artist', 'title', 'bpmKey', 'genre', 'lufs', 'truePeak', 'duration', 'codec'],
        {
          cover: 54,
          artist: 180,
          title: 260,
          bpmKey: 112,
          genre: 150,
          lufs: 72,
          truePeak: 90,
          duration: 76,
          codec: 72,
        },
        [
          {column: 'artist', direction: 'asc'},
          {column: 'title', direction: 'asc'},
        ],
        emptyFilters,
      )

    case 'metadata':
      return snapshot(
        ['cover', 'artist', 'title', 'album', 'albumArtist', 'label', 'catalogNumber', 'releaseDate', 'isrc', 'genre', 'year'],
        {
          cover: 54,
          artist: 170,
          title: 220,
          album: 180,
          albumArtist: 170,
          label: 150,
          catalogNumber: 120,
          releaseDate: 108,
          isrc: 132,
          genre: 130,
          year: 66,
        },
        [
          {column: 'artist', direction: 'asc'},
          {column: 'album', direction: 'asc'},
          {column: 'trackNumber', direction: 'asc'},
        ],
        emptyFilters,
      )

    case 'technical':
      return snapshot(
        ['artist', 'title', 'codec', 'sampleRate', 'bitRate', 'channels', 'lufs', 'truePeak', 'bpmKey', 'duration', 'path'],
        {
          artist: 170,
          title: 220,
          codec: 82,
          sampleRate: 96,
          bitRate: 94,
          channels: 78,
          lufs: 72,
          truePeak: 90,
          bpmKey: 110,
          duration: 76,
          path: 360,
        },
        [
          {column: 'codec', direction: 'asc'},
          {column: 'artist', direction: 'asc'},
          {column: 'title', direction: 'asc'},
        ],
        emptyFilters,
      )

    case 'compact':
      return snapshot(
        ['cover', 'artist', 'title', 'bpmKey', 'duration'],
        {
          cover: 52,
          artist: 180,
          title: 280,
          bpmKey: 108,
          duration: 74,
        },
        [
          {column: 'artist', direction: 'asc'},
          {column: 'title', direction: 'asc'},
        ],
        emptyFilters,
      )
  }
}

export function loadCustomTableViews(): CustomTableView[] {
  if (typeof window === 'undefined') return []

  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    if (!raw) return []

    const parsed: unknown = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []

    return parsed
      .map(readCustomView)
      .filter((value): value is CustomTableView => value !== null)
      .slice(0, 40)
  } catch {
    return []
  }
}

export function saveCustomTableViews(views: CustomTableView[]): void {
  if (typeof window === 'undefined') return

  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(views.slice(0, 40)))
  } catch {
    // Views remain usable for this session when localStorage is unavailable.
  }
}

export function addCustomTableView(
  current: CustomTableView[],
  name: string,
  snapshotValue: TableViewSnapshot,
): CustomTableView[] {
  const cleanName = cleanViewName(name)
  if (!cleanName) return current

  const now = new Date().toISOString()
  const id = makeViewID(current)
  return [
    ...current,
    {
      id,
      name: cleanName,
      snapshot: sanitizeSnapshot(snapshotValue),
      updatedAt: now,
    },
  ].slice(-40)
}

export function replaceCustomTableView(
  current: CustomTableView[],
  id: string,
  snapshotValue: TableViewSnapshot,
): CustomTableView[] {
  const now = new Date().toISOString()
  return current.map((view) => (
    view.id === id
      ? {...view, snapshot: sanitizeSnapshot(snapshotValue), updatedAt: now}
      : view
  ))
}

export function removeCustomTableView(current: CustomTableView[], id: string): CustomTableView[] {
  return current.filter((view) => view.id !== id)
}

export function sanitizeSnapshot(value: TableViewSnapshot): TableViewSnapshot {
  const order = uniqueColumns(value.layout?.order)
  const visible = uniqueColumns(value.layout?.visible)
  const widths: Partial<Record<TableColumnID, number>> = {}

  if (value.layout?.widths && typeof value.layout.widths === 'object') {
    for (const [key, width] of Object.entries(value.layout.widths)) {
      if (!isColumnID(key) || typeof width !== 'number' || !Number.isFinite(width)) continue
      widths[key] = Math.max(54, Math.min(640, Math.round(width)))
    }
  }

  const normalizedOrder = order.length > 0
    ? [...order, ...ALL_COLUMNS.filter((column) => !order.includes(column))]
    : [...ALL_COLUMNS]

  const normalizedVisible: TableColumnID[] = visible.length > 0 ? visible : ['artist', 'title']

  return {
    layout: {
      order: normalizedOrder,
      visible: normalizedVisible,
      widths,
    },
    sort: sanitizeSort(value.sort),
    filters: normalizeLibraryFilters(value.filters ?? createEmptyLibraryFilters()),
    metadataFilter: readMetadataFilter(value.metadataFilter),
  }
}

function snapshot(
  visible: TableColumnID[],
  widths: Partial<Record<TableColumnID, number>>,
  sort: TableSort[],
  filters: LibraryFilters,
): TableViewSnapshot {
  const order = [...visible, ...ALL_COLUMNS.filter((column) => !visible.includes(column))]
  return sanitizeSnapshot({
    layout: {order, visible, widths},
    sort,
    filters,
    metadataFilter: 'all',
  })
}

function readCustomView(value: unknown): CustomTableView | null {
  if (!value || typeof value !== 'object') return null
  const source = value as Record<string, unknown>

  if (typeof source.id !== 'string' || typeof source.name !== 'string') return null
  if (!source.snapshot || typeof source.snapshot !== 'object') return null

  const snapshotSource = source.snapshot as TableViewSnapshot
  const name = cleanViewName(source.name)
  if (!name) return null

  return {
    id: source.id,
    name,
    snapshot: sanitizeSnapshot(snapshotSource),
    updatedAt: typeof source.updatedAt === 'string' ? source.updatedAt : '',
  }
}

function sanitizeSort(value: unknown): TableSort[] {
  if (!Array.isArray(value)) return []

  return value
    .filter((item): item is TableSort => (
      Boolean(item) &&
      typeof item === 'object' &&
      isColumnID((item as TableSort).column) &&
      ((item as TableSort).direction === 'asc' || (item as TableSort).direction === 'desc') &&
      (item as TableSort).column !== 'cover'
    ))
    .slice(0, 4)
}

function uniqueColumns(value: unknown): TableColumnID[] {
  if (!Array.isArray(value)) return []

  const seen = new Set<TableColumnID>()
  const result: TableColumnID[] = []

  for (const item of value) {
    if (!isColumnID(item) || seen.has(item)) continue
    seen.add(item)
    result.push(item)
  }

  return result
}

function readMetadataFilter(value: unknown): MetadataStatusFilter {
  return value === 'skipped' || value === 'failed' ? value : 'all'
}

function isColumnID(value: unknown): value is TableColumnID {
  return typeof value === 'string' && (ALL_COLUMNS as string[]).includes(value)
}

function cleanViewName(value: string): string {
  return value.trim().replace(/\s+/g, ' ').slice(0, 48)
}

function makeViewID(current: CustomTableView[]): string {
  const prefix = `view-${Date.now().toString(36)}`
  let id = prefix
  let suffix = 2
  const existing = new Set(current.map((view) => view.id))

  while (existing.has(id)) {
    id = `${prefix}-${suffix}`
    suffix += 1
  }

  return id
}
