param(
    [string]$OutputDir = "tools",
    [string]$ReleaseTag = "autobuild-2026-09-10-15-31"
)

$ErrorActionPreference = "Stop"

# $IsWindows exists in PowerShell 6/7, but not in Windows PowerShell 5.1.
# Fall back to the classic Windows environment marker so the script works in
# both shells (including the default terminal commonly used by GoLand).
$isWindowsValue = Get-Variable -Name IsWindows -ErrorAction SilentlyContinue
$runningOnWindows = if ($null -ne $isWindowsValue) {
    [bool]$isWindowsValue.Value
} else {
    $env:OS -eq "Windows_NT"
}
if (-not $runningOnWindows) {
    throw "This script is for Windows. Use scripts/fetch-ffmpeg.sh on macOS."
}

# Windows PowerShell 5.1 may otherwise negotiate an obsolete TLS version when
# talking to GitHub.
if (([Net.ServicePointManager]::SecurityProtocol -band [Net.SecurityProtocolType]::Tls12) -eq 0) {
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
}

$projectRoot = Split-Path -Parent $PSScriptRoot
if (-not [System.IO.Path]::IsPathRooted($OutputDir)) {
    $OutputDir = Join-Path $projectRoot $OutputDir
}

$repo = "BtbN/FFmpeg-Builds"
$api = if ($ReleaseTag -eq "latest") {
    "https://api.github.com/repos/$repo/releases/tags/latest"
} else {
    "https://api.github.com/repos/$repo/releases/tags/$ReleaseTag"
}

function Get-CCMLWindowsArchitecture {
    try {
        $runtimeArch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
        if ($runtimeArch -eq "x64" -or $runtimeArch -eq "arm64") {
            return $runtimeArch
        }
    } catch {
        # RuntimeInformation is not available on some older Windows PowerShell/.NET combinations.
    }

    $envArch = if ($env:PROCESSOR_ARCHITEW6432) {
        $env:PROCESSOR_ARCHITEW6432
    } else {
        $env:PROCESSOR_ARCHITECTURE
    }

    switch ($envArch.ToUpperInvariant()) {
        "AMD64" { return "x64" }
        "ARM64" { return "arm64" }
        default { throw "Unsupported Windows architecture: $envArch" }
    }
}

$arch = Get-CCMLWindowsArchitecture
$target = switch ($arch) {
    "x64" { "win64" }
    "arm64" { "winarm64" }
    default { throw "Unsupported Windows architecture: $arch" }
}

$headers = @{
    "Accept" = "application/vnd.github+json"
    "User-Agent" = "CCML-build/0.2"
}
$release = Invoke-RestMethod -Uri $api -Headers $headers
$pattern = "^ffmpeg-.*-$target-gpl-9\.0\.zip$"
$asset = $release.assets | Where-Object { $_.name -match $pattern } | Select-Object -First 1
if ($null -eq $asset) {
    throw "Release $($release.tag_name) does not contain a $target GPL 9.0 archive"
}
if ([string]::IsNullOrWhiteSpace([string]$asset.digest) -or -not ([string]$asset.digest).StartsWith("sha256:")) {
    throw "Asset $($asset.name) does not publish a SHA-256 digest"
}

New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("ccml-ffmpeg-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $tempRoot | Out-Null
$archive = Join-Path $tempRoot "ffmpeg.zip"
$extractDir = Join-Path $tempRoot "extract"
try {
    Write-Host "Downloading $($asset.name)..."
    Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $archive -Headers @{"User-Agent"="CCML-build/0.2"} -UseBasicParsing

    $actual = (Get-FileHash -Algorithm SHA256 -Path $archive).Hash.ToLowerInvariant()
    $expected = ([string]$asset.digest).Substring(7).ToLowerInvariant()
    if ($actual -ne $expected) {
        throw "SHA-256 mismatch for $($asset.name): got $actual expected $expected"
    }

    Expand-Archive -Path $archive -DestinationPath $extractDir -Force
    $ffmpegSource = Get-ChildItem -Path $extractDir -Filter ffmpeg.exe -Recurse -File | Where-Object { $_.DirectoryName -match '[\\/]bin$' } | Select-Object -First 1
    $ffprobeSource = Get-ChildItem -Path $extractDir -Filter ffprobe.exe -Recurse -File | Where-Object { $_.DirectoryName -match '[\\/]bin$' } | Select-Object -First 1
    if ($null -eq $ffmpegSource -or $null -eq $ffprobeSource) {
        throw "Downloaded archive does not contain bin/ffmpeg.exe and bin/ffprobe.exe"
    }

    $ffmpegTarget = Join-Path $OutputDir "ffmpeg.exe"
    $ffprobeTarget = Join-Path $OutputDir "ffprobe.exe"
    Copy-Item -Force $ffmpegSource.FullName $ffmpegTarget
    Copy-Item -Force $ffprobeSource.FullName $ffprobeTarget

    $firstLine = (& $ffmpegTarget -version | Select-Object -First 1)
    if ($LASTEXITCODE -ne 0) {
        throw "Downloaded ffmpeg.exe failed its version check"
    }
    $version = if ($firstLine -match '^ffmpeg version\s+(\S+)') { $Matches[1] } else { $release.tag_name }
    $manifest = [ordered]@{
        version = $version
        updateId = "BtbN/FFmpeg-Builds:$($asset.digest)"
        dir = ""
    }
    $manifestPath = Join-Path $OutputDir "CCML-FFMPEG-MANIFEST.json"
    $manifestJSON = $manifest | ConvertTo-Json
    $utf8NoBom = New-Object -TypeName System.Text.UTF8Encoding -ArgumentList $false
    [System.IO.File]::WriteAllText($manifestPath, $manifestJSON, $utf8NoBom)

    Copy-Item -Force (Join-Path $projectRoot "third_party/ffmpeg/NOTICE.md") (Join-Path $OutputDir "FFMPEG-NOTICE.md")
    Copy-Item -Force (Join-Path $projectRoot "third_party/ffmpeg/COPYING.GPLv3") (Join-Path $OutputDir "COPYING.GPLv3")
    Write-Host "FFmpeg $version ($($release.tag_name)) prepared in $OutputDir"
} finally {
    if (Test-Path $tempRoot) {
        Remove-Item -Recurse -Force $tempRoot
    }
}
