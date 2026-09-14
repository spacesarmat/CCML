import {useEffect, useMemo, useRef, useState} from 'react'
import BatchPreviewModal from './BatchPreviewModal'
import BatchTagFields from './BatchTagFields'
import BatchTagTransforms from './BatchTagTransforms'
import {type AppLanguage} from './i18n'
import type {TagPatch, TagPreview, TagTransformRequest, Track} from './types'

type EditableField = Exclude<keyof TagPatch, 'fields'>
type OpenTool = 'edit' | 'transform' | null

type PreviewState =
  | {kind: 'edit'; items: TagPreview[]; patch: TagPatch}
  | {kind: 'transform'; items: TagPreview[]; request: TagTransformRequest}

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
  const [preview, setPreview] = useState<PreviewState | null>(null)
  const [localBusy, setLocalBusy] = useState(false)
  const closeTimer = useRef<number | null>(null)
  const selectionKey = useMemo(() => tracks.map((track) => track.id).join(','), [tracks])

  const copy = language === 'ru'
    ? {
        edit: 'Массовые теги',
        transform: 'Преобразования',
        editTitle: 'Массовое редактирование тегов',
        editHint: (count: number) => `Выбрано треков: ${count}. Изменяются только поля «Заменить» и «Очистить».`,
        transformTitle: 'Преобразования тегов',
        transformHint: (count: number) => `Выбрано треков: ${count}. Результат рассчитывается отдельно для каждого файла.`,
        preview: 'Открыть предпросмотр',
        selectFieldFirst: 'Сначала выберите поля для изменения.',
        previewing: 'Подготовка предпросмотра тегов…',
        previewReady: (count: number) => `Предпросмотр готов: ${count}`,
        applyingTags: 'Запись массовых тегов…',
        appliedTags: (changed: number, failed: number) => `Теги: изменено ${changed}, ошибок ${failed}`,
        applyingTransform: 'Применение массового преобразования…',
        appliedTransform: (changed: number, failed: number) => `Преобразование: изменено ${changed}, ошибок ${failed}`,
      }
    : {
        edit: 'Batch tags',
        transform: 'Transforms',
        editTitle: 'Batch tag editing',
        editHint: (count: number) => `${count} tracks selected. Only Replace and Clear fields are written.`,
        transformTitle: 'Tag transforms',
        transformHint: (count: number) => `${count} tracks selected. Output is calculated independently for each file.`,
        preview: 'Open preview',
        selectFieldFirst: 'Select fields to change first.',
        previewing: 'Preparing tag preview…',
        previewReady: (count: number) => `Preview ready: ${count}`,
        applyingTags: 'Writing batch tags…',
        appliedTags: (changed: number, failed: number) => `Tags: changed ${changed}, failed ${failed}`,
        applyingTransform: 'Applying batch transform…',
        appliedTransform: (changed: number, failed: number) => `Transform: changed ${changed}, failed ${failed}`,
      }

  useEffect(() => {
    setPatch(buildInitialPatch(tracks))
    setPreview(null)
    if (tracks.length < 2) setOpenTool(null)
  }, [selectionKey, revision])

  useEffect(() => {
    setOpenTool(null)
    setPreview(null)
  }, [closeSignal])

  useEffect(() => {
    function onEscape(event: KeyboardEvent) {
      if (event.key !== 'Escape') return
      if (!localBusy && preview) {
        setPreview(null)
        return
      }
      setOpenTool(null)
    }

    window.addEventListener('keydown', onEscape)
    return () => {
      window.removeEventListener('keydown', onEscape)
      if (closeTimer.current !== null) window.clearTimeout(closeTimer.current)
    }
  }, [localBusy, preview])

  if (tracks.length < 2) return null

  function cancelClose() {
    if (closeTimer.current !== null) {
      window.clearTimeout(closeTimer.current)
      closeTimer.current = null
    }
  }

  function scheduleClose() {
    if (preview) return
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
  }

  function toggleField(field: EditableField) {
    setPatch((current) => ({
      ...current,
      fields: current.fields.includes(field)
        ? current.fields.filter((item) => item !== field)
        : [...current.fields, field],
    }))
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
      const patchSnapshot: TagPatch = {...patch, fields: [...patch.fields]}
      const result = await api().PreviewTagEdits(tracks.map((track) => track.id), patchSnapshot)
      setPreview({kind: 'edit', items: result ?? [], patch: patchSnapshot})
      onMessage(copy.previewReady(result?.length ?? 0))
      cancelClose()
    })
  }

  function previewTransform(items: TagPreview[], request: TagTransformRequest) {
    setPreview({
      kind: 'transform',
      items,
      request: {...request, fields: [...request.fields]},
    })
    cancelClose()
  }

  async function applyPreview() {
    const current = preview
    if (!current) return

    if (current.kind === 'edit') {
      await run(copy.applyingTags, async () => {
        const result = await api().ApplyTagEdits(tracks.map((track) => track.id), current.patch)
        await onChanged()
        setPatch((value) => ({...value, fields: []}))
        setPreview(null)
        setOpenTool(null)
        onMessage(copy.appliedTags(result.changed, result.failed))
      })
      return
    }

    await run(copy.applyingTransform, async () => {
      const result = await api().ApplyTagTransforms(tracks.map((track) => track.id), current.request)
      await onChanged()
      setPreview(null)
      setOpenTool(null)
      onMessage(copy.appliedTransform(result.changed, result.failed))
    })
  }

  return (
    <>
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

              <div className="batch-tools-actions single">
                <button
                  type="button"
                  className="primary"
                  disabled={disabled || localBusy || patch.fields.length === 0}
                  onClick={previewEdits}
                >
                  {copy.preview}
                </button>
              </div>
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
                onPreview={previewTransform}
              />
            </div>
          </div>
        )}
      </div>

      {preview && (
        <BatchPreviewModal
          language={language}
          items={preview.items}
          busy={localBusy}
          onClose={() => {
            if (!localBusy) setPreview(null)
          }}
          onApply={applyPreview}
        />
      )}
    </>
  )
}

export default BatchTagTools
