import type { Track } from './types'

export type TableColumnID =
  | 'cover' | 'trackNumber' | 'artist' | 'title' | 'album' | 'albumArtist'
  | 'year' | 'genre' | 'label' | 'catalogNumber' | 'releaseDate'
  | 'duration' | 'codec' | 'sampleRate' | 'bitRate' | 'channels'
  | 'lufs' | 'truePeak' | 'bpmKey' | 'isrc' | 'fileName' | 'path'

export type TableSort = {
  column: TableColumnID
  direction: 'asc' | 'desc'
}

const STORAGE_KEY = 'ccml.table-sort.v1'

export function loadTableSort(): TableSort[] {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    if (!raw) return []
    const value = JSON.parse(raw)
    if (!Array.isArray(value)) return []

    return value
      .filter((item): item is TableSort => (
        item &&
        typeof item === 'object' &&
        isColumnID(item.column) &&
        (item.direction === 'asc' || item.direction === 'desc')
      ))
      .slice(0, 4)
  } catch {
    return []
  }
}

export function saveTableSort(sort: TableSort[]): void {
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(sort.slice(0, 4)))
  } catch {
    // Sorting still works for this session when WebView storage is unavailable.
  }
}

export function nextTableSort(current: TableSort[], column: TableColumnID, multi: boolean): TableSort[] {
  const existingIndex = current.findIndex((item) => item.column === column)

  if (!multi) {
    if (existingIndex === 0 && current.length === 1) {
      return [{column, direction: current[0].direction === 'asc' ? 'desc' : 'asc'}]
    }
    return [{column, direction: 'asc'}]
  }

  const next = [...current]
  if (existingIndex >= 0) {
    next[existingIndex] = {
      column,
      direction: next[existingIndex].direction === 'asc' ? 'desc' : 'asc',
    }
    return next
  }

  const added: TableSort = {column, direction: 'asc'}
  return [...next, added].slice(0, 4)
}

export function sortTracks(tracks: Track[], sort: TableSort[], locale: string): Track[] {
  if (sort.length === 0 || tracks.length < 2) return tracks

  const collator = new Intl.Collator(locale, {numeric: true, sensitivity: 'base'})
  return tracks
    .map((track, index) => ({track, index}))
    .sort((a, b) => {
      for (const rule of sort) {
        const left = sortValue(a.track, rule.column)
        const right = sortValue(b.track, rule.column)

        const leftEmpty = left === null || left === ''
        const rightEmpty = right === null || right === ''
        if (leftEmpty || rightEmpty) {
          if (leftEmpty && rightEmpty) continue
          return leftEmpty ? 1 : -1
        }

        let compared = 0
        if (typeof left === 'number' && typeof right === 'number') {
          compared = left - right
        } else {
          compared = collator.compare(String(left), String(right))
        }

        if (compared !== 0) return rule.direction === 'asc' ? compared : -compared
      }

      return a.index - b.index
    })
    .map(({track}) => track)
}

function sortValue(track: Track, column: TableColumnID): string | number | null {
  switch (column) {
    case 'cover': return null
    case 'trackNumber': return track.trackNumber > 0 ? track.trackNumber : null
    case 'artist': return clean(track.artist)
    case 'title': return clean(track.title || track.fileName)
    case 'album': return clean(track.album)
    case 'albumArtist': return clean(track.albumArtist)
    case 'year': return track.year > 0 ? track.year : null
    case 'genre': return clean(track.genre)
    case 'label': return clean(track.label)
    case 'catalogNumber': return clean(track.catalogNumber)
    case 'releaseDate': return clean(track.releaseDate)
    case 'duration': return track.durationMs > 0 ? track.durationMs : null
    case 'codec': return clean(track.codec || track.extension)
    case 'sampleRate': return track.sampleRate > 0 ? track.sampleRate : null
    case 'bitRate': return track.bitRate > 0 ? track.bitRate : null
    case 'channels': return track.channels > 0 ? track.channels : null
    case 'lufs': return track.loudnessI !== 0 ? track.loudnessI : null
    case 'truePeak': return track.truePeak !== 0 ? track.truePeak : null
    case 'bpmKey': return track.bpm > 0 ? track.bpm : clean(track.key)
    case 'isrc': return clean(track.isrc)
    case 'fileName': return clean(track.fileName)
    case 'path': return clean(track.path)
  }
}

function clean(value: string): string | null {
  const normalized = value.trim()
  return normalized === '' ? null : normalized
}

function isColumnID(value: unknown): value is TableColumnID {
  return typeof value === 'string' && [
    'cover', 'trackNumber', 'artist', 'title', 'album', 'albumArtist', 'year', 'genre',
    'label', 'catalogNumber', 'releaseDate', 'duration', 'codec', 'sampleRate',
    'bitRate', 'channels', 'lufs', 'truePeak', 'bpmKey', 'isrc', 'fileName', 'path',
  ].includes(value)
}