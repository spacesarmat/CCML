import {useEffect, useMemo, useState} from 'react'
import type {AppLanguage} from './i18n'
import type {DuplicateGroup, DuplicateTrackQuality, Track} from './types'

type Props = {
  language: AppLanguage
  groups: DuplicateGroup[]
  busy: boolean
  onBack: () => void
  onRefresh: () => void
  onOpenTrack: (track: Track) => void
}

type FilterMode = 'all' | 'exact' | 'possible'

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

function DuplicateGroupsView({language, groups, busy, onBack, onRefresh, onOpenTrack}: Props) {
  const [filter, setFilter] = useState<FilterMode>('all')
  const [query, setQuery] = useState('')
  const [openGroups, setOpenGroups] = useState<Set<string>>(new Set())

  const copy = language === 'ru'
    ? {
        title: 'Дубликаты',
        subtitle: 'Группы строятся по ISRC, точным Artist/Title и возможным версиям с близкой длительностью.',
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
        subtitle: 'Groups use ISRC, exact Artist/Title and possible version-normalized matches with close duration.',
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

  return (
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
                  <div className="duplicate-reasons">
                    {group.reasons.map((reason) => (
                      <span key={reason}>{copy.reasons[reason] || reason}</span>
                    ))}
                  </div>

                  <div className="duplicate-compare-table quality-enabled">
                    <div className="duplicate-compare-row head">
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

                      return (
                        <div className={`duplicate-compare-row${recommended ? ' recommended' : ''}`} key={track.id}>
                          <span className="duplicate-file-cell" title={track.path}>
                            <strong>{track.fileName}</strong>
                            <small>{track.path}</small>
                            {recommended && <em>{copy.best}</em>}
                          </span>
                          <span className="duplicate-quality-cell" title={qualityTitle(quality)}>
                            <strong>{quality?.score ?? 0}</strong>
                            <small>{copy.audio} {quality?.audioScore ?? 0} · {copy.metadataScore} {quality?.metadataScore ?? 0}</small>
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
  )
}

export default DuplicateGroupsView
