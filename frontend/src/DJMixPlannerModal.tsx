import {useEffect, useMemo, useRef, useState, type DragEvent} from 'react'
import {translate, type AppLanguage, type TranslationKey} from './i18n'
import type {DJMixPin, DJMixPlan, DJMixPlanOptions, DJMixPlanStep, DJMixSavedPlan, TrackMedia, TrackWaveform} from './types'
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
  const [previewTrackID, setPreviewTrackID] = useState(0)
  const [previewMedia, setPreviewMedia] = useState<TrackMedia | null>(null)
  const [previewLoading, setPreviewLoading] = useState(false)
  const [previewFallbackLoading, setPreviewFallbackLoading] = useState(false)
  const [previewError, setPreviewError] = useState('')
  const [previewPlaying, setPreviewPlaying] = useState(false)
  const [previewCurrentTime, setPreviewCurrentTime] = useState(0)
  const [previewDuration, setPreviewDuration] = useState(0)
  const [previewVolume, setPreviewVolume] = useState(1)
  const [previewWaveform, setPreviewWaveform] = useState<TrackWaveform | null>(null)
  const [previewWaveformLoading, setPreviewWaveformLoading] = useState(false)
  const [previewWaveformError, setPreviewWaveformError] = useState('')
  const previewAudioRef = useRef<HTMLAudioElement | null>(null)
  const previewRequestRef = useRef(0)
  const previewWaveformRequestRef = useRef(0)
  const previewFallbackTriedRef = useRef(false)
  const previewAutoplayRef = useRef(0)

  const t = (key: TranslationKey, params?: Record<string, string | number>) => translate(language, key, params)
  const scopeIDs = useMemo(() => selectedIDs.length >= 2 ? selectedIDs : [], [selectedIDs])
  const previewStep = useMemo(
    () => plan?.steps?.find((step) => step.track.id === previewTrackID) ?? null,
    [plan, previewTrackID],
  )
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
    setPreviewTrackID(0)
    setPreviewMedia(null)
    setPreviewError('')
    setPreviewPlaying(false)
    setPreviewCurrentTime(0)
    setPreviewDuration(0)
    setPreviewWaveform(null)
    setPreviewWaveformLoading(false)
    setPreviewWaveformError('')
    previewAutoplayRef.current = 0
    void refreshSavedPlans()
    void build(seed, {}, scopeIDs)
  }, [open])

  useEffect(() => {
    if (!open) return
    const onKeyDown = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null
      const editing = Boolean(target?.closest('input, textarea, select, button, [contenteditable="true"]'))

      if (event.key === 'Escape') {
        event.preventDefault()
        onClose()
        return
      }

      if (
        event.key === ' ' &&
        !editing &&
        previewTrackID > 0 &&
        previewMedia?.audioUrl &&
        !previewLoading &&
        !previewFallbackLoading
      ) {
        event.preventDefault()
        void togglePreviewPlayback()
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [
    open,
    onClose,
    previewFallbackLoading,
    previewLoading,
    previewMedia?.audioUrl,
    previewTrackID,
  ])

  useEffect(() => {
    if (!open || previewTrackID <= 0) return
    const app = window.go?.main?.App
    if (!app) return

    const request = ++previewRequestRef.current
    const audio = previewAudioRef.current
    if (audio) {
      audio.pause()
      audio.currentTime = 0
    }

    previewFallbackTriedRef.current = false
    setPreviewMedia(null)
    setPreviewLoading(true)
    setPreviewFallbackLoading(false)
    setPreviewError('')
    setPreviewPlaying(false)
    setPreviewCurrentTime(0)
    setPreviewDuration(0)

    void app.PrepareTrackMedia(previewTrackID)
      .then((result) => {
        if (previewRequestRef.current === request) setPreviewMedia(result)
      })
      .catch((err) => {
        if (previewRequestRef.current === request) {
          setPreviewError(err instanceof Error ? err.message : String(err))
        }
      })
      .finally(() => {
        if (previewRequestRef.current === request) setPreviewLoading(false)
      })
  }, [open, previewTrackID])

  useEffect(() => {
    if (!open || previewTrackID <= 0) return
    const app = window.go?.main?.App
    if (!app) return

    const request = ++previewWaveformRequestRef.current
    setPreviewWaveform(null)
    setPreviewWaveformLoading(true)
    setPreviewWaveformError('')

    void app.PrepareTrackWaveform(previewTrackID, 900)
      .then((result) => {
        if (previewWaveformRequestRef.current === request) setPreviewWaveform(result)
      })
      .catch((err) => {
        if (previewWaveformRequestRef.current === request) {
          setPreviewWaveformError(err instanceof Error ? err.message : String(err))
        }
      })
      .finally(() => {
        if (previewWaveformRequestRef.current === request) setPreviewWaveformLoading(false)
      })
  }, [open, previewTrackID])

  useEffect(() => {
    setPreviewPlaying(false)
    setPreviewCurrentTime(0)
    setPreviewDuration(previewMedia?.durationMs ? previewMedia.durationMs / 1000 : 0)

    if (!previewMedia?.audioUrl || previewAutoplayRef.current !== previewTrackID) return
    const timer = window.setTimeout(() => {
      const audio = previewAudioRef.current
      if (!audio || previewAutoplayRef.current !== previewTrackID) return
      previewAutoplayRef.current = 0
      void audio.play().catch(() => void handlePreviewPlaybackError())
    }, 0)
    return () => window.clearTimeout(timer)
  }, [previewMedia?.audioUrl, previewMedia?.durationMs, previewTrackID])

  useEffect(() => {
    if (previewAudioRef.current) previewAudioRef.current.volume = previewVolume
  }, [previewVolume, previewMedia?.audioUrl])

  useEffect(() => {
    if (open) return
    previewAutoplayRef.current = 0
    if (previewAudioRef.current) {
      previewAudioRef.current.pause()
      previewAudioRef.current.currentTime = 0
    }
  }, [open])

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
      setPreviewTrackID((current) => (
        current > 0 && (result.steps ?? []).some((step) => step.track.id === current)
          ? current
          : (result.steps?.[0]?.track.id ?? 0)
      ))
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
      setPreviewTrackID(saved.plan.steps?.[0]?.track.id ?? 0)
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

  function selectPreviewTrack(trackID: number, autoplay = false) {
    if (trackID <= 0) return
    if (autoplay) previewAutoplayRef.current = trackID

    if (previewTrackID === trackID) {
      if (autoplay && previewMedia?.audioUrl && !previewLoading && !previewFallbackLoading) {
        previewAutoplayRef.current = 0
        void togglePreviewPlayback()
      }
      return
    }

    setPreviewTrackID(trackID)
  }

  async function togglePreviewPlayback() {
    const audio = previewAudioRef.current
    if (!audio || previewLoading || previewFallbackLoading) return
    if (!audio.paused) {
      audio.pause()
      return
    }

    setPreviewError('')
    try {
      await audio.play()
    } catch {
      await handlePreviewPlaybackError()
    }
  }

  function stopPreviewPlayback() {
    const audio = previewAudioRef.current
    if (!audio) return
    audio.pause()
    audio.currentTime = 0
    setPreviewCurrentTime(0)
    setPreviewPlaying(false)
  }

  function updatePreviewMetadata() {
    const audio = previewAudioRef.current
    if (!audio) return
    const duration = Number.isFinite(audio.duration) && audio.duration > 0
      ? audio.duration
      : (previewMedia?.durationMs || 0) / 1000
    setPreviewDuration(duration)
    setPreviewCurrentTime(Number.isFinite(audio.currentTime) ? audio.currentTime : 0)
    audio.volume = previewVolume
  }

  function seekPreview(value: number) {
    const audio = previewAudioRef.current
    if (!audio || !Number.isFinite(value)) return
    const nativeDuration = Number.isFinite(audio.duration) ? audio.duration : 0
    const duration = previewDuration > 0 ? previewDuration : Math.max(0, nativeDuration)
    const next = Math.max(0, Math.min(value, duration || value))
    audio.currentTime = next
    setPreviewCurrentTime(next)
  }

  function skipPreview(deltaSeconds: number) {
    const audio = previewAudioRef.current
    if (!audio) return
    seekPreview((Number.isFinite(audio.currentTime) ? audio.currentTime : previewCurrentTime) + deltaSeconds)
  }

  function changePreviewVolume(value: number) {
    const next = Math.max(0, Math.min(1, value))
    setPreviewVolume(next)
    if (previewAudioRef.current) previewAudioRef.current.volume = next
  }

  async function handlePreviewPlaybackError() {
    setPreviewPlaying(false)
    const app = window.go?.main?.App
    if (!app || previewTrackID <= 0 || !previewMedia || previewFallbackLoading) return

    if (previewMedia.isPreview || previewFallbackTriedRef.current) {
      setPreviewError(t('media.playbackFailed'))
      return
    }

    previewFallbackTriedRef.current = true
    previewAutoplayRef.current = previewTrackID
    const request = previewRequestRef.current
    setPreviewFallbackLoading(true)
    setPreviewError('')

    try {
      const url = await app.PrepareTrackAudioPreview(previewTrackID)
      if (previewRequestRef.current !== request) return
      setPreviewMedia((current) => current ? {...current, audioUrl: url, isPreview: true} : current)
    } catch (err) {
      previewAutoplayRef.current = 0
      if (previewRequestRef.current === request) {
        setPreviewError(err instanceof Error ? err.message : String(err))
      }
    } finally {
      if (previewRequestRef.current === request) setPreviewFallbackLoading(false)
    }
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
            {previewStep && (
              <div className="mix-planner-preview">
                <div className="mix-planner-preview-track">
                  <div className="mix-planner-preview-cover">
                    {previewMedia?.coverUrl
                      ? <img src={previewMedia.coverUrl} alt="" />
                      : <span>в™Є</span>}
                  </div>
                  <div className="mix-planner-preview-title">
                    <strong>{previewStep.track.artist || 'вЂ”'} вЂ” {previewStep.track.title || previewStep.track.fileName}</strong>
                    <span>{previewStep.track.genre || previewStep.track.album || previewStep.track.path}</span>
                    <div className="mix-planner-preview-meta">
                      <em>{formatBPM(previewStep.adjustedBpm || previewStep.track.bpm)} BPM</em>
                      <em>{previewStep.camelot || previewStep.openKey || 'вЂ”'}</em>
                      <em>{formatDuration(previewStep.track.durationMs)}</em>
                      {previewMedia?.isPreview && <em className="is-preview">{t('media.preview')}</em>}
                    </div>
                  </div>
                </div>

                <div className="mix-planner-preview-player">
                  <audio
                    ref={previewAudioRef}
                    key={previewMedia?.audioUrl || `preview-${previewTrackID}`}
                    preload="metadata"
                    src={previewMedia?.audioUrl || undefined}
                    onLoadedMetadata={updatePreviewMetadata}
                    onDurationChange={updatePreviewMetadata}
                    onTimeUpdate={() => setPreviewCurrentTime(previewAudioRef.current?.currentTime || 0)}
                    onPlay={() => setPreviewPlaying(true)}
                    onPause={() => setPreviewPlaying(false)}
                    onEnded={() => setPreviewPlaying(false)}
                    onError={() => void handlePreviewPlaybackError()}
                  />

                  {previewLoading || previewFallbackLoading
                    ? <span className="mix-planner-preview-status">{previewFallbackLoading ? t('media.compatibilityPreview') : t('media.preparing')}</span>
                    : previewError
                      ? <span className="mix-planner-preview-status is-error" title={previewError}>{t('media.unavailable')}: {previewError}</span>
                      : <span className="mix-planner-preview-status">{previewPlaying ? t('media.pause') : t('media.play')} В· Space</span>}

                  <WaveformOverview
                    waveform={previewWaveform}
                    loading={previewWaveformLoading}
                    error={previewWaveformError}
                    currentTime={previewCurrentTime}
                    duration={previewDuration || ((previewWaveform?.durationMs || 0) / 1000)}
                    loadingLabel={t('media.preparing')}
                    unavailableLabel={t('media.unavailable')}
                    onSeek={seekPreview}
                  />

                  <div className="mix-planner-preview-controls">
                    <button
                      type="button"
                      className="mix-preview-toggle"
                      onClick={() => void togglePreviewPlayback()}
                      disabled={!previewMedia?.audioUrl || previewLoading || previewFallbackLoading}
                      title={previewPlaying ? t('media.pause') : t('media.play')}
                      aria-label={previewPlaying ? t('media.pause') : t('media.play')}
                    >
                      {previewPlaying ? 'в…Ў' : 'в–¶'}
                    </button>
                    <button type="button" onClick={stopPreviewPlayback} disabled={!previewMedia?.audioUrl} title="Stop" aria-label="Stop">в– </button>
                    <button type="button" onClick={() => skipPreview(-10)} disabled={!previewMedia?.audioUrl} title="в€’10 s">в€’10</button>
                    <span className="mix-planner-preview-time">{formatPlayerTime(previewCurrentTime)}</span>
                    <input
                      className="mix-planner-preview-seek"
                      type="range"
                      min="0"
                      max={Math.max(previewDuration, 0.1)}
                      step="0.1"
                      value={Math.min(previewCurrentTime, previewDuration || previewCurrentTime)}
                      onChange={(event) => seekPreview(Number(event.target.value))}
                      aria-label={t('media.seek')}
                      disabled={previewDuration <= 0}
                    />
                    <span className="mix-planner-preview-time">{formatPlayerTime(previewDuration)}</span>
                    <button type="button" onClick={() => skipPreview(10)} disabled={!previewMedia?.audioUrl} title="+10 s">+10</button>
                    <input
                      className="mix-planner-preview-volume"
                      type="range"
                      min="0"
                      max="1"
                      step="0.05"
                      value={previewVolume}
                      onChange={(event) => changePreviewVolume(Number(event.target.value))}
                      aria-label={t('media.volume')}
                      title={t('media.volume')}
                    />
                  </div>
                </div>
              </div>
            )}

            <DJMixTimeline
              steps={plan.steps ?? []}
              selectedTrackID={previewTrackID}
              startTrackID={plan.startTrackId}
              t={t}
              onSelectTrack={(trackID) => selectPreviewTrack(trackID)}
              onPlayTrack={(trackID) => selectPreviewTrack(trackID, true)}
            />

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
                        isPreviewSelected={step.track.id === previewTrackID}
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
                        onSelectPreview={() => selectPreviewTrack(step.track.id)}
                        onPreviewPlay={() => selectPreviewTrack(step.track.id, true)}
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
  isPreviewSelected,
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
  onSelectPreview,
  onPreviewPlay,
  onDragStart,
  onDragOver,
  onDrop,
  onDragEnd,
  onReveal,
}: {
  step: DJMixPlanStep
  timelineStartMS: number
  isStart: boolean
  isPreviewSelected: boolean
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
  onSelectPreview: () => void
  onPreviewPlay: () => void
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
    isPreviewSelected ? 'is-preview-selected' : '',
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
      onClick={(event) => {
        if ((event.target as HTMLElement).closest('input, textarea, select, button')) return
        onSelectPreview()
      }}
      onDoubleClick={(event) => {
        if ((event.target as HTMLElement).closest('input, textarea, select, button')) return
        onPreviewPlay()
      }}
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

function formatPlayerTime(valueSeconds: number): string {
  const total = Math.max(0, Math.floor(Number.isFinite(valueSeconds) ? valueSeconds : 0))
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  const seconds = total % 60
  return hours > 0
    ? `${hours}:${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
    : `${minutes}:${String(seconds).padStart(2, '0')}`
}

function DJMixTimeline({
  steps,
  selectedTrackID,
  startTrackID,
  t,
  onSelectTrack,
  onPlayTrack,
}: {
  steps: DJMixPlanStep[]
  selectedTrackID: number
  startTrackID: number
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
  onSelectTrack: (trackID: number) => void
  onPlayTrack: (trackID: number) => void
}) {
  if (steps.length === 0) return null

  const lastIndex = steps.length - 1
  const totalMS = effectiveTimelineStartMS(steps, lastIndex) + Math.max(0, steps[lastIndex]?.track.durationMs || 0)

  return (
    <div className="mix-planner-flow">
      <div className="mix-planner-flow-head">
        <strong>{t('mixPlanner.timeline')}</strong>
        <span>{steps.length} tracks В· {formatTimeline(totalMS)}</span>
      </div>
      <div className="mix-planner-flow-scroll">
        <div className="mix-planner-flow-track">
          {steps.map((step, index) => {
            const warning = (step.warnings ?? []).length > 0
            const laneClass = index % 2 === 0 ? 'is-lane-a' : 'is-lane-b'
            const classes = [
              'mix-planner-flow-segment',
              laneClass,
              warning ? 'has-warning' : '',
            ].filter(Boolean).join(' ')
            const startMS = effectiveTimelineStartMS(steps, index)
            const energyPct = Math.max(0, Math.min(100, Math.round((step.energy || 0) * 100)))
            const relation = step.keyRelation === 'start' ? '' : relationLabel(step.keyRelation, t)

            return (
              <div className={classes} key={`flow-${step.position}-${step.track.id}`}>
                {index > 0 && (
                  <div
                    className="mix-planner-flow-transition"
                    title={`${t('mixPlanner.transition')}: ${formatSigned(step.tempoDeltaPct)}% В· ${relation}`}
                  >
                    <strong>{Math.round(step.score * 100)}%</strong>
                    <span>{formatSigned(step.tempoDeltaPct)}% В· {step.camelot || relation || 'вЂ”'}</span>
                  </div>
                )}
                <button
                  type="button"
                  className={[
                    'mix-planner-flow-card',
                    step.track.id === selectedTrackID ? 'is-selected' : '',
                    step.track.id === startTrackID ? 'is-start' : '',
                  ].filter(Boolean).join(' ')}
                  style={{width: `${mixTimelineTrackWidth(step.track.durationMs)}px`}}
                  onClick={() => onSelectTrack(step.track.id)}
                  onDoubleClick={() => onPlayTrack(step.track.id)}
                  aria-pressed={step.track.id === selectedTrackID}
                  title={`${step.track.artist || 'вЂ”'} вЂ” ${step.track.title || step.track.fileName}`}
                >
                  <span className="mix-planner-flow-card-top">
                    <b className="mix-planner-flow-position">#{step.position}</b>
                    <span className="mix-planner-flow-time">{formatTimeline(startMS)}</span>
                  </span>
                  <span className="mix-planner-flow-title">
                    {step.track.artist || 'вЂ”'} вЂ” {step.track.title || step.track.fileName}
                  </span>
                  <span className="mix-planner-flow-card-meta">
                    <b>{formatBPM(step.adjustedBpm || step.track.bpm)} BPM</b>
                    <span>{step.camelot || step.openKey || 'вЂ”'}</span>
                    <span>{formatDuration(step.track.durationMs)}</span>
                  </span>
                  <span className="mix-planner-flow-energy" title={`Energy ${energyPct}%`}>
                    <span style={{width: `${energyPct}%`}} />
                  </span>
                </button>
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}

function mixTimelineTrackWidth(durationMS: number): number {
  if (!Number.isFinite(durationMS) || durationMS <= 0) return 190
  const minutes = durationMS / 60000
  return Math.round(Math.max(170, Math.min(280, 158 + minutes * 18)))
}

function WaveformOverview({
  waveform,
  loading,
  error,
  currentTime,
  duration,
  loadingLabel,
  unavailableLabel,
  onSeek,
}: {
  waveform: TrackWaveform | null
  loading: boolean
  error: string
  currentTime: number
  duration: number
  loadingLabel: string
  unavailableLabel: string
  onSeek: (value: number) => void
}) {
  const [hoverRatio, setHoverRatio] = useState<number | null>(null)
  const peaks = waveform?.peaks ?? []

  if (loading) {
    return <div className="mix-planner-waveform is-loading" title={loadingLabel} aria-label={loadingLabel} />
  }
  if (error || peaks.length === 0) {
    return (
      <div
        className="mix-planner-waveform is-unavailable"
        title={error || unavailableLabel}
        aria-label={unavailableLabel}
      >
        <span>{unavailableLabel}</span>
      </div>
    )
  }

  const safeDuration = Math.max(0, duration || ((waveform?.durationMs || 0) / 1000))
  const progress = safeDuration > 0 ? Math.max(0, Math.min(1, currentTime / safeDuration)) : 0
  const playedInset = Math.max(0, Math.min(100, 100 - progress * 100))
  const path = buildWaveformPath(peaks)

  function ratioFromClientX(clientX: number, element: HTMLDivElement): number {
    const rect = element.getBoundingClientRect()
    if (rect.width <= 0) return 0
    return Math.max(0, Math.min(1, (clientX - rect.left) / rect.width))
  }

  return (
    <div
      className="mix-planner-waveform"
      role="slider"
      tabIndex={0}
      aria-label="Waveform seek"
      aria-valuemin={0}
      aria-valuemax={Math.max(0, safeDuration)}
      aria-valuenow={Math.max(0, Math.min(currentTime, safeDuration || currentTime))}
      onPointerMove={(event) => {
        const ratio = ratioFromClientX(event.clientX, event.currentTarget)
        setHoverRatio(ratio)
        if (safeDuration > 0 && (event.buttons & 1) === 1) {
          onSeek(ratio * safeDuration)
        }
      }}
      onPointerLeave={() => setHoverRatio(null)}
      onPointerDown={(event) => {
        if (safeDuration <= 0) return
        event.currentTarget.setPointerCapture?.(event.pointerId)
        const ratio = ratioFromClientX(event.clientX, event.currentTarget)
        setHoverRatio(ratio)
        onSeek(ratio * safeDuration)
      }}
      onKeyDown={(event) => {
        if (safeDuration <= 0) return
        if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') {
          event.preventDefault()
          onSeek(currentTime + (event.key === 'ArrowLeft' ? -5 : 5))
        }
      }}
    >
      <svg viewBox="0 0 1000 100" preserveAspectRatio="none" aria-hidden="true">
        <path className="mix-waveform-base" d={path} />
      </svg>
      <svg
        viewBox="0 0 1000 100"
        preserveAspectRatio="none"
        aria-hidden="true"
        style={{clipPath: `inset(0 ${playedInset}% 0 0)`}}
      >
        <path className="mix-waveform-played" d={path} />
      </svg>
      <span className="mix-planner-waveform-playhead" style={{left: `${progress * 100}%`}} />
      {hoverRatio !== null && safeDuration > 0 && (
        <span className="mix-planner-waveform-hover" style={{left: `${hoverRatio * 100}%`}}>
          {formatPlayerTime(hoverRatio * safeDuration)}
        </span>
      )}
      <span className="mix-planner-waveform-hint">click / drag seek В· в†ђ в†’ 5s</span>
    </div>
  )
}

function buildWaveformPath(peaks: number[]): string {
  if (peaks.length === 0) return ''
  const width = 1000
  const center = 50
  const amplitude = 46
  const top: string[] = []
  const bottom: string[] = []

  for (let index = 0; index < peaks.length; index += 1) {
    const ratio = peaks.length === 1 ? 0 : index / (peaks.length - 1)
    const x = ratio * width
    const raw = Number.isFinite(peaks[index]) ? Math.max(0, Math.min(1, peaks[index])) : 0
    const shaped = Math.pow(raw, 0.62)
    const delta = shaped * amplitude
    top.push(`${x.toFixed(2)},${(center - delta).toFixed(2)}`)
    bottom.push(`${x.toFixed(2)},${(center + delta).toFixed(2)}`)
  }

  return `M ${top.join(' L ')} L ${bottom.reverse().join(' L ')} Z`
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
