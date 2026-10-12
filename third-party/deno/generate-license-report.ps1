# Third-party license report for the bundled Deno 2.9.7 (tag v2.9.7), built from its Cargo.lock.
# Needs internet access to crates.io, so it is run by the maintainer, not during a user install.
#   Stage 1 (about 20 minutes, one request per second as crates.io requires):
#       powershell -ExecutionPolicy Bypass -File third-party\deno\generate-license-report.ps1
#   Stage 2 (adds the license/notice files shipped inside each crate; slower and larger):
#       powershell -ExecutionPolicy Bypass -File third-party\deno\generate-license-report.ps1 -Texts
# Output goes to dist\deno-license-report\. Re-running continues where it stopped.
param(
    [string]$Lock = (Join-Path $PSScriptRoot 'Cargo.lock'),
    [string]$OutDir = (Join-Path (Split-Path -Parent (Split-Path -Parent $PSScriptRoot)) 'dist\deno-license-report'),
    [switch]$Texts
)

$ErrorActionPreference = 'Stop'
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$headers = @{ 'User-Agent' = 'FramePier-license-report (https://github.com/dafka007/FramePier)' }

$packages = New-Object System.Collections.ArrayList
$current = $null
foreach ($line in Get-Content -LiteralPath $Lock) {
    if ($line -eq '[[package]]') {
        $current = @{ name = ''; version = ''; source = '' }
        [void]$packages.Add($current)
        continue
    }
    if ($null -eq $current) { continue }
    if ($line -match '^name = "(.+)"$') { $current.name = $Matches[1] }
    elseif ($line -match '^version = "(.+)"$') { $current.version = $Matches[1] }
    elseif ($line -match '^source = "(.+)"$') { $current.source = $Matches[1] }
}
Write-Host "Cargo.lock packages: $($packages.Count)"

$tsv = Join-Path $OutDir 'deno-v2.9.7-licenses.tsv'
$done = @{}
if (Test-Path -LiteralPath $tsv) {
    foreach ($row in Get-Content -LiteralPath $tsv) {
        $cols = $row -split "`t"
        if ($cols.Count -ge 3) { $done["$($cols[0]) $($cols[1])"] = $true }
    }
} else {
    Set-Content -LiteralPath $tsv -Value "name`tversion`tlicense`tsource" -Encoding UTF8
}

foreach ($pkg in $packages) {
    $key = "$($pkg.name) $($pkg.version)"
    if ($done.ContainsKey($key)) { continue }
    $license = 'NOT-FETCHED'
    $kind = 'workspace-or-path'
    if ($pkg.source -like 'registry+*') {
        $kind = 'crates.io'
        for ($try = 1; $try -le 4; $try++) {
            try {
                $info = Invoke-RestMethod -UseBasicParsing -Headers $headers -Uri "https://crates.io/api/v1/crates/$($pkg.name)/$($pkg.version)"
                $license = [string]$info.version.license
                if (-not $license) { $license = 'NO-LICENSE-FIELD' }
                break
            } catch {
                Start-Sleep -Seconds (3 * $try)
            }
        }
        Start-Sleep -Milliseconds 1100
    } elseif ($pkg.source -like 'git+*') {
        $kind = $pkg.source
        $license = 'GIT-SOURCE-CHECK-MANUALLY'
    }
    Add-Content -LiteralPath $tsv -Value "$($pkg.name)`t$($pkg.version)`t$license`t$kind" -Encoding UTF8
}
Write-Host "License table written: $tsv"

if ($Texts) {
    $notices = Join-Path $OutDir 'deno-v2.9.7-crate-license-files.txt'
    $missing = Join-Path $OutDir 'deno-v2.9.7-crates-without-license-file.txt'
    $handled = @{}
    if (Test-Path -LiteralPath $notices) {
        foreach ($l in Get-Content -LiteralPath $notices) { if ($l -match '^=== (\S+ \S+) ') { $handled[$Matches[1]] = $true } }
    }
    foreach ($pkg in $packages) {
        if ($pkg.source -notlike 'registry+*') { continue }
        $key = "$($pkg.name) $($pkg.version)"
        if ($handled.ContainsKey($key)) { continue }
        $crate = Join-Path $env:TEMP "fp-$($pkg.name)-$($pkg.version).crate"
        $tmp = Join-Path $env:TEMP "fp-crate-extract"
        try {
            Invoke-WebRequest -UseBasicParsing -Headers $headers -Uri "https://static.crates.io/crates/$($pkg.name)/$($pkg.name)-$($pkg.version).crate" -OutFile $crate
            $names = & tar.exe -tzf $crate | Where-Object { $_ -match '(^|/)(LICEN[SC]E|COPYING|NOTICE|UNLICENSE|COPYRIGHT)[^/]*$' -and $_ -notmatch '/(tests?|benches|examples)/' }
            if (Test-Path -LiteralPath $tmp) { Remove-Item -LiteralPath $tmp -Recurse -Force }
            New-Item -ItemType Directory -Force -Path $tmp | Out-Null
            if ($names) {
                & tar.exe -xzf $crate -C $tmp @($names)
                Add-Content -LiteralPath $notices -Value "=== $key (licensed: see tsv) ===" -Encoding UTF8
                foreach ($n in $names) {
                    $f = Join-Path $tmp ($n -replace '/', '\')
                    if (Test-Path -LiteralPath $f -PathType Leaf) {
                        $text = Get-Content -Raw -LiteralPath $f
                        Add-Content -LiteralPath $notices -Value "--- $n ---" -Encoding UTF8
                        if ($text) { Add-Content -LiteralPath $notices -Value $text -Encoding UTF8 }
                    }
                }
            } else {
                Add-Content -LiteralPath $missing -Value $key -Encoding UTF8
                Add-Content -LiteralPath $notices -Value "=== $key (no license file in crate) ===" -Encoding UTF8
            }
        } catch {
            Write-Host "FAILED $key : $($_.Exception.Message)"
        } finally {
            if (Test-Path -LiteralPath $crate) { Remove-Item -LiteralPath $crate -Force }
        }
    }
    Write-Host "Crate license files written: $notices"
}
Write-Host 'Done.'
