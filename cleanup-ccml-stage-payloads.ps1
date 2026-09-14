$ErrorActionPreference = "Stop"
$ProjectRoot = "C:\Users\ANDYBUM\GolandProjects\CCML"

if (-not (Test-Path $ProjectRoot)) {
    throw "CCML project not found: $ProjectRoot"
}

$files = Get-ChildItem $ProjectRoot -File -Filter "CCML-stage*.go"
if (-not $files) {
    Write-Host "No stray CCML stage .go payloads found in project root."
    exit 0
}

Write-Host "Removing stray stage payloads from project root:"
$files | ForEach-Object { Write-Host "  $($_.Name)" }
$files | Remove-Item -Force
Write-Host "Cleanup complete."
