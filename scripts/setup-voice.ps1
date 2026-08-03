$ErrorActionPreference = 'Stop'
$env:VIRTUAL_ENV = $null
$env:UV_LINK_MODE = 'copy'

$repo = Split-Path -Parent $PSScriptRoot
$sidecar = Join-Path $repo 'voice-sidecar'
$script = Join-Path $sidecar 'voice_sidecar.py'
$goLite = Join-Path $repo 'go-lite'
$zeroClawRoot = Join-Path $repo '.voice\zeroclaw'
$zeroClawDir = Join-Path $zeroClawRoot 'v0.8.3'
$zeroClawZip = Join-Path $zeroClawRoot 'zeroclaw-x86_64-pc-windows-msvc-v0.8.3.zip'
$zeroClaw = Join-Path $zeroClawDir 'zeroclaw.exe'
$zeroClawConfig = Join-Path $env:USERPROFILE '.zeroclaw\config.toml'
$petToken = Join-Path $zeroClawRoot 'pet-token.txt'

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

function Invoke-NativeChecked {
    param(
        [string]$FilePath,
        [string[]]$Arguments
    )
    & $FilePath @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$FilePath $($Arguments -join ' ') failed with exit code $LASTEXITCODE"
    }
}

function Test-IsAdministrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = [Security.Principal.WindowsPrincipal]$identity
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Invoke-ZeroClawServiceChecked {
    param(
        [string[]]$Arguments,
        [switch]$AllowFailure
    )
    if (Test-IsAdministrator) {
        & $zeroClaw @Arguments
        $exitCode = $LASTEXITCODE
    }
    else {
        $process = Start-Process -FilePath $zeroClaw -Verb RunAs -ArgumentList $Arguments -PassThru -Wait
        $exitCode = $process.ExitCode
    }
    if ($exitCode -ne 0 -and -not $AllowFailure) {
        throw "$zeroClaw $($Arguments -join ' ') failed with exit code $exitCode"
    }
    return $exitCode
}

function Stop-StaleZeroClawGateway {
    $connection = Get-NetTCPConnection -State Listen -LocalPort 42617 -ErrorAction SilentlyContinue | Select-Object -First 1
    if (-not $connection) {
        return
    }
    $process = Get-Process -Id $connection.OwningProcess -ErrorAction SilentlyContinue
    if (-not $process -or $process.ProcessName -ne 'zeroclaw' -or $process.Path -ne $zeroClaw) {
        throw "port 42617 is already owned by an unexpected process (pid=$($connection.OwningProcess)); stop it before rerunning setup"
    }
    Stop-Process -Id $process.Id -Force
    $deadline = [DateTime]::UtcNow.AddSeconds(5)
    while ([DateTime]::UtcNow -lt $deadline) {
        if (-not (Get-NetTCPConnection -State Listen -LocalPort 42617 -ErrorAction SilentlyContinue)) {
            return
        }
        Start-Sleep -Milliseconds 100
    }
    throw "stale ZeroClaw Gateway pid=$($process.Id) did not release port 42617"
}

function Repair-LegacyZeroClawConfig {
    param([string]$Path)
    if (-not (Test-Path -PathType Leaf $Path)) {
        return
    }

    $text = [System.IO.File]::ReadAllText($Path)
    $fixed = [regex]::Replace($text, '(?ms)^\[plugins\.entries\]\s*(?=^\[|\z)', '')
    $lines = $fixed -split "`r?`n"
    $section = ''
    $kept = New-Object System.Collections.Generic.List[string]
    foreach ($line in $lines) {
        if ($line -match '^\s*\[(.+)\]\s*$') {
            $section = $Matches[1]
            $kept.Add($line)
            continue
        }
        if ($section -eq 'security.otp' -and $line -match '^\s*(challenge_delivery|challenge_timeout_secs)\s*=') {
            continue
        }
        $kept.Add($line)
    }
    $fixed = $kept -join [Environment]::NewLine
    if ($fixed -eq $text) {
        return
    }

    $backup = "$Path.phase10-v083.bak"
    if (-not (Test-Path $backup)) {
        Copy-Item -LiteralPath $Path -Destination $backup
    }
    [System.IO.File]::WriteAllText($Path, $fixed, [System.Text.UTF8Encoding]::new($false))
    Write-Host "Repaired legacy ZeroClaw v0.1.x config fields; backup: $backup"
}

function Get-ZeroClawConfigValue {
    param([string]$Path)
    $value = (& $zeroClaw config get $Path | Out-String).Trim()
    if ($LASTEXITCODE -ne 0) {
        throw "zeroclaw config get $Path failed with exit code $LASTEXITCODE"
    }
    return $value
}

function Try-GetZeroClawConfigValue {
    param([string]$Path)
    $value = (& $zeroClaw config get $Path 2>$null | Out-String).Trim()
    if ($LASTEXITCODE -ne 0) {
        return ''
    }
    return $value
}

function Test-ZeroClawModelProviderReady {
    param([string]$Reference)
    if (-not $Reference) {
        return $false
    }
    if (-not $Reference.StartsWith('custom.')) {
        return $true
    }
    $model = Try-GetZeroClawConfigValue "providers.models.$Reference.model"
    $apiKey = Try-GetZeroClawConfigValue "providers.models.$Reference.api_key"
    return [bool]($model -and $apiKey)
}

if (-not (Get-Command uv -ErrorAction SilentlyContinue)) {
    throw 'uv is required. Install uv, then rerun .\scripts\setup-voice.ps1'
}
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw 'Go is required. Install Go, then rerun .\scripts\setup-voice.ps1'
}

uv sync --project $sidecar --frozen --inexact --python 3.11
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
$venvPython = Join-Path $sidecar '.venv\Scripts\python.exe'
if (-not (Test-Path -PathType Leaf $venvPython)) {
    throw "voice-sidecar Python missing after uv sync: $venvPython"
}
uv pip install --python $venvPython 'kokoro==0.9.4' 'misaki[ja]==0.9.4'
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
& $venvPython -c "from pathlib import Path; import sys, unidic; sys.exit(0 if (Path(unidic.DICDIR) / 'dicrc').is_file() else 1)"
if ($LASTEXITCODE -ne 0) {
    & $venvPython -m unidic download
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

uv run --project $sidecar --no-sync python $script --setup
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
uv run --project $sidecar --no-sync python $script --list-devices
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Get-VerifiedDownload `
    -Url 'https://github.com/zeroclaw-labs/zeroclaw/releases/download/v0.8.3/zeroclaw-x86_64-pc-windows-msvc.zip' `
    -Path $zeroClawZip `
    -Sha256 '00da56062ff3f96f7dae20d9cc471e8e63e569dabffea6fda45793f5728a4db5'
if (-not (Test-Path -PathType Leaf $zeroClaw)) {
    if (Test-Path $zeroClawDir) {
        Remove-Item -Recurse -Force $zeroClawDir
    }
    New-Item -ItemType Directory -Force -Path $zeroClawDir | Out-Null
    Expand-Archive -Force -Path $zeroClawZip -DestinationPath $zeroClawDir
}
if (-not (Test-Path -PathType Leaf $zeroClaw)) {
    throw "ZeroClaw executable missing after verified v0.8.3 setup: $zeroClaw"
}
$zeroClawVersion = (& $zeroClaw --version | Out-String).Trim()
if ($LASTEXITCODE -ne 0 -or $zeroClawVersion -notmatch '^zeroclaw 0\.8\.3(?:\s|$)') {
    throw "unexpected ZeroClaw binary version: $zeroClawVersion"
}

Repair-LegacyZeroClawConfig $zeroClawConfig
Invoke-NativeChecked $zeroClaw @('config', 'migrate', '--json')
$aliases = @(& $zeroClaw agents list | ForEach-Object { $_.Trim() } | Where-Object { $_ })
if ($LASTEXITCODE -ne 0) {
    throw "zeroclaw agents list failed with exit code $LASTEXITCODE"
}
$sourceAgent = $aliases | Where-Object { $_ -ne 'pet' } | Select-Object -First 1
if ($aliases -notcontains 'pet') {
    if (-not $sourceAgent) {
        throw 'ZeroClaw has no configured source agent whose model/runtime profiles can be reused for pet'
    }
    Invoke-NativeChecked $zeroClaw @('agents', 'create', 'pet')
}

$providerCandidates = New-Object System.Collections.Generic.List[string]
$currentPetProvider = Try-GetZeroClawConfigValue 'agents.pet.model_provider'
$providerCandidates.Add('custom.default')
if ($currentPetProvider) { $providerCandidates.Add($currentPetProvider) }
foreach ($alias in $aliases | Where-Object { $_ -ne 'pet' }) {
    $candidate = Try-GetZeroClawConfigValue "agents.$alias.model_provider"
    if ($candidate) { $providerCandidates.Add($candidate) }
}
$petProvider = $providerCandidates | Select-Object -Unique | Where-Object { Test-ZeroClawModelProviderReady $_ } | Select-Object -First 1
if (-not $petProvider) {
    throw 'ZeroClaw has no dispatchable model provider for agent pet; configure provider credentials in ZeroClaw, then rerun setup'
}
if ($currentPetProvider -ne $petProvider) {
    Invoke-NativeChecked $zeroClaw @('config', 'set', 'agents.pet.model_provider', $petProvider, '--no-interactive')
}

foreach ($name in @('risk_profile', 'runtime_profile')) {
    $petValue = Get-ZeroClawConfigValue "agents.pet.$name"
    if (-not $petValue) {
        if (-not $sourceAgent) {
            throw "agents.pet.$name is empty and no configured source agent exists"
        }
        $sourceValue = Get-ZeroClawConfigValue "agents.$sourceAgent.$name"
        if (-not $sourceValue) {
            throw "agents.$sourceAgent.$name is empty; cannot configure agents.pet.$name"
        }
        Invoke-NativeChecked $zeroClaw @('config', 'set', "agents.pet.$name", $sourceValue, '--no-interactive')
    }
}
$petRuntimeProfile = Get-ZeroClawConfigValue 'agents.pet.runtime_profile'
$petAgentic = Try-GetZeroClawConfigValue "runtime_profiles.$petRuntimeProfile.agentic"
if ($petAgentic -ne 'true') {
    Invoke-NativeChecked $zeroClaw @('config', 'set', 'runtime_profiles.pet.agentic', 'true', '--no-interactive')
    Invoke-NativeChecked $zeroClaw @('config', 'set', 'agents.pet.runtime_profile', 'pet', '--no-interactive')
}
& $zeroClaw agent -a pet -m 'Reply only with OK.' *> $null
if ($LASTEXITCODE -ne 0) {
    throw 'ZeroClaw agent pet provider/model self-check failed; repair ZeroClaw-owned provider/model configuration, then rerun setup'
}
Invoke-NativeChecked $zeroClaw @('config', 'set', 'gateway.host', '127.0.0.1', '--no-interactive')
Invoke-NativeChecked $zeroClaw @('config', 'set', 'gateway.port', '42617', '--no-interactive')
Invoke-NativeChecked $zeroClaw @('config', 'set', 'gateway.require_pairing', 'true', '--no-interactive')

if (-not (Test-Path -PathType Leaf $petToken) -or -not ([System.IO.File]::ReadAllText($petToken).Trim())) {
    New-Item -ItemType Directory -Force -Path $zeroClawRoot | Out-Null
    $pairOut = Join-Path $env:TEMP ("desktop-pet-zeroclaw-pair-$([guid]::NewGuid().ToString('N')).out")
    $pairErr = "$pairOut.err"
    $pairProcess = $null
    try {
        $pairProcess = Start-Process -FilePath $zeroClaw -ArgumentList @('gateway') -WorkingDirectory $zeroClawDir -RedirectStandardOutput $pairOut -RedirectStandardError $pairErr -PassThru -WindowStyle Hidden
        $pairCode = $null
        $deadline = [DateTime]::UtcNow.AddSeconds(15)
        while ([DateTime]::UtcNow -lt $deadline -and -not $pairCode) {
            Start-Sleep -Milliseconds 100
            if ($pairProcess.HasExited) {
                $details = if (Test-Path $pairErr) { Get-Content $pairErr -Raw } else { '' }
                throw "ZeroClaw pairing gateway exited early: $details"
            }
            if (Test-Path $pairOut) {
                $pairText = Get-Content $pairOut -Raw -ErrorAction SilentlyContinue
                if ($null -eq $pairText) { $pairText = '' }
                $match = [regex]::Match($pairText, 'X-Pairing-Code:\s*(\d+)')
                if ($match.Success) {
                    $pairCode = $match.Groups[1].Value
                }
            }
        }
        if (-not $pairCode) {
            throw 'ZeroClaw pairing code was not emitted within 15 seconds'
        }
        $pairResponse = Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:42617/pair' -Headers @{ 'X-Pairing-Code' = $pairCode }
        if (-not $pairResponse.paired -or -not $pairResponse.persisted -or -not $pairResponse.token) {
            throw 'ZeroClaw localhost pairing did not return a persisted bearer token'
        }
        [System.IO.File]::WriteAllText($petToken, "$($pairResponse.token)`n", [System.Text.UTF8Encoding]::new($false))
    }
    finally {
        if ($pairProcess -and -not $pairProcess.HasExited) {
            Stop-Process -Id $pairProcess.Id -Force
            $pairProcess.WaitForExit()
        }
        Remove-Item -Force $pairOut, $pairErr -ErrorAction SilentlyContinue
    }
}

Invoke-ZeroClawServiceChecked @('service', 'stop') -AllowFailure | Out-Null
Start-Sleep -Milliseconds 300
Stop-StaleZeroClawGateway
Invoke-ZeroClawServiceChecked @('service', 'install') | Out-Null
Invoke-ZeroClawServiceChecked @('service', 'start') | Out-Null
$healthy = $false
$deadline = [DateTime]::UtcNow.AddSeconds(15)
while ([DateTime]::UtcNow -lt $deadline -and -not $healthy) {
    Start-Sleep -Milliseconds 200
    try {
        $response = Invoke-WebRequest -UseBasicParsing -Uri 'http://127.0.0.1:42617/health' -TimeoutSec 2
        $healthy = $response.StatusCode -eq 200
    }
    catch {
        $healthy = $false
    }
}
if (-not $healthy) {
    throw 'ZeroClaw daemon did not become healthy at http://127.0.0.1:42617/health'
}
$serviceTask = Get-ScheduledTask -TaskName 'ZeroClaw Daemon' -ErrorAction SilentlyContinue
if (-not $serviceTask -or $serviceTask.State -ne 'Running') {
    throw "ZeroClaw Daemon task is not running after setup (state=$($serviceTask.State))"
}
Invoke-NativeChecked $zeroClaw @('service', 'status')

Push-Location $goLite
try {
    go build -ldflags '-s -w -H=windowsgui' -o pet-lite.exe .
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}
finally {
    Pop-Location
}

Write-Host "ZeroClaw ready: $zeroClawVersion, agent=pet, gateway=127.0.0.1:42617"
Write-Host 'Run: .\go-lite\pet-lite.exe -assets .\assets -pet pet5 -voice'
