# Pi Coding — Desktop Pet OSW-parity implementation handoff

## Session directive

**FIRST: read root `GOAL.md` as the controlling GOAL CONTRACT.** It defines the one active milestone, evidence-based acceptance gate states, scope freezes, anti-overengineering tests, durable progress reporting and commit/push checkpoints. Update the live status/gates and checkpoint log only after real evidence; do not mark a gate PASS from source inspection alone. This instruction also applies to an already-running Pi session when it next reads this handoff. Confirm the actual active gate before coding.

You are the sole coding owner for Desktop Pet on md G6. Work directly in `E:\git-project\desktop-pet-lite` on **develop only**. Do not create a second branch/worktree or start a competing Pi coding session. Read `AGENTS.md` and all `docs/voice-dictation/{SPEC,PLAN,TASKS,BASELINE,REFERENCE_MATRIX}.md`, then the current product source. The user explicitly prioritizes proven mature implementations, correct OSW parity, reliability, performance, and UX over development cost/token/time. Use `AGENTS.md` SOLUTION QUALITY POLICY verbatim as governing rules, without substituting cheap approximations.

## Autonomous continuation (owner instruction)

Read `AUTONOMOUS_WORKFLOW.md` and the installed global `C:\Users\admin\.agents\skills\multi-model-orchestration\SKILL.md` before further coding. **Create, commit and actually run** a persistent `.agents/workflows/desktop-pet-delivery.js` through the `SubagentWorkflow` tool in this SAME Pi session; do not end a main turn after each gate. Delegate implementation to one writer subagent and independent review to read-only Opus/Sonnet subagents. An independent model review may approve a technical design/phase; owner review is NOT required for routine G05/P1.4 or G14 decisions. Escalate only real authorization/security/privacy/destructive/material scope issues. After passing a gate, continue to the next without waiting for a new user prompt. See `GOAL.md` for live evidence and the installed Pi subagents workflow documentation for real API syntax.

## Objective

Deliver a high-quality, local-first, Windows dictation/live transcript integrated into the existing Go desktop pet. UX is one pet with click text chat, click mic voice conversation, configurable hold-to-talk and F5 toggle dictation. Dictation shows stable/unstable provisional transcript while speaking, final-only exactly-once insert at the original safe caret, does not auto-submit and never routes dictation through agent/TTS/shell. CPU ThinkBook default, optional benchmarked LAN NVIDIA GPUs, cloud only explicit opt-in. Existing voice/ZeroClaw/reader/animation must not regress.

## Mandatory initial work (research first)

1. Confirm `git status --short --branch` clean develop, remote upstream and exact HEAD. **Mandatory regression remediation first:** `cd go-lite; go test -count=1 ./...` on canonical root currently fails `TestAutoDiscoverAndMergedPetManifest` because ignored historical `assets/pets/pet1..pet4` are present and the test incorrectly assumes default `pet5`. A clean isolated worktree passed previously. Fix the test's fixture isolation and assert discovery semantics deterministically; never delete or hide user asset directories just to force green. Then full Go suite plus `uv sync --frozen` / sidecar unittest, real Windows GUI/mic when feasible. Record actual commands and outputs.
2. Inspect `Starmel/OpenSuperWhisper` source and pin SHA. **Map 100% of relevant dictation behavior/technical flow**: hotkey semantics, recording state, microphone switching, capture/pre-roll/VAD, resident model lifecycle, partial/stable/final (and identify capabilities not actually present upstream), cancellation, retries, UI/overlay, clipboard/caret/focus insertion, dictation history, glossary, privacy and settings. Include upstream file/function/SHA/license references.
3. Inspect `cjpais/Handy` and `OpenWhispr/openwhispr` source and releases; try real Windows offline Handy flow and its CLI. Examine `local-live-caption-engine` already on G6 before writing ASR partial/stitching logic. Prefer supported integration or reuse of mature techniques. No blind cloning/wholesale embed where interfaces do not meet contract.
4. Reconcile old July changes preserved under `archive/pre-develop-20261011`: compare before porting. Inspect `docs/voice-dictation/LEGACY_INVENTORY.md` and do not delete any dirty sibling clone. Record decisions.
5. Update spec/tasks/plan and reference evidence with measured G6 behavior and a concrete implementation owner/file boundary. Decide if external Handy API can really satisfy all hotkey, partial, focus and process lifecycle requirements.
6. Execute implementation incrementally on develop after evidence supports design; commit every independently testable slice. Do not stop at mock/design where actual acceptance was requested. Run Go and Python regression plus real GUI/clipboard/target tests when accessible. Mark genuinely blocked acceptance truthfully with reproduction and next action.
7. At any material fork where the only simpler alternative loses reliability/UX/feature parity, implement the proven correct design instead. Ask user approval only for real authorization boundaries (secrets, external cloud upload, destructive operations or a known permanent capability tradeoff). No unlimited goal loops or hidden background costs.

## Reporting & persistence

- Keep `TASKS.md` statuses accurate, no reported PASS without evidence.
- Use actual source-relative paths in reports, avoid hardcoding unverified Windows home folders. This device home is `C:\Users\admin`, not `C:\Users\DuongNH66`.
- Commit locally continuously, record HEAD and acceptance outputs in `docs/voice-dictation`, keep `git status` clean at milestones. Push to `origin/develop` only after checks or if needed to checkpoint collaborative provenance.
- Keep the same native Pi session ID/name so operator can inspect/resume; no parallel workers, no new branch.
- Final response: completed items, commit SHAs, tests and actual limitations, outstanding exact acceptance gates.
