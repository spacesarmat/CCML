$ErrorActionPreference = "Stop"

$ProjectRoot = "C:\Users\ANDYBUM\GolandProjects\CCML"
$AppPath = Join-Path $ProjectRoot "frontend\src\App.tsx"
$CssPath = Join-Path $ProjectRoot "frontend\src\styles.css"

if (-not (Test-Path $AppPath)) { throw "App.tsx not found: $AppPath" }
if (-not (Test-Path $CssPath)) { throw "styles.css not found: $CssPath" }

$Utf8NoBom = New-Object System.Text.UTF8Encoding($false)
$app = [System.IO.File]::ReadAllText($AppPath)
$css = [System.IO.File]::ReadAllText($CssPath)

$appMarker = "const [activeTrackID, setActiveTrackID] = useState<number | null>(null)"
$cssMarker = ".workspace-table tbody tr.active-row"

if ($app.Contains($appMarker) -and $css.Contains($cssMarker)) {
    Write-Host "Stage 4 v2 is already applied."
    exit 0
}
if ($app.Contains($appMarker) -xor $css.Contains($cssMarker)) {
    throw "Partial Stage 4 state detected. Restore the frontend files or send App.tsx/styles.css for repair."
}

function Replace-ExactlyOnce {
    param(
        [string]$Text,
        [string]$Old,
        [string]$New,
        [string]$Label
    )
    $first = $Text.IndexOf($Old, [System.StringComparison]::Ordinal)
    if ($first -lt 0) {
        throw "Stage 4 v2 anchor not found: $Label. No files were changed."
    }
    $second = $Text.IndexOf($Old, $first + $Old.Length, [System.StringComparison]::Ordinal)
    if ($second -ge 0) {
        throw "Stage 4 v2 anchor is ambiguous: $Label. No files were changed."
    }
    return $Text.Substring(0, $first) + $New + $Text.Substring($first + $Old.Length)
}

$old = @'
  const [tracks, setTracks] = useState<Track[]>([])
  const [selectedIDs, setSelectedIDs] = useState<number[]>([])
  const [tagRevision, setTagRevision] = useState(0)
'@
$new = @'
  const [tracks, setTracks] = useState<Track[]>([])
  const [selectedIDs, setSelectedIDs] = useState<number[]>([])
  const [activeTrackID, setActiveTrackID] = useState<number | null>(null)
  const [tagRevision, setTagRevision] = useState(0)
'@
$app = Replace-ExactlyOnce $app $old $new "active track state"

$old = @'
  const mediaRequest = useRef(0)
  const mediaFallbackTried = useRef(false)
  const audioRef = useRef<HTMLAudioElement | null>(null)
  const spectrogramRequest = useRef(0)
'@
$new = @'
  const mediaRequest = useRef(0)
  const mediaFallbackTried = useRef(false)
  const audioRef = useRef<HTMLAudioElement | null>(null)
  const searchInputRef = useRef<HTMLInputElement | null>(null)
  const selectionAnchor = useRef<number | null>(null)
  const spectrogramRequest = useRef(0)
'@
$app = Replace-ExactlyOnce $app $old $new "keyboard refs"

$old = @'
    if (track.lastMetadataJobStatus === 'failed') classes.push('metadata-failed')
    if (selectedIDs.includes(track.id)) classes.push('selected')
    return classes.join(' ')
'@
$new = @'
    if (track.lastMetadataJobStatus === 'failed') classes.push('metadata-failed')
    if (selectedIDs.includes(track.id)) classes.push('selected')
    if (activeTrackID === track.id) classes.push('active-row')
    return classes.join(' ')
'@
$app = Replace-ExactlyOnce $app $old $new "active row class"

$old = @'
  useEffect(() => {
    document.documentElement.lang = language
    saveLanguage(language)
  }, [language])

  useEffect(() => {
    setMetadata([])
'@
$new = @'
  useEffect(() => {
    document.documentElement.lang = language
    saveLanguage(language)
  }, [language])

  useEffect(() => {
    if (activeTrackID === null || mainView !== 'library') return
    const frame = window.requestAnimationFrame(() => {
      const row = document.querySelector<HTMLTableRowElement>(`tr[data-track-id="${activeTrackID}"]`)
      row?.scrollIntoView({block: 'nearest'})
    })
    return () => window.cancelAnimationFrame(frame)
  }, [activeTrackID, mainView])

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const command = event.ctrlKey || event.metaKey
      const key = event.key
      const target = event.target as HTMLElement | null

      if (command && key.toLowerCase() === 'f') {
        if (settingsOpen || jobsOpen || mainView !== 'library') return
        event.preventDefault()
        searchInputRef.current?.focus()
        searchInputRef.current?.select()
        return
      }

      if (settingsOpen || jobsOpen || mainView !== 'library') return
      if (target?.closest('input, textarea, select, button, a, [contenteditable="true"], [role="slider"]')) return

      if (command && key.toLowerCase() === 'a') {
        event.preventDefault()
        selectAllVisibleTracks()
        return
      }

      switch (key) {
        case 'ArrowDown':
          event.preventDefault()
          moveActiveTrack(1, event.shiftKey, command)
          break
        case 'ArrowUp':
          event.preventDefault()
          moveActiveTrack(-1, event.shiftKey, command)
          break
        case 'PageDown':
          event.preventDefault()
          moveActiveTrack(10, event.shiftKey, command)
          break
        case 'PageUp':
          event.preventDefault()
          moveActiveTrack(-10, event.shiftKey, command)
          break
        case 'Home':
          event.preventDefault()
          moveActiveTrackTo(0, event.shiftKey, command)
          break
        case 'End':
          event.preventDefault()
          moveActiveTrackTo(filteredTracks.length - 1, event.shiftKey, command)
          break
        case 'Enter':
          if (activeTrackID !== null && filteredTracks.some((track) => track.id === activeTrackID)) {
            event.preventDefault()
            selectOnlyTrack(activeTrackID)
            setInspectorTab('tags')
          }
          break
        case ' ':
          if (selectedIDs.length === 1 && trackMedia?.audioUrl && !mediaLoading && !mediaFallbackLoading) {
            event.preventDefault()
            void toggleAudioPlayback()
          }
          break
        case 'Escape':
          if (selectedIDs.length > 0 || activeTrackID !== null) {
            event.preventDefault()
            clearTrackSelection()
          }
          break
      }
    }

    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [
    activeTrackID,
    filteredTracks,
    jobsOpen,
    mainView,
    mediaFallbackLoading,
    mediaLoading,
    selectedIDs,
    settingsOpen,
    trackMedia?.audioUrl,
  ])

  useEffect(() => {
    setMetadata([])
'@
$app = Replace-ExactlyOnce $app $old $new "keyboard effects"

$old = @'
      setSelectedIDs((current) => current.filter((id) => visibleIDs.has(id)))
      setMessage(t('message.tracksLoaded', {count: result.length}))
'@
$new = @'
      setSelectedIDs((current) => current.filter((id) => visibleIDs.has(id)))
      setActiveTrackID((current) => current !== null && visibleIDs.has(current) ? current : null)
      setMessage(t('message.tracksLoaded', {count: result.length}))
'@
$app = Replace-ExactlyOnce $app $old $new "refresh active row"

$old = @'
  function toggleTrackSelection(trackID: number) {
    setSelectedIDs((current) => current.includes(trackID)
      ? current.filter((id) => id !== trackID)
      : [...current, trackID])
  }

  function selectOnlyTrack(trackID: number) {
    setSelectedIDs([trackID])
  }

  function changeMetadataFilter(next: 'all' | 'skipped' | 'failed') {
    setMainView('library')
    setMetadataFilter(next)
    setSelectedIDs([])
  }

  function toggleAllVisible() {
    if (filteredTracks.length > 0 && filteredTracks.every((track) => selectedIDs.includes(track.id))) {
      setSelectedIDs([])
      return
    }
    setSelectedIDs(filteredTracks.map((track) => track.id))
  }
'@
$new = @'
  function clearTrackSelection() {
    setSelectedIDs([])
    setActiveTrackID(null)
    selectionAnchor.current = null
  }

  function toggleTrackSelection(trackID: number) {
    setActiveTrackID(trackID)
    selectionAnchor.current = trackID
    setSelectedIDs((current) => current.includes(trackID)
      ? current.filter((id) => id !== trackID)
      : [...current, trackID])
  }

  function selectOnlyTrack(trackID: number) {
    setActiveTrackID(trackID)
    selectionAnchor.current = trackID
    setSelectedIDs([trackID])
  }

  function selectRangeTo(trackID: number) {
    const targetIndex = filteredTracks.findIndex((track) => track.id === trackID)
    if (targetIndex < 0) return

    const anchorID = selectionAnchor.current ?? activeTrackID ?? selectedIDs[0] ?? trackID
    let anchorIndex = filteredTracks.findIndex((track) => track.id === anchorID)
    if (anchorIndex < 0) anchorIndex = targetIndex

    const start = Math.min(anchorIndex, targetIndex)
    const end = Math.max(anchorIndex, targetIndex)
    setSelectedIDs(filteredTracks.slice(start, end + 1).map((track) => track.id))
    setActiveTrackID(trackID)
    if (selectionAnchor.current === null) selectionAnchor.current = anchorID
  }

  function handleTrackRowClick(trackID: number, extendSelection: boolean, toggleSelection: boolean) {
    if (extendSelection) {
      selectRangeTo(trackID)
      return
    }
    if (toggleSelection) {
      toggleTrackSelection(trackID)
      return
    }
    selectOnlyTrack(trackID)
  }

  function moveActiveTrack(delta: number, extendSelection: boolean, preserveSelection: boolean) {
    if (filteredTracks.length === 0) return
    let index = filteredTracks.findIndex((track) => track.id === activeTrackID)
    if (index < 0) index = filteredTracks.findIndex((track) => selectedIDs.includes(track.id))
    if (index < 0) index = delta > 0 ? -1 : 0
    moveActiveTrackTo(index + delta, extendSelection, preserveSelection)
  }

  function moveActiveTrackTo(index: number, extendSelection: boolean, preserveSelection: boolean) {
    if (filteredTracks.length === 0) return
    const clamped = Math.max(0, Math.min(index, filteredTracks.length - 1))
    const trackID = filteredTracks[clamped].id

    if (extendSelection) {
      selectRangeTo(trackID)
      return
    }

    setActiveTrackID(trackID)
    if (!preserveSelection) {
      selectionAnchor.current = trackID
      setSelectedIDs([trackID])
    }
  }

  function selectAllVisibleTracks() {
    if (filteredTracks.length === 0) return
    const ids = filteredTracks.map((track) => track.id)
    setSelectedIDs(ids)
    const active = activeTrackID !== null && ids.includes(activeTrackID) ? activeTrackID : ids[0]
    setActiveTrackID(active)
    selectionAnchor.current = active
  }

  function changeMetadataFilter(next: 'all' | 'skipped' | 'failed') {
    setMainView('library')
    setMetadataFilter(next)
    clearTrackSelection()
  }

  function toggleAllVisible() {
    if (filteredTracks.length > 0 && filteredTracks.every((track) => selectedIDs.includes(track.id))) {
      clearTrackSelection()
      return
    }
    selectAllVisibleTracks()
  }
'@
$app = Replace-ExactlyOnce $app $old $new "selection functions"

$old = @'
                  <input value={search} onChange={(event) => setSearch(event.target.value)} onKeyDown={(event) => event.key === 'Enter' && void refreshTracks()} placeholder={t('library.searchPlaceholder')} />
'@
$new = @'
                  <input ref={searchInputRef} value={search} onChange={(event) => setSearch(event.target.value)} onKeyDown={(event) => event.key === 'Enter' && void refreshTracks()} placeholder={t('library.searchPlaceholder')} />
'@
$app = Replace-ExactlyOnce $app $old $new "search ref"

$old = @'
                  <button onClick={() => setSelectedIDs([])}>{t('workspace.clearSelection')}</button>
'@
$new = @'
                  <button onClick={clearTrackSelection}>{t('workspace.clearSelection')}</button>
'@
$app = Replace-ExactlyOnce $app $old $new "clear selection button"

$old = @'
                    {filteredTracks.map((track) => (
                      <tr key={track.id} className={metadataRowClass(track)} title={metadataRowTitle(track)} onClick={() => selectOnlyTrack(track.id)}>
                        <td className="selection-col" onClick={(event: { stopPropagation(): void }) => event.stopPropagation()}><input type="checkbox" checked={selectedIDs.includes(track.id)} aria-label={t('library.selectTrack', {title: track.title || track.fileName})} onChange={() => toggleTrackSelection(track.id)} /></td>
'@
$new = @'
                    {filteredTracks.map((track) => (
                      <tr
                        key={track.id}
                        data-track-id={track.id}
                        className={metadataRowClass(track)}
                        title={metadataRowTitle(track)}
                        aria-current={activeTrackID === track.id ? 'true' : undefined}
                        onClick={(event) => handleTrackRowClick(track.id, event.shiftKey, event.ctrlKey || event.metaKey)}
                      >
                        <td className="selection-col" onClick={(event: { stopPropagation(): void }) => event.stopPropagation()}><input type="checkbox" checked={selectedIDs.includes(track.id)} aria-label={t('library.selectTrack', {title: track.title || track.fileName})} onChange={() => toggleTrackSelection(track.id)} /></td>
'@
$app = Replace-ExactlyOnce $app $old $new "table row mouse selection"

$old = @'
.workspace-table tbody tr:hover { background: #121c28; }
.workspace-table tbody tr.selected { background: #1b2940; }
.workspace-table tbody tr.metadata-unchanged { box-shadow: inset 3px 0 0 #b78a32; background: rgba(167,126,47,.08); }
'@
$new = @'
.workspace-table tbody tr:hover { background: #121c28; }
.workspace-table tbody tr.selected { background: #1b2940; }
.workspace-table tbody tr.active-row { outline: 1px solid #6b86ff; outline-offset: -1px; }
.workspace-table tbody tr.active-row.selected { background: #213453; }
.workspace-table tbody tr.metadata-unchanged { box-shadow: inset 3px 0 0 #b78a32; background: rgba(167,126,47,.08); }
'@
$css = Replace-ExactlyOnce $css $old $new "active row CSS"

# All validation/replacements succeeded in memory. Only now touch disk.
Copy-Item $AppPath "$AppPath.stage4v2.bak" -Force
Copy-Item $CssPath "$CssPath.stage4v2.bak" -Force

[System.IO.File]::WriteAllText($AppPath, $app, $Utf8NoBom)
[System.IO.File]::WriteAllText($CssPath, $css, $Utf8NoBom)

Write-Host "Stage 4 v2 keyboard navigation applied successfully."
Write-Host "Backups:"
Write-Host "  $AppPath.stage4v2.bak"
Write-Host "  $CssPath.stage4v2.bak"
