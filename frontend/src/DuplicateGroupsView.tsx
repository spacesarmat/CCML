import {useEffect, useMemo, useState} from 'react'
import type {AppLanguage} from './i18n'
import type {DuplicateAudioVerification, DuplicateGroup, DuplicateTrackQuality, Track} from './types'

type Props = {
  language: AppLanguage
  groups: DuplicateGroup[]
  busy: boolean
  onBack: () => void
  onRefresh: () => void
  onOpenTrack: (track: Track) => void
  canVerifyAudio: boolean
  onVerifyAudio: (trackIDs: number[]) => Promise<DuplicateAudioVerification | null>
  onQuarantine: (trackIDs: number[]) => Promise<boolean>
  onDelete: (trackIDs: number[]) => Promise<boolean>
}

type FilterMode = 'all' | 'exact' | 'possible' | 'unresolved'

type QueueDecisionKind = 'keepBest' | 'keepAll' | 'quarantineSelected' | 'skip'

type QueueDecision = {
  kind: QueueDecisionKind
  trackIDs: number[]
  signature: string
}

const duplicateQueueStorageKey = 'ccml.duplicate-decisions.v1'

function duplicateGroupSignature(group: DuplicateGroup): string {
  return group.tracks.map((track) => track.id).sort((left, right) => left - right).join(',')
}

function duplicateVerificationMatchesGroup(
  group: DuplicateGroup,
  verification: DuplicateAudioVerification | undefined,
): boolean {
  if (!verification || verification.groupKey !== group.key) return false

  const expected = group.tracks.map((track) => track.id).sort((left, right) => left - right)
  const actual = verification.comparisons.map((item) => item.trackId).sort((left, right) => left - right)
  if (expected.length !== actual.length) return false
  return expected.every((id, index) => id === actual[index])
}

function expectedKeepBestIDs(group: DuplicateGroup): number[] {
  if (!group.recommendedTrackId) return []
  return group.tracks
    .filter((track) => track.id !== group.recommendedTrackId)
    .map((track) => track.id)
    .sort((left, right) => left - right)
}

function validQueueDecision(group: DuplicateGroup, decision: QueueDecision | undefined): QueueDecision | null {
  if (!decision || decision.signature !== duplicateGroupSignature(group)) return null

  const groupIDs = new Set(group.tracks.map((track) => track.id))
  const ids = [...new Set(decision.trackIDs)].sort((left, right) => left - right)
  if (ids.some((id) => !groupIDs.has(id))) return null

  if (decision.kind === 'keepBest') {
    const expected = expectedKeepBestIDs(group)
    if (expected.length === 0 || expected.length !== ids.length) return null
    if (expected.some((id, index) => id !== ids[index])) return null
  } else if (decision.kind === 'quarantineSelected') {
    if (ids.length === 0 || ids.length >= group.tracks.length) return null
  } else if (ids.length !== 0) {
    return null
  }

  return {...decision, trackIDs: ids}
}

function loadQueueDecisions(): Record<string, QueueDecision> {
  try {
    const raw = window.localStorage.getItem(duplicateQueueStorageKey)
    if (!raw) return {}
    const parsed = JSON.parse(raw)
    return parsed && typeof parsed === 'object' ? parsed as Record<string, QueueDecision> : {}
  } catch {
    return {}
  }
}

type PendingDelete = {
  groupKey: string
  label: string
  tracks: Track[]
}

function formatDuration(ms: number): string {
  if (!ms) return '—'
  const total = Math.round(ms / 1000)
  const minutes = Math.floor(total / 60)
  const seconds = total % 60
  return `${minutes}:${seconds.toString().padStart(2, '0')}`
}

function formatSpread(ms: number): string {
  if (!ms) return '0 ms'
  if (ms < 1000) return `${ms} ms`
  return `${(ms / 1000).toFixed(ms < 10_000 ? 1 : 0)} s`
}

function formatSize(bytes: number): string {
  if (!bytes) return '—'
  const mb = bytes / (1024 * 1024)
  return `${mb.toFixed(mb >= 100 ? 0 : 1)} MB`
}

function formatBitRate(value: number): string {
  if (!value) return '—'
  const kbps = value >= 10_000 ? value / 1000 : value
  return `${Math.round(kbps)} kbps`
}

function formatSampleRate(value: number): string {
  if (!value) return '—'
  return value >= 1000 ? `${(value / 1000).toFixed(value % 1000 === 0 ? 0 : 1)} kHz` : `${value} Hz`
}

function groupTrackCount(groups: DuplicateGroup[]): number {
  const ids = new Set<number>()
  for (const group of groups) {
    for (const track of group.tracks) ids.add(track.id)
  }
  return ids.size
}

function DuplicateGroupsView({
  language,
  groups,
  busy,
  onBack,
  onRefresh,
  onOpenTrack,
  canVerifyAudio,
  onVerifyAudio,
  onQuarantine,
  onDelete,
}: Props) {
  const [filter, setFilter] = useState<FilterMode>('all')
  const [query, setQuery] = useState('')
  const [activeGroupKey, setActiveGroupKey] = useState('')
  const [selectedIDs, setSelectedIDs] = useState<Set<number>>(new Set())
  const [pendingDelete, setPendingDelete] = useState<PendingDelete | null>(null)
  const [deleteConfirm, setDeleteConfirm] = useState('')
  const [audioChecks, setAudioChecks] = useState<Record<string, DuplicateAudioVerification>>({})
  const [queueDecisions, setQueueDecisions] = useState<Record<string, QueueDecision>>(loadQueueDecisions)
  const [queueReviewOpen, setQueueReviewOpen] = useState(false)

  const copy = language === 'ru'
    ? {
        title: 'Дубликаты',
        subtitle: 'Выберите группу слева и сравните её файлы справа.',
        back: 'Назад к медиатеке',
        refresh: 'Пересчитать',
        search: 'Исполнитель, название, файл, путь или ISRC…',
        all: 'Все',
        exact: 'Точные',
        filterPossible: 'Возможные',
        groups: 'Групп',
        files: 'Файлов в группах',
        exactGroups: 'Точных',
        possibleGroups: 'Возможных',
        unresolved: 'Не обработано',
        processed: 'Обработано',
        queue: 'Очередь',
        reviewQueue: 'Проверить очередь',
        queueDecision: 'Решение группы',
        queuePending: 'Без решения',
        queueKeepBest: 'Оставить лучший',
        queueKeepAll: 'Оставить всё',
        queueQuarantineSelected: 'Карантин выбранных',
        queueSkip: 'Пропустить',
        queueReset: 'Сбросить решение',
        selectExceptBest: 'Выделить кроме лучшего',
        nextUnresolved: 'Следующая необработанная',
        queueReviewTitle: 'План обработки дубликатов',
        queueReviewHint: 'Пакетное применение использует только безопасный карантин. Безвозвратное удаление остаётся доступно только для одной группы вручную.',
        queueBestGroups: 'Оставить лучший',
        queueKeepAllGroups: 'Оставить всё',
        queueManualGroups: 'Карантин выбранных',
        queueSkippedGroups: 'Пропущено',
        queueUnresolvedGroups: 'Без решения',
        queueFilesToQuarantine: 'Файлов в карантин',
        queueFilesRemain: 'Файлов останется',
        queuePossibleWarning: (count: number) => `В плане есть ${count} возможных групп (Remix/Edit/Intro и т. п.). Проверьте их особенно внимательно перед применением.`,
        queueApply: 'Применить карантин',
        queueClose: 'Закрыть',
        queueNothingToApply: 'В плане нет файлов для карантина.',
        queueLimit: 'За одну пакетную операцию можно обработать не более 500 файлов.',
        queuePlanned: 'Запланировано',
        none: 'Подходящих групп не найдено.',
        selectGroup: 'Выберите группу дубликатов слева.',
        groupList: 'Группы',
        isrc: 'ISRC',
        metadata: 'Artist / Title',
        badgePossible: 'Возможный',
        confidence: 'уверенность',
        spread: 'Разброс',
        sharedIsrc: 'Общий ISRC',
        file: 'Файл',
        quality: 'Качество',
        codec: 'Кодек',
        bitrate: 'Битрейт',
        sampleRate: 'Sample rate',
        size: 'Размер',
        duration: 'Время',
        trackIsrc: 'ISRC',
        open: 'Открыть',
        best: 'Лучшее качество',
        tie: 'Нет явного лидера',
        audio: 'аудио',
        metadataScore: 'теги',
        scoreTitle: 'Эвристическая оценка качества. Не является доказательством, что версии идентичны.',
        selected: (count: number, total: number) => `Выбрано ${count} из ${total}`,
        keepBest: 'Выделить кроме лучшего',
        keepBestHint: 'Только выделить все файлы группы, кроме рекомендованного. Для очереди используйте решение «Оставить лучший».',
        noLeader: 'Нет явного лидера',
        clear: 'Снять выделение',
        quarantine: 'В карантин…',
        quarantineConfirm: (count: number) => `Переместить ${count} выбранных файлов в карантин и убрать их из медиатеки?\n\nСледующим шагом CCML попросит выбрать папку ВНЕ папок медиатеки.`,
        delete: 'Удалить…',
        deleteTitle: 'Безвозвратное удаление дублей',
        deleteWarning: 'Файлы будут удалены с диска и из медиатеки. Это действие не входит в Undo истории тегов.',
        deletePossibleWarning: 'В группе есть возможные версии трека. Убедитесь, что Remix/Edit/Intro действительно не нужны.',
        deleteType: 'Для подтверждения введите DELETE',
        deletePlaceholder: 'DELETE',
        deleteCancel: 'Отмена',
        deleteConfirmButton: 'Удалить безвозвратно',
        deleteFiles: (count: number) => `Файлов к удалению: ${count}`,
        keepOneSafety: 'Backend дополнительно проверит, что в группе останется хотя бы один файл.',
        selectTrack: 'Выбрать файл для действия',
        verifyAudio: 'Проверить аудио',
        verifyAudioUnavailable: 'Для проверки аудио требуется FFmpeg.',
        verifyAudioHint: 'FFmpeg декодирует файлы в низкочастотный mono PCM и сравнивает форму сигнала вместе с грубым спектральным профилем. Выравнивание адаптивно расширяется до 60 секунд для Intro/Edit/Extended-различий; Similar также требует разницу полной длительности не более 60 секунд. Это дополнительная эвристика, а не доказательство идентичности.',
        audioReference: 'Эталон',
        audioSame: 'Совпадает',
        audioSimilar: 'Похоже',
        audioDifferent: 'Отличается',
        audioError: 'Ошибка',
        audioSummary: (same: number, similar: number, different: number, errors: number) => `Аудио: совпадает ${same} · похоже ${similar} · отличается ${different} · ошибок ${errors}`,
        similarity: 'сходство',
        offset: 'сдвиг',
        qualityReasons: {
          lossless: 'lossless',
          efficient_lossy: 'эффективный lossy-кодек',
          lossy: 'lossy',
          unknown_codec: 'неизвестный кодек',
          lossless_bitrate: 'lossless без штрафа за bitrate',
          bitrate_320: '≥320 kbps',
          bitrate_256: '≥256 kbps',
          bitrate_192: '≥192 kbps',
          bitrate_160: '≥160 kbps',
          bitrate_128: '≥128 kbps',
          bitrate_96: '≥96 kbps',
          bitrate_low: 'низкий bitrate',
          bitrate_unknown: 'bitrate неизвестен',
          sample_rate_high: 'высокий sample rate',
          sample_rate_standard: 'стандартный sample rate',
          sample_rate_low: 'низкий sample rate',
          sample_rate_unknown: 'sample rate неизвестен',
          stereo: 'stereo',
          mono: 'mono',
          channels_unknown: 'каналы неизвестны',
          embedded_cover: 'есть обложка',
          metadata_complete: 'теги заполнены хорошо',
          metadata_partial: 'теги заполнены частично',
          metadata_sparse: 'мало тегов',
          scan_error: 'ошибка сканирования',
        } as Record<string, string>,
        reasons: {
          same_isrc: 'совпадает ISRC',
          same_artist_title: 'совпадают Artist и Title',
          same_artist: 'совпадает Artist',
          version_normalized_title: 'Title совпал без Remix/Edit/Intro/Clean',
          duration_close: 'близкая длительность',
          linked_isrc: 'часть файлов связана ISRC',
          linked_candidates: 'связанные кандидаты',
        } as Record<string, string>,
      }
    : {
        title: 'Duplicates',
        subtitle: 'Select a group on the left and compare its files on the right.',
        back: 'Back to library',
        refresh: 'Recalculate',
        search: 'Artist, title, file, path or ISRC…',
        all: 'All',
        exact: 'Exact',
        filterPossible: 'Possible',
        groups: 'Groups',
        files: 'Files in groups',
        exactGroups: 'Exact',
        possibleGroups: 'Possible',
        unresolved: 'Unresolved',
        processed: 'Processed',
        queue: 'Queue',
        reviewQueue: 'Review queue',
        queueDecision: 'Group decision',
        queuePending: 'No decision',
        queueKeepBest: 'Keep best',
        queueKeepAll: 'Keep all',
        queueQuarantineSelected: 'Quarantine selected',
        queueSkip: 'Skip',
        queueReset: 'Reset decision',
        selectExceptBest: 'Select except best',
        nextUnresolved: 'Next unresolved',
        queueReviewTitle: 'Duplicate processing plan',
        queueReviewHint: 'Batch apply uses safe quarantine only. Permanent deletion remains a manual single-group action.',
        queueBestGroups: 'Keep best',
        queueKeepAllGroups: 'Keep all',
        queueManualGroups: 'Quarantine selected',
        queueSkippedGroups: 'Skipped',
        queueUnresolvedGroups: 'Unresolved',
        queueFilesToQuarantine: 'Files to quarantine',
        queueFilesRemain: 'Files remaining',
        queuePossibleWarning: (count: number) => `The plan contains ${count} possible groups (Remix/Edit/Intro etc.). Review them especially carefully before applying.`,
        queueApply: 'Apply quarantine',
        queueClose: 'Close',
        queueNothingToApply: 'There are no files queued for quarantine.',
        queueLimit: 'A single batch operation can process at most 500 files.',
        queuePlanned: 'Planned',
        none: 'No matching duplicate groups.',
        selectGroup: 'Select a duplicate group on the left.',
        groupList: 'Groups',
        isrc: 'ISRC',
        metadata: 'Artist / Title',
        badgePossible: 'Possible',
        confidence: 'confidence',
        spread: 'Spread',
        sharedIsrc: 'Shared ISRC',
        file: 'File',
        quality: 'Quality',
        codec: 'Codec',
        bitrate: 'Bitrate',
        sampleRate: 'Sample rate',
        size: 'Size',
        duration: 'Time',
        trackIsrc: 'ISRC',
        open: 'Open',
        best: 'Best quality',
        tie: 'No clear leader',
        audio: 'audio',
        metadataScore: 'tags',
        scoreTitle: 'Heuristic quality score. It does not prove that the versions are identical.',
        selected: (count: number, total: number) => `Selected ${count} of ${total}`,
        keepBest: 'Select except best',
        keepBestHint: 'Only select every file except the recommendation. Use the Keep best queue decision to plan batch quarantine.',
        noLeader: 'No clear leader',
        clear: 'Clear selection',
        quarantine: 'Quarantine…',
        quarantineConfirm: (count: number) => `Move ${count} selected files to quarantine and remove them from the library index?\n\nCCML will next ask for a folder OUTSIDE the managed library folders.`,
        delete: 'Delete…',
        deleteTitle: 'Permanently delete duplicates',
        deleteWarning: 'Files will be removed from disk and from the library. This action is not part of tag Undo history.',
        deletePossibleWarning: 'This group contains possible track versions. Make sure Remix/Edit/Intro files are really unwanted.',
        deleteType: 'Type DELETE to confirm',
        deletePlaceholder: 'DELETE',
        deleteCancel: 'Cancel',
        deleteConfirmButton: 'Delete permanently',
        deleteFiles: (count: number) => `Files to delete: ${count}`,
        keepOneSafety: 'The backend also verifies that at least one file remains in the group.',
        selectTrack: 'Select file for action',
        verifyAudio: 'Check audio',
        verifyAudioUnavailable: 'FFmpeg is required for audio verification.',
        verifyAudioHint: 'FFmpeg decodes files to low-rate mono PCM and compares waveform features together with a coarse spectral profile. Alignment expands adaptively up to 60 seconds for Intro/Edit/Extended differences; Similar also requires the full-duration difference to stay within 60 seconds. This is an additional heuristic, not proof of identity.',
        audioReference: 'Reference',
        audioSame: 'Same',
        audioSimilar: 'Similar',
        audioDifferent: 'Different',
        audioError: 'Error',
        audioSummary: (same: number, similar: number, different: number, errors: number) => `Audio: same ${same} · similar ${similar} · different ${different} · errors ${errors}`,
        similarity: 'similarity',
        offset: 'offset',
        qualityReasons: {
          lossless: 'lossless',
          efficient_lossy: 'efficient lossy codec',
          lossy: 'lossy',
          unknown_codec: 'unknown codec',
          lossless_bitrate: 'lossless bitrate neutral',
          bitrate_320: '≥320 kbps',
          bitrate_256: '≥256 kbps',
          bitrate_192: '≥192 kbps',
          bitrate_160: '≥160 kbps',
          bitrate_128: '≥128 kbps',
          bitrate_96: '≥96 kbps',
          bitrate_low: 'low bitrate',
          bitrate_unknown: 'bitrate unknown',
          sample_rate_high: 'high sample rate',
          sample_rate_standard: 'standard sample rate',
          sample_rate_low: 'low sample rate',
          sample_rate_unknown: 'sample rate unknown',
          stereo: 'stereo',
          mono: 'mono',
          channels_unknown: 'channels unknown',
          embedded_cover: 'embedded cover',
          metadata_complete: 'metadata well populated',
          metadata_partial: 'metadata partially populated',
          metadata_sparse: 'sparse metadata',
          scan_error: 'scan error',
        } as Record<string, string>,
        reasons: {
          same_isrc: 'same ISRC',
          same_artist_title: 'same Artist and Title',
          same_artist: 'same Artist',
          version_normalized_title: 'Title matches without Remix/Edit/Intro/Clean',
          duration_close: 'close duration',
          linked_isrc: 'some files linked by ISRC',
          linked_candidates: 'linked candidates',
        } as Record<string, string>,
      }

  const validDecisions = useMemo(() => {
    const result: Record<string, QueueDecision> = {}
    for (const group of groups) {
      const decision = validQueueDecision(group, queueDecisions[group.key])
      if (decision) result[group.key] = decision
    }
    return result
  }, [groups, queueDecisions])

  const processedCount = Object.keys(validDecisions).length
  const unresolvedCount = Math.max(0, groups.length - processedCount)

  const filtered = useMemo(() => {
    const needle = query.trim().toLocaleLowerCase()

    return groups.filter((group) => {
      if (filter === 'exact' && group.matchClass === 'possible') return false
      if (filter === 'possible' && group.matchClass !== 'possible') return false
      if (filter === 'unresolved' && validDecisions[group.key]) return false
      if (!needle) return true

      const haystack = [
        group.artist,
        group.title,
        group.sharedIsrc,
        ...group.tracks.flatMap((track) => [
          track.artist,
          track.title,
          track.fileName,
          track.path,
          track.isrc,
        ]),
      ].join('\n').toLocaleLowerCase()

      return haystack.includes(needle)
    })
  }, [groups, filter, query, validDecisions])

  useEffect(() => {
    setSelectedIDs(new Set())
    setPendingDelete(null)
    setDeleteConfirm('')
    setAudioChecks({})
    setQueueDecisions((current) => {
      const next: Record<string, QueueDecision> = {}
      for (const group of groups) {
        const decision = validQueueDecision(group, current[group.key])
        if (decision) next[group.key] = decision
      }
      return next
    })
  }, [groups])

  useEffect(() => {
    try {
      window.localStorage.setItem(duplicateQueueStorageKey, JSON.stringify(queueDecisions))
    } catch {
      // Local persistence is helpful but must never block duplicate work.
    }
  }, [queueDecisions])

  useEffect(() => {
    if (filtered.length === 0) {
      setActiveGroupKey('')
      return
    }
    if (!filtered.some((group) => group.key === activeGroupKey)) {
      setActiveGroupKey(filtered[0].key)
    }
  }, [filtered, activeGroupKey])

  useEffect(() => {
    if (!activeGroupKey) {
      setSelectedIDs(new Set())
      return
    }
    const decision = validDecisions[activeGroupKey]
    if (decision?.kind === 'keepBest' || decision?.kind === 'quarantineSelected') {
      setSelectedIDs(new Set(decision.trackIDs))
    } else {
      setSelectedIDs(new Set())
    }
  }, [activeGroupKey, validDecisions])

  const exactCount = groups.filter((group) => group.matchClass !== 'possible').length
  const possibleCount = groups.length - exactCount
  const activeGroup = filtered.find((group) => group.key === activeGroupKey) ?? null

  const queuePlan = useMemo(() => {
    const quarantineIDs = new Set<number>()
    let keepBestGroups = 0
    let keepAllGroups = 0
    let manualGroups = 0
    let skippedGroups = 0
    let possibleActionGroups = 0

    for (const group of groups) {
      const decision = validDecisions[group.key]
      if (!decision) continue

      switch (decision.kind) {
        case 'keepBest':
          keepBestGroups++
          decision.trackIDs.forEach((id) => quarantineIDs.add(id))
          if (group.matchClass === 'possible') possibleActionGroups++
          break
        case 'quarantineSelected':
          manualGroups++
          decision.trackIDs.forEach((id) => quarantineIDs.add(id))
          if (group.matchClass === 'possible') possibleActionGroups++
          break
        case 'keepAll':
          keepAllGroups++
          break
        case 'skip':
          skippedGroups++
          break
      }
    }

    return {
      keepBestGroups,
      keepAllGroups,
      manualGroups,
      skippedGroups,
      unresolvedGroups: unresolvedCount,
      possibleActionGroups,
      quarantineTrackIDs: [...quarantineIDs],
      filesRemain: Math.max(0, groupTrackCount(groups) - quarantineIDs.size),
    }
  }, [groups, validDecisions, unresolvedCount])

  function queueDecisionLabel(decision: QueueDecision | undefined): string {
    switch (decision?.kind) {
      case 'keepBest': return copy.queueKeepBest
      case 'keepAll': return copy.queueKeepAll
      case 'quarantineSelected': return copy.queueQuarantineSelected
      case 'skip': return copy.queueSkip
      default: return copy.queuePending
    }
  }

  function setGroupDecision(group: DuplicateGroup, kind: QueueDecisionKind) {
    let trackIDs: number[] = []

    if (kind === 'keepBest') {
      trackIDs = expectedKeepBestIDs(group)
      if (trackIDs.length === 0) return
    } else if (kind === 'quarantineSelected') {
      trackIDs = selectedTracks(group).map((track) => track.id).sort((left, right) => left - right)
      if (trackIDs.length === 0 || trackIDs.length >= group.tracks.length) return
    }

    const decision: QueueDecision = {
      kind,
      trackIDs,
      signature: duplicateGroupSignature(group),
    }
    setQueueDecisions((current) => ({...current, [group.key]: decision}))
    setSelectedIDs(new Set(trackIDs))
  }

  function resetGroupDecision(group: DuplicateGroup) {
    setQueueDecisions((current) => {
      const next = {...current}
      delete next[group.key]
      return next
    })
    setSelectedIDs(new Set())
  }

  function selectNextUnresolved() {
    if (groups.length === 0) return
    const start = Math.max(0, groups.findIndex((group) => group.key === activeGroupKey))
    for (let offset = 1; offset <= groups.length; offset++) {
      const group = groups[(start + offset) % groups.length]
      if (!validDecisions[group.key]) {
        // Navigation is defined over the complete duplicate-group list, not the
        // current search subset. Clear the search in the same state update so
        // the filtered-list effect cannot replace this target with a different
        // visible group.
        setQuery('')
        setFilter('all')
        setActiveGroupKey(group.key)
        return
      }
    }
  }

  async function applyQueuePlan() {
    if (queuePlan.quarantineTrackIDs.length === 0 || queuePlan.quarantineTrackIDs.length > 500) return
    const ok = await onQuarantine(queuePlan.quarantineTrackIDs)
    if (!ok) return

    setQueueReviewOpen(false)
    setQueueDecisions({})
    setSelectedIDs(new Set())
  }

  function badge(group: DuplicateGroup) {
    if (group.matchClass === 'isrc') return copy.isrc
    if (group.matchClass === 'metadata') return copy.metadata
    return copy.badgePossible
  }

  function qualityMap(group: DuplicateGroup): Map<number, DuplicateTrackQuality> {
    return new Map(group.quality.map((item) => [item.trackId, item]))
  }

  function qualityTitle(quality: DuplicateTrackQuality | undefined): string {
    if (!quality) return copy.scoreTitle
    const reasons = quality.reasons.map((reason) => copy.qualityReasons[reason] || reason)
    return `${copy.scoreTitle}\n${quality.score}/100 · ${copy.audio}: ${quality.audioScore}/80 · ${copy.metadataScore}: ${quality.metadataScore}/20\n${reasons.join(' · ')}`
  }

  function audioStatusLabel(status: string): string {
    switch (status) {
      case 'reference': return copy.audioReference
      case 'same': return copy.audioSame
      case 'similar': return copy.audioSimilar
      case 'different': return copy.audioDifferent
      case 'error': return copy.audioError
      default: return status
    }
  }

  function audioComparisonTitle(
    verification: DuplicateAudioVerification | undefined,
    trackID: number,
  ): string {
    const comparison = verification?.comparisons.find((item) => item.trackId === trackID)
    if (!comparison) return copy.verifyAudioHint
    if (comparison.error) return `${audioStatusLabel(comparison.status)}: ${comparison.error}`
    if (comparison.status === 'reference') return `${copy.audioReference}. ${copy.verifyAudioHint}`

    const percent = Math.round(comparison.similarity * 1000) / 10
    const offset = comparison.offsetMs === 0 ? '0 ms' : `${comparison.offsetMs > 0 ? '+' : ''}${comparison.offsetMs} ms`
    return `${audioStatusLabel(comparison.status)} · ${copy.similarity}: ${percent}% · ${copy.offset}: ${offset}\n${copy.verifyAudioHint}`
  }

  async function verifyGroupAudio(group: DuplicateGroup) {
    const result = await onVerifyAudio(group.tracks.map((track) => track.id))
    if (!result || !duplicateVerificationMatchesGroup(group, result)) return
    setAudioChecks((current) => ({...current, [result.groupKey]: result}))
  }

  function selectedTracks(group: DuplicateGroup): Track[] {
    return group.tracks.filter((track) => selectedIDs.has(track.id))
  }

  function toggleTrackSelection(trackID: number) {
    setSelectedIDs((current) => {
      const next = new Set(current)
      if (next.has(trackID)) next.delete(trackID)
      else next.add(trackID)
      return next
    })
  }

  function selectAllExceptRecommended(group: DuplicateGroup) {
    if (!group.recommendedTrackId) return
    setSelectedIDs((current) => {
      const next = new Set(current)
      for (const track of group.tracks) {
        next.delete(track.id)
        if (track.id !== group.recommendedTrackId) next.add(track.id)
      }
      return next
    })
  }

  function clearGroupSelection(group: DuplicateGroup) {
    setSelectedIDs((current) => {
      const next = new Set(current)
      for (const track of group.tracks) next.delete(track.id)
      return next
    })
  }

  async function quarantineGroup(group: DuplicateGroup) {
    const tracks = selectedTracks(group)
    if (tracks.length === 0) return
    if (!window.confirm(copy.quarantineConfirm(tracks.length))) return

    const ok = await onQuarantine(tracks.map((track) => track.id))
    if (ok) clearGroupSelection(group)
  }

  function requestDelete(group: DuplicateGroup) {
    const tracks = selectedTracks(group)
    if (tracks.length === 0) return
    setDeleteConfirm('')
    setPendingDelete({
      groupKey: group.key,
      label: `${group.artist || '—'} — ${group.title || '—'}`,
      tracks,
    })
  }

  async function confirmDelete() {
    if (!pendingDelete || deleteConfirm.trim().toUpperCase() !== 'DELETE') return
    const ids = pendingDelete.tracks.map((track) => track.id)
    const ok = await onDelete(ids)
    if (!ok) return
    setPendingDelete(null)
    setDeleteConfirm('')
    setSelectedIDs((current) => {
      const next = new Set(current)
      for (const id of ids) next.delete(id)
      return next
    })
  }

  const activeScores = activeGroup ? qualityMap(activeGroup) : new Map<number, DuplicateTrackQuality>()
  const activeVerificationCandidate = activeGroup ? audioChecks[activeGroup.key] : undefined
  const activeVerification = activeGroup && duplicateVerificationMatchesGroup(activeGroup, activeVerificationCandidate)
    ? activeVerificationCandidate
    : undefined
  const activeAudioByTrack = new Map(activeVerification?.comparisons.map((item) => [item.trackId, item]) ?? [])
  const activeSelected = activeGroup ? selectedTracks(activeGroup) : []
  const activeSortedTracks = activeGroup
    ? [...activeGroup.tracks].sort((left, right) => {
        const leftScore = activeScores.get(left.id)
        const rightScore = activeScores.get(right.id)
        return (rightScore?.score ?? 0) - (leftScore?.score ?? 0)
          || (rightScore?.audioScore ?? 0) - (leftScore?.audioScore ?? 0)
          || left.fileName.localeCompare(right.fileName)
      })
    : []

  return (
    <>
      <section className="duplicate-workspace duplicate-master-detail">
        <header className="duplicate-commandbar">
          <div>
            <h2>{copy.title}</h2>
            <span>{copy.subtitle}</span>
          </div>
          <div className="duplicate-command-actions">
            <button
              type="button"
              className="duplicate-queue-review-trigger"
              onClick={() => setQueueReviewOpen(true)}
            >
              {copy.reviewQueue} <b>{processedCount}/{groups.length}</b>
            </button>
            <button type="button" onClick={onBack}>{copy.back}</button>
            <button type="button" onClick={onRefresh} disabled={busy}>{copy.refresh}</button>
          </div>
        </header>

        <div className="duplicate-summary">
          <div><strong>{groups.length}</strong><span>{copy.groups}</span></div>
          <div><strong>{groupTrackCount(groups)}</strong><span>{copy.files}</span></div>
          <div><strong>{exactCount}</strong><span>{copy.exactGroups}</span></div>
          <div><strong>{possibleCount}</strong><span>{copy.possibleGroups}</span></div>
        </div>

        <div className="duplicate-filterbar">
          <input
            value={query}
            placeholder={copy.search}
            onChange={(event) => setQuery(event.target.value)}
          />
          <div className="duplicate-filter-buttons">
            {(['all', 'exact', 'possible', 'unresolved'] as FilterMode[]).map((mode) => (
              <button
                type="button"
                key={mode}
                className={filter === mode ? 'active' : ''}
                onClick={() => setFilter(mode)}
              >
                {mode === 'all'
                  ? copy.all
                  : mode === 'exact'
                    ? copy.exact
                    : mode === 'possible'
                      ? copy.filterPossible
                      : copy.unresolved}
                <b>
                  {mode === 'all'
                    ? groups.length
                    : mode === 'exact'
                      ? exactCount
                      : mode === 'possible'
                        ? possibleCount
                        : unresolvedCount}
                </b>
              </button>
            ))}
          </div>
        </div>

        <div className="duplicate-browser">
          <aside className="duplicate-master-list" aria-label={copy.groupList}>
            {filtered.length === 0 && <div className="duplicate-master-empty">{copy.none}</div>}

            {filtered.map((group) => {
              const active = group.key === activeGroupKey
              const recommended = group.quality.find((item) => item.trackId === group.recommendedTrackId)
              return (
                <button
                  type="button"
                  className={`duplicate-master-item ${group.matchClass}${active ? ' active' : ''}`}
                  key={group.key}
                  onClick={() => setActiveGroupKey(group.key)}
                >
                  <span className={`duplicate-master-badge ${group.matchClass}`}>{badge(group)}</span>
                  <span className="duplicate-master-title">
                    <strong>{group.artist || '—'} — {group.title || '—'}</strong>
                    <small>
                      {group.tracks.length} {copy.files.toLocaleLowerCase()} · {Math.round(group.confidence * 100)}%
                      {' · '}{formatSpread(group.durationSpreadMs)}
                    </small>
                    <em className={`duplicate-master-decision ${validDecisions[group.key]?.kind || 'pending'}`}>
                      {queueDecisionLabel(validDecisions[group.key])}
                    </em>
                  </span>
                  <span className="duplicate-master-score" title={copy.scoreTitle}>
                    {recommended?.score ?? '—'}
                  </span>
                </button>
              )
            })}
          </aside>

          <main className="duplicate-detail-pane">
            {!activeGroup && (
              <div className="duplicate-detail-empty">{copy.selectGroup}</div>
            )}

            {activeGroup && (
              <>
                <header className="duplicate-detail-head">
                  <div>
                    <div className="duplicate-detail-title-row">
                      <span className={`duplicate-master-badge ${activeGroup.matchClass}`}>{badge(activeGroup)}</span>
                      <h3>{activeGroup.artist || '—'} — {activeGroup.title || '—'}</h3>
                    </div>
                    <p>
                      {activeGroup.tracks.length} · {Math.round(activeGroup.confidence * 100)}% {copy.confidence}
                      {' · '}{copy.spread}: {formatSpread(activeGroup.durationSpreadMs)}
                      {activeGroup.sharedIsrc ? ` · ${copy.sharedIsrc}: ${activeGroup.sharedIsrc}` : ''}
                      {' · '}{activeGroup.recommendedTrackId ? copy.best : copy.tie}
                    </p>
                  </div>
                </header>

                <div className="duplicate-detail-scroll">
                  <div className="duplicate-queue-toolbar">
                    <span>
                      {copy.queueDecision}: <strong>{queueDecisionLabel(validDecisions[activeGroup.key])}</strong>
                    </span>
                    <button
                      type="button"
                      className={validDecisions[activeGroup.key]?.kind === 'keepBest' ? 'active' : ''}
                      disabled={busy || !activeGroup.recommendedTrackId}
                      title={activeGroup.recommendedTrackId ? copy.keepBestHint : copy.noLeader}
                      onClick={() => setGroupDecision(activeGroup, 'keepBest')}
                    >
                      {copy.queueKeepBest}
                    </button>
                    <button
                      type="button"
                      className={validDecisions[activeGroup.key]?.kind === 'keepAll' ? 'active' : ''}
                      disabled={busy}
                      onClick={() => setGroupDecision(activeGroup, 'keepAll')}
                    >
                      {copy.queueKeepAll}
                    </button>
                    <button
                      type="button"
                      className={validDecisions[activeGroup.key]?.kind === 'quarantineSelected' ? 'active' : ''}
                      disabled={busy || activeSelected.length === 0 || activeSelected.length >= activeGroup.tracks.length}
                      onClick={() => setGroupDecision(activeGroup, 'quarantineSelected')}
                    >
                      {copy.queueQuarantineSelected}
                    </button>
                    <button
                      type="button"
                      className={validDecisions[activeGroup.key]?.kind === 'skip' ? 'active' : ''}
                      disabled={busy}
                      onClick={() => setGroupDecision(activeGroup, 'skip')}
                    >
                      {copy.queueSkip}
                    </button>
                    {validDecisions[activeGroup.key] && (
                      <button
                        type="button"
                        disabled={busy}
                        onClick={() => resetGroupDecision(activeGroup)}
                      >
                        {copy.queueReset}
                      </button>
                    )}
                    <button type="button" disabled={busy || unresolvedCount === 0} onClick={selectNextUnresolved}>
                      {copy.nextUnresolved}
                    </button>
                  </div>

                  <div className="duplicate-group-actions">
                    <span>{copy.selected(activeSelected.length, activeGroup.tracks.length)}</span>
                    <button
                      type="button"
                      title={activeGroup.recommendedTrackId ? copy.keepBestHint : copy.noLeader}
                      disabled={busy || !activeGroup.recommendedTrackId}
                      onClick={() => selectAllExceptRecommended(activeGroup)}
                    >
                      {copy.keepBest}
                    </button>
                    <button
                      type="button"
                      disabled={busy || activeSelected.length === 0}
                      onClick={() => clearGroupSelection(activeGroup)}
                    >
                      {copy.clear}
                    </button>
                    <button
                      type="button"
                      className="duplicate-verify-audio"
                      title={canVerifyAudio ? copy.verifyAudioHint : copy.verifyAudioUnavailable}
                      disabled={busy || !canVerifyAudio}
                      onClick={() => void verifyGroupAudio(activeGroup)}
                    >
                      {copy.verifyAudio}
                    </button>
                    <button
                      type="button"
                      disabled={busy || activeSelected.length === 0}
                      onClick={() => void quarantineGroup(activeGroup)}
                    >
                      {copy.quarantine}
                    </button>
                    <button
                      type="button"
                      className="duplicate-action-danger"
                      disabled={busy || activeSelected.length === 0}
                      onClick={() => requestDelete(activeGroup)}
                    >
                      {copy.delete}
                    </button>
                  </div>

                  {activeVerification && (
                    <div className="duplicate-audio-summary" title={copy.verifyAudioHint}>
                      <strong>{copy.audioSummary(
                        activeVerification.sameCount,
                        activeVerification.similarCount,
                        activeVerification.differentCount,
                        activeVerification.errorCount,
                      )}</strong>
                      <span>
                        {copy.audioReference}: {activeGroup.tracks.find((track) => track.id === activeVerification.referenceTrackId)?.fileName || activeVerification.referenceTrackId}
                      </span>
                    </div>
                  )}

                  <div className="duplicate-reasons">
                    {activeGroup.reasons.map((reason) => (
                      <span key={reason}>{copy.reasons[reason] || reason}</span>
                    ))}
                  </div>

                  <div className="duplicate-compare-table quality-enabled action-enabled">
                    <div className="duplicate-compare-row head">
                      <span aria-hidden="true" />
                      <span>{copy.file}</span>
                      <span>{copy.quality}</span>
                      <span>{copy.codec}</span>
                      <span>{copy.bitrate}</span>
                      <span>{copy.sampleRate}</span>
                      <span>{copy.size}</span>
                      <span>{copy.duration}</span>
                      <span>{copy.trackIsrc}</span>
                      <span />
                    </div>

                    {activeSortedTracks.map((track) => {
                      const quality = activeScores.get(track.id)
                      const recommended = activeGroup.recommendedTrackId === track.id
                      const selected = selectedIDs.has(track.id)
                      const audioComparison = activeAudioByTrack.get(track.id)

                      return (
                        <div
                          className={`duplicate-compare-row${recommended ? ' recommended' : ''}${selected ? ' action-selected' : ''}`}
                          key={track.id}
                        >
                          <span className="duplicate-action-check">
                            <input
                              type="checkbox"
                              checked={selected}
                              disabled={busy}
                              onChange={() => toggleTrackSelection(track.id)}
                              aria-label={`${copy.selectTrack}: ${track.fileName}`}
                            />
                          </span>
                          <span className="duplicate-file-cell" title={track.path}>
                            <strong>{track.fileName}</strong>
                            <small>{track.path}</small>
                            {recommended && <em>{copy.best}</em>}
                          </span>
                          <span className="duplicate-quality-cell" title={qualityTitle(quality)}>
                            <strong>{quality?.score ?? 0}</strong>
                            <small>{copy.audio} {quality?.audioScore ?? 0} · {copy.metadataScore} {quality?.metadataScore ?? 0}</small>
                            {audioComparison && (
                              <em
                                className={`duplicate-audio-badge ${audioComparison.status}`}
                                title={audioComparisonTitle(activeVerification, track.id)}
                              >
                                {audioStatusLabel(audioComparison.status)}
                                {audioComparison.status !== 'reference' && audioComparison.status !== 'error'
                                  ? ` ${Math.round(audioComparison.similarity * 100)}%`
                                  : ''}
                              </em>
                            )}
                          </span>
                          <span>{track.codec || track.extension.replace('.', '').toUpperCase() || '—'}</span>
                          <span>{formatBitRate(track.bitRate)}</span>
                          <span>{formatSampleRate(track.sampleRate)}</span>
                          <span>{formatSize(track.size)}</span>
                          <span>{formatDuration(track.durationMs)}</span>
                          <span title={track.isrc || undefined}>{track.isrc || '—'}</span>
                          <button type="button" onClick={() => onOpenTrack(track)}>{copy.open}</button>
                        </div>
                      )
                    })}
                  </div>
                </div>
              </>
            )}
          </main>
        </div>
      </section>

      {queueReviewOpen && (
        <div
          className="duplicate-queue-backdrop"
          role="presentation"
          onMouseDown={(event) => {
            if (event.target === event.currentTarget && !busy) setQueueReviewOpen(false)
          }}
        >
          <section className="duplicate-queue-modal" role="dialog" aria-modal="true" aria-labelledby="duplicate-queue-title">
            <header>
              <div>
                <h2 id="duplicate-queue-title">{copy.queueReviewTitle}</h2>
                <span>{copy.queueReviewHint}</span>
              </div>
              <button type="button" disabled={busy} onClick={() => setQueueReviewOpen(false)} aria-label={copy.queueClose}>×</button>
            </header>

            <div className="duplicate-queue-body">
              <div className="duplicate-queue-progress">
                <strong>{copy.processed}: {processedCount}/{groups.length}</strong>
                <span>{copy.queueUnresolvedGroups}: {queuePlan.unresolvedGroups}</span>
              </div>

              <div className="duplicate-queue-summary-grid">
                <div><strong>{queuePlan.keepBestGroups}</strong><span>{copy.queueBestGroups}</span></div>
                <div><strong>{queuePlan.keepAllGroups}</strong><span>{copy.queueKeepAllGroups}</span></div>
                <div><strong>{queuePlan.manualGroups}</strong><span>{copy.queueManualGroups}</span></div>
                <div><strong>{queuePlan.skippedGroups}</strong><span>{copy.queueSkippedGroups}</span></div>
                <div><strong>{queuePlan.quarantineTrackIDs.length}</strong><span>{copy.queueFilesToQuarantine}</span></div>
                <div><strong>{queuePlan.filesRemain}</strong><span>{copy.queueFilesRemain}</span></div>
              </div>

              {queuePlan.possibleActionGroups > 0 && (
                <p className="duplicate-queue-warning">{copy.queuePossibleWarning(queuePlan.possibleActionGroups)}</p>
              )}

              {queuePlan.quarantineTrackIDs.length > 500 && (
                <p className="duplicate-queue-warning danger">{copy.queueLimit}</p>
              )}

              {queuePlan.quarantineTrackIDs.length === 0 && (
                <p className="duplicate-queue-note">{copy.queueNothingToApply}</p>
              )}

              <div className="duplicate-queue-plan-list">
                {groups.map((group) => {
                  const decision = validDecisions[group.key]
                  if (!decision) return null
                  return (
                    <div className={`duplicate-queue-plan-row ${decision.kind}`} key={group.key}>
                      <span>
                        <strong>{group.artist || '—'} — {group.title || '—'}</strong>
                        <small>{queueDecisionLabel(decision)}</small>
                      </span>
                      <b>
                        {decision.kind === 'keepBest' || decision.kind === 'quarantineSelected'
                          ? `${decision.trackIDs.length} → ${copy.quarantine}`
                          : '—'}
                      </b>
                    </div>
                  )
                })}
              </div>
            </div>

            <footer>
              <button type="button" disabled={busy} onClick={() => setQueueReviewOpen(false)}>
                {copy.queueClose}
              </button>
              <button
                type="button"
                className="primary"
                disabled={busy || queuePlan.quarantineTrackIDs.length === 0 || queuePlan.quarantineTrackIDs.length > 500}
                onClick={() => void applyQueuePlan()}
              >
                {copy.queueApply} · {queuePlan.quarantineTrackIDs.length}
              </button>
            </footer>
          </section>
        </div>
      )}

      {pendingDelete && (
        <div
          className="duplicate-delete-backdrop"
          role="presentation"
          onMouseDown={(event) => {
            if (event.target === event.currentTarget && !busy) {
              setPendingDelete(null)
              setDeleteConfirm('')
            }
          }}
        >
          <section className="duplicate-delete-modal" role="dialog" aria-modal="true" aria-labelledby="duplicate-delete-title">
            <header>
              <div>
                <h2 id="duplicate-delete-title">{copy.deleteTitle}</h2>
                <span>{pendingDelete.label}</span>
              </div>
              <button
                type="button"
                disabled={busy}
                onClick={() => {
                  setPendingDelete(null)
                  setDeleteConfirm('')
                }}
                aria-label={copy.deleteCancel}
              >
                ×
              </button>
            </header>

            <div className="duplicate-delete-body">
              <strong>{copy.deleteFiles(pendingDelete.tracks.length)}</strong>
              <p>{copy.deleteWarning}</p>
              {groups.find((group) => group.key === pendingDelete.groupKey)?.matchClass === 'possible' && (
                <p className="duplicate-delete-possible">{copy.deletePossibleWarning}</p>
              )}
              <p>{copy.keepOneSafety}</p>

              <div className="duplicate-delete-files">
                {pendingDelete.tracks.map((track) => (
                  <span key={track.id} title={track.path}>{track.fileName}</span>
                ))}
              </div>

              <label>
                <span>{copy.deleteType}</span>
                <input
                  autoFocus
                  value={deleteConfirm}
                  disabled={busy}
                  placeholder={copy.deletePlaceholder}
                  onChange={(event) => setDeleteConfirm(event.target.value)}
                />
              </label>
            </div>

            <footer>
              <button
                type="button"
                disabled={busy}
                onClick={() => {
                  setPendingDelete(null)
                  setDeleteConfirm('')
                }}
              >
                {copy.deleteCancel}
              </button>
              <button
                type="button"
                className="duplicate-action-danger"
                disabled={busy || deleteConfirm.trim().toUpperCase() !== 'DELETE'}
                onClick={() => void confirmDelete()}
              >
                {copy.deleteConfirmButton}
              </button>
            </footer>
          </section>
        </div>
      )}
    </>
  )
}

export default DuplicateGroupsView
