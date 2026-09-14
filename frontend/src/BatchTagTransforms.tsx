import {useMemo, useState} from 'react'
import {translate, type AppLanguage, type TranslationKey} from './i18n'
import type {TagApplyResult, TagPreview, TagTransformRequest, Track} from './types'

type TextField =
  | 'title'
  | 'artist'
  | 'album'
  | 'albumArtist'
  | 'genre'
  | 'composer'
  | 'comment'
  | 'label'
  | 'catalogNumber'
  | 'isrc'
  | 'releaseDate'

type Operation = 'trim' | 'upper' | 'lower' | 'replace' | 'prefix' | 'suffix' | 'copy'

type Props = {
  language: AppLanguage
  tracks: Track[]
  disabled: boolean
  onBusyChange: (busy: boolean) => void
  onMessage: (message: string) => void
  onPreview: (preview: TagPreview[]) => void
  onChanged: () => Promise<void>
}

const textFields: Array<{field: TextField; label: TranslationKey}> = [
  {field: 'title', label: 'tags.field.title'},
  {field: 'artist', label: 'tags.field.artist'},
  {field: 'album', label: 'tags.field.album'},
  {field: 'albumArtist', label: 'tags.field.albumArtist'},
  {field: 'genre', label: 'tags.field.genre'},
  {field: 'composer', label: 'tags.field.composer'},
  {field: 'comment', label: 'tags.field.comment'},
  {field: 'label', label: 'tags.field.label'},
  {field: 'catalogNumber', label: 'tags.field.catalogNumber'},
  {field: 'isrc', label: 'tags.field.isrc'},
  {field: 'releaseDate', label: 'tags.field.releaseDate'},
]

function api() {
  if (!window.go?.main?.App) throw new Error('Wails backend is not available')
  return window.go.main.App
}

function BatchTagTransforms({
  language,
  tracks,
  disabled,
  onBusyChange,
  onMessage,
  onPreview,
  onChanged,
}: Props) {
  const [operation, setOperation] = useState<Operation>('trim')
  const [fields, setFields] = useState<TextField[]>([])
  const [search, setSearch] = useState('')
  const [replacement, setReplacement] = useState('')
  const [affix, setAffix] = useState('')
  const [caseSensitive, setCaseSensitive] = useState(false)
  const [copyDirection, setCopyDirection] = useState<'artistToAlbumArtist' | 'albumArtistToArtist'>('artistToAlbumArtist')
  const [previewKey, setPreviewKey] = useState('')
  const [busy, setBusy] = useState(false)

  const copy = language === 'ru'
    ? {
        title: 'Преобразования',
        hint: 'Преобразования рассчитываются отдельно для каждого файла. Перед применением обязателен предпросмотр.',
        operation: 'Операция',
        fields: 'Поля',
        trim: 'Обрезать пробелы по краям',
        upper: 'В ВЕРХНИЙ РЕГИСТР',
        lower: 'в нижний регистр',
        replace: 'Найти и заменить',
        prefix: 'Добавить префикс',
        suffix: 'Добавить суффикс',
        copy: 'Копировать Artist ↔ Album Artist',
        search: 'Найти',
        replacement: 'Заменить на',
        prefixValue: 'Префикс',
        suffixValue: 'Суффикс',
        caseSensitive: 'Учитывать регистр',
        artistToAlbumArtist: 'Исполнитель → Исполнитель альбома',
        albumArtistToArtist: 'Исполнитель альбома → Исполнитель',
        preview: 'Предпросмотр преобразования',
        apply: 'Применить преобразование',
        previewRequired: 'После изменения параметров снова выполните предпросмотр.',
        selectField: 'Выберите хотя бы одно поле.',
        invalidReplace: 'Введите строку для поиска.',
        invalidAffix: 'Введите текст префикса/суффикса.',
        previewing: 'Подготовка предпросмотра преобразования…',
        previewReady: (count: number) => `Предпросмотр готов: ${count}`,
        confirm: (count: number) => `Применить преобразование к ${count} трекам?`,
        applying: 'Применение массового преобразования…',
        applied: (result: TagApplyResult) => `Преобразование: изменено ${result.changed}, ошибок ${result.failed}`,
      }
    : {
        title: 'Transforms',
        hint: 'Transforms are calculated separately for each file. Preview is required before applying.',
        operation: 'Operation',
        fields: 'Fields',
        trim: 'Trim leading/trailing whitespace',
        upper: 'UPPERCASE',
        lower: 'lowercase',
        replace: 'Find and replace',
        prefix: 'Add prefix',
        suffix: 'Add suffix',
        copy: 'Copy Artist ↔ Album Artist',
        search: 'Find',
        replacement: 'Replace with',
        prefixValue: 'Prefix',
        suffixValue: 'Suffix',
        caseSensitive: 'Case sensitive',
        artistToAlbumArtist: 'Artist → Album Artist',
        albumArtistToArtist: 'Album Artist → Artist',
        preview: 'Preview transform',
        apply: 'Apply transform',
        previewRequired: 'Preview again after changing any transform setting.',
        selectField: 'Select at least one field.',
        invalidReplace: 'Enter text to find.',
        invalidAffix: 'Enter prefix/suffix text.',
        previewing: 'Preparing transform preview…',
        previewReady: (count: number) => `Preview ready: ${count}`,
        confirm: (count: number) => `Apply transform to ${count} tracks?`,
        applying: 'Applying batch transform…',
        applied: (result: TagApplyResult) => `Transform: changed ${result.changed}, failed ${result.failed}`,
      }

  const request = useMemo<TagTransformRequest>(() => {
    const sourceField = copyDirection === 'artistToAlbumArtist' ? 'artist' : 'albumArtist'
    const targetField = copyDirection === 'artistToAlbumArtist' ? 'albumArtist' : 'artist'

    return {
      operation,
      fields: operation === 'copy' ? [] : [...fields],
      search,
      replace: replacement,
      prefix: operation === 'prefix' ? affix : '',
      suffix: operation === 'suffix' ? affix : '',
      sourceField,
      targetField,
      caseSensitive,
    }
  }, [operation, fields, search, replacement, affix, copyDirection, caseSensitive])

  const currentKey = useMemo(() => JSON.stringify(request), [request])

  const validationError = useMemo(() => {
    if (operation !== 'copy' && fields.length === 0) return copy.selectField
    if (operation === 'replace' && search.length === 0) return copy.invalidReplace
    if ((operation === 'prefix' || operation === 'suffix') && affix.length === 0) return copy.invalidAffix
    return ''
  }, [operation, fields, search, affix, copy.selectField, copy.invalidReplace, copy.invalidAffix])

  const previewCurrent = previewKey !== '' && previewKey === currentKey

  function toggleField(field: TextField) {
    setFields((current) => (
      current.includes(field)
        ? current.filter((item) => item !== field)
        : [...current, field]
    ))
  }

  async function run(label: string, work: () => Promise<void>) {
    setBusy(true)
    onBusyChange(true)
    onMessage(label)
    try {
      await work()
    } catch (error) {
      onMessage(error instanceof Error ? error.message : String(error))
    } finally {
      setBusy(false)
      onBusyChange(false)
    }
  }

  async function previewTransform() {
    if (validationError) {
      onMessage(validationError)
      return
    }

    const key = currentKey
    await run(copy.previewing, async () => {
      const result = await api().PreviewTagTransforms(tracks.map((track) => track.id), request)
      onPreview(result ?? [])
      setPreviewKey(key)
      onMessage(copy.previewReady(result?.length ?? 0))
    })
  }

  async function applyTransform() {
    if (validationError) {
      onMessage(validationError)
      return
    }
    if (!previewCurrent) {
      onMessage(copy.previewRequired)
      return
    }
    if (!window.confirm(copy.confirm(tracks.length))) return

    await run(copy.applying, async () => {
      const result = await api().ApplyTagTransforms(tracks.map((track) => track.id), request)
      await onChanged()
      onPreview([])
      setPreviewKey('')
      onMessage(copy.applied(result))
    })
  }

  return (
    <section className="batch-tag-transforms">
      <div className="batch-transform-heading">
        <div>
          <strong>{copy.title}</strong>
          <span>{copy.hint}</span>
        </div>
      </div>

      <label className="batch-transform-operation">
        <span>{copy.operation}</span>
        <select
          value={operation}
          disabled={disabled || busy}
          onChange={(event) => setOperation(event.target.value as Operation)}
        >
          <option value="trim">{copy.trim}</option>
          <option value="upper">{copy.upper}</option>
          <option value="lower">{copy.lower}</option>
          <option value="replace">{copy.replace}</option>
          <option value="prefix">{copy.prefix}</option>
          <option value="suffix">{copy.suffix}</option>
          <option value="copy">{copy.copy}</option>
        </select>
      </label>

      {operation === 'copy' ? (
        <label className="batch-transform-operation">
          <span>{copy.fields}</span>
          <select
            value={copyDirection}
            disabled={disabled || busy}
            onChange={(event) => setCopyDirection(event.target.value as typeof copyDirection)}
          >
            <option value="artistToAlbumArtist">{copy.artistToAlbumArtist}</option>
            <option value="albumArtistToArtist">{copy.albumArtistToArtist}</option>
          </select>
        </label>
      ) : (
        <div className="batch-transform-fields">
          <span>{copy.fields}</span>
          <div>
            {textFields.map((item) => (
              <label key={item.field} className={fields.includes(item.field) ? 'active' : ''}>
                <input
                  type="checkbox"
                  checked={fields.includes(item.field)}
                  disabled={disabled || busy}
                  onChange={() => toggleField(item.field)}
                />
                <span>{translate(language, item.label)}</span>
              </label>
            ))}
          </div>
        </div>
      )}

      {operation === 'replace' && (
        <div className="batch-transform-pair">
          <label>
            <span>{copy.search}</span>
            <input
              value={search}
              disabled={disabled || busy}
              onChange={(event) => setSearch(event.target.value)}
            />
          </label>
          <label>
            <span>{copy.replacement}</span>
            <input
              value={replacement}
              disabled={disabled || busy}
              onChange={(event) => setReplacement(event.target.value)}
            />
          </label>
          <label className="batch-transform-check">
            <input
              type="checkbox"
              checked={caseSensitive}
              disabled={disabled || busy}
              onChange={(event) => setCaseSensitive(event.target.checked)}
            />
            <span>{copy.caseSensitive}</span>
          </label>
        </div>
      )}

      {(operation === 'prefix' || operation === 'suffix') && (
        <label className="batch-transform-operation">
          <span>{operation === 'prefix' ? copy.prefixValue : copy.suffixValue}</span>
          <input
            value={affix}
            disabled={disabled || busy}
            onChange={(event) => setAffix(event.target.value)}
          />
        </label>
      )}

      {validationError && <small className="batch-transform-warning">{validationError}</small>}
      {!validationError && !previewCurrent && <small className="batch-transform-note">{copy.previewRequired}</small>}

      <div className="batch-transform-actions">
        <button
          type="button"
          disabled={disabled || busy || Boolean(validationError)}
          onClick={previewTransform}
        >
          {copy.preview}
        </button>
        <button
          type="button"
          className="primary"
          disabled={disabled || busy || Boolean(validationError) || !previewCurrent}
          onClick={applyTransform}
        >
          {copy.apply}
        </button>
      </div>
    </section>
  )
}

export default BatchTagTransforms
