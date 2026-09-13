import { useEffect, useState } from 'react'
import { EventsOn } from '../wailsjs/runtime/runtime'
import { translate, type AppLanguage, type TranslationKey } from './i18n'
import type { BackgroundJob, BackgroundJobItem } from './types'

type Props = {
  language: AppLanguage
  open: boolean
  onClose: () => void
  onMessage: (message: string) => void
}

function JobsPanel({language, open, onClose, onMessage}: Props) {
  const t = (key: TranslationKey, params?: Record<string, string | number>) => translate(language, key, params)
  const [jobs, setJobs] = useState<BackgroundJob[]>([])
  const [items, setItems] = useState<Record<number, BackgroundJobItem[]>>({})
  const [expanded, setExpanded] = useState<Record<number, boolean>>({})
  const [loading, setLoading] = useState(false)
  const [failedOnly, setFailedOnly] = useState(false)

  async function refresh() {
    if (!open) return
    setLoading(true)
    try {
      const result = await window.go.main.App.ListBackgroundJobs(100)
      setJobs(result ?? [])
      const expandedIDs = Object.entries(expanded).filter(([, value]) => value).map(([key]) => Number(key))
      for (const jobID of expandedIDs) {
        const detail = await window.go.main.App.ListBackgroundJobItems(jobID, 500, 0)
        setItems((current) => ({...current, [jobID]: detail ?? []}))
      }
    } catch (error) {
      onMessage(error instanceof Error ? error.message : String(error))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (!open) return
    void refresh()
    const timer = window.setInterval(() => void refresh(), 3000)
    const offCreated = EventsOn('jobs:created', () => void refresh())
    const offUpdated = EventsOn('jobs:updated', () => void refresh())
    const offError = EventsOn('jobs:error', (message: string) => onMessage(message))
    return () => {
      window.clearInterval(timer)
      offCreated()
      offUpdated()
      offError()
    }
  }, [open, language, expanded])

  if (!open) return null

  async function loadItems(jobID: number) {
    const next = !expanded[jobID]
    setExpanded((current) => ({...current, [jobID]: next}))
    if (!next) return
    try {
      const result = await window.go.main.App.ListBackgroundJobItems(jobID, 500, 0)
      setItems((current) => ({...current, [jobID]: result ?? []}))
    } catch (error) {
      onMessage(error instanceof Error ? error.message : String(error))
    }
  }

  async function control(action: 'pause' | 'resume' | 'cancel' | 'retry', jobID: number) {
    try {
      if (action === 'pause') await window.go.main.App.PauseBackgroundJob(jobID)
      if (action === 'resume') await window.go.main.App.ResumeBackgroundJob(jobID)
      if (action === 'cancel') await window.go.main.App.CancelBackgroundJob(jobID)
      if (action === 'retry') await window.go.main.App.RetryFailedBackgroundJob(jobID)
      await refresh()
    } catch (error) {
      onMessage(error instanceof Error ? error.message : String(error))
    }
  }

  return (
    <div className="settings-backdrop" role="presentation" onMouseDown={(event) => event.target === event.currentTarget && onClose()}>
      <section className="settings-modal jobs-modal panel" role="dialog" aria-modal="true" aria-label={t('jobs.title')}>
        <div className="settings-header">
          <div>
            <h2>{t('jobs.title')}</h2>
            <span>{t('jobs.subtitle')}</span>
          </div>
          <button onClick={onClose}>×</button>
        </div>
        <div className="settings-body jobs-body">
          <div className="jobs-toolbar">
            <label className="jobs-filter"><input type="checkbox" checked={failedOnly} onChange={(event) => setFailedOnly(event.target.checked)} /> {t('jobs.failedOnly')}</label>
            <button onClick={() => void refresh()} disabled={loading}>{t('jobs.refresh')}</button>
          </div>
          {jobs.length === 0 && <p className="settings-state">{t('jobs.empty')}</p>}
          <div className="jobs-list">
            {jobs.map((job) => {
              const cancelled = job.cancelledItems ?? 0
              const done = job.completedItems + job.skippedItems + job.failedItems + cancelled
              const statusKey = (`jobs.status.${job.status}`) as TranslationKey
              const jobTypeKey = (`jobs.type.${job.type}`) as TranslationKey
              return (
                <article className={`job-card ${job.status}`} key={job.id}>
                  <div className="job-head">
                    <div>
                      <strong>{t(jobTypeKey)}</strong>
                      <small>#{job.id} · {t(statusKey)}</small>
                    </div>
                    <span>{Math.round(Math.max(0, Math.min(1, job.progress)) * 100)}%</span>
                  </div>
                  <div className="job-progress"><span style={{width: `${Math.round(Math.max(0, Math.min(1, job.progress)) * 100)}%`}} /></div>
                  <div className="job-summary">{t('jobs.summary', {done, total: job.totalItems, skipped: job.skippedItems, failed: job.failedItems, cancelled})}</div>
                  {job.currentItem && <div className="job-current"><span>{t('jobs.current')}</span><code title={job.currentItem}>{job.currentItem}</code></div>}
                  {job.lastError && <p className="job-error">{job.lastError}</p>}
                  <div className="job-actions">
                    {job.status === 'running' || job.status === 'queued' ? <button onClick={() => void control('pause', job.id)}>{t('jobs.pause')}</button> : null}
                    {job.status === 'paused' ? <button onClick={() => void control('resume', job.id)}>{t('jobs.resume')}</button> : null}
                    {job.status === 'running' || job.status === 'queued' || job.status === 'paused' ? <button onClick={() => void control('cancel', job.id)}>{t('jobs.cancel')}</button> : null}
                    {job.failedItems > 0 ? <button onClick={() => void control('retry', job.id)}>{t('jobs.retry')}</button> : null}
                    <button onClick={() => void loadItems(job.id)}>{expanded[job.id] ? t('jobs.hideItems') : t('jobs.showItems')}</button>
                  </div>
                  {expanded[job.id] && (
                    <div className="job-items">
                      {(items[job.id] ?? []).filter((item) => !failedOnly || item.status === 'failed').map((item) => {
                        const result = parseEnrichmentResult(item.resultJson)
                        const resultText = result ? formatEnrichmentResult(result, t) : ''
                        return (
                          <div className={`job-item ${item.status}`} key={item.id}>
                            <span>{t((`jobs.item.${item.status}`) as TranslationKey)}</span>
                            <code title={item.path}>{item.path}</code>
                            <small>{t('jobs.attempts')}: {item.attempts}</small>
                            {resultText && <small className="job-item-result">{resultText}</small>}
                            {item.error && <small className="job-item-error">{item.error}</small>}
                          </div>
                        )
                      })}
                    </div>
                  )}
                </article>
              )
            })}
          </div>
        </div>
        <div className="settings-footer">
          <button onClick={onClose}>{t('jobs.close')}</button>
        </div>
      </section>
    </div>
  )
}

type EnrichmentItemResult = {
  source?: string
  confidence?: number
  applied?: boolean
  skipped?: boolean
}

function parseEnrichmentResult(raw: string): EnrichmentItemResult | null {
  if (!raw) return null
  try {
    const value = JSON.parse(raw) as EnrichmentItemResult
    return value && typeof value === 'object' ? value : null
  } catch {
    return null
  }
}

function formatEnrichmentResult(result: EnrichmentItemResult, t: (key: TranslationKey, params?: Record<string, string | number>) => string): string {
  if (result.applied) {
    return t('jobs.resultApplied', {source: result.source || '—', confidence: Math.round((result.confidence ?? 0) * 100)})
  }
  if (result.skipped) {
    return t('jobs.resultSkipped', {source: result.source || '—', confidence: Math.round((result.confidence ?? 0) * 100)})
  }
  return ''
}

export default JobsPanel
