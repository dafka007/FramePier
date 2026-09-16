param(
    [string]$InstallRoot = "$env:LOCALAPPDATA\Programs\VidDock",
    [switch]$Unregister
)

$ErrorActionPreference = 'Stop'
$helper = Join-Path $InstallRoot 'VidDockHelper.exe'
if (-not (Test-Path -LiteralPath $helper -PathType Leaf)) {
    throw "VidDockHelper.exe was not found at $helper"
}

$argument = if ($Unregister) { '--unregister' } else { '--register' }
& $helper $argument
if ($LASTEXITCODE -ne 0) {
    throw "VidDock browser registration failed with exit code $LASTEXITCODE"
}
Write-Host 'VidDock browser registration completed.'
