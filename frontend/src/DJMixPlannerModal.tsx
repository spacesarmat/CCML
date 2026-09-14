import {useEffect, useMemo, useState} from 'react'
import {translate, type AppLanguage, type TranslationKey} from './i18n'
import type {DJMixPlan, DJMixPlanOptions, DJMixPlanStep} from './types'
import './djMixPlanner.css'

type Props = {
  language: AppLanguage
  open: boolean
  selectedIDs: number[]
  libraryCount: number
  onClose: () => void
  onMessage: (message: string) => void
  onRevealTrack: (trackID: number) => void | Promise<void>
}

function DJMixPlannerModal({
  language,
  open,
  selectedIDs,
  libraryCount,
  onClose,
  onMessage,
  onRevealTrack,
}: Props) {
  const [plan, setPlan] = useState<DJMixPlan | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [startTrackID, setStartTrackID] = useState(0)
  const [limit, setLimit] = useState(20)
  const [maxTempoShiftPct, setMaxTempoShiftPct] = useState(8)
  const [direction, setDirection] = useState<'any' | 'up' | 'down'>('any')
  const [preferHarmonic, setPreferHarmonic] = useState(true)
  const [avoidSameArtist, setAvoidSameArtist] = useState(true)

  const t = (key: TranslationKey, params?: Record<string, string | number>) => translate(language, key, params)
  const scopeIDs = useMemo(() => selectedIDs.length >= 2 ? selectedIDs : [], [selectedIDs])
  const scopeLabel = scopeIDs.length > 0
    ? t('mixPlanner.scopeSelected', {count: scopeIDs.length})
    : t('mixPlanner.scopeLibrary', {count: libraryCount})

  useEffect(() => {
    if (!open) return
    const seed = selectedIDs.length === 1 ? selectedIDs[0] : 0
    setStartTrackID(seed)
    setError('')
    setPlan(null)
    void build(seed)
  }, [open])

  useEffect(() => {
    if (!open) return
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return
      event.preventDefault()
      onClose()
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [open, onClose])

  async function build(seed = startTrackID) {
    if (!window.go?.main?.App) return
    setLoading(true)
    setError('')
    try {
      const options: DJMixPlanOptions = {
        startTrackId: seed,
        limit,
        maxTempoShiftPct,
        direction,
        preferHarmonic,
        avoidSameArtist,
      }
      const result = await window.go.main.App.PlanDJMix(scopeIDs, options)
      setPlan(result)
      setStartTrackID(result.startTrackId || seed)
      onMessage(t('mixPlanner.ready', {count: result.steps?.length ?? 0}))
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(false)
    }
  }

  function useAsStart(trackID: number) {
    setStartTrackID(trackID)
    void build(trackID)
  }

  if (!open) return null

  return (
    <div className="mix-planner-backdrop" role="presentation" onMouseDown={(event) => event.target === event.currentTarget && onClose()}>
      <section className="mix-planner-modal panel" role="dialog" aria-modal="true" aria-label={t('mixPlanner.title')}>
        <header className="mix-planner-header">
          <div>
            <h2>{t('mixPlanner.title')}</h2>
            <p>{t('mixPlanner.subtitle')}</p>
            <span>{scopeLabel}</span>
          </div>
          <button type="button" onClick={onClose} aria-label={t('mixPlanner.close')} title={t('mixPlanner.close')}>×</button>
        </header>

        <div className="mix-planner-controls">
          <label>
            <span>{t('mixPlanner.limit')}</span>
            <input type="number" min={2} max={100} step={1} value={limit} onChange={(event) => setLimit(clampInt(Number(event.target.value), 2, 100, 20))} />
          </label>
          <label>
            <span>{t('mixPlanner.maxTempoShift')}</span>
            <input type="number" min={1} max={25} step={0.5} value={maxTempoShiftPct} onChange={(event) => setMaxTempoShiftPct(clampNumber(Number(event.target.value), 1, 25, 8))} />
          </label>
          <label>
            <span>{t('mixPlanner.direction')}</span>
            <select value={direction} onChange={(event) => setDirection(event.target.value as 'any' | 'up' | 'down')}>
              <option value="any">{t('mixPlanner.directionAny')}</option>
              <option value="up">{t('mixPlanner.directionUp')}</option>
              <option value="down">{t('mixPlanner.directionDown')}</option>
            </select>
          </label>
          <label className="mix-planner-check">
            <input type="checkbox" checked={preferHarmonic} onChange={(event) => setPreferHarmonic(event.target.checked)} />
            <span>{t('mixPlanner.preferHarmonic')}</span>
          </label>
          <label className="mix-planner-check">
            <input type="checkbox" checked={avoidSameArtist} onChange={(event) => setAvoidSameArtist(event.target.checked)} />
            <span>{t('mixPlanner.avoidSameArtist')}</span>
          </label>
          <button className="primary" type="button" onClick={() => void build()} disabled={loading}>
            {loading ? t('mixPlanner.building') : t('mixPlanner.build')}
          </button>
        </div>

        {error && <div className="mix-planner-error">{error}</div>}

        {plan && (
          <div className="mix-planner-body">
            <div className="mix-planner-summary">
              <Summary value={plan.steps?.length ?? 0} label={t('mixPlanner.planTracks')} />
              <Summary value={`${Math.round((plan.averageScore || 0) * 100)}%`} label={t('mixPlanner.avgScore')} />
              <Summary value={plan.excludedMissingBpm} label={t('mixPlanner.missingBpm')} />
              <Summary value={plan.tracksMissingKey} label={t('mixPlanner.missingKey')} />
            </div>

            {(plan.steps?.length ?? 0) === 0 ? (
              <div className="mix-planner-empty">{t('mixPlanner.empty')}</div>
            ) : (
              <div className="mix-planner-table-wrap">
                <table className="mix-planner-table">
                  <thead>
                    <tr>
                      <th>#</th>
                      <th>{t('mixPlanner.track')}</th>
                      <th>{t('mixPlanner.bpm')}</th>
                      <th>{t('mixPlanner.key')}</th>
                      <th>{t('mixPlanner.transition')}</th>
                      <th>{t('mixPlanner.score')}</th>
                      <th />
                    </tr>
                  </thead>
                  <tbody>
                    {(plan.steps ?? []).map((step) => (
                      <PlannerRow
                        key={`${step.position}-${step.track.id}`}
                        step={step}
                        isStart={step.track.id === plan.startTrackId}
                        t={t}
                        onStart={() => useAsStart(step.track.id)}
                        onReveal={() => void onRevealTrack(step.track.id)}
                      />
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        )}

        <footer className="mix-planner-footer">
          <span>{startTrackID > 0 ? `#${startTrackID}` : t('mixPlanner.startAuto')}</span>
          <button type="button" onClick={onClose}>{t('mixPlanner.close')}</button>
        </footer>
      </section>
    </div>
  )
}

function PlannerRow({
  step,
  isStart,
  t,
  onStart,
  onReveal,
}: {
  step: DJMixPlanStep
  isStart: boolean
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
  onStart: () => void
  onReveal: () => void
}) {
  const factor = Math.abs(step.tempoFactor - 1) > 0.001
    ? step.tempoFactor > 1 ? `×${formatFactor(step.tempoFactor)}` : `÷${formatFactor(1 / step.tempoFactor)}`
    : ''
  const bpm = factor
    ? `${formatBPM(step.track.bpm)} ${factor} → ${formatBPM(step.adjustedBpm)}`
    : formatBPM(step.track.bpm)
  const transition = step.keyRelation === 'start'
    ? t('mixPlanner.relation.start')
    : `${formatSigned(step.tempoDeltaPct)}% · ${relationLabel(step.keyRelation, t)}`

  return (
    <tr className={isStart ? 'is-start' : ''}>
      <td className="mix-position">{step.position}</td>
      <td className="mix-track">
        <strong>{step.track.artist || '—'} — {step.track.title || step.track.fileName}</strong>
        <small>{step.track.genre || step.track.album || step.track.path}</small>
        {(step.warnings ?? []).length > 0 && (
          <span className="mix-warning-row">
            {(step.warnings ?? []).map((warning) => <em key={warning}>{warningLabel(warning, t)}</em>)}
          </span>
        )}
      </td>
      <td className="mix-bpm">{bpm}</td>
      <td>{step.camelot || '—'}{step.openKey ? <small className="mix-open-key">{step.openKey}</small> : null}</td>
      <td>{transition}</td>
      <td><b className="mix-score">{Math.round(step.score * 100)}%</b></td>
      <td className="mix-row-actions">
        {!isStart && <button type="button" onClick={onStart}>{t('mixPlanner.startHere')}</button>}
        <button type="button" onClick={onReveal}>{t('mixPlanner.reveal')}</button>
      </td>
    </tr>
  )
}

function Summary({value, label}: {value: string | number; label: string}) {
  return <div><strong>{value}</strong><span>{label}</span></div>
}

function relationLabel(value: string, t: (key: TranslationKey) => string): string {
  const key = ({
    same: 'mixPlanner.relation.same',
    adjacent: 'mixPlanner.relation.adjacent',
    relative: 'mixPlanner.relation.relative',
    conflict: 'mixPlanner.relation.conflict',
    unknown: 'mixPlanner.relation.unknown',
    start: 'mixPlanner.relation.start',
  } as Record<string, TranslationKey>)[value] ?? 'mixPlanner.relation.unknown'
  return t(key)
}

function warningLabel(value: string, t: (key: TranslationKey) => string): string {
  const key = ({
    tempo_jump: 'mixPlanner.warning.tempo_jump',
    half_double: 'mixPlanner.warning.half_double',
    key_conflict: 'mixPlanner.warning.key_conflict',
    key_unknown: 'mixPlanner.warning.key_unknown',
    same_artist: 'mixPlanner.warning.same_artist',
    direction_reverse: 'mixPlanner.warning.direction_reverse',
  } as Record<string, TranslationKey>)[value]
  return key ? t(key) : value
}

function formatBPM(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '—'
  return value.toFixed(Math.abs(value - Math.round(value)) < 0.05 ? 0 : 1)
}

function formatSigned(value: number): string {
  if (!Number.isFinite(value)) return '0.0'
  return `${value >= 0 ? '+' : ''}${value.toFixed(1)}`
}

function formatFactor(value: number): string {
  return Number.isInteger(value) ? String(value) : value.toFixed(1)
}

function clampInt(value: number, min: number, max: number, fallback: number): number {
  if (!Number.isFinite(value)) return fallback
  return Math.max(min, Math.min(max, Math.round(value)))
}

function clampNumber(value: number, min: number, max: number, fallback: number): number {
  if (!Number.isFinite(value)) return fallback
  return Math.max(min, Math.min(max, value))
}

export default DJMixPlannerModal
