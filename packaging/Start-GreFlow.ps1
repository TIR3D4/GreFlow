$ErrorActionPreference = 'Stop'
$binary = Join-Path $PSScriptRoot 'greflow-console.exe'
$dataPath = Join-Path $env:LOCALAPPDATA 'GreFlow\data'
$keyPath = Join-Path $env:APPDATA 'GreFlow\console-master.key'
New-Item -ItemType Directory -Force -Path $dataPath | Out-Null
$pidPath = Join-Path $dataPath 'console.pid'
if (Test-Path -LiteralPath $pidPath) {
    $existingId = 0
    if ([int]::TryParse((Get-Content -LiteralPath $pidPath -Raw).Trim(), [ref]$existingId)) {
        $existing = Get-Process -Id $existingId -ErrorAction SilentlyContinue
        if ($existing -and $existing.Path -eq $binary) {
            Start-Process 'http://127.0.0.1:8787'
            return
        }
    }
}
$arguments = @('--data', ('"' + $dataPath + '"'), '--key-file', ('"' + $keyPath + '"'), '--listen', '127.0.0.1:8787')
$process = Start-Process -FilePath $binary -ArgumentList $arguments -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $dataPath 'console.stdout.log') -RedirectStandardError (Join-Path $dataPath 'console.stderr.log')
$ready = $false
for ($attempt = 0; $attempt -lt 50; $attempt++) {
    if ($process.HasExited) { throw "Console could not start. Read $dataPath\console.stderr.log" }
    try { Invoke-RestMethod 'http://127.0.0.1:8787/api/v1/status' -TimeoutSec 2 | Out-Null; $ready = $true; break } catch { Start-Sleep -Milliseconds 200 }
}
if ($process.HasExited) { throw "Console could not start. Read $dataPath\console.stderr.log" }
Set-Content -LiteralPath $pidPath -Value $process.Id
if (-not $ready) { throw "Console is still starting. Read $dataPath\console.stderr.log; use Stop-GreFlow.ps1 to stop it." }
Write-Host "GreFlow is ready. Choose your panel password in the browser on first run."
Start-Process 'http://127.0.0.1:8787'
