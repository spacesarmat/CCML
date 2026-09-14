$ErrorActionPreference = "Stop"

$ProjectRoot = "C:\Users\ANDYBUM\GolandProjects\CCML"
$CssPath = Join-Path $ProjectRoot "frontend\src\styles.css"

if (-not (Test-Path $CssPath)) {
    throw "styles.css not found: $CssPath"
}

$Utf8NoBom = New-Object System.Text.UTF8Encoding($false)
$css = [System.IO.File]::ReadAllText($CssPath)

$doneMarker = "grid-template-rows: auto auto minmax(0, 1fr) auto;"
if ($css.Contains($doneMarker) -and $css.Contains(".help-search-row { position: relative; z-index: 2;")) {
    Write-Host "Stage 5.1 Help layout hotfix is already applied."
    exit 0
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
        throw "Stage 5.1 anchor not found: $Label. No files were changed."
    }
    $second = $Text.IndexOf($Old, $first + $Old.Length, [System.StringComparison]::Ordinal)
    if ($second -ge 0) {
        throw "Stage 5.1 anchor is ambiguous: $Label. No files were changed."
    }
    return $Text.Substring(0, $first) + $New + $Text.Substring($first + $Old.Length)
}

$css = Replace-ExactlyOnce $css @'
.help-modal { width: min(1040px, calc(100vw - 48px)); }
'@ @'
.help-modal {
  width: min(1040px, calc(100vw - 48px));
  grid-template-rows: auto auto minmax(0, 1fr) auto;
  position: relative;
  isolation: isolate;
  background: #0f1621;
}
'@ "help modal grid"

$css = Replace-ExactlyOnce $css @'
.help-search-row { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 7px; padding: 11px 14px; border-bottom: 1px solid #263142; background: #0d141e; }
'@ @'
.help-search-row { position: relative; z-index: 2; display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 7px; padding: 11px 14px; border-bottom: 1px solid #263142; background: #0d141e; }
'@ "help search layer"

$css = Replace-ExactlyOnce $css @'
.help-layout { min-height: 0; display: grid; grid-template-columns: 190px minmax(0, 1fr); overflow: hidden; }
'@ @'
.help-layout { min-height: 0; display: grid; grid-template-columns: 190px minmax(0, 1fr); overflow: hidden; position: relative; z-index: 1; }
'@ "help content layer"

$css = Replace-ExactlyOnce $css @'
.help-content { min-height: 0; overflow: auto; scroll-behavior: smooth; padding: 14px; }
'@ @'
.help-content { min-height: 0; overflow: auto; overscroll-behavior: contain; scroll-behavior: smooth; padding: 14px; background: #0f1621; }
'@ "help content scrolling"

$css = Replace-ExactlyOnce $css @'
.help-nav { min-height: 0; overflow: auto; padding: 12px 9px; border-right: 1px solid #263142; background: #0d131c; }
'@ @'
.help-nav { min-height: 0; overflow: auto; overscroll-behavior: contain; padding: 12px 9px; border-right: 1px solid #263142; background: #0d131c; }
'@ "help navigation scrolling"

Copy-Item $CssPath "$CssPath.stage5.1.bak" -Force
[System.IO.File]::WriteAllText($CssPath, $css, $Utf8NoBom)

Write-Host "Stage 5.1 Help layout hotfix applied successfully."
Write-Host "Help now uses an explicit 4-row layout: header / search / content / footer."
