import {translate, type AppLanguage, type TranslationKey} from './i18n'
import type {TagPreview} from './types'

type EditableField =
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
  | 'year'
  | 'trackNumber'
  | 'trackTotal'
  | 'discNumber'
  | 'discTotal'

type Props = {
  language: AppLanguage
  items: TagPreview[]
  busy: boolean
  onClose: () => void
  onApply: () => Promise<void>
}

const fields: Array<{field: EditableField; label: TranslationKey}> = [
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
  {field: 'year', label: 'tags.field.year'},
  {field: 'trackNumber', label: 'tags.field.track'},
  {field: 'trackTotal', label: 'tags.field.trackTotal'},
  {field: 'discNumber', label: 'tags.field.disc'},
  {field: 'discTotal', label: 'tags.field.discTotal'},
]

function displayValue(value: string | number): string {
  if (value === '' || value === 0) return '—'
  return String(value)
}

function BatchPreviewModal({language, items, busy, onClose, onApply}: Props) {
  const copy = language === 'ru'
    ? {
        title: 'Предпросмотр изменений',
        subtitle: 'Проверьте значения до записи. Применение выполнит одну массовую операцию с единым Undo.',
        tracks: 'Файлов',
        changedTracks: 'Будет изменено',
        changes: 'Изменений полей',
        unchanged: 'Без изменений',
        field: 'Поле',
        before: 'До',
        after: 'После',
        close: 'Закрыть',
        apply: 'Применить изменения',
        applying: 'Применение…',
        noChanges: 'Фактических изменений нет.',
      }
    : {
        title: 'Change preview',
        subtitle: 'Review values before writing. Apply performs one batch operation with a single Undo.',
        tracks: 'Files',
        changedTracks: 'Will change',
        changes: 'Field changes',
        unchanged: 'Unchanged',
        field: 'Field',
        before: 'Before',
        after: 'After',
        close: 'Close',
        apply: 'Apply changes',
        applying: 'Applying…',
        noChanges: 'There are no actual changes.',
      }

  const prepared = items.map((item) => {
    const changes = fields
      .filter(({field}) => item.before[field] !== item.after[field])
      .map(({field, label}) => ({
        field,
        label: translate(language, label),
        before: displayValue(item.before[field]),
        after: displayValue(item.after[field]),
      }))

    return {item, changes}
  })

  const changed = prepared.filter((entry) => entry.changes.length > 0)
  const fieldChanges = changed.reduce((total, entry) => total + entry.changes.length, 0)
  const unchanged = items.length - changed.length

  return (
    <div
      className="batch-preview-backdrop"
      role="presentation"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget && !busy) onClose()
      }}
    >
      <section
        className="batch-preview-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="batch-preview-title"
      >
        <header className="batch-preview-head">
          <div>
            <h2 id="batch-preview-title">{copy.title}</h2>
            <p>{copy.subtitle}</p>
          </div>
          <button type="button" disabled={busy} onClick={onClose} aria-label={copy.close}>×</button>
        </header>

        <div className="batch-preview-stats">
          <span><strong>{items.length}</strong>{copy.tracks}</span>
          <span><strong>{changed.length}</strong>{copy.changedTracks}</span>
          <span><strong>{fieldChanges}</strong>{copy.changes}</span>
          <span><strong>{unchanged}</strong>{copy.unchanged}</span>
        </div>

        <div className="batch-preview-body">
          {changed.length === 0 ? (
            <div className="batch-preview-empty">{copy.noChanges}</div>
          ) : (
            changed.map(({item, changes}) => (
              <article className="batch-preview-file" key={item.trackId}>
                <header title={item.path}>{item.path}</header>
                <div className="batch-preview-table" role="table">
                  <div className="batch-preview-row batch-preview-row-head" role="row">
                    <span role="columnheader">{copy.field}</span>
                    <span role="columnheader">{copy.before}</span>
                    <span role="columnheader">{copy.after}</span>
                  </div>
                  {changes.map((change) => (
                    <div className="batch-preview-row" role="row" key={change.field}>
                      <strong role="cell">{change.label}</strong>
                      <span role="cell" title={change.before}>{change.before}</span>
                      <span role="cell" className="after" title={change.after}>{change.after}</span>
                    </div>
                  ))}
                </div>
              </article>
            ))
          )}
        </div>

        <footer className="batch-preview-footer">
          <button type="button" disabled={busy} onClick={onClose}>{copy.close}</button>
          <button
            type="button"
            className="primary"
            disabled={busy || changed.length === 0}
            onClick={() => void onApply()}
          >
            {busy ? copy.applying : copy.apply}
          </button>
        </footer>
      </section>
    </div>
  )
}

export default BatchPreviewModal
