param(
    [switch]$LocalChat
)

$ErrorActionPreference = 'Stop'
$env:VIRTUAL_ENV = $null
$env:UV_LINK_MODE = 'copy'

$repo = Split-Path -Parent $PSScriptRoot
$sidecar = Join-Path $repo 'voice-sidecar'
$script = Join-Path $sidecar 'voice_sidecar.py'
$goLite = Join-Path $repo 'go-lite'

function Get-VerifiedDownload {
    param(
        [string]$Url,
        [string]$Path,
        [string]$Sha256
    )
    if (Test-Path $Path) {
        $existing = (Get-FileHash -Algorithm SHA256 $Path).Hash.ToLowerInvariant()
        if ($existing -eq $Sha256) {
            Write-Host "Using verified $Path"
            return
        }
        Remove-Item -Force $Path
    }
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Path) | Out-Null
    Invoke-WebRequest -Uri $Url -OutFile $Path
    $actual = (Get-FileHash -Algorithm SHA256 $Path).Hash.ToLowerInvariant()
    if ($actual -ne $Sha256) {
        Remove-Item -Force $Path
        throw "SHA-256 mismatch for $Path`: expected $Sha256, got $actual"
    }
}

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

if ($LocalChat) {
    $chatDir = Join-Path $repo '.voice\chat'
    $runtimeZip = Join-Path $chatDir 'llama-b10223-bin-win-cpu-x64.zip'
    Get-VerifiedDownload `
        -Url 'https://github.com/ggml-org/llama.cpp/releases/download/b10223/llama-b10223-bin-win-cpu-x64.zip' `
        -Path $runtimeZip `
        -Sha256 '74c1ded0512818d98b51940bf9150e16da8ed79cf0cbe8d85788e01cdd00ff67'
    $runtimeDir = Join-Path $chatDir 'llama-b10223'
    New-Item -ItemType Directory -Force -Path $runtimeDir | Out-Null
    Expand-Archive -Force -Path $runtimeZip -DestinationPath $runtimeDir

    $modelPath = Join-Path $chatDir 'gemma-3-4b-it-Q4_K_M.gguf'
    Get-VerifiedDownload `
        -Url 'https://huggingface.co/ggml-org/gemma-3-4b-it-GGUF/resolve/d0976223747697cb51e056d85c532013931fe52e/gemma-3-4b-it-Q4_K_M.gguf?download=true' `
        -Path $modelPath `
        -Sha256 '882e8d2db44dc554fb0ea5077cb7e4bc49e7342a1f0da57901c0802ea21a0863'
}

Push-Location $goLite
try {
    go build -ldflags '-s -w -H=windowsgui' -o pet-lite.exe .
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}
finally {
    Pop-Location
}

Write-Host 'Run: .\go-lite\pet-lite.exe -assets .\assets -pet pet5 -voice'
if ($LocalChat) {
    Write-Host 'Local chat: .\.voice\chat\llama-b10223\llama-server.exe -m .\.voice\chat\gemma-3-4b-it-Q4_K_M.gguf --host 127.0.0.1 --port 8080 --alias desktop-pet --ctx-size 2048 --threads 8 --threads-batch 8 --parallel 1 --reasoning off'
}
