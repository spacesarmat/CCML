import {useEffect, useRef, useState} from 'react'
import type {Track} from './types'

type Props = {
  track: Track
  revision: number
}

const MAX_CONCURRENT_COVERS = 4

const resolvedCoverURLs = new Map<string, string>()
const pendingCoverURLs = new Map<string, Promise<string>>()
const coverQueue: Array<() => Promise<void>> = []
let activeCoverRequests = 0

function cacheKey(track: Track, revision: number): string {
  return [
    track.id,
    track.modifiedUnix,
    track.lastMetadataJobUpdatedAt || '',
    revision,
  ].join(':')
}

function pumpCoverQueue(): void {
  while (activeCoverRequests < MAX_CONCURRENT_COVERS && coverQueue.length > 0) {
    const task = coverQueue.shift()
    if (!task) return

    activeCoverRequests += 1
    void task().finally(() => {
      activeCoverRequests -= 1
      pumpCoverQueue()
    })
  }
}

function loadCoverURL(key: string, trackID: number): Promise<string> {
  const resolved = resolvedCoverURLs.get(key)
  if (resolved !== undefined) return Promise.resolve(resolved)

  const pending = pendingCoverURLs.get(key)
  if (pending) return pending

  const request = new Promise<string>((resolve) => {
    coverQueue.push(async () => {
      let url = ''
      try {
        url = (await window.go.main.App.PrepareTrackCover(trackID)) || ''
      } catch {
        // Missing/corrupt artwork should not turn the Library table into an error UI.
        url = ''
      }

      resolvedCoverURLs.set(key, url)
      pendingCoverURLs.delete(key)
      resolve(url)
    })
    pumpCoverQueue()
  })

  pendingCoverURLs.set(key, request)
  return request
}

function TrackCoverCell({track, revision}: Props) {
  const hostRef = useRef<HTMLDivElement | null>(null)
  const key = cacheKey(track, revision)
  const [visible, setVisible] = useState(false)
  const [coverURL, setCoverURL] = useState<string | null>(() => resolvedCoverURLs.get(key) ?? null)

  useEffect(() => {
    setVisible(false)
    setCoverURL(resolvedCoverURLs.get(key) ?? null)
  }, [key])

  useEffect(() => {
    const node = hostRef.current
    if (!node) return

    if (typeof IntersectionObserver === 'undefined') {
      setVisible(true)
      return
    }

    const root = node.closest('.configurable-track-table')
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          setVisible(true)
          observer.disconnect()
        }
      },
      {root, threshold: 0.01},
    )

    observer.observe(node)
    return () => observer.disconnect()
  }, [key])

  useEffect(() => {
    if (!visible || coverURL !== null) return

    let current = true
    void loadCoverURL(key, track.id).then((url) => {
      if (current) setCoverURL(url)
    })

    return () => {
      current = false
    }
  }, [coverURL, key, track.id, visible])

  return (
    <div className="track-cover-thumb" ref={hostRef}>
      {coverURL === null ? (
        <span className="track-cover-loading" aria-hidden="true" />
      ) : coverURL ? (
        <img
          src={coverURL}
          alt=""
          loading="lazy"
          decoding="async"
          onError={() => {
            resolvedCoverURLs.set(key, '')
            setCoverURL('')
          }}
        />
      ) : (
        <span className="track-cover-placeholder" aria-hidden="true">♪</span>
      )}
    </div>
  )
}

export default TrackCoverCell
