import { useState } from 'react'
import { translate, type AppLanguage, type TranslateParams, type TranslationKey } from './i18n'

type Props = {
  language: AppLanguage
  selectedIDs: number[]
  libraryCount: number
  essentiaReady: boolean
  disabled: boolean
  onMessage: (message: string) => void
  onQueued: () => void
}

export default function EssentiaBatchControls({
  language,
  selectedIDs,
  libraryCount,
  essentiaReady,
  disabled,
  onMessage,
  onQueued,
}: Props) {
  const t = (key: TranslationKey, params?: TranslateParams) => translate(language, key, params)
  const [writeTags, setWriteTags] = useState(false)
  const [onlyMissing, setOnlyMissing] = useState(true)
  const [skipUnchanged, setSkipUnchanged] = useState(true)
  const [busy, setBusy] = useState(false)

  async function queueSelected() {
    if (selectedIDs.length === 0 || busy || disabled || !essentiaReady) return
    if (writeTags && !window.confirm(t('essentia.selectedConfirm', {count: selectedIDs.length}))) return
    setBusy(true)
    try {
      await window.go.main.App.CreateEssentiaAnalysisJob(selectedIDs, writeTags, onlyMissing, skipUnchanged)
      onMessage(t('essentia.queued', {count: selectedIDs.length}))
      onQueued()
    } catch (error) {
      onMessage(error instanceof Error ? error.message : String(error))
    } finally {
      setBusy(false)
    }
  }

  async function queueLibrary() {
    if (libraryCount <= 0 || busy || disabled || !essentiaReady) return
    if (!window.confirm(t('essentia.libraryConfirm', {count: libraryCount}))) return
    setBusy(true)
    try {
      await window.go.main.App.CreateLibraryEssentiaAnalysisJob(writeTags, onlyMissing, skipUnchanged)
      onMessage(t('essentia.libraryQueued', {count: libraryCount}))
      onQueued()
    } catch (error) {
      onMessage(error instanceof Error ? error.message : String(error))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="compact-enrichment">
      <strong>{t('essentia.batchTitle')}</strong>
      <small className="enrichment-mode-hint">{t('essentia.batchHint')}</small>
      <label className="inline-check">
        <input type="checkbox" checked={writeTags} onChange={(event) => setWriteTags(event.target.checked)} />
        {t('essentia.writeTags')}
      </label>
      <label className="inline-check">
        <input type="checkbox" checked={onlyMissing} disabled={!writeTags} onChange={(event) => setOnlyMissing(event.target.checked)} />
        {t('essentia.onlyMissing')}
      </label>
      <label className="inline-check">
        <input type="checkbox" checked={skipUnchanged} onChange={(event) => setSkipUnchanged(event.target.checked)} />
        {t('essentia.skipUnchanged')}
      </label>
      <div className="inspector-action-row">
        <button type="button" onClick={() => void queueSelected()} disabled={busy || disabled || !essentiaReady || selectedIDs.length === 0}>
          {t('essentia.selected', {count: selectedIDs.length})}
        </button>
        <button type="button" onClick={() => void queueLibrary()} disabled={busy || disabled || !essentiaReady || libraryCount <= 0}>
          {t('essentia.library', {count: libraryCount})}
        </button>
      </div>
    </div>
  )
}
