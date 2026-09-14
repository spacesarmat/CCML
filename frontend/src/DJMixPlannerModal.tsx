import {useEffect, useMemo, useState, type DragEvent} from 'react'
import {translate, type AppLanguage, type TranslationKey} from './i18n'
import type {DJMixPin, DJMixPlan, DJMixPlanOptions, DJMixPlanStep, DJMixSavedPlan} from './types'
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
  const [lookahead, setLookahead] = useState(3)
  const [direction, setDirection] = useState<'any' | 'up' | 'down'>('any')
  const [preferHarmonic, setPreferHarmonic] = useState(true)
  const [preferGenreContinuity, setPreferGenreContinuity] = useState(true)
  const [preferEnergyFlow, setPreferEnergyFlow] = useState(true)
  const [avoidSameArtist, setAvoidSameArtist] = useState(true)
  const [pins, setPins] = useState<Record<number, number>>({})
  const [savedPlans, setSavedPlans] = useState<DJMixSavedPlan[]>([])
  const [savedPlanID, setSavedPlanID] = useState(0)
  const [planName, setPlanName] = useState('')
  const [savedScopeIDs, setSavedScopeIDs] = useState<number[] | null>(null)
  const [savedLoading, setSavedLoading] = useState(false)
  const [manualBusy, setManualBusy] = useState(false)
  const [dragTrackID, setDragTrackID] = useState(0)
  const [dragOverTrackID, setDragOverTrackID] = useState(0)

  const t = (key: TranslationKey, params?: Record<string, string | number>) => translate(language, key, params)
  const scopeIDs = useMemo(() => selectedIDs.length >= 2 ? selectedIDs : [], [selectedIDs])
  const effectiveScopeIDs = savedScopeIDs ?? scopeIDs
  const scopeLabel = savedScopeIDs !== null
    ? t('mixPlanner.scopeSaved', {count: savedScopeIDs.length > 0 ? savedScopeIDs.length : libraryCount})
    : scopeIDs.length > 0
      ? t('mixPlanner.scopeSelected', {count: scopeIDs.length})
      : t('mixPlanner.scopeLibrary', {count: libraryCount})

  useEffect(() => {
    if (!open) return
    const seed = selectedIDs.length === 1 ? selectedIDs[0] : 0
    setStartTrackID(seed)
    setPins({})
    setSavedPlanID(0)
    setPlanName(t('mixPlanner.defaultName'))
    setSavedScopeIDs(null)
    setError('')
    setPlan(null)
    void refreshSavedPlans()
    void build(seed, {}, scopeIDs)
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

  function plannerOptions(seed = startTrackID, pinState = pins): DJMixPlanOptions {
    const pinnedTracks: DJMixPin[] = Object.entries(pinState)
      .map(([position, trackId]) => ({position: Number(position), trackId}))
      .filter((pin) => Number.isFinite(pin.position) && pin.position > 0 && pin.trackId > 0)
      .sort((left, right) => left.position - right.position)
    return {
      startTrackId: seed,
      limit,
      maxTempoShiftPct,
      direction,
      preferHarmonic,
      avoidSameArtist,
      lookahead,
      preferGenreContinuity,
      preferEnergyFlow,
      pinnedTracks,
    }
  }

  async function build(seed = startTrackID, pinState = pins, planScope = effectiveScopeIDs) {
    if (!window.go?.main?.App) return
    setLoading(true)
    setError('')
    try {
      const result = await window.go.main.App.PlanDJMix(planScope, plannerOptions(seed, pinState))
      setPlan(result)
      setStartTrackID(result.startTrackId || seed)
      onMessage(t('mixPlanner.ready', {count: result.steps?.length ?? 0}))
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(false)
    }
  }

  async function refreshSavedPlans() {
    if (!window.go?.main?.App) return
    try {
      const saved = await window.go.main.App.ListSavedDJMixPlans(100)
      setSavedPlans(saved ?? [])
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  async function saveCurrentPlan(asNew: boolean) {
    if (!window.go?.main?.App || !plan) return
    const name = planName.trim()
    if (!name) {
      setError(t('mixPlanner.nameRequired'))
      return
    }
    setSavedLoading(true)
    setError('')
    try {
      const saved = await window.go.main.App.SaveDJMixPlan(
        asNew ? 0 : savedPlanID,
        name,
        effectiveScopeIDs,
        plannerOptions(),
        plan,
      )
      setSavedPlanID(saved.id)
      setPlanName(saved.name)
      setSavedScopeIDs(saved.scopeTrackIds ?? [])
      await refreshSavedPlans()
      onMessage(t('mixPlanner.saved', {name: saved.name}))
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setSavedLoading(false)
    }
  }

  async function loadSavedPlan(id: number) {
    if (!window.go?.main?.App || id <= 0) return
    setSavedLoading(true)
    setError('')
    try {
      const saved = await window.go.main.App.LoadSavedDJMixPlan(id)
      const options = saved.options
      const nextPins: Record<number, number> = {}
      for (const pin of options.pinnedTracks ?? []) {
        if (pin.position > 0 && pin.trackId > 0) nextPins[pin.position] = pin.trackId
      }
      setSavedPlanID(saved.id)
      setPlanName(saved.name)
      setSavedScopeIDs(saved.scopeTrackIds ?? [])
      setStartTrackID(options.startTrackId || saved.plan.startTrackId || 0)
      setLimit(options.limit || 20)
      setMaxTempoShiftPct(options.maxTempoShiftPct || 8)
      setLookahead(options.lookahead || 3)
      setDirection(options.direction === 'up' || options.direction === 'down' ? options.direction : 'any')
      setPreferHarmonic(options.preferHarmonic)
      setAvoidSameArtist(options.avoidSameArtist)
      setPreferGenreContinuity(options.preferGenreContinuity)
      setPreferEnergyFlow(options.preferEnergyFlow)
      setPins(nextPins)
      setPlan(saved.plan)
      onMessage(t('mixPlanner.loaded', {name: saved.name}))
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setSavedLoading(false)
    }
  }

  async function deleteSavedPlan() {
    if (!window.go?.main?.App || savedPlanID <= 0) return
    const current = savedPlans.find((item) => item.id === savedPlanID)
    if (!window.confirm(t('mixPlanner.deleteConfirm', {name: current?.name || planName}))) return
    setSavedLoading(true)
    setError('')
    try {
      await window.go.main.App.DeleteSavedDJMixPlan(savedPlanID)
      setSavedPlanID(0)
      setPlanName(t('mixPlanner.defaultName'))
      setSavedScopeIDs(null)
      await refreshSavedPlans()
      onMessage(t('mixPlanner.deleted'))
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setSavedLoading(false)
    }
  }

  async function exportPlan(format: 'm3u8' | 'csv') {
    if (!window.go?.main?.App || !plan) return
    setSavedLoading(true)
    setError('')
    try {
      const path = await window.go.main.App.ExportDJMixPlan(planName.trim() || t('mixPlanner.defaultName'), format, plan)
      if (path) onMessage(t('mixPlanner.exported', {path}))
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setSavedLoading(false)
    }
  }

  function newPlan() {
    const seed = selectedIDs.length === 1 ? selectedIDs[0] : 0
    setSavedPlanID(0)
    setPlanName(t('mixPlanner.defaultName'))
    setSavedScopeIDs(null)
    setStartTrackID(seed)
    setPins({})
    void build(seed, {}, scopeIDs)
  }

  function useAsStart(trackID: number) {
    const nextPins = {...pins}
    delete nextPins[1]
    setPins(nextPins)
    setStartTrackID(trackID)
    void build(trackID, nextPins)
  }

  function togglePin(position: number, trackID: number) {
    const nextPins = {...pins}
    for (const [rawPosition, pinnedTrackID] of Object.entries(nextPins)) {
      if (pinnedTrackID === trackID) delete nextPins[Number(rawPosition)]
    }
    if (pins[position] === trackID) {
      delete nextPins[position]
    } else {
      nextPins[position] = trackID
    }
    setPins(nextPins)
    void build(startTrackID, nextPins)
  }

  async function applyManualOrder(nextSteps: DJMixPlanStep[]) {
    if (!window.go?.main?.App || !plan || nextSteps.length === 0) return
    setManualBusy(true)
    setError('')
    try {
      const result = await window.go.main.App.RecalculateDJMixPlan(
        {...plan, steps: nextSteps},
        plannerOptions(),
      )
      setPlan(result)
      setStartTrackID(result.startTrackId)
      setPins(pinsFromSteps(result.steps ?? []))
      onMessage(t('mixPlanner.manualRecalculated'))
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setManualBusy(false)
    }
  }

  function moveManualStep(trackID: number, direction: -1 | 1) {
    if (!plan?.steps) return
    const next = moveUnlockedStep(plan.steps, trackID, direction)
    if (next) void applyManualOrder(next)
  }

  function dropManualStep(sourceTrackID: number, targetTrackID: number) {
    if (!plan?.steps || sourceTrackID <= 0 || targetTrackID <= 0) return
    const next = reorderUnlockedSteps(plan.steps, sourceTrackID, targetTrackID)
    if (next) void applyManualOrder(next)
  }

  function toggleLock(trackID: number) {
    setPlan((current) => {
      if (!current?.steps) return current
      const steps = current.steps.map((step) => step.track.id === trackID ? {...step, locked: !step.locked} : step)
      return {...current, steps, lockedCount: steps.filter((step) => step.locked).length}
    })
  }

  function updateManualNote(trackID: number, field: 'transitionNote' | 'cueNote', value: string) {
    setPlan((current) => {
      if (!current?.steps) return current
      return {
        ...current,
        steps: current.steps.map((step) => step.track.id === trackID ? {...step, [field]: value} : step),
      }
    })
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

        <div className="mix-planner-savedbar">
          <select
            value={savedPlanID || ''}
            onChange={(event) => {
              const id = Number(event.target.value)
              if (id > 0) void loadSavedPlan(id)
            }}
            disabled={savedLoading}
            aria-label={t('mixPlanner.savedPlans')}
          >
            <option value="">{t('mixPlanner.savedChoose')}</option>
            {savedPlans.map((saved) => (
              <option key={saved.id} value={saved.id}>{saved.name}</option>
            ))}
          </select>
          <input
            value={planName}
            maxLength={120}
            onChange={(event) => setPlanName(event.target.value)}
            placeholder={t('mixPlanner.name')}
            aria-label={t('mixPlanner.name')}
          />
          <button type="button" onClick={newPlan} disabled={savedLoading}>{t('mixPlanner.newPlan')}</button>
          <button type="button" onClick={() => void saveCurrentPlan(false)} disabled={savedLoading || !plan}>{t('mixPlanner.save')}</button>
          <button type="button" onClick={() => void saveCurrentPlan(true)} disabled={savedLoading || !plan}>{t('mixPlanner.saveAs')}</button>
          <button type="button" onClick={() => void deleteSavedPlan()} disabled={savedLoading || savedPlanID <= 0}>{t('mixPlanner.delete')}</button>
          <span className="mix-planner-saved-spacer" />
          <button type="button" onClick={() => void exportPlan('m3u8')} disabled={savedLoading || !plan}>{t('mixPlanner.exportM3U8')}</button>
          <button type="button" onClick={() => void exportPlan('csv')} disabled={savedLoading || !plan}>{t('mixPlanner.exportCSV')}</button>
        </div>

        <div className="mix-planner-controls">
          <label>
            <span>{t('mixPlanner.limit')}</span>
            <input type="number" min={2} max={100} step={1} value={limit} onChange={(event) => setLimit(clampInt(Number(event.target.value), 2, 100, 20))} />
          </label>
          <label>
            <span>{t('mixPlanner.lookahead')}</span>
            <select value={lookahead} onChange={(event) => setLookahead(clampInt(Number(event.target.value), 1, 4, 3))}>
              <option value={1}>1</option>
              <option value={2}>2</option>
              <option value={3}>3</option>
              <option value={4}>4</option>
            </select>
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
            <input type="checkbox" checked={preferGenreContinuity} onChange={(event) => setPreferGenreContinuity(event.target.checked)} />
            <span>{t('mixPlanner.preferGenre')}</span>
          </label>
          <label className="mix-planner-check">
            <input type="checkbox" checked={preferEnergyFlow} onChange={(event) => setPreferEnergyFlow(event.target.checked)} />
            <span>{t('mixPlanner.preferEnergy')}</span>
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
              <Summary value={plan.lookahead || lookahead} label={t('mixPlanner.lookahead')} />
              <Summary value={plan.pinnedCount} label={t('mixPlanner.pinned')} />
              <Summary value={plan.lockedCount ?? 0} label={t('mixPlanner.locked')} />
              <Summary value={plan.excludedMissingBpm} label={t('mixPlanner.missingBpm')} />
              <Summary value={plan.tracksMissingKey} label={t('mixPlanner.missingKey')} />
            </div>
            {plan.ignoredPins > 0 && <div className="mix-planner-pin-note">{t('mixPlanner.ignoredPins', {count: plan.ignoredPins})}</div>}
            {plan.manualOrder && <div className="mix-planner-manual-note">{t('mixPlanner.manualActive')}</div>}

            {(plan.steps?.length ?? 0) === 0 ? (
              <div className="mix-planner-empty">{t('mixPlanner.empty')}</div>
            ) : (
              <div className="mix-planner-table-wrap">
                <table className="mix-planner-table">
                  <thead>
                    <tr>
                      <th>#</th>
                      <th>{t('mixPlanner.timeline')}</th>
                      <th>{t('mixPlanner.track')}</th>
                      <th>{t('mixPlanner.bpm')}</th>
                      <th>{t('mixPlanner.key')}</th>
                      <th>{t('mixPlanner.flow')}</th>
                      <th>{t('mixPlanner.transition')}</th>
                      <th>{t('mixPlanner.notes')}</th>
                      <th>{t('mixPlanner.score')}</th>
                      <th />
                    </tr>
                  </thead>
                  <tbody>
                    {(plan.steps ?? []).map((step, index, steps) => (
                      <PlannerRow
                        key={`${step.position}-${step.track.id}`}
                        step={step}
                        timelineStartMS={effectiveTimelineStartMS(steps, index)}
                        isStart={step.track.id === plan.startTrackId}
                        isDragging={dragTrackID === step.track.id}
                        isDropTarget={dragOverTrackID === step.track.id}
                        busy={manualBusy}
                        t={t}
                        onStart={() => useAsStart(step.track.id)}
                        onPin={() => togglePin(step.position, step.track.id)}
                        onLock={() => toggleLock(step.track.id)}
                        onMoveUp={() => moveManualStep(step.track.id, -1)}
                        onMoveDown={() => moveManualStep(step.track.id, 1)}
                        onTransitionNote={(value) => updateManualNote(step.track.id, 'transitionNote', value)}
                        onCueNote={(value) => updateManualNote(step.track.id, 'cueNote', value)}
                        onDragStart={(event) => {
                          if (step.locked || (event.target as HTMLElement).closest('input, textarea, button')) {
                            event.preventDefault()
                            return
                          }
                          event.dataTransfer.effectAllowed = 'move'
                          event.dataTransfer.setData('text/plain', String(step.track.id))
                          setDragTrackID(step.track.id)
                          setDragOverTrackID(0)
                        }}
                        onDragOver={(event) => {
                          if (step.locked || dragTrackID <= 0 || dragTrackID === step.track.id) return
                          event.preventDefault()
                          event.dataTransfer.dropEffect = 'move'
                          setDragOverTrackID(step.track.id)
                        }}
                        onDrop={(event) => {
                          event.preventDefault()
                          const source = dragTrackID || Number(event.dataTransfer.getData('text/plain'))
                          setDragTrackID(0)
                          setDragOverTrackID(0)
                          dropManualStep(source, step.track.id)
                        }}
                        onDragEnd={() => {
                          setDragTrackID(0)
                          setDragOverTrackID(0)
                        }}
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
  timelineStartMS,
  isStart,
  isDragging,
  isDropTarget,
  busy,
  t,
  onStart,
  onPin,
  onLock,
  onMoveUp,
  onMoveDown,
  onTransitionNote,
  onCueNote,
  onDragStart,
  onDragOver,
  onDrop,
  onDragEnd,
  onReveal,
}: {
  step: DJMixPlanStep
  timelineStartMS: number
  isStart: boolean
  isDragging: boolean
  isDropTarget: boolean
  busy: boolean
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
  onStart: () => void
  onPin: () => void
  onLock: () => void
  onMoveUp: () => void
  onMoveDown: () => void
  onTransitionNote: (value: string) => void
  onCueNote: (value: string) => void
  onDragStart: (event: DragEvent<HTMLTableRowElement>) => void
  onDragOver: (event: DragEvent<HTMLTableRowElement>) => void
  onDrop: (event: DragEvent<HTMLTableRowElement>) => void
  onDragEnd: () => void
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
  const flow = step.keyRelation === 'start'
    ? `${genreRelationLabel(step.genreRelation, t)} · E ${Math.round(step.energy * 100)}%`
    : `${genreRelationLabel(step.genreRelation, t)} · E ${formatSigned(step.energyDelta * 100)}%`
  const timelineEndMS = timelineStartMS + Math.max(0, step.track.durationMs || 0)
  const classes = [
    isStart ? 'is-start' : '',
    step.pinned ? 'is-pinned' : '',
    step.locked ? 'is-locked' : '',
    isDragging ? 'is-dragging' : '',
    isDropTarget ? 'is-drop-target' : '',
  ].filter(Boolean).join(' ')

  return (
    <tr
      className={classes}
      draggable={!step.locked && !busy}
      onDragStart={onDragStart}
      onDragOver={onDragOver}
      onDrop={onDrop}
      onDragEnd={onDragEnd}
      title={step.locked ? t('mixPlanner.lockedHint') : t('mixPlanner.dragHint')}
    >
      <td className="mix-position">{step.position}</td>
      <td className="mix-timeline" title={`${formatTimeline(timelineStartMS)} → ${formatTimeline(timelineEndMS)}`}>
        <strong>{formatTimeline(timelineStartMS)}</strong>
        <small>{formatDuration(step.track.durationMs)}</small>
      </td>
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
      <td className="mix-flow">{flow}</td>
      <td>{transition}</td>
      <td className="mix-notes">
        <input
          value={step.transitionNote ?? ''}
          maxLength={500}
          onChange={(event) => onTransitionNote(event.target.value)}
          placeholder={t('mixPlanner.transitionNote')}
          aria-label={t('mixPlanner.transitionNote')}
        />
        <input
          value={step.cueNote ?? ''}
          maxLength={500}
          onChange={(event) => onCueNote(event.target.value)}
          placeholder={t('mixPlanner.cueNote')}
          aria-label={t('mixPlanner.cueNote')}
        />
      </td>
      <td><b className="mix-score">{Math.round(step.score * 100)}%</b></td>
      <td className="mix-row-actions">
        <button type="button" onClick={onMoveUp} disabled={busy || step.locked} title={t('mixPlanner.moveUp')}>↑</button>
        <button type="button" onClick={onMoveDown} disabled={busy || step.locked} title={t('mixPlanner.moveDown')}>↓</button>
        <button type="button" className={step.locked ? 'is-active' : ''} onClick={onLock}>
          {step.locked ? t('mixPlanner.unlock') : t('mixPlanner.lock')}
        </button>
        {!isStart && <button type="button" onClick={onStart} disabled={busy}>{t('mixPlanner.startHere')}</button>}
        <button type="button" className={step.pinned ? 'is-active' : ''} onClick={onPin} disabled={busy}>
          {step.pinned ? t('mixPlanner.unpin') : t('mixPlanner.pin')}
        </button>
        <button type="button" onClick={onReveal}>{t('mixPlanner.reveal')}</button>
      </td>
    </tr>
  )
}

function genreRelationLabel(value: string, t: (key: TranslationKey) => string): string {
  const key = ({
    same: 'mixPlanner.genre.same',
    related: 'mixPlanner.genre.related',
    different: 'mixPlanner.genre.different',
    unknown: 'mixPlanner.genre.unknown',
    start: 'mixPlanner.genre.start',
  } as Record<string, TranslationKey>)[value] ?? 'mixPlanner.genre.unknown'
  return t(key)
}

function pinsFromSteps(steps: DJMixPlanStep[]): Record<number, number> {
  const result: Record<number, number> = {}
  for (const step of steps) {
    if (step.pinned) result[step.position] = step.track.id
  }
  return result
}

function reorderUnlockedSteps(
  steps: DJMixPlanStep[],
  sourceTrackID: number,
  targetTrackID: number,
): DJMixPlanStep[] | null {
  if (sourceTrackID === targetTrackID) return null
  const source = steps.find((step) => step.track.id === sourceTrackID)
  const target = steps.find((step) => step.track.id === targetTrackID)
  if (!source || !target || source.locked || target.locked) return null

  const unlocked = steps.filter((step) => !step.locked)
  const sourceIndex = unlocked.findIndex((step) => step.track.id === sourceTrackID)
  if (sourceIndex < 0) return null
  const [moved] = unlocked.splice(sourceIndex, 1)
  const targetIndex = unlocked.findIndex((step) => step.track.id === targetTrackID)
  if (targetIndex < 0) return null
  unlocked.splice(targetIndex, 0, moved)
  return mergeUnlockedIntoLockedSlots(steps, unlocked)
}

function moveUnlockedStep(
  steps: DJMixPlanStep[],
  trackID: number,
  direction: -1 | 1,
): DJMixPlanStep[] | null {
  const unlocked = steps.filter((step) => !step.locked)
  const sourceIndex = unlocked.findIndex((step) => step.track.id === trackID)
  const targetIndex = sourceIndex + direction
  if (sourceIndex < 0 || targetIndex < 0 || targetIndex >= unlocked.length) return null
  const temp = unlocked[sourceIndex]
  unlocked[sourceIndex] = unlocked[targetIndex]
  unlocked[targetIndex] = temp
  return mergeUnlockedIntoLockedSlots(steps, unlocked)
}

function mergeUnlockedIntoLockedSlots(original: DJMixPlanStep[], unlocked: DJMixPlanStep[]): DJMixPlanStep[] {
  let index = 0
  return original.map((step) => step.locked ? step : unlocked[index++])
}

function effectiveTimelineStartMS(steps: DJMixPlanStep[], index: number): number {
  const stored = steps[index]?.timelineStartMs
  if (Number.isFinite(stored) && (stored > 0 || index === 0)) return stored
  let total = 0
  for (let i = 0; i < index; i += 1) {
    total += Math.max(0, steps[i]?.track.durationMs || 0)
  }
  return total
}

function formatTimeline(valueMS: number): string {
  const total = Math.max(0, Math.floor((Number.isFinite(valueMS) ? valueMS : 0) / 1000))
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  const seconds = total % 60
  return hours > 0
    ? `${hours}:${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
    : `${minutes}:${String(seconds).padStart(2, '0')}`
}

function formatDuration(valueMS: number): string {
  return formatTimeline(Math.max(0, valueMS || 0))
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
    genre_jump: 'mixPlanner.warning.genre_jump',
    energy_jump: 'mixPlanner.warning.energy_jump',
    pinned_transition: 'mixPlanner.warning.pinned_transition',
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
