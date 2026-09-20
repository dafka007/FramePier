param(
    [string]$InstallRoot = "$env:LOCALAPPDATA\Programs\FramePier",
    [switch]$Unregister
)

$ErrorActionPreference = 'Stop'
$helper = Join-Path $InstallRoot 'FramePierHelper.exe'
if (-not (Test-Path -LiteralPath $helper -PathType Leaf)) {
    throw "FramePierHelper.exe was not found at $helper"
}

$argument = if ($Unregister) { '--unregister' } else { '--register' }
& $helper $argument
if ($LASTEXITCODE -ne 0) {
    throw "FramePier browser registration failed with exit code $LASTEXITCODE"
}
Write-Host 'FramePier browser registration completed.'
