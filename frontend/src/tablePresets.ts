import type {TableColumnID, TableSort} from './tableSort'

export type TablePresetID = 'dj' | 'metadata' | 'technical' | 'compact'

export type TablePreset = {
  layout: {
    order: TableColumnID[]
    visible: TableColumnID[]
    widths: Partial<Record<TableColumnID, number>>
  }
  sort: TableSort[]
}

export type CustomTablePreset = {
  id: string
  name: string
  preset: TablePreset
  updatedAt: string
}

export const TABLE_PRESET_IDS: TablePresetID[] = [
  'dj',
  'metadata',
  'technical',
  'compact',
]

const STORAGE_KEY = 'ccml.table-presets.v1'

const ALL_COLUMNS: TableColumnID[] = [
  'cover', 'trackNumber', 'artist', 'title', 'album', 'albumArtist',
  'year', 'genre', 'label', 'catalogNumber', 'releaseDate',
  'duration', 'codec', 'sampleRate', 'bitRate', 'channels',
  'lufs', 'truePeak', 'bpm', 'musicalKey', 'camelot', 'bpmKey',
  'isrc', 'fileName', 'path',
]

export function tablePreset(id: TablePresetID): TablePreset {
  switch (id) {
    case 'dj':
      return preset(
        ['cover', 'artist', 'title', 'bpm', 'camelot', 'musicalKey', 'genre', 'lufs', 'truePeak', 'duration', 'codec'],
        {
          cover: 54,
          artist: 180,
          title: 250,
          bpm: 74,
          camelot: 92,
          musicalKey: 108,
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
      )

    case 'metadata':
      return preset(
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
      )

    case 'technical':
      return preset(
        ['artist', 'title', 'codec', 'sampleRate', 'bitRate', 'channels', 'lufs', 'truePeak', 'bpm', 'camelot', 'musicalKey', 'duration', 'path'],
        {
          artist: 170,
          title: 220,
          codec: 82,
          sampleRate: 96,
          bitRate: 94,
          channels: 78,
          lufs: 72,
          truePeak: 90,
          bpm: 72,
          camelot: 90,
          musicalKey: 108,
          duration: 76,
          path: 360,
        },
        [
          {column: 'codec', direction: 'asc'},
          {column: 'artist', direction: 'asc'},
          {column: 'title', direction: 'asc'},
        ],
      )

    case 'compact':
      return preset(
        ['cover', 'artist', 'title', 'bpm', 'camelot', 'duration'],
        {
          cover: 52,
          artist: 180,
          title: 270,
          bpm: 72,
          camelot: 90,
          duration: 74,
        },
        [
          {column: 'artist', direction: 'asc'},
          {column: 'title', direction: 'asc'},
        ],
      )
  }
}

export function loadCustomTablePresets(): CustomTablePreset[] {
  if (typeof window === 'undefined') return []

  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    if (!raw) return []

    const value: unknown = JSON.parse(raw)
    if (!Array.isArray(value)) return []

    return value
      .map(readCustomPreset)
      .filter((item): item is CustomTablePreset => item !== null)
      .slice(0, 30)
  } catch {
    return []
  }
}

export function saveCustomTablePresets(presets: CustomTablePreset[]): void {
  if (typeof window === 'undefined') return

  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(presets.slice(0, 30)))
  } catch {
    // Presets remain available in memory if WebView storage is unavailable.
  }
}

export function addCustomTablePreset(
  current: CustomTablePreset[],
  name: string,
  value: TablePreset,
): CustomTablePreset[] {
  const cleanName = normalizeName(name)
  if (!cleanName) return current

  const now = new Date().toISOString()
  const next: CustomTablePreset = {
    id: makeID(current),
    name: cleanName,
    preset: sanitizePreset(value),
    updatedAt: now,
  }

  return [...current, next].slice(-30)
}

export function updateCustomTablePreset(
  current: CustomTablePreset[],
  id: string,
  value: TablePreset,
): CustomTablePreset[] {
  const now = new Date().toISOString()

  return current.map((item) => (
    item.id === id
      ? {...item, preset: sanitizePreset(value), updatedAt: now}
      : item
  ))
}

export function removeCustomTablePreset(
  current: CustomTablePreset[],
  id: string,
): CustomTablePreset[] {
  return current.filter((item) => item.id !== id)
}

export function sanitizePreset(value: TablePreset): TablePreset {
  const order = uniqueColumns(value.layout?.order)
  const visible = uniqueColumns(value.layout?.visible)

  const normalizedOrder = order.length > 0
    ? [...order, ...ALL_COLUMNS.filter((column) => !order.includes(column))]
    : [...ALL_COLUMNS]

  const normalizedVisible: TableColumnID[] = visible.length > 0
    ? visible
    : ['artist', 'title']

  const widths: Partial<Record<TableColumnID, number>> = {}
  if (value.layout?.widths && typeof value.layout.widths === 'object') {
    for (const [key, rawWidth] of Object.entries(value.layout.widths)) {
      if (!isColumnID(key) || typeof rawWidth !== 'number' || !Number.isFinite(rawWidth)) continue
      widths[key] = Math.max(54, Math.min(640, Math.round(rawWidth)))
    }
  }

  return {
    layout: {
      order: normalizedOrder,
      visible: normalizedVisible,
      widths,
    },
    sort: sanitizeSort(value.sort),
  }
}

function preset(
  visible: TableColumnID[],
  widths: Partial<Record<TableColumnID, number>>,
  sort: TableSort[],
): TablePreset {
  const order = [...visible, ...ALL_COLUMNS.filter((column) => !visible.includes(column))]
  return sanitizePreset({
    layout: {
      order,
      visible: [...visible],
      widths: {...widths},
    },
    sort: sort.map((rule) => ({...rule})),
  })
}

function readCustomPreset(value: unknown): CustomTablePreset | null {
  if (!value || typeof value !== 'object') return null
  const source = value as Record<string, unknown>

  if (typeof source.id !== 'string' || typeof source.name !== 'string') return null
  if (!source.preset || typeof source.preset !== 'object') return null

  const name = normalizeName(source.name)
  if (!name) return null

  return {
    id: source.id,
    name,
    preset: sanitizePreset(source.preset as TablePreset),
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
      (item as TableSort).column !== 'cover' &&
      ((item as TableSort).direction === 'asc' || (item as TableSort).direction === 'desc')
    ))
    .slice(0, 4)
    .map((item) => ({...item}))
}

function uniqueColumns(value: unknown): TableColumnID[] {
  if (!Array.isArray(value)) return []

  const result: TableColumnID[] = []
  const seen = new Set<TableColumnID>()

  for (const item of value) {
    if (!isColumnID(item) || seen.has(item)) continue
    seen.add(item)
    result.push(item)
  }

  return result
}

function isColumnID(value: unknown): value is TableColumnID {
  return typeof value === 'string' && (ALL_COLUMNS as string[]).includes(value)
}

function normalizeName(value: string): string {
  return value.trim().replace(/\s+/g, ' ').slice(0, 48)
}

function makeID(current: CustomTablePreset[]): string {
  const base = `preset-${Date.now().toString(36)}`
  const ids = new Set(current.map((item) => item.id))

  if (!ids.has(base)) return base

  let suffix = 2
  while (ids.has(`${base}-${suffix}`)) suffix += 1
  return `${base}-${suffix}`
}
