# Launch one interactive Pi Coding owner in Windows Terminal.
# The session ID and governing contract live in GOAL.md.
[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
Set-Location -LiteralPath $repoRoot

if ((git rev-parse --show-toplevel).Trim().Replace("\", "/") -ne $repoRoot.Replace("\", "/")) {
    throw "Unexpected Git root. Aborting Pi launch."
}
if ((git branch --show-current).Trim() -ne "develop") {
    throw "Pi must work on develop. Aborting Pi launch."
}

$goal = Get-Content -LiteralPath (Join-Path $repoRoot "GOAL.md") -Raw
if ($goal -notmatch '(?m)^\*\*Pi session ID:\*\* `([0-9a-fA-F-]{36})`') {
    throw "Cannot resolve Pi session ID from GOAL.md."
}
$sessionId = $Matches[1]
$sessionRoot = Join-Path $env:USERPROFILE ".pi\agent\sessions"
$existingSession = Get-ChildItem -LiteralPath $sessionRoot -Filter "*$sessionId.jsonl" -File -Recurse -ErrorAction SilentlyContinue | Select-Object -First 1
if (-not $existingSession) {
    throw "Existing Pi session $sessionId was not found. Refusing to create a new one."
}

# A second active Pi on the same session risks corrupting turns and Git.
$active = @(Get-CimInstance Win32_Process | Where-Object {
    $_.Name -eq "node.exe" -and
    $_.CommandLine -like "*pi-coding-agent*" -and
    $_.CommandLine -like "*$sessionId*"
})
if ($active.Count -gt 0) {
    $owners = ($active | ForEach-Object { "$($_.ProcessId)" }) -join ", "
    throw "Pi session is already open (PIDs: $owners). Use the existing terminal."
}

$pi = (Get-Command pi.cmd -ErrorAction Stop).Source
Write-Host "Desktop Pet Pi LIVE / develop"
Write-Host "Repo: $repoRoot"
Write-Host "Session: $sessionId"
Write-Host "Existing uncommitted files are preserved:"
git status --short

$prompt = @'
Read GOAL.md first and treat it as the authoritative scope and progress contract.
This is the same Pi session, now interactive in Windows Terminal; continue the interrupted task, do not restart planning from scratch.
Preserve existing uncommitted docs/voice-dictation/SOURCE_AUDIT.md and BENCHMARK.md; inspect them before modifying or committing.
Work only on develop, one active gate at a time. Start from the verified current G03/G04 evidence. Complete upstream OSW/Handy/OpenWhispr source mapping and Windows validations, then proceed through the approved implementation gates.
Use proven long-term solutions. Maintain full required parity; do not invent cheap approximations, unrelated subsystems, speculative features or new branches.
Commit verified milestones locally and push develop for GOAL.md visibility. Never call an untested gate PASS. Stop and ask only for authorization, privacy, material capability tradeoffs or destructive operations.
'@

& $pi --provider cliproxy-g8 --model gpt-6.1-sol --thinking high --session-id $sessionId --name desktop-pet-osw-develop --approve $prompt
if ($LASTEXITCODE -ne 0) {
    Write-Warning "Pi exited with code $LASTEXITCODE. Session is persisted on disk."
}
