#requires -Version 5.1
<#
.SYNOPSIS
    Ensures Chrome for Testing 152.0.7977.75 (win64) is installed in
    .tools\browser-tests\chrome-for-testing\chrome-win64\chrome.exe.

.DESCRIPTION
    Uses Google's official Chrome for Testing per-version JSON API to obtain
    the Windows x64 download URL. Downloads and extracts only if the expected
    chrome.exe is missing. Verifies the extracted binary reports the pinned
    version. Cleans up temporary files with try/finally.

.PARAMETER Version
    Chrome for Testing version to install. Default: 152.0.7977.75.

.PARAMETER Root
    Project root directory. Default: parent of $PSScriptRoot.

.EXAMPLE
    .\ensure-chrome-for-testing.ps1

.EXAMPLE
    .\ensure-chrome-for-testing.ps1 -Version 152.0.7977.75 -Root 'C:\local AI\VidDock'
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $false)]
    [ValidatePattern('^\d+\.\d+\.\d+\.\d+$')]
    [string]$Version = '152.0.7977.75',

    [Parameter(Mandatory = $false)]
    [string]$Root = (Split-Path -Parent $PSScriptRoot)
)

$ErrorActionPreference = 'Stop'

# --- Resolve paths -----------------------------------------------------------
$cftRoot      = Join-Path $Root '.tools\browser-tests\chrome-for-testing'
$chromeDir    = Join-Path $cftRoot 'chrome-win64'
$chromeExe    = Join-Path $chromeDir 'chrome.exe'
$apiUrl       = "https://googlechromelabs.github.io/chrome-for-testing/$Version.json"
$tempArchive  = Join-Path $env:TEMP "cft-$Version-win64.zip"
$tempExtract  = Join-Path $env:TEMP "cft-$Version-win64-extract"

# --- Verify existing installation ---------------------------------------------
if (Test-Path -LiteralPath $chromeExe -PathType Leaf) {
    Write-Host "Chrome for Testing $Version already present at: $chromeExe"

    $manifestPath = Join-Path $chromeDir "${Version}.manifest"
    $versionVerified = $false

    # Primary: read the version from the embedded manifest file
    if (Test-Path -LiteralPath $manifestPath) {
        $manifestContent = Get-Content -LiteralPath $manifestPath -Raw
        if ($manifestContent -match "version='$Version'") {
            Write-Host "Embedded version metadata confirms: $Version"
            $versionVerified = $true
        }
    }

    # Fallback: run chrome.exe --version
    if (-not $versionVerified) {
        try {
            $versionOutput = & $chromeExe --version 2>&1
            $versionText = $versionOutput.ToString().Trim()
            if ($versionText -eq "Chrome/$Version") {
                Write-Host "Chrome version: $versionText"
                $versionVerified = $true
            } else {
                Write-Host "Chrome version output: $versionText"
            }
        } catch {
            Write-Host "Unable to run chrome.exe --version: $_"
        }
    }

    if ($versionVerified) {
        Write-Host "SUCCESS: Correct pinned version $Version is already installed."
        return
    } else {
        throw "Existing Chrome for Testing at $chromeExe does not match expected version $Version. " +
              "Version cannot be verified. Do not overwrite - investigate manually."
    }
}

# --- Fetch download URL from official JSON API --------------------------------
Write-Host "Fetching download URL for Chrome for Testing $Version (win64)..."
try {
    $api = Invoke-RestMethod -Uri $apiUrl -UseBasicParsing
} catch {
    throw "Failed to fetch Chrome for Testing API for version ${Version}: $_"
}

# Select the win64 chrome binary URL
$win64Url = $api.downloads.chrome | Where-Object { $_.platform -eq 'win64' } | Select-Object -First 1
if (-not $win64Url -or -not $win64Url.url) {
    throw "Chrome for Testing API did not return a win64 download URL for version ${Version}."
}
$downloadUrl = $win64Url.url
Write-Host "Download URL: $downloadUrl"

# --- Download ----------------------------------------------------------------
try {
    Write-Host "Downloading Chrome for Testing $Version..."
    Invoke-WebRequest -Uri $downloadUrl -OutFile $tempArchive -UseBasicParsing
    if (-not (Test-Path -LiteralPath $tempArchive -PathType Leaf)) {
        throw "Downloaded archive not found at: $tempArchive"
    }
    $archiveSize = (Get-Item -LiteralPath $tempArchive).Length
    Write-Host "Downloaded archive: $archiveSize bytes"

    # --- Extract ----------------------------------------------------------------
    Write-Host "Extracting..."
    if (Test-Path -LiteralPath $tempExtract) {
        Remove-Item -LiteralPath $tempExtract -Recurse -Force
    }
    Expand-Archive -LiteralPath $tempArchive -DestinationPath $tempExtract

    # --- Install ------------------------------------------------------------------
    Write-Host "Installing to: $cftRoot"
    # The zip contains a top-level 'chrome-win64' folder
    $extractedDir = Join-Path $tempExtract 'chrome-win64'
    if (-not (Test-Path -LiteralPath $extractedDir -PathType Container)) {
        # Fallback: look for chrome.exe anywhere in the extraction
        $extractedDir = Get-ChildItem -LiteralPath $tempExtract -Recurse -Filter 'chrome.exe' -ErrorAction SilentlyContinue |
            ForEach-Object { $_.Directory.FullName } | Select-Object -First 1
        if (-not $extractedDir) {
            throw 'Extracted archive does not contain a chrome-win64 directory or chrome.exe.'
        }
    }

    # Create destination directories
    New-Item -ItemType Directory -Force -Path $cftRoot | Out-Null
    if (Test-Path -LiteralPath $chromeDir) {
        Remove-Item -LiteralPath $chromeDir -Recurse -Force
    }

    Move-Item -LiteralPath $extractedDir -Destination $chromeDir
    if (-not (Test-Path -LiteralPath $chromeExe -PathType Leaf)) {
        throw "chrome.exe not found at expected path after install: ${chromeExe}"
    }

    # --- Verify version -------------------------------------------------------------
    Write-Host "Verifying Chrome version..."
    $expectedVersion = "Chrome/$Version"
    $versionVerified = $false

    # Primary: read the version from the embedded manifest file (version metadata, no process needed)
    $manifestPath = Join-Path $chromeDir "${Version}.manifest"
    if (Test-Path -LiteralPath $manifestPath) {
        $manifestContent = Get-Content -LiteralPath $manifestPath -Raw
        if ($manifestContent -match "version='$Version'") {
            Write-Host "Version verified from manifest: $Version"
            $versionVerified = $true
        }
    }

    # Fallback: run chrome.exe --version
    if (-not $versionVerified) {
        try {
            $versionOutput = & $chromeExe --version 2>&1
            $versionText = $versionOutput.ToString().Trim()
            if ($versionText -eq $expectedVersion) {
                Write-Host "Chrome version: $versionText"
                $versionVerified = $true
            } else {
                Write-Host "Chrome version output: $versionText"
            }
        } catch {
            Write-Host "Unable to run chrome.exe --version: $_"
        }
    }

    if (-not $versionVerified) {
        throw "Version verification failed: expected '${expectedVersion}'"
    }

    Write-Host "SUCCESS: Chrome for Testing $Version installed and verified at $chromeExe"
}
finally {
    # --- Cleanup temporary files ------------------------------------------------------
    if (Test-Path -LiteralPath $tempArchive) {
        Remove-Item -LiteralPath $tempArchive -Force -ErrorAction SilentlyContinue
    }
    if (Test-Path -LiteralPath $tempExtract) {
        Remove-Item -LiteralPath $tempExtract -Recurse -Force -ErrorAction SilentlyContinue
    }
}
