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
    Download-File 'https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp.exe' $ytdlp
    Invoke-WebRequest -UseBasicParsing -Uri 'https://github.com/yt-dlp/yt-dlp/releases/latest/download/SHA2-256SUMS' -OutFile $sumFile
    $expectedLine = Get-Content -LiteralPath $sumFile | Where-Object { $_ -match '\syt-dlp\.exe$' } | Select-Object -First 1
    if (-not $expectedLine) { throw 'yt-dlp checksum file did not contain yt-dlp.exe.' }
    Assert-Sha256 $ytdlp (($expectedLine -split '\s+')[0])

    # Gyan is linked by ffmpeg.org. This immutable release URL and SHA-256 are
    # also published in the Windows Package Manager Gyan.FFmpeg.Essentials manifest.
    $ffmpegArchive = Join-Path $cacheRoot 'ffmpeg-8.1.1-essentials_build.zip'
    Download-File 'https://github.com/GyanD/codexffmpeg/releases/download/8.1.1/ffmpeg-8.1.1-essentials_build.zip' $ffmpegArchive
    Assert-Sha256 $ffmpegArchive '6f58ce889f59c311410f7d2b18895b33c03456463486f3b1ebc93d97a0f54541'
    $ffmpegExtract = Join-Path $toolsRoot 'ffmpeg-extract'
    if (Test-Path -LiteralPath $ffmpegExtract) { Remove-Item -LiteralPath $ffmpegExtract -Recurse -Force }
    Expand-Archive -LiteralPath $ffmpegArchive -DestinationPath $ffmpegExtract
    foreach ($name in @('ffmpeg.exe', 'ffprobe.exe')) {
        $binary = Get-ChildItem -LiteralPath $ffmpegExtract -Filter $name -Recurse | Select-Object -First 1
        if (-not $binary) { throw "$name was not present in the verified FFmpeg archive." }
        Copy-Item -LiteralPath $binary.FullName -Destination (Join-Path $dependencyDir $name) -Force
    }
    Remove-Item -LiteralPath $ffmpegExtract -Recurse -Force
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
