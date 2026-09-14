$ErrorActionPreference = "Stop"

$ProjectRoot = "C:\Users\ANDYBUM\GolandProjects\CCML"
$CssPath = Join-Path $ProjectRoot "frontend\src\styles.css"

if (-not (Test-Path $CssPath)) {
    throw "styles.css not found: $CssPath"
}

$Utf8NoBom = New-Object System.Text.UTF8Encoding($false)
$css = [System.IO.File]::ReadAllText($CssPath)

$marker = ".workspace-table tbody { user-select: none;"
if ($css.Contains($marker)) {
    Write-Host "Stage 4.1 selection polish is already applied."
    exit 0
}

$old = @'
.workspace-table .title-cell { color: #d4deea; font-weight: 570; }
.workspace-table tbody tr:hover { background: #121c28; }
.workspace-table tbody tr.selected { background: #1b2940; }
.workspace-table tbody tr.active-row { outline: 1px solid #6b86ff; outline-offset: -1px; }
.workspace-table tbody tr.active-row.selected { background: #213453; }
'@

$new = @'
.workspace-table .title-cell { color: #d4deea; font-weight: 570; }
.workspace-table tbody { user-select: none; -webkit-user-select: none; }
.workspace-table tbody tr { cursor: default; }
.workspace-table tbody tr:hover { background: #121c28; }
.workspace-table tbody tr.selected { background: #1b2940; }
.workspace-table tbody tr.selected > td:first-child { box-shadow: inset 2px 0 0 #5579e8; }
.workspace-table tbody tr.active-row { outline: 1px solid #6b86ff; outline-offset: -1px; }
.workspace-table tbody tr.active-row.selected { background: #213453; }
.workspace-table tbody tr.active-row.selected > td:first-child { box-shadow: inset 3px 0 0 #7f99ff; }
'@

$first = $css.IndexOf($old, [System.StringComparison]::Ordinal)
if ($first -lt 0) {
    throw "Stage 4.1 CSS anchor not found. No files were changed."
}
$second = $css.IndexOf($old, $first + $old.Length, [System.StringComparison]::Ordinal)
if ($second -ge 0) {
    throw "Stage 4.1 CSS anchor is ambiguous. No files were changed."
}

$css = $css.Substring(0, $first) + $new + $css.Substring($first + $old.Length)

Copy-Item $CssPath "$CssPath.stage4.1.bak" -Force
[System.IO.File]::WriteAllText($CssPath, $css, $Utf8NoBom)

Write-Host "Stage 4.1 selection polish applied successfully."
Write-Host "Native browser text selection is disabled inside library table rows."
