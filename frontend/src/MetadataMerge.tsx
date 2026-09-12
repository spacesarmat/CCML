import { useEffect, useMemo, useState } from 'react'
import { translate, type AppLanguage, type TranslationKey } from './i18n'
import type { MetadataCandidate, MetadataFieldOption, MetadataLookupResult } from './types'

type Props = {
  language: AppLanguage
  lookup: MetadataLookupResult
  disabled: boolean
  onApply: (candidate: MetadataCandidate, includeArtwork: boolean) => Promise<void>
}

type MergeField =
  | 'title' | 'artist' | 'album' | 'albumArtist' | 'genre'
  | 'releaseDate' | 'year' | 'label' | 'catalogNumber' | 'isrc'
  | 'trackNumber' | 'trackTotal' | 'discNumber' | 'discTotal'

const fields: Array<{field: MergeField; label: TranslationKey; numeric?: boolean}> = [
  {field: 'title', label: 'tags.field.title'},
  {field: 'artist', label: 'tags.field.artist'},
  {field: 'album', label: 'tags.field.album'},
  {field: 'albumArtist', label: 'tags.field.albumArtist'},
  {field: 'genre', label: 'tags.field.genre'},
  {field: 'releaseDate', label: 'tags.field.releaseDate'},
  {field: 'year', label: 'tags.field.year', numeric: true},
  {field: 'label', label: 'tags.field.label'},
  {field: 'catalogNumber', label: 'tags.field.catalogNumber'},
  {field: 'isrc', label: 'tags.field.isrc'},
  {field: 'trackNumber', label: 'tags.field.track', numeric: true},
  {field: 'trackTotal', label: 'tags.field.trackTotal', numeric: true},
  {field: 'discNumber', label: 'tags.field.disc', numeric: true},
  {field: 'discTotal', label: 'tags.field.discTotal', numeric: true},
]

export default function MetadataMerge({language, lookup, disabled, onApply}: Props) {
  const t = (key: TranslationKey) => translate(language, key)
  const grouped = useMemo(() => {
    const result = new Map<string, MetadataFieldOption[]>()
    for (const option of lookup.fieldOptions ?? []) {
      const list = result.get(option.field) ?? []
      list.push(option)
      result.set(option.field, list)
    }
    return result
  }, [lookup])
  const [selected, setSelected] = useState<Record<string, number>>({})

  useEffect(() => {
    const defaults: Record<string, number> = {}
    for (const {field, numeric} of fields) {
      const options = grouped.get(field) ?? []
      const suggestedValue = numeric
        ? Number(lookup.suggested[field] ?? 0)
        : String(lookup.suggested[field] ?? '')
      const index = options.findIndex((option) => numeric ? option.number === suggestedValue : option.value === suggestedValue)
      defaults[field] = index >= 0 ? index : 0
    }
    setSelected(defaults)
  }, [lookup, grouped])

  function composedCandidate(): MetadataCandidate {
    const candidate: MetadataCandidate = {...lookup.suggested, source: 'CCML Merge', externalId: '', sourceUrl: ''}
    for (const {field, numeric} of fields) {
      const options = grouped.get(field) ?? []
      const option = options[selected[field] ?? 0]
      if (!option) continue
      const value = numeric ? option.number : option.value
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
      }
    }
    return candidate
  }

  return (
    <div className="metadata-merge">
      <div className="panel-title"><h3>{t('metadata.mergeTitle')}</h3><span>{t('metadata.mergeHint')}</span></div>
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
                    {numeric ? option.number : option.value} — {option.source} ({Math.round(option.confidence * 100)}%)
                  </option>
                ))}
              </select>
            </label>
          )
        })}
      </div>
      <div className="metadata-actions">
        <button onClick={() => void onApply(composedCandidate(), false)} disabled={disabled}>{t('metadata.applyMerged')}</button>
        {lookup.suggested.artworkEmbeddable && lookup.suggested.artworkUrl && (
          <button onClick={() => void onApply(composedCandidate(), true)} disabled={disabled}>{t('metadata.applyMergedArtwork')}</button>
        )}
      </div>
    </div>
  )
}
