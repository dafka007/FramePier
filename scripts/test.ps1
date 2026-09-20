$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$goExe = Get-Command go.exe -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source -First 1
if (-not $goExe) { $goExe = Join-Path $projectRoot '.tools\go\bin\go.exe' }
if (-not (Test-Path -LiteralPath $goExe)) { throw 'Go compiler not found. Run scripts/build.ps1 first.' }
$env:GOCACHE = Join-Path $projectRoot '.tools\go-cache'
New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null

Push-Location (Join-Path $projectRoot 'helper')
try {
    & $goExe fmt ./src
    & $goExe vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'go vet failed.' }
    & $goExe test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go tests failed.' }
} finally { Pop-Location }

$manifest = Get-Content -Raw -LiteralPath (Join-Path $projectRoot 'extension\manifest.json') | ConvertFrom-Json
if ($manifest.manifest_version -ne 3) { throw 'Extension is not Manifest V3.' }
if ($manifest.version -ne '0.2.2') { throw 'Extension version is inconsistent.' }
$packageVersion = Get-Content -Raw -LiteralPath (Join-Path $projectRoot 'installer\package-version.json') | ConvertFrom-Json
if ($packageVersion.product -ne 'VidDock' -or $packageVersion.packageVersion -ne $manifest.version -or $packageVersion.extensionId -ne 'kclnooibijmfenaldmpkffdbednfipkk') {
    throw 'Trusted package-version marker is inconsistent.'
}
if ($manifest.host_permissions -contains '<all_urls>') { throw 'Forbidden <all_urls> permission found.' }
$expectedHostPermissions = @(
    'https://youtube.com/*', 'https://www.youtube.com/*', 'https://m.youtube.com/*',
    'https://music.youtube.com/*', 'https://youtu.be/*', 'https://twitch.tv/*',
    'https://www.twitch.tv/*', 'https://m.twitch.tv/*', 'https://clips.twitch.tv/*'
)
foreach ($permission in $expectedHostPermissions) {
    if ($manifest.host_permissions -notcontains $permission) { throw "Required narrow host permission missing: $permission" }
}
if ($manifest.permissions -contains 'cookies') { throw 'Forbidden cookies permission found.' }
if ($manifest.content_security_policy.extension_pages -match 'unsafe-eval|unsafe-inline|https:.*script') { throw 'Unsafe extension CSP found.' }

$sourceFiles = Get-ChildItem -LiteralPath (Join-Path $projectRoot 'extension') -Filter '*.js'
foreach ($file in $sourceFiles) {
    & node --check $file.FullName
    if ($LASTEXITCODE -ne 0) { throw "JavaScript syntax failed: $($file.FullName)" }
    $source = Get-Content -Raw -LiteralPath $file.FullName
    if ($source -match '\.innerHTML\s*=|\beval\s*\(|new\s+Function\s*\(') { throw "Unsafe DOM/dynamic code pattern found: $($file.FullName)" }
}

$helperSource = Get-Content -Raw -LiteralPath (Join-Path $projectRoot 'helper\src\main.go')
$registrationSource = Get-Content -Raw -LiteralPath (Join-Path $projectRoot 'helper\src\registration.go')
if ($helperSource -match 'exec\.Command(?:Context)?\s*\(' -or $registrationSource -match 'exec\.Command(?:Context)?\s*\(') {
    throw 'A helper subprocess bypasses the hidden Windows command constructor.'
}
$publishableSource = Get-ChildItem -LiteralPath $projectRoot -File -Recurse | Where-Object { $_.FullName -notmatch '[\\/](\.git|\.tools|dist)[\\/]' }
$authBypassPattern = '(?i)(--cookies(?:-from-browser)?|oauth[_-]?token|client[_-]?secret|authorization\s*:.*bearer)'
foreach ($file in $publishableSource | Where-Object { $_.FullName -match '[\\/](extension|helper[\\/]src)[\\/]' }) {
    if ($file.Extension -notin @('.png', '.ico', '.exe', '.zip')) {
        $content = Get-Content -Raw -LiteralPath $file.FullName -ErrorAction SilentlyContinue
        if ($content -match $authBypassPattern) { throw "Forbidden authentication/cookie integration found: $($file.FullName)" }
    }
}
$nativeTemplate = Get-Content -Raw -LiteralPath (Join-Path $projectRoot 'helper\native-host\com.viddock.helper.json.in')
if ($nativeTemplate -notmatch 'chrome-extension://kclnooibijmfenaldmpkffdbednfipkk/') {
    throw 'Native Messaging allowed origin is inconsistent.'
}

$allSource = $publishableSource
$secretPattern = '(?i)(api[_-]?key|password|secret|token)\s*[:=]\s*["''][A-Za-z0-9_\-]{16,}'
foreach ($file in $allSource) {
    if ($file.Extension -notin @('.png', '.ico', '.exe', '.zip')) {
        $content = Get-Content -Raw -LiteralPath $file.FullName -ErrorAction SilentlyContinue
        if ($content -match $secretPattern) { throw "Potential embedded secret found: $($file.FullName)" }
    }
}

$helper = Join-Path $projectRoot 'dist\helper\FramePierHelper.exe'
if (Test-Path -LiteralPath $helper) {
    $start = New-Object System.Diagnostics.ProcessStartInfo
    $start.FileName = $helper
    if ($null -ne $start.ArgumentList) {
        $start.ArgumentList.Add('--stdio-test')
    } else {
        $start.Arguments = '--stdio-test'
    }
    $start.UseShellExecute = $false
    $start.RedirectStandardInput = $true
    $start.RedirectStandardOutput = $true
    $testData = Join-Path $projectRoot '.tools\test-localappdata'
    New-Item -ItemType Directory -Force -Path $testData | Out-Null
    if ($null -ne $start.Environment) {
        $start.Environment['LOCALAPPDATA'] = $testData
    } else {
        $start.EnvironmentVariables['LOCALAPPDATA'] = $testData
    }
    $process = [System.Diagnostics.Process]::Start($start)
    try {
        $payload = [System.Text.Encoding]::UTF8.GetBytes('{"id":"test_ping","command":"ping"}')
        $prefix = [BitConverter]::GetBytes([uint32]$payload.Length)
        $process.StandardInput.BaseStream.Write($prefix, 0, 4)
        $process.StandardInput.BaseStream.Write($payload, 0, $payload.Length)
        $process.StandardInput.BaseStream.Flush()
        $sizeBytes = New-Object byte[] 4
        $read = $process.StandardOutput.BaseStream.Read($sizeBytes, 0, 4)
        if ($read -ne 4) { throw 'Native Messaging smoke test returned no frame.' }
        $size = [BitConverter]::ToUInt32($sizeBytes, 0)
        $body = New-Object byte[] $size
        $offset = 0
        while ($offset -lt $size) {
            $count = $process.StandardOutput.BaseStream.Read($body, $offset, $size - $offset)
            if ($count -le 0) { throw 'Native Messaging response ended early.' }
            $offset += $count
        }
        $response = [System.Text.Encoding]::UTF8.GetString($body) | ConvertFrom-Json
        if (-not $response.ok -or $response.version -ne $manifest.version) { throw 'Native Messaging ping returned an invalid response.' }
    } finally {
        if (-not $process.HasExited) { $process.Kill() }
        $process.Dispose()
    }
}

Write-Host 'All FramePier automated tests passed.'
& node --test (Join-Path $projectRoot 'scripts\folder-ui.test.mjs') (Join-Path $projectRoot 'scripts\background.test.mjs')
if ($LASTEXITCODE -ne 0) { throw 'Extension regression tests failed.' }
