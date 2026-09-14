$ErrorActionPreference = "Stop"

$ProjectRoot = "C:\Users\ANDYBUM\GolandProjects\CCML"
$ManagerPath = Join-Path $ProjectRoot "internal\jobs\manager.go"
$AppPath = Join-Path $ProjectRoot "app.go"
$HelpPath = Join-Path $ProjectRoot "frontend\src\HelpModal.tsx"
$ParallelPath = Join-Path $ProjectRoot "internal\jobs\parallel.go"
$ParallelTestPath = Join-Path $ProjectRoot "internal\jobs\parallel_test.go"
$ParallelPayload = Join-Path $PSScriptRoot "CCML-stage6-v2-parallel_jobs.payload.txt"
$TestPayload = Join-Path $PSScriptRoot "CCML-stage6-v2-parallel_jobs_test.payload.txt"

foreach ($path in @($ManagerPath, $AppPath, $ParallelPayload, $TestPayload)) {
    if (-not (Test-Path $path)) { throw "Required file not found: $path" }
}

# Remove packaging artifacts from older CCML hotfix archives. They are never
# real project source files and can otherwise make Go report mixed packages.
$stray = Get-ChildItem $ProjectRoot -File -Filter "CCML-stage*.go"
if ($stray) {
    Write-Host "Cleaning old stage .go payloads from project root:"
    $stray | ForEach-Object { Write-Host "  $($_.Name)" }
    $stray | Remove-Item -Force
}

$Utf8NoBom = New-Object System.Text.UTF8Encoding($false)
$manager = [System.IO.File]::ReadAllText($ManagerPath)
$app = [System.IO.File]::ReadAllText($AppPath)
$help = if (Test-Path $HelpPath) { [System.IO.File]::ReadAllText($HelpPath) } else { $null }

if ($manager.Contains("RegisterConcurrent(") -and (Test-Path $ParallelPath)) {
    Write-Host "Stage 6 v2 parallel metadata enrichment is already installed."
    exit 0
}
if ($manager.Contains("RegisterConcurrent(") -or (Test-Path $ParallelPath) -or (Test-Path $ParallelTestPath)) {
    throw "Partial Stage 6 state detected. No files were changed beyond safe root-payload cleanup."
}

function Find-OneRegex {
    param(
        [string]$Text,
        [string]$Pattern,
        [string]$Label,
        [System.Text.RegularExpressions.RegexOptions]$Options
    )
    $rx = New-Object System.Text.RegularExpressions.Regex($Pattern, $Options)
    $matches = $rx.Matches($Text)
    if ($matches.Count -ne 1) {
        throw "Stage 6 v2 expected exactly one match for $Label, found $($matches.Count). No project source files were changed."
    }
    return $matches[0]
}

function Replace-OneRegex {
    param(
        [string]$Text,
        [string]$Pattern,
        [string]$Replacement,
        [string]$Label,
        [System.Text.RegularExpressions.RegexOptions]$Options
    )
    $m = Find-OneRegex $Text $Pattern $Label $Options
    return $Text.Substring(0, $m.Index) + $Replacement + $Text.Substring($m.Index + $m.Length)
}

$multi = [System.Text.RegularExpressions.RegexOptions]::Multiline
$singleMulti = [System.Text.RegularExpressions.RegexOptions]::Multiline -bor [System.Text.RegularExpressions.RegexOptions]::Singleline

# Manager fields.
$m = Find-OneRegex $manager '^[ \t]*runners[ \t]+map\[string\]Runner[ \t]*$' "Manager runners field" $multi
$manager = $manager.Substring(0,$m.Index+$m.Length) + "`r`n`tconcurrency map[string]int" + $manager.Substring($m.Index+$m.Length)

# Manager constructor map init.
$m = Find-OneRegex $manager '^[ \t]*runners:[ \t]*make\(map\[string\]Runner\),[ \t]*$' "Manager runners init" $multi
$manager = $manager.Substring(0,$m.Index+$m.Length) + "`r`n`t`tconcurrency: make(map[string]int)," + $manager.Substring($m.Index+$m.Length)

# Replace Register regardless of local whitespace/comments inside its small body.
$registerReplacement = @'
func (m *Manager) Register(jobType string, runner Runner) {
	m.RegisterConcurrent(jobType, 1, runner)
}

// RegisterConcurrent registers a runner with bounded per-job item concurrency.
// Existing callers retain serial behavior through Register.
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
$manager = Replace-OneRegex $manager '(?ms)^func[ \t]+\(m[ \t]+\*Manager\)[ \t]+Register\(jobType[ \t]+string,[ \t]+runner[ \t]+Runner\)[ \t]*\{.*?^\}' $registerReplacement "Manager Register function" $singleMulti

# Find runJob as a bounded block ending immediately before runner().
$runJobMatch = Find-OneRegex $manager '(?ms)^func[ \t]+\(m[ \t]+\*Manager\)[ \t]+runJob\(root[ \t]+context\.Context,[ \t]+job[ \t]+model\.BackgroundJob\)[ \t]+error[ \t]*\{.*?^func[ \t]+\(m[ \t]+\*Manager\)[ \t]+runner\(' "runJob block" $singleMulti
$runBlock = $runJobMatch.Value
# Remove the beginning of runner() from the captured block before editing.
$runnerStart = $runBlock.LastIndexOf("func (m *Manager) runner(", [System.StringComparison]::Ordinal)
if ($runnerStart -lt 0) {
    throw "Stage 6 v2 could not isolate runner() after runJob. No project source files were changed."
}
$runOnly = $runBlock.Substring(0, $runnerStart)
$runnerPrefix = $runBlock.Substring($runnerStart)

# Insert parallel branch after the first jobs:updated emit inside runJob and before its processing loop.
$emitRx = New-Object System.Text.RegularExpressions.Regex('m\.emit\("jobs:updated",[ \t]*job\)[ \t]*\r?\n', $multi)
$emitMatches = $emitRx.Matches($runOnly)
if ($emitMatches.Count -lt 1) {
    throw "Stage 6 v2 could not find jobs:updated emit inside runJob. No project source files were changed."
}
$emit = $emitMatches[0]
$parallelBranch = @'

	if workers := m.workerCount(job.Type); workers > 1 {
		return m.runJobConcurrent(root, job, runner, workers)
	}

'@
$runOnly = $runOnly.Substring(0,$emit.Index+$emit.Length) + $parallelBranch + $runOnly.Substring($emit.Index+$emit.Length)
$editedRunBlock = $runOnly + $runnerPrefix
$manager = $manager.Substring(0,$runJobMatch.Index) + $editedRunBlock + $manager.Substring($runJobMatch.Index+$runJobMatch.Length)

# Add workerCount after runner(), using function-level regex.
$runnerFunction = Find-OneRegex $manager '(?ms)^func[ \t]+\(m[ \t]+\*Manager\)[ \t]+runner\(jobType[ \t]+string\)[ \t]+Runner[ \t]*\{.*?^\}' "runner function" $singleMulti
$workerCount = @'

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
$manager = $manager.Substring(0,$runnerFunction.Index+$runnerFunction.Length) + $workerCount + $manager.Substring($runnerFunction.Index+$runnerFunction.Length)

# App opts only metadata enrichment into four workers.
$app = Replace-OneRegex $app 'app\.jobs\.Register\([ \t]*"metadata_enrichment"[ \t]*,[ \t]*app\.runMetadataJobItem[ \t]*\)' 'app.jobs.RegisterConcurrent("metadata_enrichment", 4, app.runMetadataJobItem)' "metadata job registration" $multi

# Help update is best-effort and never blocks Stage 6.
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

# All required transformations are now validated in memory.
Copy-Item $ManagerPath "$ManagerPath.stage6v2.bak" -Force
Copy-Item $AppPath "$AppPath.stage6v2.bak" -Force
if ($help -ne $null) { Copy-Item $HelpPath "$HelpPath.stage6v2.bak" -Force }

[System.IO.File]::WriteAllText($ManagerPath, $manager, $Utf8NoBom)
[System.IO.File]::WriteAllText($AppPath, $app, $Utf8NoBom)
if ($help -ne $null) { [System.IO.File]::WriteAllText($HelpPath, $help, $Utf8NoBom) }
[System.IO.File]::WriteAllText($ParallelPath, [System.IO.File]::ReadAllText($ParallelPayload), $Utf8NoBom)
[System.IO.File]::WriteAllText($ParallelTestPath, [System.IO.File]::ReadAllText($TestPayload), $Utf8NoBom)

& gofmt -w $ManagerPath $AppPath $ParallelPath $ParallelTestPath
if ($LASTEXITCODE -ne 0) {
    throw "gofmt failed. Backups with .stage6v2.bak are available."
}

Write-Host "Stage 6 v2 parallel metadata enrichment installed successfully."
Write-Host "Metadata enrichment concurrency: 4 tracks."
Write-Host "Old root-level CCML-stage*.go payloads were cleaned."
