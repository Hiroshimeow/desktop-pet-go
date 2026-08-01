$ErrorActionPreference = 'Stop'
$env:VIRTUAL_ENV = $null
$env:UV_LINK_MODE = 'copy'

$repo = Split-Path -Parent $PSScriptRoot
$sidecar = Join-Path $repo 'voice-sidecar'
$script = Join-Path $sidecar 'voice_sidecar.py'
$goLite = Join-Path $repo 'go-lite'

if (-not (Get-Command uv -ErrorAction SilentlyContinue)) {
    throw 'uv is required. Install uv, then rerun .\scripts\setup-voice.ps1'
}
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw 'Go is required. Install Go, then rerun .\scripts\setup-voice.ps1'
}

uv sync --project $sidecar --frozen --python 3.11
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

uv run --project $sidecar --frozen python $script --setup
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

uv run --project $sidecar --frozen python $script --list-devices
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Push-Location $goLite
try {
    go build -ldflags '-s -w -H=windowsgui' -o pet-lite.exe .
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}
finally {
    Pop-Location
}

Write-Host 'Run: .\go-lite\pet-lite.exe -assets .\assets -pet pet5 -voice'
