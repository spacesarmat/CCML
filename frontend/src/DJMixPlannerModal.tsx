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
  const [previewAutoAdvance, setPreviewAutoAdvance] = useState(false)
  const [transitionPreviewIndex, setTransitionPreviewIndex] = useState(-1)
  const [transitionPreviewLoading, setTransitionPreviewLoading] = useState(false)
  const [transitionPreviewPlaying, setTransitionPreviewPlaying] = useState(false)
  const [transitionPreviewProgress, setTransitionPreviewProgress] = useState(0)
  const [transitionPreviewError, setTransitionPreviewError] = useState('')
  const [previewWaveform, setPreviewWaveform] = useState<TrackWaveform | null>(null)
  const [previewWaveformLoading, setPreviewWaveformLoading] = useState(false)
  const [previewWaveformError, setPreviewWaveformError] = useState('')
  const previewAudioRef = useRef<HTMLAudioElement | null>(null)
  const previewRequestRef = useRef(0)
  const previewWaveformRequestRef = useRef(0)
  const previewFallbackTriedRef = useRef(false)
  const previewAutoplayRef = useRef(0)
  const transitionOutAudioRef = useRef<HTMLAudioElement | null>(null)
  const transitionInAudioRef = useRef<HTMLAudioElement | null>(null)
  const transitionRequestRef = useRef(0)
  const transitionAnimationRef = useRef<number | null>(null)

  const t = (key: TranslationKey, params?: Record<string, string | number>) => translate(language, key, params)
  const scopeIDs = useMemo(() => selectedIDs.length >= 2 ? selectedIDs : [], [selectedIDs])
  const previewStep = useMemo(
    () => plan?.steps?.find((step) => step.track.id === previewTrackID) ?? null,
    [plan, previewTrackID],
  )
  const previewStepIndex = useMemo(
    () => plan?.steps?.findIndex((step) => step.track.id === previewTrackID) ?? -1,
    [plan, previewTrackID],
  )
  const previewPreviousStep = previewStepIndex > 0 ? plan?.steps?.[previewStepIndex - 1] ?? null : null
  const previewNextStep = previewStepIndex >= 0 && previewStepIndex < (plan?.steps?.length ?? 0) - 1
    ? plan?.steps?.[previewStepIndex + 1] ?? null
    : null
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
    stopTransitionPreview()
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
        return
      }

      if (!editing && (event.key === 'ArrowUp' || event.key === 'ArrowDown')) {
        const steps = plan?.steps ?? []
        if (steps.length === 0) return
        const currentIndex = steps.findIndex((step) => step.track.id === previewTrackID)
        const fallbackIndex = currentIndex >= 0 ? currentIndex : 0
        const delta = event.key === 'ArrowUp' ? -1 : 1
        const nextIndex = Math.max(0, Math.min(steps.length - 1, fallbackIndex + delta))
        const nextTrackID = steps[nextIndex]?.track.id ?? 0
        if (nextTrackID > 0 && nextTrackID !== previewTrackID) {
          event.preventDefault()
          setPreviewTrackID(nextTrackID)
        }
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
    plan,
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
    if (transitionPreviewIndex >= 0) stopTransitionPreview()
  }, [previewTrackID])

  useEffect(() => {
    if (open) return
    stopTransitionPreview()
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

  function selectRelativePreview(direction: -1 | 1) {
    const steps = plan?.steps ?? []
    if (steps.length === 0) return

    const currentIndex = steps.findIndex((step) => step.track.id === previewTrackID)
    const fallbackIndex = currentIndex >= 0 ? currentIndex : (direction > 0 ? -1 : steps.length)
    const nextIndex = Math.max(0, Math.min(steps.length - 1, fallbackIndex + direction))
    const nextTrackID = steps[nextIndex]?.track.id ?? 0
    if (nextTrackID > 0 && nextTrackID !== previewTrackID) {
      setPreviewTrackID(nextTrackID)
    }
  }

  function stopTransitionPreview() {
    transitionRequestRef.current += 1

    if (transitionAnimationRef.current !== null) {
      window.cancelAnimationFrame(transitionAnimationRef.current)
      transitionAnimationRef.current = null
    }

    for (const audio of [transitionOutAudioRef.current, transitionInAudioRef.current]) {
      if (!audio) continue
      audio.pause()
      audio.currentTime = 0
      audio.volume = previewVolume
      audio.playbackRate = 1
    }

    setTransitionPreviewIndex(-1)
    setTransitionPreviewLoading(false)
    setTransitionPreviewPlaying(false)
    setTransitionPreviewProgress(0)
    setTransitionPreviewError('')
  }

  function finishTransitionPreview(request: number, error = '') {
    if (transitionRequestRef.current !== request) return

    if (transitionAnimationRef.current !== null) {
      window.cancelAnimationFrame(transitionAnimationRef.current)
      transitionAnimationRef.current = null
    }

    for (const audio of [transitionOutAudioRef.current, transitionInAudioRef.current]) {
      if (!audio) continue
      audio.pause()
      audio.volume = previewVolume
      audio.playbackRate = 1
    }

    setTransitionPreviewLoading(false)
    setTransitionPreviewPlaying(false)
    setTransitionPreviewProgress(error ? 0 : 1)
    setTransitionPreviewError(error)
  }

  async function auditionTransition(index: number) {
    const steps = plan?.steps ?? []
    if (index <= 0 || index >= steps.length) return

    if (
      transitionPreviewIndex === index &&
      (transitionPreviewLoading || transitionPreviewPlaying)
    ) {
      stopTransitionPreview()
      return
    }

    const app = window.go?.main?.App
    const outgoingAudio = transitionOutAudioRef.current
    const incomingAudio = transitionInAudioRef.current
    if (!app || !outgoingAudio || !incomingAudio) return

    stopTransitionPreview()
    const request = ++transitionRequestRef.current
    const outgoingStep = steps[index - 1]
    const incomingStep = steps[index]

    previewAudioRef.current?.pause()
    setPreviewPlaying(false)
    setTransitionPreviewIndex(index)
    setTransitionPreviewLoading(true)
    setTransitionPreviewPlaying(false)
    setTransitionPreviewProgress(0)
    setTransitionPreviewError('')

    try {
      const [outgoingMedia, incomingMedia] = await Promise.all([
        app.PrepareTrackMedia(outgoingStep.track.id),
        app.PrepareTrackMedia(incomingStep.track.id),
      ])
      if (transitionRequestRef.current !== request) return

      const loadWithFallback = async (
        audio: HTMLAudioElement,
        trackID: number,
        initialURL: string,
      ) => {
        try {
          await loadTransitionPreviewAudio(audio, initialURL)
        } catch {
          const fallbackURL = await app.PrepareTrackAudioPreview(trackID)
          if (transitionRequestRef.current !== request) return
          await loadTransitionPreviewAudio(audio, fallbackURL)
        }
      }

      await Promise.all([
        loadWithFallback(outgoingAudio, outgoingStep.track.id, outgoingMedia.audioUrl),
        loadWithFallback(incomingAudio, incomingStep.track.id, incomingMedia.audioUrl),
      ])
      if (transitionRequestRef.current !== request) return

      const metadataDuration = Number.isFinite(outgoingAudio.duration) ? outgoingAudio.duration : 0
      const fallbackDuration = Math.max(0, (outgoingStep.track.durationMs || 0) / 1000)
      const outgoingDuration = metadataDuration > 0 ? metadataDuration : fallbackDuration
      if (outgoingDuration <= 0) {
        throw new Error('Outgoing track duration is unavailable')
      }

      const outgoingWindow = Math.max(2, Math.min(12, outgoingDuration))
      const crossfadeSeconds = Math.max(1, Math.min(5, outgoingWindow * 0.4))
      const leadSeconds = Math.max(0, outgoingWindow - crossfadeSeconds)
      const incomingTailSeconds = 6
      const totalSeconds = outgoingWindow + incomingTailSeconds
      const tempoRate = mixTransitionPlaybackRate(outgoingStep, incomingStep)

      outgoingAudio.currentTime = Math.max(0, outgoingDuration - outgoingWindow)
      outgoingAudio.volume = previewVolume
      outgoingAudio.playbackRate = 1
      incomingAudio.currentTime = 0
      incomingAudio.volume = 0
      incomingAudio.playbackRate = tempoRate

      await outgoingAudio.play()
      if (transitionRequestRef.current !== request) {
        outgoingAudio.pause()
        return
      }

      setTransitionPreviewLoading(false)
      setTransitionPreviewPlaying(true)

      const startedAt = performance.now()
      let incomingStarted = false
      let outgoingStopped = false

      const tick = (now: number) => {
        if (transitionRequestRef.current !== request) return

        const elapsed = Math.max(0, (now - startedAt) / 1000)

        if (!incomingStarted && elapsed >= leadSeconds) {
          incomingStarted = true
          incomingAudio.currentTime = 0
          void incomingAudio.play().catch((err) => {
            finishTransitionPreview(request, err instanceof Error ? err.message : String(err))
          })
        }

        const mixRatio = incomingStarted
          ? Math.max(0, Math.min(1, (elapsed - leadSeconds) / crossfadeSeconds))
          : 0

        outgoingAudio.volume = Math.max(0, Math.min(1, previewVolume * (1 - mixRatio)))
        incomingAudio.volume = Math.max(0, Math.min(1, previewVolume * mixRatio))

        if (!outgoingStopped && elapsed >= outgoingWindow) {
          outgoingStopped = true
          outgoingAudio.pause()
          outgoingAudio.volume = 0
          incomingAudio.volume = previewVolume
        }

        setTransitionPreviewProgress(Math.max(0, Math.min(1, elapsed / totalSeconds)))

        if (elapsed >= totalSeconds || (incomingStarted && incomingAudio.ended)) {
          finishTransitionPreview(request)
          return
        }

        transitionAnimationRef.current = window.requestAnimationFrame(tick)
      }

      transitionAnimationRef.current = window.requestAnimationFrame(tick)
    } catch (err) {
      finishTransitionPreview(request, err instanceof Error ? err.message : String(err))
    }
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
    stopTransitionPreview()
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

  function handlePreviewEnded() {
    setPreviewPlaying(false)
    if (previewDuration > 0) setPreviewCurrentTime(previewDuration)

    const nextTrackID = previewNextStep?.track.id ?? 0
    if (!previewAutoAdvance || nextTrackID <= 0) return

    previewAutoplayRef.current = nextTrackID
    setPreviewTrackID(nextTrackID)
  }

  function stopPreviewPlayback() {
    stopTransitionPreview()
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
                      : <span>♪</span>}
                  </div>
                  <div className="mix-planner-preview-title">
                    <strong>{previewStep.track.artist || '—'} — {previewStep.track.title || previewStep.track.fileName}</strong>
                    <span>{previewStep.track.genre || previewStep.track.album || previewStep.track.path}</span>
                    <div className="mix-planner-preview-meta">
                      <em>{formatBPM(previewStep.adjustedBpm || previewStep.track.bpm)} BPM</em>
                      <em>{previewStep.camelot || previewStep.openKey || '—'}</em>
                      <em>{formatDuration(previewStep.track.durationMs)}</em>
                      {previewMedia?.isPreview && <em className="is-preview">{t('media.preview')}</em>}
                    </div>
                  </div>
                </div>

                <div className="mix-planner-preview-player">
                  <audio
                    className="mix-planner-transition-audio"
                    ref={transitionOutAudioRef}
                    preload="metadata"
                  />
                  <audio
                    className="mix-planner-transition-audio"
                    ref={transitionInAudioRef}
                    preload="metadata"
                  />
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
                    onEnded={handlePreviewEnded}
                    onError={() => void handlePreviewPlaybackError()}
                  />

                  <div className="mix-planner-preview-status-row">
                    {previewLoading || previewFallbackLoading
                      ? <span className="mix-planner-preview-status">{previewFallbackLoading ? t('media.compatibilityPreview') : t('media.preparing')}</span>
                      : previewError
                        ? <span className="mix-planner-preview-status is-error" title={previewError}>{t('media.unavailable')}: {previewError}</span>
                        : <span className="mix-planner-preview-status">{previewPlaying ? t('media.pause') : t('media.play')} · Space · ↑↓ track</span>}
                    <span className="mix-planner-preview-position">
                      {previewStepIndex >= 0 ? `${previewStepIndex + 1} / ${plan.steps?.length ?? 0}` : `0 / ${plan.steps?.length ?? 0}`}
                    </span>
                  </div>

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
                      className="mix-preview-nav"
                      onClick={() => selectRelativePreview(-1)}
                      disabled={!previewPreviousStep}
                      title={previewPreviousStep ? `${previewPreviousStep.track.artist || '—'} — ${previewPreviousStep.track.title || previewPreviousStep.track.fileName}` : 'Previous track'}
                      aria-label="Previous track"
                    >
                      ⏮
                    </button>
                    <button
                      type="button"
                      className="mix-preview-toggle"
                      onClick={() => void togglePreviewPlayback()}
                      disabled={!previewMedia?.audioUrl || previewLoading || previewFallbackLoading}
                      title={previewPlaying ? t('media.pause') : t('media.play')}
                      aria-label={previewPlaying ? t('media.pause') : t('media.play')}
                    >
                      {previewPlaying ? 'Ⅱ' : '▶'}
                    </button>
                    <button type="button" onClick={stopPreviewPlayback} disabled={!previewMedia?.audioUrl} title="Stop" aria-label="Stop">■</button>
                    <button
                      type="button"
                      className="mix-preview-nav"
                      onClick={() => selectRelativePreview(1)}
                      disabled={!previewNextStep}
                      title={previewNextStep ? `${previewNextStep.track.artist || '—'} — ${previewNextStep.track.title || previewNextStep.track.fileName}` : 'Next track'}
                      aria-label="Next track"
                    >
                      ⏭
                    </button>
                    <button
                      type="button"
                      className={`mix-preview-auto${previewAutoAdvance ? ' is-active' : ''}`}
                      onClick={() => setPreviewAutoAdvance((current) => !current)}
                      aria-pressed={previewAutoAdvance}
                      title="Auto-play next track when the current preview ends"
                      aria-label="Auto-play next track"
                    >
                      AUTO
                    </button>
                    <button type="button" onClick={() => skipPreview(-10)} disabled={!previewMedia?.audioUrl} title="−10 s">−10</button>
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
              playing={previewPlaying}
              currentTime={previewCurrentTime}
              duration={previewDuration}
              transitionPreviewIndex={transitionPreviewIndex}
              transitionPreviewLoading={transitionPreviewLoading}
              transitionPreviewPlaying={transitionPreviewPlaying}
              transitionPreviewProgress={transitionPreviewProgress}
              transitionPreviewError={transitionPreviewError}
              t={t}
              onSelectTrack={(trackID) => selectPreviewTrack(trackID)}
              onPlayTrack={(trackID) => selectPreviewTrack(trackID, true)}
              onPreviewTransition={(index) => void auditionTransition(index)}
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
  const rowRef = useRef<HTMLTableRowElement | null>(null)

  useEffect(() => {
    if (!isPreviewSelected) return
    const timer = window.setTimeout(() => {
      rowRef.current?.scrollIntoView({
        behavior: 'smooth',
        block: 'nearest',
        inline: 'nearest',
      })
    }, 0)
    return () => window.clearTimeout(timer)
  }, [isPreviewSelected])

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
      ref={rowRef}
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
  playing,
  currentTime,
  duration,
  transitionPreviewIndex,
  transitionPreviewLoading,
  transitionPreviewPlaying,
  transitionPreviewProgress,
  transitionPreviewError,
  t,
  onSelectTrack,
  onPlayTrack,
  onPreviewTransition,
}: {
  steps: DJMixPlanStep[]
  selectedTrackID: number
  startTrackID: number
  playing: boolean
  currentTime: number
  duration: number
  transitionPreviewIndex: number
  transitionPreviewLoading: boolean
  transitionPreviewPlaying: boolean
  transitionPreviewProgress: number
  transitionPreviewError: string
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
  onSelectTrack: (trackID: number) => void
  onPlayTrack: (trackID: number) => void
  onPreviewTransition: (index: number) => void
}) {
  const selectedCardRef = useRef<HTMLButtonElement | null>(null)

  useEffect(() => {
    if (selectedTrackID <= 0) return
    const timer = window.setTimeout(() => {
      selectedCardRef.current?.scrollIntoView({
        behavior: 'smooth',
        block: 'nearest',
        inline: 'center',
      })
    }, 0)
    return () => window.clearTimeout(timer)
  }, [selectedTrackID])

  if (steps.length === 0) return null

  const lastIndex = steps.length - 1
  const totalMS = effectiveTimelineStartMS(steps, lastIndex) + Math.max(0, steps[lastIndex]?.track.durationMs || 0)

  return (
    <div className="mix-planner-flow">
      <div className="mix-planner-flow-head">
        <strong>{t('mixPlanner.timeline')}</strong>
        {transitionPreviewIndex > 0 && (
          <span
            className={`mix-planner-transition-status${transitionPreviewError ? ' is-error' : ''}`}
            title={transitionPreviewError || `Transition #${transitionPreviewIndex} -> #${transitionPreviewIndex + 1}`}
          >
            {transitionPreviewError
              ? `MIX error: ${transitionPreviewError}`
              : transitionPreviewLoading
                ? `MIX #${transitionPreviewIndex} -> #${transitionPreviewIndex + 1} loading`
                : transitionPreviewPlaying
                  ? `MIX #${transitionPreviewIndex} -> #${transitionPreviewIndex + 1} playing`
                  : `MIX #${transitionPreviewIndex} -> #${transitionPreviewIndex + 1} ready`}
          </span>
        )}
        <span>{steps.length} tracks · {formatTimeline(totalMS)}</span>
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
            const isSelected = step.track.id === selectedTrackID
            const playbackDuration = duration > 0 ? duration : Math.max(0, (step.track.durationMs || 0) / 1000)
            const playbackProgress = isSelected && playbackDuration > 0
              ? Math.max(0, Math.min(1, currentTime / playbackDuration))
              : 0

            return (
              <div className={classes} key={`flow-${step.position}-${step.track.id}`}>
                {index > 0 && (
                  <button
                    type="button"
                    className={[
                      'mix-planner-flow-transition',
                      transitionPreviewIndex === index ? 'is-active' : '',
                      transitionPreviewIndex === index && transitionPreviewLoading ? 'is-loading' : '',
                      transitionPreviewIndex === index && transitionPreviewPlaying ? 'is-playing' : '',
                    ].filter(Boolean).join(' ')}
                    onClick={() => onPreviewTransition(index)}
                    aria-pressed={transitionPreviewIndex === index}
                    title={`Preview transition #${index} -> #${index + 1} / ${mixTransitionPlaybackRate(steps[index - 1], step).toFixed(3)}x / ${formatSigned(step.tempoDeltaPct)}% / ${relation}`}
                  >
                    <strong>
                      {transitionPreviewIndex === index && transitionPreviewLoading
                        ? 'LOAD'
                        : transitionPreviewIndex === index && transitionPreviewPlaying
                          ? 'STOP'
                          : 'MIX'} {Math.round(step.score * 100)}%
                    </strong>
                    <span>{formatSigned(step.tempoDeltaPct)}% / {step.camelot || relation || '-'}</span>
                    {transitionPreviewIndex === index && (
                      <i className="mix-planner-transition-progress" aria-hidden="true">
                        <i style={{width: `${transitionPreviewProgress * 100}%`}} />
                      </i>
                    )}
                  </button>
                )}
                <button
                  ref={isSelected ? selectedCardRef : undefined}
                  type="button"
                  className={[
                    'mix-planner-flow-card',
                    isSelected ? 'is-selected' : '',
                    isSelected && playing ? 'is-playing' : '',
                    step.track.id === startTrackID ? 'is-start' : '',
                  ].filter(Boolean).join(' ')}
                  style={{width: `${mixTimelineTrackWidth(step.track.durationMs)}px`}}
                  onClick={() => onSelectTrack(step.track.id)}
                  onDoubleClick={() => onPlayTrack(step.track.id)}
                  aria-pressed={step.track.id === selectedTrackID}
                  title={`${step.track.artist || '—'} — ${step.track.title || step.track.fileName}`}
                >
                  <span className="mix-planner-flow-card-top">
                    <b className="mix-planner-flow-position">#{step.position}</b>
                    <span className="mix-planner-flow-time">{formatTimeline(startMS)}</span>
                  </span>
                  <span className="mix-planner-flow-title">
                    {step.track.artist || '—'} — {step.track.title || step.track.fileName}
                  </span>
                  <span className="mix-planner-flow-card-meta">
                    <b>{formatBPM(step.adjustedBpm || step.track.bpm)} BPM</b>
                    <span>{step.camelot || step.openKey || '—'}</span>
                    <span>{formatDuration(step.track.durationMs)}</span>
                  </span>
                  <span className="mix-planner-flow-energy" title={`Energy ${energyPct}%`}>
                    <span style={{width: `${energyPct}%`}} />
                  </span>
                  {isSelected && (
                    <span className="mix-planner-flow-playback" aria-hidden="true">
                      <span style={{width: `${playbackProgress * 100}%`}} />
                    </span>
                  )}
                </button>
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}

function mixTransitionPlaybackRate(outgoing: DJMixPlanStep, incoming: DJMixPlanStep): number {
  const outgoingBPM = outgoing.adjustedBpm || outgoing.track.bpm
  const incomingBPM = incoming.adjustedBpm || incoming.track.bpm
  if (!Number.isFinite(outgoingBPM) || !Number.isFinite(incomingBPM) || outgoingBPM <= 0 || incomingBPM <= 0) {
    return 1
  }
  return Math.max(0.8, Math.min(1.25, outgoingBPM / incomingBPM))
}

function loadTransitionPreviewAudio(audio: HTMLAudioElement, url: string): Promise<void> {
  return new Promise((resolve, reject) => {
    if (!url) {
      reject(new Error('Audio preview URL is unavailable'))
      return
    }

    let settled = false
    let timer = 0

    const cleanup = () => {
      window.clearTimeout(timer)
      audio.removeEventListener('loadedmetadata', onReady)
      audio.removeEventListener('canplay', onReady)
      audio.removeEventListener('error', onError)
    }

    const onReady = () => {
      if (settled) return
      settled = true
      cleanup()
      resolve()
    }

    const onError = () => {
      if (settled) return
      settled = true
      cleanup()
      reject(new Error('Audio preview could not be loaded'))
    }

    audio.pause()
    audio.preload = 'auto'
    audio.src = url
    audio.addEventListener('loadedmetadata', onReady)
    audio.addEventListener('canplay', onReady)
    audio.addEventListener('error', onError)
    timer = window.setTimeout(() => {
      if (settled) return
      settled = true
      cleanup()
      reject(new Error('Audio preview loading timed out'))
    }, 15000)
    audio.load()

    if (audio.readyState >= 1) onReady()
  })
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
      <span className="mix-planner-waveform-hint">click / drag seek · ← → 5s</span>
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
