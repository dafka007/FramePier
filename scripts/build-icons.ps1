param(
    [Parameter(Mandatory = $true)]
    [string]$Source
)

$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing

$projectRoot = Split-Path -Parent $PSScriptRoot
$assetsDir = Join-Path $projectRoot 'assets'
$extensionIcons = Join-Path $projectRoot 'extension\icons'
New-Item -ItemType Directory -Force -Path $assetsDir, $extensionIcons | Out-Null

$sourceImage = [System.Drawing.Image]::FromFile((Resolve-Path -LiteralPath $Source))
try {
    $entries = @()
    foreach ($size in @(16, 32, 48, 128, 256)) {
        $bitmap = New-Object System.Drawing.Bitmap($size, $size, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
        try {
            $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
            try {
                $graphics.Clear([System.Drawing.Color]::Transparent)
                $graphics.CompositingMode = [System.Drawing.Drawing2D.CompositingMode]::SourceCopy
                $graphics.CompositingQuality = [System.Drawing.Drawing2D.CompositingQuality]::HighQuality
                $graphics.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
                $graphics.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::HighQuality
                $graphics.DrawImage($sourceImage, 0, 0, $size, $size)
            } finally {
                $graphics.Dispose()
            }
            $memory = New-Object System.IO.MemoryStream
            $bitmap.Save($memory, [System.Drawing.Imaging.ImageFormat]::Png)
            $bytes = $memory.ToArray()
            $memory.Dispose()
            $entries += [pscustomobject]@{ Size = $size; Bytes = $bytes }
            if ($size -le 128) {
                [System.IO.File]::WriteAllBytes((Join-Path $extensionIcons "icon$size.png"), $bytes)
            }
        } finally {
            $bitmap.Dispose()
        }
    }

    $icoPath = Join-Path $assetsDir 'FramePier.ico'
    $stream = [System.IO.File]::Create($icoPath)
    $writer = New-Object System.IO.BinaryWriter($stream)
    try {
        $writer.Write([uint16]0)
        $writer.Write([uint16]1)
        $writer.Write([uint16]$entries.Count)
        $offset = 6 + (16 * $entries.Count)
        foreach ($entry in $entries) {
            $dimension = if ($entry.Size -ge 256) { 0 } else { $entry.Size }
            $writer.Write([byte]$dimension)
            $writer.Write([byte]$dimension)
            $writer.Write([byte]0)
            $writer.Write([byte]0)
            $writer.Write([uint16]1)
            $writer.Write([uint16]32)
            $writer.Write([uint32]$entry.Bytes.Length)
            $writer.Write([uint32]$offset)
            $offset += $entry.Bytes.Length
        }
        foreach ($entry in $entries) { $writer.Write($entry.Bytes) }
    } finally {
        $writer.Dispose()
        $stream.Dispose()
    }
} finally {
    $sourceImage.Dispose()
}

Write-Host "Created FramePier icons in $extensionIcons and $assetsDir"
