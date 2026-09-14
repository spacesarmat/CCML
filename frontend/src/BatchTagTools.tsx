import {useEffect, useMemo, useRef, useState} from 'react'
import BatchTagFields from './BatchTagFields'
import BatchTagTransforms from './BatchTagTransforms'
import {translate, type AppLanguage, type TranslationKey} from './i18n'
import type {TagPatch, TagPreview, Track} from './types'

type EditableField = Exclude<keyof TagPatch, 'fields'>
type OpenTool = 'edit' | 'transform' | null

type Props = {
  language: AppLanguage
  tracks: Track[]
  revision: number
  disabled: boolean
  closeSignal: number
  onRequestOpen: () => void
  onBusyChange: (busy: boolean) => void
  onMessage: (message: string) => void
  onChanged: () => Promise<void>
}

const editableFields: EditableField[] = [
  'title',
  'artist',
  'album',
  'albumArtist',
  'genre',
  'composer',
  'comment',
  'label',
  'catalogNumber',
  'isrc',
  'releaseDate',
  'year',
  'trackNumber',
  'trackTotal',
  'discNumber',
  'discTotal',
]

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

function buildInitialPatch(tracks: Track[]): TagPatch {
  if (tracks.length === 0) return {...emptyPatch, fields: []}

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
    title: 'tags.field.title',
    artist: 'tags.field.artist',
    album: 'tags.field.album',
    albumArtist: 'tags.field.albumArtist',
    genre: 'tags.field.genre',
    composer: 'tags.field.composer',
    comment: 'tags.field.comment',
    label: 'tags.field.label',
    catalogNumber: 'tags.field.catalogNumber',
    isrc: 'tags.field.isrc',
    releaseDate: 'tags.field.releaseDate',
    year: 'tags.field.year',
    trackNumber: 'tags.field.track',
    trackTotal: 'tags.field.trackTotal',
    discNumber: 'tags.field.disc',
    discTotal: 'tags.field.discTotal',
  }

  const changes: string[] = []
  for (const field of editableFields) {
    if (item.before[field] !== item.after[field]) {
      changes.push(
        `${translate(language, labels[field])}: ${String(item.before[field] || '—')} → ${String(item.after[field] || '—')}`,
      )
    }
  }

  return changes.join(' · ') || translate(language, 'tags.noChanges')
}

function BatchTagTools({
  language,
  tracks,
  revision,
  disabled,
  closeSignal,
  onRequestOpen,
  onBusyChange,
  onMessage,
  onChanged,
}: Props) {
  const [openTool, setOpenTool] = useState<OpenTool>(null)
  const [patch, setPatch] = useState<TagPatch>(() => buildInitialPatch(tracks))
  const [editPreview, setEditPreview] = useState<TagPreview[]>([])
  const [transformPreview, setTransformPreview] = useState<TagPreview[]>([])
  const [localBusy, setLocalBusy] = useState(false)
  const closeTimer = useRef<number | null>(null)
  const selectionKey = useMemo(() => tracks.map((track) => track.id).join(','), [tracks])

  const copy = language === 'ru'
    ? {
        edit: 'Массовые теги',
        transform: 'Преобразования',
        editTitle: 'Массовое редактирование тегов',
        editHint: (count: number) => `Выбрано треков: ${count}. Изменяются только поля, отмеченные как «Заменить» или «Очистить».`,
        transformTitle: 'Преобразования тегов',
        transformHint: (count: number) => `Выбрано треков: ${count}. Результат рассчитывается отдельно для каждого файла.`,
        preview: 'Предпросмотр',
        apply: 'Записать теги',
        selectFieldFirst: 'Сначала выберите поля для изменения.',
        previewing: 'Подготовка предпросмотра тегов…',
        previewReady: (count: number) => `Предпросмотр готов: ${count}`,
        confirm: (count: number) => `Записать изменения в ${count} треков?`,
        applying: 'Запись массовых тегов…',
        applied: (changed: number, failed: number) => `Теги: изменено ${changed}, ошибок ${failed}`,
        previewTitle: 'Предпросмотр изменений',
        previewMore: (count: number) => `Ещё файлов: ${count}`,
      }
    : {
        edit: 'Batch tags',
        transform: 'Transforms',
        editTitle: 'Batch tag editing',
        editHint: (count: number) => `${count} tracks selected. Only fields set to Replace or Clear are written.`,
        transformTitle: 'Tag transforms',
        transformHint: (count: number) => `${count} tracks selected. Output is calculated independently for each file.`,
        preview: 'Preview',
        apply: 'Write tags',
        selectFieldFirst: 'Select fields to change first.',
        previewing: 'Preparing tag preview…',
        previewReady: (count: number) => `Preview ready: ${count}`,
        confirm: (count: number) => `Write changes to ${count} tracks?`,
        applying: 'Writing batch tags…',
        applied: (changed: number, failed: number) => `Tags: changed ${changed}, failed ${failed}`,
        previewTitle: 'Change preview',
        previewMore: (count: number) => `${count} more files`,
      }

  useEffect(() => {
    setPatch(buildInitialPatch(tracks))
    setEditPreview([])
    setTransformPreview([])
    if (tracks.length < 2) setOpenTool(null)
  }, [selectionKey, revision])

  useEffect(() => {
    setOpenTool(null)
  }, [closeSignal])

  useEffect(() => {
    function onEscape(event: KeyboardEvent) {
      if (event.key === 'Escape') setOpenTool(null)
    }

    window.addEventListener('keydown', onEscape)
    return () => {
      window.removeEventListener('keydown', onEscape)
      if (closeTimer.current !== null) window.clearTimeout(closeTimer.current)
    }
  }, [])

  if (tracks.length < 2) return null

  function cancelClose() {
    if (closeTimer.current !== null) {
      window.clearTimeout(closeTimer.current)
      closeTimer.current = null
    }
  }

  function scheduleClose() {
    cancelClose()
    closeTimer.current = window.setTimeout(() => {
      setOpenTool(null)
      closeTimer.current = null
    }, 650)
  }

  function toggleTool(tool: Exclude<OpenTool, null>) {
    cancelClose()
    setOpenTool((current) => {
      const next = current === tool ? null : tool
      if (next !== null) onRequestOpen()
      return next
    })
  }

  function updateField(field: EditableField, value: string | number) {
    setPatch((current) => ({
      ...current,
      [field]: value,
      fields: current.fields.includes(field) ? current.fields : [...current.fields, field],
    }))
    setEditPreview([])
  }

  function toggleField(field: EditableField) {
    setPatch((current) => ({
      ...current,
      fields: current.fields.includes(field)
        ? current.fields.filter((item) => item !== field)
        : [...current.fields, field],
    }))
    setEditPreview([])
  }

  async function run(label: string, work: () => Promise<void>) {
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

  async function previewEdits() {
    if (patch.fields.length === 0) {
      onMessage(copy.selectFieldFirst)
      return
    }

    await run(copy.previewing, async () => {
      const result = await api().PreviewTagEdits(tracks.map((track) => track.id), patch)
      setEditPreview(result ?? [])
      onMessage(copy.previewReady(result?.length ?? 0))
    })
  }

  async function applyEdits() {
    if (patch.fields.length === 0) {
      onMessage(copy.selectFieldFirst)
      return
    }
    if (!window.confirm(copy.confirm(tracks.length))) return

    await run(copy.applying, async () => {
      const result = await api().ApplyTagEdits(tracks.map((track) => track.id), patch)
      await onChanged()
      setEditPreview([])
      setPatch((current) => ({...current, fields: []}))
      onMessage(copy.applied(result.changed, result.failed))
    })
  }

  function renderPreview(items: TagPreview[]) {
    if (items.length === 0) return null

    return (
      <div className="batch-tools-preview">
        <strong>{copy.previewTitle}</strong>
        {items.slice(0, 20).map((item) => (
          <div className="batch-tools-preview-item" key={item.trackId}>
            <span title={item.path}>{item.path}</span>
            <small>{describeChanges(item, language)}</small>
          </div>
        ))}
        {items.length > 20 && <small>{copy.previewMore(items.length - 20)}</small>}
      </div>
    )
  }

  return (
    <div
      className="batch-tools-toolbar"
      onMouseEnter={cancelClose}
      onMouseLeave={scheduleClose}
    >
      <button
        type="button"
        className={`batch-tools-trigger${openTool === 'edit' ? ' active' : ''}`}
        aria-expanded={openTool === 'edit'}
        disabled={disabled}
        onClick={() => toggleTool('edit')}
      >
        <span aria-hidden="true">✎</span>
        <strong>{copy.edit}</strong>
        <b>{tracks.length}</b>
      </button>

      <button
        type="button"
        className={`batch-tools-trigger${openTool === 'transform' ? ' active' : ''}`}
        aria-expanded={openTool === 'transform'}
        disabled={disabled}
        onClick={() => toggleTool('transform')}
      >
        <span aria-hidden="true">↔</span>
        <strong>{copy.transform}</strong>
      </button>

      {openTool === 'edit' && (
        <div className="batch-tools-popover">
          <header className="batch-tools-popover-head">
            <strong>{copy.editTitle}</strong>
            <span>{copy.editHint(tracks.length)}</span>
          </header>

          <div className="batch-tools-scroll">
            <BatchTagFields
              language={language}
              tracks={tracks}
              patch={patch}
              disabled={disabled || localBusy}
              onToggle={toggleField}
              onChange={updateField}
            />

            <div className="batch-tools-actions">
              <button
                type="button"
                disabled={disabled || localBusy || patch.fields.length === 0}
                onClick={previewEdits}
              >
                {copy.preview}
              </button>
              <button
                type="button"
                className="primary"
                disabled={disabled || localBusy || patch.fields.length === 0}
                onClick={applyEdits}
              >
                {copy.apply}
              </button>
            </div>

            {renderPreview(editPreview)}
          </div>
        </div>
      )}

      {openTool === 'transform' && (
        <div className="batch-tools-popover">
          <header className="batch-tools-popover-head">
            <strong>{copy.transformTitle}</strong>
            <span>{copy.transformHint(tracks.length)}</span>
          </header>

          <div className="batch-tools-scroll">
            <BatchTagTransforms
              language={language}
              tracks={tracks}
              disabled={disabled || localBusy}
              onBusyChange={(busy) => {
                setLocalBusy(busy)
                onBusyChange(busy)
              }}
              onMessage={onMessage}
              onPreview={setTransformPreview}
              onChanged={onChanged}
            />

            {renderPreview(transformPreview)}
          </div>
        </div>
      )}
    </div>
  )
}

export default BatchTagTools
