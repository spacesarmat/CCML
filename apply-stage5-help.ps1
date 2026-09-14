$ErrorActionPreference = "Stop"

$ProjectRoot = "C:\Users\ANDYBUM\GolandProjects\CCML"
$AppPath = Join-Path $ProjectRoot "frontend\src\App.tsx"
$CssPath = Join-Path $ProjectRoot "frontend\src\styles.css"
$HelpTarget = Join-Path $ProjectRoot "frontend\src\HelpModal.tsx"
$HelpPayload = Join-Path $PSScriptRoot "CCML-stage5-HelpModal.tsx"

if (-not (Test-Path $AppPath)) { throw "App.tsx not found: $AppPath" }
if (-not (Test-Path $CssPath)) { throw "styles.css not found: $CssPath" }
if (-not (Test-Path $HelpPayload)) { throw "Help payload not found: $HelpPayload" }

$Utf8NoBom = New-Object System.Text.UTF8Encoding($false)
$app = [System.IO.File]::ReadAllText($AppPath)
$css = [System.IO.File]::ReadAllText($CssPath)

$markers = @(
    "import HelpModal from './HelpModal'",
    "const [helpOpen, setHelpOpen] = useState(false)",
    "<HelpModal language={language}",
    ".help-modal"
)
$markerCount = 0
foreach ($marker in $markers) {
    if ($app.Contains($marker) -or $css.Contains($marker)) { $markerCount++ }
}
if ($markerCount -eq $markers.Count -and (Test-Path $HelpTarget)) {
    Write-Host "Stage 5 Help is already installed."
    exit 0
}
if ($markerCount -gt 0 -or (Test-Path $HelpTarget)) {
    throw "Partial Stage 5 Help installation detected. No files were changed."
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
        throw "Stage 5 anchor not found: $Label. No files were changed."
    }
    $second = $Text.IndexOf($Old, $first + $Old.Length, [System.StringComparison]::Ordinal)
    if ($second -ge 0) {
        throw "Stage 5 anchor is ambiguous: $Label. No files were changed."
    }
    return $Text.Substring(0, $first) + $New + $Text.Substring($first + $Old.Length)
}

$app = Replace-ExactlyOnce $app @'
import JobsPanel from './JobsPanel'
'@ @'
import JobsPanel from './JobsPanel'
import HelpModal from './HelpModal'
'@ "HelpModal import"

$app = Replace-ExactlyOnce $app @'
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [jobsOpen, setJobsOpen] = useState(false)
'@ @'
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [jobsOpen, setJobsOpen] = useState(false)
  const [helpOpen, setHelpOpen] = useState(false)
'@ "Help state"

$app = Replace-ExactlyOnce $app @'
  useEffect(() => {
    document.documentElement.lang = language
    saveLanguage(language)
  }, [language])
'@ @'
  useEffect(() => {
    document.documentElement.lang = language
    saveLanguage(language)
  }, [language])

  useEffect(() => {
    const onHelpKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'F1') return
      event.preventDefault()
      setHelpOpen(true)
    }
    window.addEventListener('keydown', onHelpKeyDown)
    return () => window.removeEventListener('keydown', onHelpKeyDown)
  }, [])
'@ "F1 help shortcut"

# If Stage 4 keyboard navigation is installed, prevent it from operating behind Help.
if ($app.Contains("const [activeTrackID, setActiveTrackID]")) {
    $oldCondition = "if (settingsOpen || jobsOpen || mainView !== 'library') return"
    $newCondition = "if (settingsOpen || jobsOpen || helpOpen || mainView !== 'library') return"
    $conditionCount = ([regex]::Matches($app, [regex]::Escape($oldCondition))).Count
    if ($conditionCount -ne 2) {
        throw "Stage 5 expected two Stage 4 modal guards but found $conditionCount. No files were changed."
    }
    $app = $app.Replace($oldCondition, $newCondition)

    $app = Replace-ExactlyOnce $app @'
    jobsOpen,
    mainView,
'@ @'
    jobsOpen,
    helpOpen,
    mainView,
'@ "Stage 4 Help dependency"
}

$app = Replace-ExactlyOnce $app @'
          <button className="top-action" onClick={() => setJobsOpen(true)}>{t('jobs.button')}</button>
          <button className="top-action" onClick={() => setSettingsOpen(true)} disabled={busy || scanning}>{t('settings.button')}</button>
'@ @'
          <button className="top-action" onClick={() => setJobsOpen(true)}>{t('jobs.button')}</button>
          <button className="top-action" onClick={() => setSettingsOpen(true)} disabled={busy || scanning}>{t('settings.button')}</button>
          <button className="top-action" onClick={() => setHelpOpen(true)} title={language === 'ru' ? 'Справка (F1)' : 'Help (F1)'}>{language === 'ru' ? 'Справка' : 'Help'}</button>
'@ "Help top-bar button"

$app = Replace-ExactlyOnce $app @'
      <JobsPanel language={language} open={jobsOpen} onClose={() => setJobsOpen(false)} onMessage={setMessage} />
      <SettingsModal language={language} open={settingsOpen} status={status} onClose={() => setSettingsOpen(false)} onSaved={async () => { await refreshStatus() }} onMessage={setMessage} />
'@ @'
      <HelpModal language={language} open={helpOpen} version={t('footer.version')} onClose={() => setHelpOpen(false)} />
      <JobsPanel language={language} open={jobsOpen} onClose={() => setJobsOpen(false)} onMessage={setMessage} />
      <SettingsModal language={language} open={settingsOpen} status={status} onClose={() => setSettingsOpen(false)} onSaved={async () => { await refreshStatus() }} onMessage={setMessage} />
'@ "Help modal mount"

$helpCss = @'

/* Built-in CCML Help. */
.help-modal { width: min(1040px, calc(100vw - 48px)); }
.help-header span { display: block; margin-top: 3px; }
.help-search-row { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 7px; padding: 11px 14px; border-bottom: 1px solid #263142; background: #0d141e; }
.help-search-row input { width: 100%; min-width: 0; }
.help-search-row button { width: 34px; padding: 0; }
.help-layout { min-height: 0; display: grid; grid-template-columns: 190px minmax(0, 1fr); overflow: hidden; }
.help-nav { min-height: 0; overflow: auto; padding: 12px 9px; border-right: 1px solid #263142; background: #0d131c; }
.help-nav > strong { display: block; padding: 3px 7px 8px; color: #68788f; font-size: 8px; text-transform: uppercase; letter-spacing: .09em; }
.help-nav button { width: 100%; padding: 7px 8px; border-color: transparent; background: transparent; color: #9dacbe; font-size: 9px; text-align: left; }
.help-nav button:hover { border-color: #2c3a4e; background: #151f2c; color: #d7e1ed; }
.help-content { min-height: 0; overflow: auto; scroll-behavior: smooth; padding: 14px; }
.help-empty { padding: 30px 14px; color: #7e8da2; text-align: center; }
.help-section { scroll-margin-top: 12px; padding: 14px; border: 1px solid #273548; border-radius: 10px; background: #0f1722; }
.help-section + .help-section { margin-top: 10px; }
.help-section header { margin-bottom: 10px; }
.help-section h3 { margin: 0; color: #d9e2ee; font-size: 14px; }
.help-section header p { margin: 4px 0 0; color: #7d8da3; font-size: 10px; line-height: 1.45; }
.help-section ul { margin: 8px 0 0; padding-left: 18px; color: #a5b2c3; font-size: 10px; line-height: 1.55; }
.help-section li + li { margin-top: 5px; }
.help-rows { display: grid; gap: 5px; }
.help-row { display: grid; grid-template-columns: minmax(150px, .55fr) minmax(0, 1.45fr); gap: 10px; align-items: start; padding: 7px 8px; border: 1px solid #202d3e; border-radius: 7px; background: #0b121b; }
.help-row-command { display: flex; flex-wrap: wrap; align-items: center; gap: 4px; }
.help-row kbd { min-width: 24px; padding: 3px 6px; border: 1px solid #3a4a60; border-bottom-color: #536680; border-radius: 5px; background: #172231; color: #d5e0ed; font: 600 9px/1.2 ui-monospace, SFMono-Regular, Consolas, monospace; text-align: center; box-shadow: inset 0 -1px 0 rgba(255,255,255,.05); }
.help-row strong { display: block; color: #b9c7d8; font-size: 10px; line-height: 1.4; }
.help-row span { display: block; margin-top: 2px; color: #788ba3; font-size: 9px; line-height: 1.45; }
.help-footer { justify-content: space-between; }
.help-footer > span { color: #65768d; font-size: 8px; }

@media (max-width: 820px) {
  .help-layout { grid-template-columns: 1fr; }
  .help-nav { display: none; }
  .help-row { grid-template-columns: 1fr; }
}
'@

$css += $helpCss

# Validation has completed. Only now write project files.
Copy-Item $AppPath "$AppPath.stage5help.bak" -Force
Copy-Item $CssPath "$CssPath.stage5help.bak" -Force
if (Test-Path $HelpTarget) { Copy-Item $HelpTarget "$HelpTarget.stage5help.bak" -Force }

[System.IO.File]::WriteAllText($AppPath, $app, $Utf8NoBom)
[System.IO.File]::WriteAllText($CssPath, $css, $Utf8NoBom)
[System.IO.File]::WriteAllText($HelpTarget, [System.IO.File]::ReadAllText($HelpPayload), $Utf8NoBom)

Write-Host "Stage 5 built-in Help installed successfully."
Write-Host "Open Help from the top bar or press F1."
