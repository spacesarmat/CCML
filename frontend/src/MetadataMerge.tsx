import { useEffect, useMemo, useState } from 'react'
import { translate, type AppLanguage, type TranslationKey } from './i18n'
import type { MetadataCandidate, MetadataFieldOption, MetadataLookupResult, MetadataScore, Track } from './types'

type Props = {
  language: AppLanguage
  lookup: MetadataLookupResult
  current: Track | null
  disabled: boolean
  onApply: (candidate: MetadataCandidate, includeArtwork: boolean) => Promise<void>
}

type MergeField =
  | 'title' | 'artist' | 'album' | 'albumArtist' | 'genre'
  | 'releaseDate' | 'year' | 'label' | 'catalogNumber' | 'isrc'
  | 'trackNumber' | 'trackTotal' | 'discNumber' | 'discTotal'
  | 'bpm' | 'key' | 'keyScale'

type MergeNumericKind = 'int' | 'float'

const fields: Array<{field: MergeField; label: TranslationKey; numeric?: MergeNumericKind}> = [
  {field: 'title', label: 'tags.field.title'},
  {field: 'artist', label: 'tags.field.artist'},
  {field: 'album', label: 'tags.field.album'},
  {field: 'albumArtist', label: 'tags.field.albumArtist'},
  {field: 'genre', label: 'tags.field.genre'},
  {field: 'releaseDate', label: 'tags.field.releaseDate'},
  {field: 'year', label: 'tags.field.year', numeric: 'int'},
  {field: 'label', label: 'tags.field.label'},
  {field: 'catalogNumber', label: 'tags.field.catalogNumber'},
  {field: 'isrc', label: 'tags.field.isrc'},
  {field: 'trackNumber', label: 'tags.field.track', numeric: 'int'},
  {field: 'trackTotal', label: 'tags.field.trackTotal', numeric: 'int'},
  {field: 'discNumber', label: 'tags.field.disc', numeric: 'int'},
  {field: 'discTotal', label: 'tags.field.discTotal', numeric: 'int'},
  {field: 'bpm', label: 'tags.field.bpm', numeric: 'float'},
  {field: 'key', label: 'tags.field.key'},
  {field: 'keyScale', label: 'tags.field.keyScale'},
]

const emptyScore: MetadataScore = {
  title: 0, artist: 0, album: 0, version: 0, duration: 0, identifier: 0, completeness: 0, total: 0,
}

const emptyCandidate: MetadataCandidate = {
  source: 'CCML Merge', sourceKind: 'catalog', externalId: '', sourceUrl: '', title: '', artist: '', album: '', albumArtist: '',
  releaseDate: '', year: 0, genre: '', label: '', catalogNumber: '', isrc: '', trackNumber: 0, trackTotal: 0,
  discNumber: 0, discTotal: 0, bpm: 0, key: '', keyScale: '', artworkUrl: '', artworkWidth: 0, artworkHeight: 0, artworkEmbeddable: false,
  durationMs: 0, confidence: 0, matchClass: 'rejected', matchIssues: [], score: emptyScore,
}

function candidateValue(candidate: MetadataCandidate, field: MergeField, numeric?: MergeNumericKind): string | number {
  const value = candidate[field]
  return numeric ? Number(value ?? 0) : String(value ?? '')
}

function optionValue(option: MetadataFieldOption, numeric?: MergeNumericKind): string | number {
  if (numeric === 'int') return option.number
  if (numeric === 'float') return option.decimal ?? 0
  return option.value
}

function buildFallbackOptions(candidates: MetadataCandidate[]): MetadataFieldOption[] {
  const result: MetadataFieldOption[] = []
  const seen = new Set<string>()
  for (const candidate of candidates) {
    for (const {field, numeric} of fields) {
      const raw = candidateValue(candidate, field, numeric)
      if (numeric ? Number(raw) <= 0 : String(raw).trim() === '') continue
      const key = `${field}\u0000${String(raw).trim().toLowerCase()}`
      if (seen.has(key)) continue
      seen.add(key)
      result.push({
        field,
        value: numeric ? '' : String(raw),
        number: numeric === 'int' ? Number(raw) : 0,
        decimal: numeric === 'float' ? Number(raw) : 0,
        source: candidate.source,
        externalId: candidate.externalId,
        confidence: candidate.confidence,
      })
    }
  }
  return result
}

export default function MetadataMerge({language, lookup, current, disabled, onApply}: Props) {
  const t = (key: TranslationKey) => translate(language, key)
  const candidates = lookup.candidates ?? []
  const suggested = lookup.suggested ?? candidates[0] ?? emptyCandidate
  const optionsSource = (lookup.fieldOptions?.length ?? 0) > 0
    ? lookup.fieldOptions ?? []
    : buildFallbackOptions(candidates)

  const grouped = useMemo(() => {
    const result = new Map<string, MetadataFieldOption[]>()
    for (const option of optionsSource) {
      const list = result.get(option.field) ?? []
      list.push(option)
      result.set(option.field, list)
    }
    return result
  }, [lookup, candidates.length, optionsSource.length])

  const [selected, setSelected] = useState<Record<string, number>>({})

  useEffect(() => {
    const defaults: Record<string, number> = {}
    for (const {field, numeric} of fields) {
      const options = grouped.get(field) ?? []
      const suggestedValue = candidateValue(suggested, field, numeric)
      const index = options.findIndex((option) => optionValue(option, numeric) === suggestedValue)
      defaults[field] = index >= 0 ? index : 0
    }
    setSelected(defaults)
  }, [lookup, grouped, suggested])

  function composedCandidate(): MetadataCandidate {
    const candidate: MetadataCandidate = {...emptyCandidate, ...suggested, score: suggested.score ?? emptyScore, source: 'CCML Merge', externalId: '', sourceUrl: ''}
    for (const {field, numeric} of fields) {
      const options = grouped.get(field) ?? []
      const option = options[selected[field] ?? 0]
      if (!option) continue
      const value = optionValue(option, numeric)
      switch (field) {
        case 'title': candidate.title = String(value); break
        case 'artist': candidate.artist = String(value); break
        case 'album': candidate.album = String(value); break
        case 'albumArtist': candidate.albumArtist = String(value); break
        case 'genre': candidate.genre = String(value); break
        case 'releaseDate': candidate.releaseDate = String(value); break
        case 'year': candidate.year = Number(value); break
        case 'label': candidate.label = String(value); break
        case 'catalogNumber': candidate.catalogNumber = String(value); break
        case 'isrc': candidate.isrc = String(value); break
        case 'trackNumber': candidate.trackNumber = Number(value); break
        case 'trackTotal': candidate.trackTotal = Number(value); break
        case 'discNumber': candidate.discNumber = Number(value); break
        case 'discTotal': candidate.discTotal = Number(value); break
        case 'bpm': candidate.bpm = Number(value); break
        case 'key': candidate.key = String(value); break
        case 'keyScale': candidate.keyScale = String(value); break
      }
    }
    return candidate
  }

  const composed = composedCandidate()
  const hasFields = fields.some(({field}) => (grouped.get(field)?.length ?? 0) > 0)
  const preview = current ? fields.flatMap(({field, label, numeric}) => {
    const after = candidateValue(composed, field, numeric)
    const before = current[field as keyof Track]
    const afterEmpty = numeric ? Number(after) <= 0 : String(after).trim() === ''
    if (afterEmpty || String(before ?? '') === String(after ?? '')) return []
    return [{field, label, before: String(before ?? ''), after: String(after ?? '')}]
  }) : []

  return (
    <div className="metadata-merge" data-testid="metadata-merge">
      <div className="panel-title"><h3>{t('metadata.mergeTitle')}</h3><span>{t('metadata.mergeHint')}</span></div>
      {hasFields ? (
        <div className="metadata-merge-grid">
          {fields.map(({field, label, numeric}) => {
            const options = grouped.get(field) ?? []
            if (options.length === 0) return null
            return (
              <label key={field}>
                <span>{t(label)}</span>
                <select
                  value={selected[field] ?? 0}
                  onChange={(event) => setSelected((current) => ({...current, [field]: Number(event.target.value)}))}
                  disabled={disabled}
                >
                  {options.map((option, index) => (
                    <option key={`${field}-${option.source}-${option.externalId}-${index}`} value={index}>
                      {optionValue(option, numeric)} — {option.source} ({Math.round(option.confidence * 100)}%)
                    </option>
                  ))}
                </select>
              </label>
            )
          })}
        </div>
      ) : (
        <p className="warning">{t('metadata.mergeHint')}</p>
      )}
      <div className="metadata-merge-preview">
        <strong>{t('metadata.previewTitle')}</strong>
        {preview.length > 0 ? preview.map((item) => (
          <div key={item.field}><span>{t(item.label)}</span><code>{item.before || '—'}</code><b>→</b><code>{item.after || '—'}</code></div>
        )) : <small>{t('metadata.previewNoChanges')}</small>}
      </div>
      <div className="metadata-actions">
        <button onClick={() => void onApply(composed, false)} disabled={disabled || candidates.length === 0}>{t('metadata.applyMerged')}</button>
        {composed.artworkEmbeddable && composed.artworkUrl && (
          <button onClick={() => void onApply(composed, true)} disabled={disabled}>{t('metadata.applyMergedArtwork')}</button>
        )}
      </div>
    </div>
  )
}
