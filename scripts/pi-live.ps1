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
Read GOAL.md and docs/voice-dictation/AUTONOMOUS_WORKFLOW.md before doing anything. Load the installed multi-model-orchestration skill from C:\Users\admin\.agents\skills\multi-model-orchestration\SKILL.md.
This is the same interactive Pi session, not a new project. Resume the approved full-delivery SubagentWorkflow or create and run it if missing; do not stop after a gate waiting for user review.
Keep one serial writer on develop, with independent read-only multi-model subagent reviewers. Preserve current changes, use verified upstream techniques, meet the complete acceptance contract and automatically advance through G14 after each validated checkpoint.
Only seek permission for real privacy/security, credentials, destructive action or a material reduction in product quality. No uncontrolled infinite loops, duplicate Pi workers or new branches.
'@

& $pi --provider cliproxy-g8 --model gpt-6.1-sol --thinking high --session-id $sessionId --name desktop-pet-osw-develop --approve $prompt
if ($LASTEXITCODE -ne 0) {
    Write-Warning "Pi exited with code $LASTEXITCODE. Session is persisted on disk."
}
