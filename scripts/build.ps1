param(
    [switch]$SkipDownloads,
    [switch]$SkipInstaller
)

$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$toolsRoot = Join-Path $projectRoot '.tools'
$cacheRoot = Join-Path $toolsRoot 'cache'
$distRoot = Join-Path $projectRoot 'dist'
$helperDist = Join-Path $distRoot 'helper'
$dependencyDir = Join-Path $helperDist 'tools'
New-Item -ItemType Directory -Force -Path $toolsRoot, $cacheRoot, $distRoot, $helperDist, $dependencyDir | Out-Null
$env:GOCACHE = Join-Path $toolsRoot 'go-cache'
New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null

function Download-File([string]$Uri, [string]$OutFile) {
    if (-not (Test-Path -LiteralPath $OutFile -PathType Leaf)) {
        Write-Host "Downloading $Uri"
        Invoke-WebRequest -UseBasicParsing -Uri $Uri -OutFile $OutFile
    }
}

function Assert-Sha256([string]$Path, [string]$Expected) {
    $actual = (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $Expected.ToLowerInvariant()) {
        throw "SHA-256 mismatch for $Path. Expected $Expected, got $actual"
    }
}

$goExe = Get-Command go.exe -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source -First 1
if (-not $goExe) {
    $goRoot = Join-Path $toolsRoot 'go'
    $goExe = Join-Path $goRoot 'bin\go.exe'
    if (-not (Test-Path -LiteralPath $goExe)) {
        if ($SkipDownloads) { throw 'Go SDK is unavailable and downloads were skipped.' }
        $releases = Invoke-RestMethod -Uri 'https://go.dev/dl/?mode=json'
        $release = $releases | Where-Object stable | Select-Object -First 1
        $file = $release.files | Where-Object { $_.os -eq 'windows' -and $_.arch -eq 'amd64' -and $_.kind -eq 'archive' } | Select-Object -First 1
        if (-not $file) { throw 'The official Go API returned no stable Windows amd64 archive.' }
        $archive = Join-Path $cacheRoot $file.filename
        Download-File "https://go.dev/dl/$($file.filename)" $archive
        Assert-Sha256 $archive $file.sha256
        $extractRoot = Join-Path $toolsRoot 'go-extract'
        if (Test-Path -LiteralPath $extractRoot) { Remove-Item -LiteralPath $extractRoot -Recurse -Force }
        Expand-Archive -LiteralPath $archive -DestinationPath $extractRoot
        if (Test-Path -LiteralPath $goRoot) { Remove-Item -LiteralPath $goRoot -Recurse -Force }
        Move-Item -LiteralPath (Join-Path $extractRoot 'go') -Destination $goRoot
        Remove-Item -LiteralPath $extractRoot -Recurse -Force
    }
}

Push-Location (Join-Path $projectRoot 'helper')
try {
    & $goExe test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go tests failed.' }
    $env:CGO_ENABLED = '0'
    & $goExe build -trimpath -ldflags '-s -w -H=windowsgui' -o (Join-Path $helperDist 'FramePierHelper.exe') ./src
    if ($LASTEXITCODE -ne 0) { throw 'FramePier helper build failed.' }
} finally {
    Pop-Location
}

if (-not $SkipDownloads) {
    $ytdlp = Join-Path $dependencyDir 'yt-dlp.exe'
    $sumFile = Join-Path $cacheRoot 'yt-dlp-SHA2-256SUMS'
    Download-File 'https://github.com/yt-dlp/yt-dlp/releases/download/2026.08.19/yt-dlp.exe' $ytdlp
    Invoke-WebRequest -UseBasicParsing -Uri 'https://github.com/yt-dlp/yt-dlp/releases/download/2026.08.19/SHA2-256SUMS' -OutFile $sumFile
    $expectedLine = Get-Content -LiteralPath $sumFile | Where-Object { $_ -match '\syt-dlp\.exe$' } | Select-Object -First 1
    if (-not $expectedLine) { throw 'yt-dlp checksum file did not contain yt-dlp.exe.' }
    Assert-Sha256 $ytdlp (($expectedLine -split '\s+')[0])

    # BtbN/FFmpeg-Builds LGPL static build (immutable release-tag URL).
    # Provider: https://github.com/BtbN/FFmpeg-Builds
    # Release:  autobuild-2026-08-31-13-27
    # Build-repo commit: 8267213e26c1031621e6e1210fe3aa4867214f6a
    # FFmpeg source commit: 1a748fe2cd43e3ead22fafb1b5b7d77f153898a8
    # Artifact: ffmpeg-n8.1.2-50-g1a748fe2cd-win64-lgpl-8.1.zip
    $ffmpegArchive = Join-Path $cacheRoot 'ffmpeg-n8.1.2-50-g1a748fe2cd-win64-lgpl-8.1.zip'
    Download-File 'https://github.com/BtbN/FFmpeg-Builds/releases/download/autobuild-2026-08-31-13-27/ffmpeg-n8.1.2-50-g1a748fe2cd-win64-lgpl-8.1.zip' $ffmpegArchive
    Assert-Sha256 $ffmpegArchive 'f6274bbd9c247f9e90c1bbed066b03ed4a3907cece2fb91be6dd352393936365'
    $ffmpegExtract = Join-Path $toolsRoot 'ffmpeg-extract'
    if (Test-Path -LiteralPath $ffmpegExtract) { Remove-Item -LiteralPath $ffmpegExtract -Recurse -Force }
    Expand-Archive -LiteralPath $ffmpegArchive -DestinationPath $ffmpegExtract
    # BtbN archive structure: ffmpeg-<version>-<target>-<variant>/bin/
    $binDir = Get-ChildItem -LiteralPath $ffmpegExtract -Directory -Recurse | Where-Object { $_.Name -eq 'bin' } | Select-Object -First 1
    if (-not $binDir) { throw 'bin directory was not present in the verified FFmpeg archive.' }
    $expectedBinaryHash = @{
        'ffmpeg.exe'  = '9c60da6c0b083110d59084ea39f60ae149aa3e031c3b4bb4f573fafa1c1e7cea'
        'ffprobe.exe' = '67176fa62f89f94c3bcd379fd05677a25651569a2eb8880ec2194e62c82be412'
    }
    foreach ($name in @('ffmpeg.exe', 'ffprobe.exe')) {
        $binary = Join-Path $binDir.FullName $name
        if (-not (Test-Path -LiteralPath $binary)) { throw "$name was not present in the verified FFmpeg archive." }
        Copy-Item -LiteralPath $binary -Destination (Join-Path $dependencyDir $name) -Force
        Assert-Sha256 (Join-Path $dependencyDir $name) $expectedBinaryHash[$name]
    }
    Remove-Item -LiteralPath $ffmpegExtract -Recurse -Force

    # Deno: the JavaScript runtime yt-dlp needs for YouTube (official release asset, pinned).
    # Release:  https://github.com/denoland/deno/releases/tag/v2.9.7
    # Archive:  deno-x86_64-pc-windows-msvc.zip (contains only deno.exe)
    # The archive is checked against the checksum file published with the release
    # AND against the SHA-256 recorded here; the extracted binary's SHA-256 is also recorded.
    $denoVersion = '2.9.7'
    $denoZipSha256 = 'a0c3101b4158d1dfb7d6a78a7bf0f3de80c96bb423c152beec8beb22786f2238'
    $denoExeSha256 = 'e020f3e232bd16e33768dee528e5983349c962952051ced0a5d58ad42f5d9b33'
    $denoBase = "https://github.com/denoland/deno/releases/download/v$denoVersion"
    $denoArchive = Join-Path $cacheRoot "deno-$denoVersion-x86_64-pc-windows-msvc.zip"
    $denoSumFile = Join-Path $cacheRoot "deno-$denoVersion-x86_64-pc-windows-msvc.zip.sha256sum"
    Download-File "$denoBase/deno-x86_64-pc-windows-msvc.zip" $denoArchive
    Invoke-WebRequest -UseBasicParsing -Uri "$denoBase/deno-x86_64-pc-windows-msvc.zip.sha256sum" -OutFile $denoSumFile
    $denoSumText = Get-Content -Raw -LiteralPath $denoSumFile
    $denoPublished = [regex]::Match($denoSumText, '(?im)^\s*Hash\s*:\s*([0-9A-Fa-f]{64})\s*$')
    if (-not $denoPublished.Success) { throw 'The published Deno checksum file did not contain a SHA-256 hash.' }
    Assert-Sha256 $denoArchive $denoPublished.Groups[1].Value
    Assert-Sha256 $denoArchive $denoZipSha256
    $denoExtract = Join-Path $toolsRoot 'deno-extract'
    if (Test-Path -LiteralPath $denoExtract) { Remove-Item -LiteralPath $denoExtract -Recurse -Force }
    Expand-Archive -LiteralPath $denoArchive -DestinationPath $denoExtract
    $denoExtracted = Join-Path $denoExtract 'deno.exe'
    if (-not (Test-Path -LiteralPath $denoExtracted -PathType Leaf)) { throw 'deno.exe was not present in the verified Deno archive.' }
    Assert-Sha256 $denoExtracted $denoExeSha256
    Copy-Item -LiteralPath $denoExtracted -Destination (Join-Path $dependencyDir 'deno.exe') -Force
    Assert-Sha256 (Join-Path $dependencyDir 'deno.exe') $denoExeSha256
    Remove-Item -LiteralPath $denoExtract -Recurse -Force
}

$packageVersion = (Get-Content -Raw -LiteralPath (Join-Path $projectRoot 'extension\manifest.json') | ConvertFrom-Json).version
$extensionZip = Join-Path $distRoot "FramePier-extension-$packageVersion.zip"
if (Test-Path -LiteralPath $extensionZip) { Remove-Item -LiteralPath $extensionZip -Force }
Compress-Archive -Path (Join-Path $projectRoot 'extension\*') -DestinationPath $extensionZip -CompressionLevel Optimal

if (-not $SkipInstaller) {
    $iscc = Get-ChildItem -Path "$env:LOCALAPPDATA\Programs\Inno Setup 6\ISCC.exe", 'C:\Program Files (x86)\Inno Setup 6\ISCC.exe', 'C:\Program Files\Inno Setup 6\ISCC.exe' -ErrorAction SilentlyContinue | Select-Object -ExpandProperty FullName -First 1
    if (-not $iscc) {
        throw 'Inno Setup 6 is required to build the installer. Install JRSoftware.InnoSetup with winget, then rerun this script.'
    }
    & $iscc (Join-Path $projectRoot 'installer\FramePier.iss')
    if ($LASTEXITCODE -ne 0) { throw 'Inno Setup compilation failed.' }
}

Write-Host "FramePier build completed: $distRoot"
