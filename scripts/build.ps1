param(
    [switch]$SkipDownloads,
    [switch]$SkipInstaller,
    [string]$FFmpegSharedDir,
    [string]$YtDlpPath
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
    # yt-dlp 2026.08.19: FramePier's own build of yt-dlp's tagged source (commit 3a08beaf031ab68f966401ead017ac81fe8486cf)
    # with yt-dlp's PyInstaller recipe on Python 3.13.16 (OpenSSL 3.5.9) instead of upstream's Python 3.10 (OpenSSL 1.1.1t).
    # See third-party\ytdlp-openssl3\ . There is deliberately no download URL: supply the verified file with -YtDlpPath
    # (default .tools\ytdlp-openssl3\yt-dlp.exe). Its SHA-256 is pinned here.
    $expectedYtdlpSha256 = '8b976e7ff71a6f8e8dd48feb2920b3c41a44641ffe192dba08cde0175890ee44'
    if (-not $YtDlpPath) { $YtDlpPath = Join-Path $toolsRoot 'ytdlp-openssl3\yt-dlp.exe' }
    if (-not (Test-Path -LiteralPath $YtDlpPath -PathType Leaf)) { throw "yt-dlp.exe was not found at $YtDlpPath. Pass -YtDlpPath with the verified FramePier build." }
    Assert-Sha256 $YtDlpPath $expectedYtdlpSha256
    $ytdlp = Join-Path $dependencyDir 'yt-dlp.exe'
    Copy-Item -LiteralPath $YtDlpPath -Destination $ytdlp -Force
    Assert-Sha256 $ytdlp $expectedYtdlpSha256

    # EXPERIMENTAL shared-LGPL FFmpeg (ffmpeg.exe, ffprobe.exe and 9 DLLs).
    # Built from FFmpeg commit 1a748fe2cd43e3ead22fafb1b5b7d77f153898a8 and LAME 3.100
    # (MSYS2 UCRT64, --enable-shared --disable-autodetect --enable-libmp3lame --enable-schannel).
    # There is deliberately no download URL: GitHub Actions artifacts expire and must not be
    # a production source. Supply a verified local directory with -FFmpegSharedDir (default
    # .tools\ffmpeg-shared). Every file is checked against the SHA-256 recorded below.
    # NOT redistribution-cleared; see THIRD_PARTY_NOTICES.md.
    if (-not $FFmpegSharedDir) { $FFmpegSharedDir = Join-Path $toolsRoot 'ffmpeg-shared' }
    $expectedFFmpegHash = [ordered]@{
        'ffmpeg.exe'         = '6dffef734d9781d9429cac5584fb98491ccf6267ada24f8f993c7c1e21ae00d2'
        'ffprobe.exe'        = '83748b310feb3b9e92c2fb9112037f5636ae7ce5e6e020802beead1422b3b057'
        'avcodec-62.dll'     = 'bab865266ada5fece37b5d02ff26b1425e56d0db5ebad9286076647a04b86ca5'
        'avdevice-62.dll'    = 'a18d5eb9e3eebda087e91e8fcf5ac8881b085eb4cd2702860a81bff2548e8d54'
        'avfilter-11.dll'    = '3a8458551e15cd54c348902d639dc63b8dd5351fe5165a49750dfcd6e788228d'
        'avformat-62.dll'    = 'a7e21c4678c427c65777ffe359664ce7d7b28ef303f620f869294553bee239eb'
        'avutil-60.dll'      = '0e50c0f0cb70f8f49417be5ff7d9803a6b90ee9eec2d051f43007fa6594f7ebb'
        'libmp3lame-0.dll'   = 'dad41978e4485e529ce0898219b59b4af867c6ae6b53a94a62be6ea89741ca78'
        'libwinpthread-1.dll' = '76cc52adfcdb8c800813b2ae8da42a908934eb1ee54ce8e9744fc8d703c76ee1'
        'swresample-6.dll'   = 'c5b8de9ce9cc50f2c49ed8edc524a317d3874476519eb4ed0a8ee394168b5c6a'
        'swscale-9.dll'      = '61616e9fca663cd26041bc7ff8fd671656d06d26cbf54bfc1d8a3d4c67684f00'
    }
    foreach ($name in $expectedFFmpegHash.Keys) {
        $source = Join-Path $FFmpegSharedDir $name
        if (-not (Test-Path -LiteralPath $source -PathType Leaf)) { throw "$name was not found in $FFmpegSharedDir. Pass -FFmpegSharedDir with the verified shared FFmpeg directory." }
        Assert-Sha256 $source $expectedFFmpegHash[$name]
    }
    foreach ($name in $expectedFFmpegHash.Keys) {
        $target = Join-Path $dependencyDir $name
        Copy-Item -LiteralPath (Join-Path $FFmpegSharedDir $name) -Destination $target -Force
        Assert-Sha256 $target $expectedFFmpegHash[$name]
    }

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

# Every license file the notices refer to must exist before an installer is built.
foreach ($licenseFile in 'DENO-2.9.7-CRATE-LICENSES.txt', 'DENO-2.9.7-CRATE-AUTHORS.txt', 'DENO-V8-LICENSES.txt', 'DENO-2.9.7-MIT.txt', 'YT-DLP-2026.08.19-NOTICES.txt', 'YT-DLP-PYTHON-RUNTIME-NOTICES.txt', 'GPL-3.0.txt', 'FFMPEG-LGPL-2.1.txt', 'LAME-LGPL-2.0.txt', 'SOURCE-AVAILABILITY.txt', 'GO-LICENSE.txt') {
    if (-not (Test-Path -LiteralPath (Join-Path $projectRoot "licenses\$licenseFile") -PathType Leaf)) { throw "licenses\$licenseFile is missing (run third-party\deno\collect-missing-crate-authors.ps1 for the Deno crate authors file)." }
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
