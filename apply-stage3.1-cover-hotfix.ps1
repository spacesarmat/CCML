$ErrorActionPreference = "Stop"

$ProjectRoot = "C:\Users\ANDYBUM\GolandProjects\CCML"
$Patch = Join-Path $ProjectRoot "CCML-stage3.1-cover-hotfix.patch"

if (-not (Test-Path (Join-Path $ProjectRoot ".git"))) {
    throw "CCML git repository not found: $ProjectRoot"
}
if (-not (Test-Path $Patch)) {
    throw "Patch not found: $Patch"
}

Set-Location $ProjectRoot

git apply --check $Patch 2>$null
if ($LASTEXITCODE -eq 0) {
    git apply $Patch
    if ($LASTEXITCODE -ne 0) { throw "git apply failed" }
    Write-Host "Stage 3.1 cover hotfix applied."
} else {
    git apply --reverse --check $Patch 2>$null
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Stage 3.1 cover hotfix is already applied."
    } else {
        throw "Patch does not match the current backend files. No files were changed."
    }
}
