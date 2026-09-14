export type DJKeyInfo = {
  camelot: string
  openKey: string
  number: number
  side: 'A' | 'B'
}

const MAJOR_CAMELOT = [8, 3, 10, 5, 12, 7, 2, 9, 4, 11, 6, 1]
const MINOR_CAMELOT = [5, 12, 7, 2, 9, 4, 11, 6, 1, 8, 3, 10]

export function djKeyFormats(key: string, scale: string): DJKeyInfo | null {
  const cleanKey = normalizeKeyToken(key)
  const cleanScale = scale.trim().toLocaleLowerCase()
  if (!cleanKey) return null

  const camelot = parseCamelot(cleanKey)
  if (camelot) return buildInfo(camelot.number, camelot.side)

  const open = parseOpenKey(cleanKey)
  if (open) {
    const number = ((open.number + 6) % 12) + 1
    return buildInfo(number, open.mode === 'm' ? 'A' : 'B')
  }

  const split = splitMusicalKey(cleanKey, cleanScale)
  const pitch = musicalPitchClass(split.note)
  const mode = normalizeMode(split.mode)
  if (pitch === null || !mode) return null

  const number = mode === 'major' ? MAJOR_CAMELOT[pitch] : MINOR_CAMELOT[pitch]
  return buildInfo(number, mode === 'major' ? 'B' : 'A')
}

export function harmonicCamelotKeys(camelot: string): string[] {
  const parsed = parseCamelot(camelot)
  if (!parsed) return []
  const previous = parsed.number === 1 ? 12 : parsed.number - 1
  const next = parsed.number === 12 ? 1 : parsed.number + 1
  const relative: 'A' | 'B' = parsed.side === 'A' ? 'B' : 'A'
  return [
    `${parsed.number}${parsed.side}`,
    `${previous}${parsed.side}`,
    `${next}${parsed.side}`,
    `${parsed.number}${relative}`,
  ]
}

export function djKeySortRank(camelot: string): number | null {
  const parsed = parseCamelot(camelot)
  if (!parsed) return null
  return parsed.number * 10 + (parsed.side === 'A' ? 0 : 1)
}

function buildInfo(number: number, side: 'A' | 'B'): DJKeyInfo {
  const openNumber = ((number + 4) % 12) + 1
  const openMode = side === 'A' ? 'm' : 'd'
  return {
    camelot: `${number}${side}`,
    openKey: `${openNumber}${openMode}`,
    number,
    side,
  }
}

function parseCamelot(value: string): {number: number; side: 'A' | 'B'} | null {
  const match = value.trim().toLocaleUpperCase().match(/^([1-9]|1[0-2])([AB])$/)
  if (!match) return null
  return {number: Number(match[1]), side: match[2] as 'A' | 'B'}
}

function parseOpenKey(value: string): {number: number; mode: 'm' | 'd'} | null {
  const match = value.trim().toLocaleLowerCase().match(/^([1-9]|1[0-2])([md])$/)
  if (!match) return null
  return {number: Number(match[1]), mode: match[2] as 'm' | 'd'}
}

function normalizeKeyToken(value: string): string {
  return value.trim().replaceAll('♯', '#').replaceAll('♭', 'b')
}

function splitMusicalKey(key: string, scale: string): {note: string; mode: string} {
  if (normalizeMode(scale)) return {note: key, mode: scale}

  const lower = key.toLocaleLowerCase()
  const suffixes: Array<[string, 'major' | 'minor']> = [
    [' major', 'major'], [' maj', 'major'], ['major', 'major'], ['maj', 'major'],
    [' minor', 'minor'], [' min', 'minor'], ['minor', 'minor'], ['min', 'minor'],
  ]
  for (const [suffix, mode] of suffixes) {
    if (!lower.endsWith(suffix)) continue
    const note = key.slice(0, key.length - suffix.length).trim()
    if (note) return {note, mode}
  }
  if (key.length >= 2 && key.endsWith('m')) return {note: key.slice(0, -1).trim(), mode: 'minor'}
  return {note: key, mode: scale}
}

function normalizeMode(value: string): 'major' | 'minor' | null {
  switch (value.trim().toLocaleLowerCase()) {
    case 'major':
    case 'maj':
    case 'dur':
    case 'd':
      return 'major'
    case 'minor':
    case 'min':
    case 'moll':
    case 'm':
      return 'minor'
    default:
      return null
  }
}

function musicalPitchClass(value: string): number | null {
  const note = normalizeKeyToken(value)
  if (!note) return null
  const normalized = note.charAt(0).toLocaleUpperCase() + note.slice(1)
  const pitches: Record<string, number> = {
    C: 0, 'B#': 0,
    'C#': 1, Db: 1,
    D: 2,
    'D#': 3, Eb: 3,
    E: 4, Fb: 4,
    F: 5, 'E#': 5,
    'F#': 6, Gb: 6,
    G: 7,
    'G#': 8, Ab: 8,
    A: 9,
    'A#': 10, Bb: 10,
    B: 11, Cb: 11,
  }
  return Object.prototype.hasOwnProperty.call(pitches, normalized) ? pitches[normalized] : null
}
