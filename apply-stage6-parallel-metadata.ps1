$ErrorActionPreference = "Stop"

$ProjectRoot = "C:\Users\ANDYBUM\GolandProjects\CCML"
$ManagerPath = Join-Path $ProjectRoot "internal\jobs\manager.go"
$AppPath = Join-Path $ProjectRoot "app.go"
$HelpPath = Join-Path $ProjectRoot "frontend\src\HelpModal.tsx"
$ParallelPath = Join-Path $ProjectRoot "internal\jobs\parallel.go"
$ParallelTestPath = Join-Path $ProjectRoot "internal\jobs\parallel_test.go"
$ParallelPayload = Join-Path $PSScriptRoot "CCML-stage6-parallel_jobs.go"
$TestPayload = Join-Path $PSScriptRoot "CCML-stage6-parallel_jobs_test.go"

foreach ($path in @($ManagerPath, $AppPath, $ParallelPayload, $TestPayload)) {
    if (-not (Test-Path $path)) { throw "Required file not found: $path" }
}

$Utf8NoBom = New-Object System.Text.UTF8Encoding($false)
$manager = [System.IO.File]::ReadAllText($ManagerPath)
$app = [System.IO.File]::ReadAllText($AppPath)
$help = if (Test-Path $HelpPath) { [System.IO.File]::ReadAllText($HelpPath) } else { $null }

if ($manager.Contains("RegisterConcurrent(") -and (Test-Path $ParallelPath)) {
    Write-Host "Stage 6 parallel metadata enrichment is already installed."
    exit 0
}
if ($manager.Contains("RegisterConcurrent(") -or (Test-Path $ParallelPath) -or (Test-Path $ParallelTestPath)) {
    throw "Partial Stage 6 installation detected. No files were changed."
}

function Replace-RegexOnce {
    param(
        [string]$Text,
        [string]$Pattern,
        [string]$Replacement,
        [string]$Label,
        [System.Text.RegularExpressions.RegexOptions]$Options = [System.Text.RegularExpressions.RegexOptions]::Multiline
    )
    $rx = New-Object System.Text.RegularExpressions.Regex($Pattern, $Options)
    $matches = $rx.Matches($Text)
    if ($matches.Count -ne 1) {
        throw "Stage 6 expected exactly one match for $Label, found $($matches.Count). No files were changed."
    }
    $m = $matches[0]
    return $Text.Substring(0, $m.Index) + $Replacement + $Text.Substring($m.Index + $m.Length)
}

function Insert-AfterRegexOnce {
    param(
        [string]$Text,
        [string]$Pattern,
        [string]$Insert,
        [string]$Label
    )
    $rx = New-Object System.Text.RegularExpressions.Regex($Pattern, [System.Text.RegularExpressions.RegexOptions]::Multiline)
    $matches = $rx.Matches($Text)
    if ($matches.Count -ne 1) {
        throw "Stage 6 expected exactly one match for $Label, found $($matches.Count). No files were changed."
    }
    $m = $matches[0]
    return $Text.Substring(0, $m.Index) + $m.Value + $Insert + $Text.Substring($m.Index + $m.Length)
}

# Add per-job-type concurrency configuration.
$manager = Insert-AfterRegexOnce $manager '^[ \t]*runners[ \t]+map\[string\]Runner[ \t]*$' "`r`n`tconcurrency map[string]int" "Manager concurrency field"

$manager = Insert-AfterRegexOnce $manager '^[ \t]*runners:[ \t]*make\(map\[string\]Runner\),[ \t]*$' "`r`n`t`tconcurrency: make(map[string]int)," "Manager concurrency initialization"

$registerReplacement = @'
func (m *Manager) Register(jobType string, runner Runner) {
	m.RegisterConcurrent(jobType, 1, runner)
}

// RegisterConcurrent registers a runner with bounded per-job item concurrency.
// Existing callers keep serial semantics through Register.
func (m *Manager) RegisterConcurrent(jobType string, workers int, runner Runner) {
	if workers < 1 {
		workers = 1
	}
	if workers > 16 {
		workers = 16
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runners[jobType] = runner
	m.concurrency[jobType] = workers
}
'@
$manager = Replace-RegexOnce $manager '(?ms)^func \(m \*Manager\) Register\(jobType string, runner Runner\) \{.*?^\}' $registerReplacement "Manager Register method" ([System.Text.RegularExpressions.RegexOptions]::Multiline -bor [System.Text.RegularExpressions.RegexOptions]::Singleline)

# Branch to bounded parallel execution after the job has been marked running and emitted.
$branchNeedle = @'
	m.emit("jobs:updated", job)

	for root.Err() == nil {
'@
$branchReplacement = @'
	m.emit("jobs:updated", job)

	if workers := m.workerCount(job.Type); workers > 1 {
		return m.runJobConcurrent(root, job, runner, workers)
	}

	for root.Err() == nil {
'@
if (-not $manager.Contains($branchNeedle)) {
    throw "Stage 6 runJob branch anchor not found. No files were changed."
}
$manager = $manager.Replace($branchNeedle, $branchReplacement)

# Add workerCount next to runner().
$runnerNeedle = @'
func (m *Manager) runner(jobType string) Runner {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.runners[jobType]
}
'@
$runnerReplacement = @'
func (m *Manager) runner(jobType string) Runner {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.runners[jobType]
}

func (m *Manager) workerCount(jobType string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	workers := m.concurrency[jobType]
	if workers < 1 {
		return 1
	}
	return workers
}
'@
if (-not $manager.Contains($runnerNeedle)) {
    throw "Stage 6 workerCount anchor not found. No files were changed."
}
$manager = $manager.Replace($runnerNeedle, $runnerReplacement)

# parallel.go uses this small timeout unit to keep shutdown persistence readable.
$manager = Insert-AfterRegexOnce $manager '^const pollInterval = 2 \* time\.Second[ \t]*$' "`r`nconst backgroundPreserveTimeout = time.Second" "parallel preserve timeout"

# Only metadata enrichment opts into four workers.
$app = Replace-RegexOnce $app 'app\.jobs\.Register\("metadata_enrichment",[ \t]*app\.runMetadataJobItem\)' 'app.jobs.RegisterConcurrent("metadata_enrichment", 4, app.runMetadataJobItem)' "metadata enrichment registration"

# Best-effort built-in Help update.
if ($help -ne $null -and -not $help.Contains("до четырёх треков одновременно")) {
    $ru = "          'Задачи могут продолжаться независимо от текущей вкладки интерфейса.',"
    if ($help.Contains($ru)) {
        $help = $help.Replace($ru, $ru + "`r`n          'Дополнение метаданных обрабатывает до четырёх треков одновременно; внутренние ограничения провайдеров продолжают соблюдать их rate limits.',")
    }
    $en = "          'Supported work can continue independently of the currently visible screen.',"
    if ($help.Contains($en)) {
        $help = $help.Replace($en, $en + "`r`n          'Metadata enrichment processes up to four tracks concurrently while provider-specific rate limits remain enforced.',")
    }
}

# Everything validated in memory. Only now touch project files.
Copy-Item $ManagerPath "$ManagerPath.stage6.bak" -Force
Copy-Item $AppPath "$AppPath.stage6.bak" -Force
if ($help -ne $null) { Copy-Item $HelpPath "$HelpPath.stage6.bak" -Force }

[System.IO.File]::WriteAllText($ManagerPath, $manager, $Utf8NoBom)
[System.IO.File]::WriteAllText($AppPath, $app, $Utf8NoBom)
if ($help -ne $null) { [System.IO.File]::WriteAllText($HelpPath, $help, $Utf8NoBom) }
[System.IO.File]::WriteAllText($ParallelPath, [System.IO.File]::ReadAllText($ParallelPayload), $Utf8NoBom)
[System.IO.File]::WriteAllText($ParallelTestPath, [System.IO.File]::ReadAllText($TestPayload), $Utf8NoBom)

# Format only Go files touched by this stage.
& gofmt -w $ManagerPath $AppPath $ParallelPath $ParallelTestPath
if ($LASTEXITCODE -ne 0) {
    throw "gofmt failed after Stage 6 install. Backups with .stage6.bak are available."
}

Write-Host "Stage 6 parallel metadata enrichment installed successfully."
Write-Host "Metadata enrichment worker count: 4 tracks."
