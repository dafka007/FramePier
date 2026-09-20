param(
    [Parameter(Mandatory=$true)][string]$Helper,
    [string]$DownloadPath = '',
    [string]$DataRoot = ''
)
$ErrorActionPreference = 'Stop'
# Integration diagnostic: reads Explorer's window state without manipulating it.
Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
public static class FramePierWindowProbe {
    [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr hwnd);
}
'@
$start = [Diagnostics.ProcessStartInfo]::new()
$start.FileName = (Resolve-Path -LiteralPath $Helper).Path
$start.Arguments = 'chrome-extension://kclnooibijmfenaldmpkffdbednfipkk/'
$start.UseShellExecute = $false
$start.CreateNoWindow = $true
$start.RedirectStandardInput = $true
$start.RedirectStandardOutput = $true
if ($DataRoot) { $start.Environment['LOCALAPPDATA'] = $DataRoot }
$process = [Diagnostics.Process]::Start($start)
function Request([string]$Command, [hashtable]$Fields = @{}) {
    $Fields.id = 'folder_smoke'
    $Fields.command = $Command
    $bytes = [Text.Encoding]::UTF8.GetBytes(($Fields | ConvertTo-Json -Compress))
    $prefix = [BitConverter]::GetBytes([uint32]$bytes.Length)
    $process.StandardInput.BaseStream.Write($prefix, 0, 4)
    $process.StandardInput.BaseStream.Write($bytes, 0, $bytes.Length)
    $process.StandardInput.BaseStream.Flush()
    $prefix = [byte[]]::new(4)
    $process.StandardOutput.BaseStream.ReadExactly($prefix, 0, 4)
    $size = [BitConverter]::ToUInt32($prefix, 0)
    if ($size -gt 1048576) { throw 'Invalid response size' }
    $body = [byte[]]::new($size)
    $process.StandardOutput.BaseStream.ReadExactly($body, 0, $size)
    return [Text.Encoding]::UTF8.GetString($body) | ConvertFrom-Json
}
function FolderWindows([string]$Path) {
    $shell = New-Object -ComObject Shell.Application
    foreach ($window in $shell.Windows()) {
        try {
            if ($window.Document.Folder.Self.Path -eq $Path) {
                [pscustomobject]@{ path = $Path; hwnd = $window.HWND; visible = [FramePierWindowProbe]::IsWindowVisible([IntPtr]$window.HWND) }
            }
        } catch {}
    }
}
try {
    $ping = Request 'ping'
    if (-not $ping.ok) { throw 'Ping failed' }
    if ($DownloadPath) {
        if (-not $DataRoot) { throw 'Changing settings requires an isolated DataRoot' }
        $saved = Request 'set_settings' @{ downloadPath=$DownloadPath; autoOpen=$false }
        if (-not $saved.ok) { throw $saved.error }
    }
    $settings = Request 'get_settings'
    $folder = $settings.settings.downloadPath
    $before = @(FolderWindows $folder)
    $response = Request 'open_download_folder'
    Start-Sleep -Seconds 3
    $after = @(FolderWindows $folder)
    [pscustomobject]@{ version=$ping.version; configuredPath=$folder; before=$before; response=$response; after=$after } | ConvertTo-Json -Depth 6
} finally {
    $process.StandardInput.Close()
    if (-not $process.WaitForExit(5000)) { $process.Kill() }
    $process.Dispose()
}
