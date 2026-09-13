import { useEffect, useMemo, useState } from 'react'
import { translate, type AppLanguage, type TranslateParams, type TranslationKey } from './i18n'
import type { TagHistory, TagPatch, TagPreview, TagSnapshot, Track } from './types'

type Props = {
  language: AppLanguage
  tracks: Track[]
  revision: number
  disabled: boolean
  onBusyChange: (busy: boolean) => void
  onMessage: (message: string) => void
  onChanged: () => Promise<void>
}

const editableFields = [
  'title', 'artist', 'album', 'albumArtist', 'genre', 'composer', 'comment',
  'label', 'catalogNumber', 'isrc', 'releaseDate',
  'year', 'trackNumber', 'trackTotal', 'discNumber', 'discTotal',
] as const

type EditableField = typeof editableFields[number]

const emptyPatch: TagPatch = {
  fields: [],
  title: '',
  artist: '',
  album: '',
  albumArtist: '',
  genre: '',
  composer: '',
  comment: '',
  label: '',
  catalogNumber: '',
  isrc: '',
  releaseDate: '',
  year: 0,
  trackNumber: 0,
  trackTotal: 0,
  discNumber: 0,
  discTotal: 0,
}

function api() {
  if (!window.go?.main?.App) throw new Error('Wails backend is not available')
  return window.go.main.App
}

export default function TagEditor({
  language,
  tracks,
  revision,
  disabled,
  onBusyChange,
  onMessage,
  onChanged,
}: Props) {
  const t = (key: TranslationKey, params?: TranslateParams) => translate(language, key, params)
  const [patch, setPatch] = useState<TagPatch>(emptyPatch)
  const [snapshot, setSnapshot] = useState<TagSnapshot | null>(null)
  const [preview, setPreview] = useState<TagPreview[]>([])
  const [history, setHistory] = useState<TagHistory[]>([])
  const [localBusy, setLocalBusy] = useState(false)

  const ids = useMemo(() => tracks.map((track) => track.id), [tracks])
  const multi = tracks.length > 1

  useEffect(() => {
    setPreview([])
    setPatch(buildInitialPatch(tracks))
    setSnapshot(null)
    if (tracks.length === 1) {
      void loadSnapshot(tracks[0].id)
    }
  }, [tracks.map((track) => track.id).join(','), revision])

  useEffect(() => {
    void refreshHistory()
  }, [revision])

  async function loadSnapshot(trackID: number) {
    try {
      const result = await api().ReadTrackTags(trackID)
      setSnapshot(result)
      setPatch(snapshotToPatch(result))
    } catch (error) {
      onMessage(error instanceof Error ? error.message : String(error))
    }
  }

  async function refreshHistory() {
    try {
      setHistory((await api().ListTagHistory(12)) ?? [])
    } catch (error) {
      onMessage(error instanceof Error ? error.message : String(error))
    }
  }

  function updateField(field: EditableField, value: string | number) {
    setPatch((current) => ({
      ...current,
      [field]: value,
      fields: current.fields.includes(field) ? current.fields : [...current.fields, field],
    }))
    setPreview([])
  }

  function toggleField(field: EditableField) {
    setPatch((current) => ({
      ...current,
      fields: current.fields.includes(field)
        ? current.fields.filter((item) => item !== field)
        : [...current.fields, field],
    }))
    setPreview([])
  }

  async function runTagAction(label: string, work: () => Promise<void>) {
    setLocalBusy(true)
    onBusyChange(true)
    onMessage(label)
    try {
      await work()
    } catch (error) {
      onMessage(error instanceof Error ? error.message : String(error))
    } finally {
      setLocalBusy(false)
      onBusyChange(false)
    }
  }

  async function previewChanges() {
    if (patch.fields.length === 0) {
      onMessage(t('tags.selectFieldFirst'))
      return
    }
    await runTagAction(t('tags.previewing'), async () => {
      const result = await api().PreviewTagEdits(ids, patch)
      setPreview(result ?? [])
      onMessage(t('tags.previewReady', {count: result?.length ?? 0}))
    })
  }

  async function applyChanges() {
    if (patch.fields.length === 0) {
      onMessage(t('tags.selectFieldFirst'))
      return
    }
    if (multi && !window.confirm(t('tags.batchConfirm', {count: tracks.length}))) return
    await runTagAction(t('tags.applying'), async () => {
      const result = await api().ApplyTagEdits(ids, patch)
      await onChanged()
      setPreview([])
      setPatch((current) => ({...current, fields: []}))
      onMessage(t('tags.applied', {changed: result.changed, failed: result.failed}))
    })
  }

  async function chooseCover() {
    await runTagAction(t('tags.selectingCover'), async () => {
      const path = await api().SelectCoverArt()
      if (!path) {
        onMessage(t('message.ready'))
        return
      }
      if (multi && !window.confirm(t('tags.coverBatchConfirm', {count: tracks.length}))) return
      const result = await api().SetCoverArt(ids, path)
      await onChanged()
      onMessage(t('tags.coverApplied', {changed: result.changed, failed: result.failed}))
    })
  }

  async function removeCover() {
    if (!window.confirm(t('tags.removeCoverConfirm', {count: tracks.length}))) return
    await runTagAction(t('tags.removingCover'), async () => {
      const result = await api().RemoveCoverArt(ids)
      await onChanged()
      onMessage(t('tags.coverRemoved', {changed: result.changed, failed: result.failed}))
    })
  }

  async function undo(change: TagHistory) {
    if (!window.confirm(t('tags.undoConfirm', {count: change.affectedCount}))) return
    await runTagAction(t('tags.undoing'), async () => {
      const result = await api().UndoTagChange(change.id)
      await onChanged()
      await refreshHistory()
      onMessage(t('tags.undoComplete', {changed: result.changed, failed: result.failed}))
    })
  }

  if (tracks.length === 0) {
    return <div className="tag-editor-empty">{t('tags.selectTracks')}</div>
  }

  const coverText = snapshot
    ? snapshot.coverSize > 0
      ? `${snapshot.coverMime || t('tags.cover')} · ${formatBytes(snapshot.coverSize)}`
      : t('tags.noCover')
    : multi ? t('tags.multipleSelection') : t('tags.loading')

  return (
    <section className="tag-editor">
      <div className="tag-editor-heading">
        <div>
          <h3>{t('tags.title')}</h3>
          <span>{t('tags.selected', {count: tracks.length})}</span>
        </div>
        <button type="button" onClick={() => setPatch((current) => ({...current, fields: []}))} disabled={disabled || localBusy}>
          {t('tags.clearSelection')}
        </button>
      </div>

      <p className="tag-help">{multi ? t('tags.batchHint') : t('tags.singleHint')}</p>

      <div className="tag-grid">
        <TagTextField field="title" label={t('tags.field.title')} patch={patch} onToggle={toggleField} onChange={updateField} disabled={disabled || localBusy} />
        <TagTextField field="artist" label={t('tags.field.artist')} patch={patch} onToggle={toggleField} onChange={updateField} disabled={disabled || localBusy} />
        <TagTextField field="album" label={t('tags.field.album')} patch={patch} onToggle={toggleField} onChange={updateField} disabled={disabled || localBusy} />
        <TagTextField field="albumArtist" label={t('tags.field.albumArtist')} patch={patch} onToggle={toggleField} onChange={updateField} disabled={disabled || localBusy} />
        <TagTextField field="genre" label={t('tags.field.genre')} patch={patch} onToggle={toggleField} onChange={updateField} disabled={disabled || localBusy} />
        <TagTextField field="composer" label={t('tags.field.composer')} patch={patch} onToggle={toggleField} onChange={updateField} disabled={disabled || localBusy} />
        <TagTextField field="label" label={t('tags.field.label')} patch={patch} onToggle={toggleField} onChange={updateField} disabled={disabled || localBusy} />
        <TagTextField field="catalogNumber" label={t('tags.field.catalogNumber')} patch={patch} onToggle={toggleField} onChange={updateField} disabled={disabled || localBusy} />
        <TagTextField field="isrc" label={t('tags.field.isrc')} patch={patch} onToggle={toggleField} onChange={updateField} disabled={disabled || localBusy} />
        <TagTextField field="releaseDate" label={t('tags.field.releaseDate')} patch={patch} onToggle={toggleField} onChange={updateField} disabled={disabled || localBusy} />
        <TagNumberField field="year" label={t('tags.field.year')} patch={patch} onToggle={toggleField} onChange={updateField} disabled={disabled || localBusy} />
        <TagNumberField field="trackNumber" label={t('tags.field.track')} patch={patch} onToggle={toggleField} onChange={updateField} disabled={disabled || localBusy} />
        <TagNumberField field="trackTotal" label={t('tags.field.trackTotal')} patch={patch} onToggle={toggleField} onChange={updateField} disabled={disabled || localBusy} />
        <TagNumberField field="discNumber" label={t('tags.field.disc')} patch={patch} onToggle={toggleField} onChange={updateField} disabled={disabled || localBusy} />
        <TagNumberField field="discTotal" label={t('tags.field.discTotal')} patch={patch} onToggle={toggleField} onChange={updateField} disabled={disabled || localBusy} />
      </div>

      <label className="tag-comment-field">
        <span><input type="checkbox" checked={patch.fields.includes('comment')} onChange={() => toggleField('comment')} /> {t('tags.field.comment')}</span>
        <textarea
          value={patch.comment}
          onChange={(event) => updateField('comment', event.target.value)}
          disabled={disabled || localBusy}
          rows={3}
        />
      </label>

      <div className="tag-actions">
        <button type="button" onClick={previewChanges} disabled={disabled || localBusy || patch.fields.length === 0}>{t('tags.preview')}</button>
        <button type="button" className="primary" onClick={applyChanges} disabled={disabled || localBusy || patch.fields.length === 0}>{t('tags.apply')}</button>
      </div>

      <div className="cover-row">
        <div><strong>{t('tags.cover')}</strong><span>{coverText}</span></div>
        <button type="button" onClick={chooseCover} disabled={disabled || localBusy}>{t('tags.chooseCover')}</button>
        <button type="button" onClick={removeCover} disabled={disabled || localBusy}>{t('tags.removeCover')}</button>
      </div>

      {preview.length > 0 && (
        <div className="tag-preview-list">
          <strong>{t('tags.previewTitle')}</strong>
          {preview.slice(0, 20).map((item) => (
            <div className="tag-preview-item" key={item.trackId}>
              <span title={item.path}>{item.path}</span>
              <small>{describeChanges(item, language)}</small>
            </div>
          ))}
          {preview.length > 20 && <small>{t('tags.previewMore', {count: preview.length - 20})}</small>}
        </div>
      )}

      {history.length > 0 && (
        <div className="tag-history">
          <strong>{t('tags.history')}</strong>
          {history.map((item) => (
            <div className="tag-history-row" key={item.id}>
              <div>
                <span>{historyLabel(item.label, language)}</span>
                <small>{new Date(item.createdAt).toLocaleString(language === 'ru' ? 'ru-RU' : 'en-US')} · {t('tags.historyTracks', {count: item.affectedCount})}</small>
              </div>
              <button type="button" onClick={() => undo(item)} disabled={disabled || localBusy || item.status !== 'applied'}>
                {item.status === 'undone' ? t('tags.undone') : t('tags.undo')}
              </button>
            </div>
          ))}
        </div>
      )}
    </section>
  )
}

type FieldProps = {
  field: EditableField
  label: string
  patch: TagPatch
  disabled: boolean
  onToggle: (field: EditableField) => void
  onChange: (field: EditableField, value: string | number) => void
}

function TagTextField({field, label, patch, disabled, onToggle, onChange}: FieldProps) {
  const value = patch[field]
  return (
    <label className="tag-field">
      <span><input type="checkbox" checked={patch.fields.includes(field)} onChange={() => onToggle(field)} /> {label}</span>
      <textarea
        className="tag-textarea"
        rows={2}
        value={String(value)}
        onChange={(event) => onChange(field, event.target.value.replace(/[\r\n]+/g, ' '))}
        onKeyDown={(event) => { if (event.key === 'Enter') event.preventDefault() }}
        disabled={disabled}
      />
    </label>
  )
}

function TagNumberField({field, label, patch, disabled, onToggle, onChange}: FieldProps) {
  const value = patch[field]
  return (
    <label className="tag-field">
      <span><input type="checkbox" checked={patch.fields.includes(field)} onChange={() => onToggle(field)} /> {label}</span>
      <input type="number" min="0" max="9999" value={Number(value)} onChange={(event) => onChange(field, Number(event.target.value))} disabled={disabled} />
    </label>
  )
}

function snapshotToPatch(snapshot: TagSnapshot): TagPatch {
  return {
    fields: [],
    title: snapshot.title,
    artist: snapshot.artist,
    album: snapshot.album,
    albumArtist: snapshot.albumArtist,
    genre: snapshot.genre,
    composer: snapshot.composer,
    comment: snapshot.comment,
    label: snapshot.label,
    catalogNumber: snapshot.catalogNumber,
    isrc: snapshot.isrc,
    releaseDate: snapshot.releaseDate,
    year: snapshot.year,
    trackNumber: snapshot.trackNumber,
    trackTotal: snapshot.trackTotal,
    discNumber: snapshot.discNumber,
    discTotal: snapshot.discTotal,
  }
}

function buildInitialPatch(tracks: Track[]): TagPatch {
  if (tracks.length === 0) return emptyPatch
  const commonString = (getter: (track: Track) => string) => {
    const first = getter(tracks[0])
    return tracks.every((track) => getter(track) === first) ? first : ''
  }
  const commonNumber = (getter: (track: Track) => number) => {
    const first = getter(tracks[0])
    return tracks.every((track) => getter(track) === first) ? first : 0
  }
  return {
    fields: [],
    title: commonString((track) => track.title),
    artist: commonString((track) => track.artist),
    album: commonString((track) => track.album),
    albumArtist: commonString((track) => track.albumArtist),
    genre: commonString((track) => track.genre),
    composer: commonString((track) => track.composer),
    comment: commonString((track) => track.comment),
    label: commonString((track) => track.label),
    catalogNumber: commonString((track) => track.catalogNumber),
    isrc: commonString((track) => track.isrc),
    releaseDate: commonString((track) => track.releaseDate),
    year: commonNumber((track) => track.year),
    trackNumber: commonNumber((track) => track.trackNumber),
    trackTotal: commonNumber((track) => track.trackTotal),
    discNumber: commonNumber((track) => track.discNumber),
    discTotal: commonNumber((track) => track.discTotal),
  }
}

function describeChanges(item: TagPreview, language: AppLanguage): string {
  const labels: Record<EditableField, TranslationKey> = {
    title: 'tags.field.title', artist: 'tags.field.artist', album: 'tags.field.album', albumArtist: 'tags.field.albumArtist',
    genre: 'tags.field.genre', composer: 'tags.field.composer', comment: 'tags.field.comment',
    label: 'tags.field.label', catalogNumber: 'tags.field.catalogNumber', isrc: 'tags.field.isrc', releaseDate: 'tags.field.releaseDate', year: 'tags.field.year',
    trackNumber: 'tags.field.track', trackTotal: 'tags.field.trackTotal', discNumber: 'tags.field.disc', discTotal: 'tags.field.discTotal',
  }
  const changes: string[] = []
  for (const field of editableFields) {
    if (item.before[field] !== item.after[field]) {
      changes.push(`${translate(language, labels[field])}: ${String(item.before[field] || '—')} → ${String(item.after[field] || '—')}`)
    }
  }
  return changes.join(' · ') || translate(language, 'tags.noChanges')
}

function historyLabel(label: string, language: AppLanguage): string {
  const key: TranslationKey = label === 'tags.metadata'
    ? 'tags.history.metadata'
    : label === 'tags.cover'
      ? 'tags.history.cover'
      : label === 'tags.removeCover'
        ? 'tags.history.removeCover'
        : 'tags.history.edit'
  return translate(language, key)
}

function formatBytes(bytes: number): string {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let value = bytes
  let index = 0
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024
    index++
  }
  return `${value.toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}
