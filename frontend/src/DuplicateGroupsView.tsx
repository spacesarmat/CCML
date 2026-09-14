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

type FilterMode = 'all' | 'exact' | 'possible'

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
  const [openGroups, setOpenGroups] = useState<Set<string>>(new Set())
  const [selectedIDs, setSelectedIDs] = useState<Set<number>>(new Set())
  const [pendingDelete, setPendingDelete] = useState<PendingDelete | null>(null)
  const [deleteConfirm, setDeleteConfirm] = useState('')
  const [audioChecks, setAudioChecks] = useState<Record<string, DuplicateAudioVerification>>({})

  const copy = language === 'ru'
    ? {
        title: 'Дубликаты',
        subtitle: 'Сравните версии, оставьте нужную и безопасно уберите лишние файлы.',
        back: 'Назад к медиатеке',
        refresh: 'Пересчитать',
        search: 'Фильтр по исполнителю, названию, файлу, пути или ISRC…',
        all: 'Все',
        exact: 'Точные',
        filterPossible: 'Возможные',
        groups: 'Групп',
        files: 'Файлов в группах',
        exactGroups: 'Точных',
        possibleGroups: 'Возможных',
        none: 'Подходящих групп не найдено.',
        isrc: 'ISRC',
        metadata: 'Artist / Title',
        badgePossible: 'Возможный дубль',
        confidence: 'уверенность',
        spread: 'Разброс длительности',
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
        keepBest: 'Оставить лучший',
        keepBestHint: 'Выделить все файлы группы, кроме рекомендованного. Ничего не удаляется автоматически.',
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
        keepOneSafety: 'Backend дополнительно проверит, что в каждой группе останется хотя бы один файл.',
        selectTrack: 'Выбрать файл для действия',
        verifyAudio: 'Проверить аудио',
        verifyAudioUnavailable: 'Для проверки аудио требуется FFmpeg.',
        verifyAudioHint: 'FFmpeg декодирует файлы в низкочастотный mono PCM и сравнивает форму сигнала. Это дополнительная эвристика, а не доказательство идентичности.',
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
        subtitle: 'Compare versions, keep the one you want and safely remove extra files.',
        back: 'Back to library',
        refresh: 'Recalculate',
        search: 'Filter by artist, title, file, path or ISRC…',
        all: 'All',
        exact: 'Exact',
        filterPossible: 'Possible',
        groups: 'Groups',
        files: 'Files in groups',
        exactGroups: 'Exact',
        possibleGroups: 'Possible',
        none: 'No matching duplicate groups.',
        isrc: 'ISRC',
        metadata: 'Artist / Title',
        badgePossible: 'Possible duplicate',
        confidence: 'confidence',
        spread: 'Duration spread',
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
        keepBest: 'Keep best',
        keepBestHint: 'Select every file in this group except the recommended one. Nothing is removed automatically.',
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
        keepOneSafety: 'The backend also verifies that at least one file remains in every affected group.',
        selectTrack: 'Select file for action',
        verifyAudio: 'Check audio',
        verifyAudioUnavailable: 'FFmpeg is required for audio verification.',
        verifyAudioHint: 'FFmpeg decodes files to low-rate mono PCM and compares waveform features. This is an additional heuristic, not proof of identity.',
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

  useEffect(() => {
    setOpenGroups(new Set(groups.slice(0, 2).map((group) => group.key)))
    setSelectedIDs(new Set())
    setPendingDelete(null)
    setDeleteConfirm('')
    setAudioChecks({})
  }, [groups])

  const filtered = useMemo(() => {
    const needle = query.trim().toLocaleLowerCase()

    return groups.filter((group) => {
      if (filter === 'exact' && group.matchClass === 'possible') return false
      if (filter === 'possible' && group.matchClass !== 'possible') return false
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
  }, [groups, filter, query])

  const exactCount = groups.filter((group) => group.matchClass !== 'possible').length
  const possibleCount = groups.length - exactCount

  function toggleGroup(key: string) {
    setOpenGroups((current) => {
      const next = new Set(current)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
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
    if (!result) return
    setAudioChecks((current) => ({...current, [group.key]: result}))
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

  return (
    <>
      <section className="duplicate-workspace">
        <header className="duplicate-commandbar">
          <div>
            <h2>{copy.title}</h2>
            <span>{copy.subtitle}</span>
          </div>
          <div className="duplicate-command-actions">
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
            {(['all', 'exact', 'possible'] as FilterMode[]).map((mode) => (
              <button
                type="button"
                key={mode}
                className={filter === mode ? 'active' : ''}
                onClick={() => setFilter(mode)}
              >
                {mode === 'all' ? copy.all : mode === 'exact' ? copy.exact : copy.filterPossible}
                <b>
                  {mode === 'all' ? groups.length : mode === 'exact' ? exactCount : possibleCount}
                </b>
              </button>
            ))}
          </div>
        </div>

        <div className="duplicate-groups-list">
          {filtered.length === 0 && <div className="duplicate-empty">{copy.none}</div>}

          {filtered.map((group) => {
            const open = openGroups.has(group.key)
            const scores = qualityMap(group)
            const verification = audioChecks[group.key]
            const audioByTrack = new Map(verification?.comparisons.map((item) => [item.trackId, item]) ?? [])
            const groupSelected = selectedTracks(group)
            const sortedTracks = [...group.tracks].sort((left, right) => {
              const leftScore = scores.get(left.id)
              const rightScore = scores.get(right.id)
              return (rightScore?.score ?? 0) - (leftScore?.score ?? 0)
                || (rightScore?.audioScore ?? 0) - (leftScore?.audioScore ?? 0)
                || left.fileName.localeCompare(right.fileName)
            })

            return (
              <article className={`duplicate-group-card ${group.matchClass}`} key={group.key}>
                <button
                  type="button"
                  className="duplicate-group-head"
                  aria-expanded={open}
                  onClick={() => toggleGroup(group.key)}
                >
                  <span className="duplicate-disclosure" aria-hidden="true">{open ? '⌄' : '›'}</span>
                  <span className={`duplicate-match-badge ${group.matchClass}`}>{badge(group)}</span>
                  <span className="duplicate-group-title">
                    <strong>{group.artist || '—'} — {group.title || '—'}</strong>
                    <small>
                      {group.tracks.length} · {Math.round(group.confidence * 100)}% {copy.confidence}
                      {' · '}{copy.spread}: {formatSpread(group.durationSpreadMs)}
                      {group.sharedIsrc ? ` · ${copy.sharedIsrc}: ${group.sharedIsrc}` : ''}
                      {' · '}{group.recommendedTrackId ? copy.best : copy.tie}
                    </small>
                  </span>
                </button>

                {open && (
                  <div className="duplicate-group-body">
                    <div className="duplicate-group-actions">
                      <span>{copy.selected(groupSelected.length, group.tracks.length)}</span>
                      <button
                        type="button"
                        title={group.recommendedTrackId ? copy.keepBestHint : copy.noLeader}
                        disabled={busy || !group.recommendedTrackId}
                        onClick={() => selectAllExceptRecommended(group)}
                      >
                        {copy.keepBest}
                      </button>
                      <button
                        type="button"
                        disabled={busy || groupSelected.length === 0}
                        onClick={() => clearGroupSelection(group)}
                      >
                        {copy.clear}
                      </button>
                      <button
                        type="button"
                        className="duplicate-verify-audio"
                        title={canVerifyAudio ? copy.verifyAudioHint : copy.verifyAudioUnavailable}
                        disabled={busy || !canVerifyAudio}
                        onClick={() => void verifyGroupAudio(group)}
                      >
                        {copy.verifyAudio}
                      </button>
                      <button
                        type="button"
                        disabled={busy || groupSelected.length === 0}
                        onClick={() => void quarantineGroup(group)}
                      >
                        {copy.quarantine}
                      </button>
                      <button
                        type="button"
                        className="duplicate-action-danger"
                        disabled={busy || groupSelected.length === 0}
                        onClick={() => requestDelete(group)}
                      >
                        {copy.delete}
                      </button>
                    </div>

                    {verification && (
                      <div className="duplicate-audio-summary" title={copy.verifyAudioHint}>
                        <strong>{copy.audioSummary(
                          verification.sameCount,
                          verification.similarCount,
                          verification.differentCount,
                          verification.errorCount,
                        )}</strong>
                        <span>{copy.audioReference}: {group.tracks.find((track) => track.id === verification.referenceTrackId)?.fileName || verification.referenceTrackId}</span>
                      </div>
                    )}

                    <div className="duplicate-reasons">
                      {group.reasons.map((reason) => (
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

                      {sortedTracks.map((track) => {
                        const quality = scores.get(track.id)
                        const recommended = group.recommendedTrackId === track.id
                        const selected = selectedIDs.has(track.id)

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
                              {audioByTrack.get(track.id) && (
                                <em
                                  className={`duplicate-audio-badge ${audioByTrack.get(track.id)?.status || ''}`}
                                  title={audioComparisonTitle(verification, track.id)}
                                >
                                  {audioStatusLabel(audioByTrack.get(track.id)?.status || '')}
                                  {audioByTrack.get(track.id)?.status !== 'reference' && audioByTrack.get(track.id)?.status !== 'error'
                                    ? ` ${Math.round((audioByTrack.get(track.id)?.similarity || 0) * 100)}%`
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
                )}
              </article>
            )
          })}
        </div>
      </section>

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
