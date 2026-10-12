# For the Deno crates that ship no license file inside the crate (listed in
# dist\deno-license-report\deno-v2.9.7-crates-without-license-file.txt), record the authors, repository and license
# declared in each crate's own Cargo.toml. Output: licenses\DENO-2.9.7-CRATE-AUTHORS.txt (installed with the app).
#   powershell -ExecutionPolicy Bypass -File third-party\deno\collect-missing-crate-authors.ps1
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$list = Join-Path $root 'dist\deno-license-report\deno-v2.9.7-crates-without-license-file.txt'
$out = Join-Path $root 'licenses\DENO-2.9.7-CRATE-AUTHORS.txt'
$headers = @{ 'User-Agent' = 'FramePier-license-report (https://github.com/dafka007/FramePier)' }
$tmp = Join-Path $env:TEMP 'fp-crate-authors'
$lines = New-Object System.Collections.ArrayList
[void]$lines.Add('Deno 2.9.7 - crates that ship no license file inside the crate')
[void]$lines.Add('=====================================================================')
[void]$lines.Add('For each crate below the license is the one declared in its Cargo.toml (standard texts for MIT, Apache-2.0 (with LLVM exception where stated), BSD, Zlib, BSL-1.0 and CC0 are in DENO-2.9.7-CRATE-LICENSES.txt). The authors and repository are copied from the same Cargo.toml.')
[void]$lines.Add('')
$count = 0
foreach ($entry in (Get-Content -LiteralPath $list | Where-Object { $_.Trim() })) {
    $name, $ver = $entry.Trim() -split ' ', 2
    $crate = Join-Path $env:TEMP "fp-$name-$ver.crate"
    try {
        Invoke-WebRequest -UseBasicParsing -Headers $headers -Uri "https://static.crates.io/crates/$name/$name-$ver.crate" -OutFile $crate
        if (Test-Path -LiteralPath $tmp) { Remove-Item -LiteralPath $tmp -Recurse -Force }
        New-Item -ItemType Directory -Force -Path $tmp | Out-Null
        & tar.exe -xzf $crate -C $tmp "$name-$ver/Cargo.toml"
        $toml = Get-Content -Raw -LiteralPath (Join-Path $tmp "$name-$ver\Cargo.toml")
        $authors = [regex]::Match($toml, '(?ms)^authors\s*=\s*\[(.*?)\]').Groups[1].Value -replace '\s+', ' '
        $repo = [regex]::Match($toml, '(?m)^repository\s*=\s*"([^"]*)"').Groups[1].Value
        $license = [regex]::Match($toml, '(?m)^license\s*=\s*"([^"]*)"').Groups[1].Value
        [void]$lines.Add("$name $ver")
        [void]$lines.Add("  license:    $license")
        [void]$lines.Add("  authors:    $(if ($authors.Trim()) { $authors.Trim() } else { '(none declared in Cargo.toml)' })")
        [void]$lines.Add("  repository: $(if ($repo) { $repo } else { '(none declared)' })")
        $count++
    } catch {
        [void]$lines.Add("$name $ver  FAILED: $($_.Exception.Message)")
        Write-Host "FAILED $name $ver : $($_.Exception.Message)"
    } finally {
        if (Test-Path -LiteralPath $crate) { Remove-Item -LiteralPath $crate -Force }
        Start-Sleep -Milliseconds 300
    }
}
Set-Content -LiteralPath $out -Value $lines -Encoding UTF8
Write-Host "Crates recorded: $count -> $out"
