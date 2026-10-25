$ErrorActionPreference = 'Stop'
$pidPath = Join-Path $env:LOCALAPPDATA 'GreFlow\data\console.pid'
$expectedBinary = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot 'greflow-console.exe'))
if (Test-Path -LiteralPath $pidPath) {
    $consoleProcessId = [int](Get-Content -LiteralPath $pidPath)
    $process = Get-Process -Id $consoleProcessId -ErrorAction SilentlyContinue
    if ($process -and $process.Path -eq $expectedBinary) { Stop-Process -Id $consoleProcessId }
    Remove-Item -LiteralPath $pidPath
}
Write-Host 'Console stopped. Remote tunnels and systemd schedules are unchanged.'
