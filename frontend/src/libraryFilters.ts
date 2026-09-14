import type {Track} from './types'
import {djKeyFormats} from './djKey'

export type CoverFilter = 'any' | 'with' | 'without'

export type LibraryFilters = {
  cover: CoverFilter
  artists: string[]
  genres: string[]
  labels: string[]
  codecs: string[]
  keys: string[]
  camelotKeys: string[]
  yearMin: number | null
  yearMax: number | null
  bpmMin: number | null
  bpmMax: number | null
  lufsMin: number | null
  lufsMax: number | null
}

export type LibraryFilterOptions = {
  artists: string[]
  genres: string[]
  labels: string[]
  codecs: string[]
  keys: string[]
  camelotKeys: string[]
}

const STORAGE_KEY = 'ccml.library-filters.v1'

export function createEmptyLibraryFilters(): LibraryFilters {
  return {
    cover: 'any',
    artists: [],
    genres: [],
    labels: [],
    codecs: [],
    keys: [],
    camelotKeys: [],
    yearMin: null,
    yearMax: null,
    bpmMin: null,
    bpmMax: null,
    lufsMin: null,
    lufsMax: null,
  }
}

export function loadLibraryFilters(): LibraryFilters {
  if (typeof window === 'undefined') return createEmptyLibraryFilters()

  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    if (!raw) return createEmptyLibraryFilters()

    const value: unknown = JSON.parse(raw)
    if (!value || typeof value !== 'object') return createEmptyLibraryFilters()
    const source = value as Record<string, unknown>

    return normalizeLibraryFilters({
      cover: readCoverFilter(source.cover),
      artists: readStringArray(source.artists),
      genres: readStringArray(source.genres),
      labels: readStringArray(source.labels),
      codecs: readStringArray(source.codecs),
      keys: readStringArray(source.keys),
      camelotKeys: readStringArray(source.camelotKeys),
      yearMin: readNullableNumber(source.yearMin),
      yearMax: readNullableNumber(source.yearMax),
      bpmMin: readNullableNumber(source.bpmMin),
      bpmMax: readNullableNumber(source.bpmMax),
      lufsMin: readNullableNumber(source.lufsMin),
      lufsMax: readNullableNumber(source.lufsMax),
    })
  } catch {
    return createEmptyLibraryFilters()
  }
}

export function saveLibraryFilters(filters: LibraryFilters): void {
  if (typeof window === 'undefined') return

  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(normalizeLibraryFilters(filters)))
  } catch {
    // Filters remain active for the current session if storage is unavailable.
  }
}

export function normalizeLibraryFilters(filters: LibraryFilters): LibraryFilters {
  const normalized = {
    cover: readCoverFilter(filters.cover),
    artists: uniqueStrings(filters.artists),
    genres: uniqueStrings(filters.genres),
    labels: uniqueStrings(filters.labels),
    codecs: uniqueStrings(filters.codecs),
    keys: uniqueStrings(filters.keys),
    camelotKeys: uniqueStrings(filters.camelotKeys),
    yearMin: finiteOrNull(filters.yearMin),
    yearMax: finiteOrNull(filters.yearMax),
    bpmMin: finiteOrNull(filters.bpmMin),
    bpmMax: finiteOrNull(filters.bpmMax),
    lufsMin: finiteOrNull(filters.lufsMin),
    lufsMax: finiteOrNull(filters.lufsMax),
  }

  if (normalized.yearMin !== null && normalized.yearMax !== null && normalized.yearMin > normalized.yearMax) {
    ;[normalized.yearMin, normalized.yearMax] = [normalized.yearMax, normalized.yearMin]
  }
  if (normalized.bpmMin !== null && normalized.bpmMax !== null && normalized.bpmMin > normalized.bpmMax) {
    ;[normalized.bpmMin, normalized.bpmMax] = [normalized.bpmMax, normalized.bpmMin]
  }
  if (normalized.lufsMin !== null && normalized.lufsMax !== null && normalized.lufsMin > normalized.lufsMax) {
    ;[normalized.lufsMin, normalized.lufsMax] = [normalized.lufsMax, normalized.lufsMin]
  }

  return normalized
}

export function countActiveLibraryFilters(filters: LibraryFilters): number {
  let count = 0
  if (filters.cover !== 'any') count++
  if (filters.artists.length > 0) count++
  if (filters.genres.length > 0) count++
  if (filters.labels.length > 0) count++
  if (filters.codecs.length > 0) count++
  if (filters.keys.length > 0) count++
  if (filters.camelotKeys.length > 0) count++
  if (filters.yearMin !== null || filters.yearMax !== null) count++
  if (filters.bpmMin !== null || filters.bpmMax !== null) count++
  if (filters.lufsMin !== null || filters.lufsMax !== null) count++
  return count
}

export function applyLibraryFilters(tracks: Track[], filters: LibraryFilters): Track[] {
  if (countActiveLibraryFilters(filters) === 0) return tracks

  return tracks.filter((track) => {
    if (filters.cover !== 'any') {
      if (!track.coverIndexed) return false
      if (filters.cover === 'with' && !track.hasCover) return false
      if (filters.cover === 'without' && track.hasCover) return false
    }

    if (filters.artists.length > 0 && !matchesSingle(track.artist, filters.artists)) return false

    if (filters.genres.length > 0) {
      const trackGenres = splitGenre(track.genre)
      if (!matchesAnyToken(trackGenres, filters.genres)) return false
    }

    if (filters.labels.length > 0 && !matchesSingle(track.label, filters.labels)) return false

    if (filters.codecs.length > 0) {
      const codec = normalizedCodec(track)
      if (!matchesSingle(codec, filters.codecs)) return false
    }

    if (filters.keys.length > 0) {
      const key = trackKey(track)
      if (!matchesSingle(key, filters.keys)) return false
    }

    if (filters.camelotKeys.length > 0) {
      const key = djKeyFormats(track.key, track.keyScale)
      if (!key || !matchesSingle(key.camelot, filters.camelotKeys)) return false
    }

    if (!matchesNumericRange(track.year > 0 ? track.year : null, filters.yearMin, filters.yearMax)) return false
    if (!matchesNumericRange(track.bpm > 0 ? track.bpm : null, filters.bpmMin, filters.bpmMax)) return false

    const lufs = track.loudnessI !== 0 ? track.loudnessI : null
    if (!matchesNumericRange(lufs, filters.lufsMin, filters.lufsMax)) return false

    return true
  })
}

export function buildLibraryFilterOptions(tracks: Track[], locale: string): LibraryFilterOptions {
  const artists = new Set<string>()
  const genres = new Set<string>()
  const labels = new Set<string>()
  const codecs = new Set<string>()
  const keys = new Set<string>()
  const camelotKeys = new Set<string>()

  for (const track of tracks) {
    addNonEmpty(artists, track.artist)

    for (const genre of splitGenre(track.genre)) {
      addNonEmpty(genres, genre)
    }

    addNonEmpty(labels, track.label)
    addNonEmpty(codecs, normalizedCodec(track))
    addNonEmpty(keys, trackKey(track))
    const djKey = djKeyFormats(track.key, track.keyScale)
    if (djKey) addNonEmpty(camelotKeys, djKey.camelot)
  }

  const collator = new Intl.Collator(locale, {numeric: true, sensitivity: 'base'})
  const sorted = (values: Set<string>) => Array.from(values).sort((a, b) => collator.compare(a, b))

  return {
    artists: sorted(artists),
    genres: sorted(genres),
    labels: sorted(labels),
    codecs: sorted(codecs),
    keys: sorted(keys),
    camelotKeys: sorted(camelotKeys),
  }
}

export function trackKey(track: Track): string {
  const key = track.key.trim()
  const scale = track.keyScale.trim()
  if (!key) return ''
  return scale ? `${key} ${scale}` : key
}

function normalizedCodec(track: Track): string {
  const value = (track.codec || track.extension.replace(/^\./, '')).trim()
  return value.toLowerCase()
}

function splitGenre(value: string): string[] {
  return value
    .split(/[;/|]+/)
    .map((item) => item.trim())
    .filter(Boolean)
}

function matchesSingle(value: string, selected: string[]): boolean {
  if (!value.trim()) return false
  const normalized = fold(value)
  return selected.some((item) => fold(item) === normalized)
}

function matchesAnyToken(values: string[], selected: string[]): boolean {
  if (values.length === 0) return false
  const selectedSet = new Set(selected.map(fold))
  return values.some((value) => selectedSet.has(fold(value)))
}

function matchesNumericRange(value: number | null, min: number | null, max: number | null): boolean {
  if (min === null && max === null) return true
  if (value === null || !Number.isFinite(value)) return false
  if (min !== null && value < min) return false
  if (max !== null && value > max) return false
  return true
}

function addNonEmpty(target: Set<string>, value: string): void {
  const clean = value.trim()
  if (clean) target.add(clean)
}

function fold(value: string): string {
  return value.trim().toLocaleLowerCase()
}

function finiteOrNull(value: number | null): number | null {
  return value !== null && Number.isFinite(value) ? value : null
}

function readCoverFilter(value: unknown): CoverFilter {
  return value === 'with' || value === 'without' ? value : 'any'
}

function readNullableNumber(value: unknown): number | null {
  return typeof value === 'number' && Number.isFinite(value) ? value : null
}

function readStringArray(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  return value.filter((item): item is string => typeof item === 'string')
}

function uniqueStrings(values: string[]): string[] {
  const seen = new Set<string>()
  const result: string[] = []

  for (const value of values) {
    const clean = value.trim()
    if (!clean) continue
    const key = fold(clean)
    if (seen.has(key)) continue
    seen.add(key)
    result.push(clean)
  }

  return result
}
