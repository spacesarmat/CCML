import {useEffect, useMemo, useState} from 'react'
import {translate, type AppLanguage, type TranslationKey} from './i18n'
import type {TagPatch, Track} from './types'

type EditableField = Exclude<keyof TagPatch, 'fields'>
type FieldMode = 'keep' | 'replace' | 'clear'
type FieldKind = 'text' | 'number' | 'comment'
type FieldGroup = 'basic' | 'release' | 'numbering'

type Descriptor = {
  field: EditableField
  label: TranslationKey
  kind: FieldKind
  group: FieldGroup
  read: (track: Track) => string | number
}

type Props = {
  language: AppLanguage
  tracks: Track[]
  patch: TagPatch
  disabled: boolean
  onToggle: (field: EditableField) => void
  onChange: (field: EditableField, value: string | number) => void
}

const descriptors: Descriptor[] = [
  {field: 'title', label: 'tags.field.title', kind: 'text', group: 'basic', read: (track) => track.title},
  {field: 'artist', label: 'tags.field.artist', kind: 'text', group: 'basic', read: (track) => track.artist},
  {field: 'album', label: 'tags.field.album', kind: 'text', group: 'basic', read: (track) => track.album},
  {field: 'albumArtist', label: 'tags.field.albumArtist', kind: 'text', group: 'basic', read: (track) => track.albumArtist},
  {field: 'genre', label: 'tags.field.genre', kind: 'text', group: 'basic', read: (track) => track.genre},
  {field: 'composer', label: 'tags.field.composer', kind: 'text', group: 'release', read: (track) => track.composer},
  {field: 'label', label: 'tags.field.label', kind: 'text', group: 'release', read: (track) => track.label},
  {field: 'catalogNumber', label: 'tags.field.catalogNumber', kind: 'text', group: 'release', read: (track) => track.catalogNumber},
  {field: 'isrc', label: 'tags.field.isrc', kind: 'text', group: 'release', read: (track) => track.isrc},
  {field: 'releaseDate', label: 'tags.field.releaseDate', kind: 'text', group: 'release', read: (track) => track.releaseDate},
  {field: 'year', label: 'tags.field.year', kind: 'number', group: 'release', read: (track) => track.year},
  {field: 'trackNumber', label: 'tags.field.track', kind: 'number', group: 'numbering', read: (track) => track.trackNumber},
  {field: 'trackTotal', label: 'tags.field.trackTotal', kind: 'number', group: 'numbering', read: (track) => track.trackTotal},
  {field: 'discNumber', label: 'tags.field.disc', kind: 'number', group: 'numbering', read: (track) => track.discNumber},
  {field: 'discTotal', label: 'tags.field.discTotal', kind: 'number', group: 'numbering', read: (track) => track.discTotal},
  {field: 'comment', label: 'tags.field.comment', kind: 'comment', group: 'release', read: (track) => track.comment},
]

const editableFields = descriptors.map((item) => item.field)

function makeKeepModes(): Record<EditableField, FieldMode> {
  return Object.fromEntries(editableFields.map((field) => [field, 'keep'])) as Record<EditableField, FieldMode>
}

function clearValue(field: EditableField): string | number {
  const descriptor = descriptors.find((item) => item.field === field)
  return descriptor?.kind === 'number' ? 0 : ''
}

function BatchTagFields({language, tracks, patch, disabled, onToggle, onChange}: Props) {
  const [modes, setModes] = useState<Record<EditableField, FieldMode>>(() => makeKeepModes())
  const [groupOpen, setGroupOpen] = useState<Record<FieldGroup, boolean>>({
    basic: true,
    release: true,
    numbering: false,
  })
  const selectionKey = useMemo(() => tracks.map((track) => track.id).join(','), [tracks])

  const copy = language === 'ru'
    ? {
        title: 'Массовое редактирование',
        summary: (tracksCount: number, fieldsCount: number) => `Треков: ${tracksCount} · Изменяется полей: ${fieldsCount}`,
        basic: 'Основные',
        release: 'Релиз и описание',
        numbering: 'Нумерация',
        keep: 'Не менять',
        replace: 'Заменить',
        clear: 'Очистить',
        mixed: 'Разные значения',
        common: 'Одинаково',
        empty: 'Пусто',
        clearHint: 'Поле будет очищено во всех выбранных треках',
        valuePlaceholder: 'Введите новое значение',
        mixedPlaceholder: 'Разные значения — введите замену',
      }
    : {
        title: 'Batch editing',
        summary: (tracksCount: number, fieldsCount: number) => `Tracks: ${tracksCount} · Fields changing: ${fieldsCount}`,
        basic: 'Basic',
        release: 'Release and description',
        numbering: 'Numbering',
        keep: 'Keep',
        replace: 'Replace',
        clear: 'Clear',
        mixed: 'Mixed values',
        common: 'Same value',
        empty: 'Empty',
        clearHint: 'This field will be cleared in every selected track',
        valuePlaceholder: 'Enter a new value',
        mixedPlaceholder: 'Mixed values — enter replacement',
      }

  const summaries = useMemo(() => {
    const result = new Map<EditableField, {mixed: boolean; common: string | number | null; empty: boolean}>()

    for (const descriptor of descriptors) {
      if (tracks.length === 0) {
        result.set(descriptor.field, {mixed: false, common: null, empty: true})
        continue
      }

      const first = descriptor.read(tracks[0])
      const mixed = tracks.some((track) => descriptor.read(track) !== first)
      const empty = mixed ? false : first === '' || first === 0
      result.set(descriptor.field, {mixed, common: mixed ? null : first, empty})
    }

    return result
  }, [tracks])

  useEffect(() => {
    setModes(makeKeepModes())
  }, [selectionKey])

  useEffect(() => {
    if (patch.fields.length === 0) setModes(makeKeepModes())
  }, [patch.fields.length])

  function changeMode(field: EditableField, next: FieldMode) {
    const previous = modes[field]
    setModes((current) => ({...current, [field]: next}))

    const active = patch.fields.includes(field)

    if (next === 'keep') {
      if (active) onToggle(field)
      return
    }

    if (next === 'clear') {
      onChange(field, clearValue(field))
      return
    }

    if (!active) onToggle(field)

    if (previous === 'clear') {
      const common = summaries.get(field)?.common
      onChange(field, common ?? clearValue(field))
    }
  }

  function editValue(field: EditableField, value: string | number) {
    if (modes[field] !== 'replace') {
      setModes((current) => ({...current, [field]: 'replace'}))
    }
    onChange(field, value)
  }

  function renderField(descriptor: Descriptor) {
    const {field, kind} = descriptor
    const mode = modes[field]
    const summary = summaries.get(field)
    const value = patch[field]
    const mixed = summary?.mixed ?? false
    const isClear = mode === 'clear'
    const isKeep = mode === 'keep'
    const inputDisabled = disabled || mode !== 'replace'

    const badge = mixed
      ? copy.mixed
      : summary?.empty
        ? copy.empty
        : copy.common

    const placeholder = isClear
      ? copy.clearHint
      : mixed
        ? copy.mixedPlaceholder
        : copy.valuePlaceholder

    return (
      <div className={`batch-tag-field${kind === 'comment' ? ' comment' : ''}`} key={field}>
        <div className="batch-tag-field-head">
          <strong>{translate(language, descriptor.label)}</strong>
          <span className={mixed ? 'mixed' : ''}>{badge}</span>
        </div>

        <div className="batch-tag-field-controls">
          <select
            value={mode}
            disabled={disabled}
            onChange={(event) => changeMode(field, event.target.value as FieldMode)}
            aria-label={`${translate(language, descriptor.label)} mode`}
          >
            <option value="keep">{copy.keep}</option>
            <option value="replace">{copy.replace}</option>
            <option value="clear">{copy.clear}</option>
          </select>

          {kind === 'comment' ? (
            <textarea
              rows={3}
              value={isClear ? '' : String(value)}
              placeholder={placeholder}
              disabled={inputDisabled}
              onChange={(event) => editValue(field, event.target.value)}
            />
          ) : kind === 'number' ? (
            <input
              type="number"
              min="0"
              max="9999"
              value={isClear ? '' : Number(value) || ''}
              placeholder={placeholder}
              disabled={inputDisabled}
              onChange={(event) => editValue(field, event.target.value === '' ? 0 : Number(event.target.value))}
            />
          ) : (
            <input
              type="text"
              value={isClear ? '' : String(value)}
              placeholder={placeholder}
              disabled={inputDisabled}
              onChange={(event) => editValue(field, event.target.value.replace(/[\r\n]+/g, ' '))}
            />
          )}
        </div>

        {!isKeep && (
          <small className={`batch-tag-mode-note ${mode}`}>
            {mode === 'clear' ? copy.clearHint : mixed ? copy.mixedPlaceholder : copy.replace}
          </small>
        )}
      </div>
    )
  }

  function renderGroup(group: FieldGroup, title: string) {
    const items = descriptors.filter((item) => item.group === group)

    return (
      <details
        className="batch-tag-group"
        open={groupOpen[group]}
        onToggle={(event) => {
          const open = event.currentTarget.open
          setGroupOpen((current) => (
            current[group] === open ? current : {...current, [group]: open}
          ))
        }}
      >
        <summary>
          <strong>{title}</strong>
          <span>{items.filter((item) => modes[item.field] !== 'keep').length}</span>
        </summary>
        <div className="batch-tag-group-body">
          {items.map(renderField)}
        </div>
      </details>
    )
  }

  return (
    <div className="batch-tag-editor">
      <div className="batch-tag-summary">
        <strong>{copy.title}</strong>
        <span>{copy.summary(tracks.length, patch.fields.length)}</span>
      </div>

      {renderGroup('basic', copy.basic)}
      {renderGroup('release', copy.release)}
      {renderGroup('numbering', copy.numbering)}
    </div>
  )
}

export default BatchTagFields
