# Live Dictation & Transcript Implementation Plan

> **For agentic workers:** Use a fresh-context review gate and task-wise implementation. The operator authorized starting Pi Coding on `develop` on 2026-10-11. Complete Phase 1 evidence gates before changing product code, then execute Phase 2 incrementally without a weaker approximation. Follow SPEC.md, TASKS.md and AGENTS.md.

**Goal:** Add mature OSW-like local dictation UX and real live transcript to Desktop Pet by reusing proven Windows implementations and the existing voice sidecar.

**Architecture:** Win32 pet owns modes/hotkeys/target-safe insertion and overlay, existing Python voice sidecar owns sound capture/VAD/ASR. Benchmark Handy as a controllable external runtime; fallback is porting proven behaviors into existing architecture. Cloud off by default.

**Tech Stack:** Go 1.26 Win32, Python 3.11, uv, sounddevice/WebRTC VAD, faster-whisper. Optional external Handy binary, local-live-caption algorithms. No new platform mandated.

## Global constraints

Develop directly on `develop`; **do not create other feature branches/worktrees**. If the operator explicitly requests a temporary worktree later, use only `.wt/<name>` inside the product repo. Former dirty master state was preserved at tag `archive/pre-develop-20261011` (commit `215ff2f`) and is subject to selective reconciliation, not blind cherry-pick. The former voice branch at `61d96af` is part of `develop`. Re-fetch without resetting/overwriting uncommitted user work on implementation. Existing `docs/VOICE_REQUIREMENTS.md` privacy/cancel/nonblocking/command safety constraints preserved. Historical voice-roadmap file/test budget applies to historical phases; OSW parity feature requires separate owner review, do not silently downgrade acceptance or rewrite the historical contract. Solution quality and mature upstream reuse outrank development cost. All tasks use evidence, not inferred pass.

## Phase 1 — verification/research/decision (no feature edits)

### P1.1 Repo baseline
**Files:** `go-lite/voice_windows.go`, `go-lite/voice_controls_windows.go`, `voice-sidecar/voice_sidecar.py`, `docs/VOICE_REQUIREMENTS.md`; output `BASELINE.md`.

- [x] Fetch origin, establish ancestry and canonical voice branch, preserve dirty master.
- [x] Create and later consolidate the temporary `.wt/live-transcript-phase1-spec` into canonical `develop`; no sibling checkout deleted.
- [x] Clean isolated worktree Go regression: `cd go-lite; go test ./...` PASS (exit 0). Canonical root regression FAILS in a fixture-dependent test because ignored historical `assets/pets/pet1..pet4` affect autodiscovery. Mandatory **first coding task**: make `TestAutoDiscoverAndMergedPetManifest` independent of ambient assets (synthetic isolated fixture), without deleting/moving user assets. Re-run full suite and record evidence.
- [x] Install isolated Python dependencies: `cd voice-sidecar; uv sync --frozen` exit 0, then `uv run --no-sync python -m unittest -q test_voice_sidecar.py` exit 0 (18 tests PASS).
- [ ] Build Windows GUI using existing `scripts/build-windows.ps1` only after inspecting its side effects; run mic + pet click/drag/voice/UI smoke with consent.
- [x] Inventory known sibling clones/checkouts in `LEGACY_INVENTORY.md`; they contain user data/dirty changes and are not safely removable without further review. Do not delete them automatically.

### P1.2 Upstream technical mapping
**Files:** `REFERENCE_MATRIX.md`.

- [ ] Pin specific SHAs/licenses for OSW upstream, Handy, OpenWhispr, OpenSuperWhisper-Windows, Whisper Local, local-live-caption-engine.
- [ ] Trace source-level hotkey states, capture start/stop, model warm/resident, VAD/pre-roll, partial/stable/final, cleanup, cancellation, target focus, clipboard insertion, overlays, history, privacy.
- [ ] Hands-on Handy on G6: launch, offline model and microphone, `handy --toggle-transcription`, repeated sessions; verify process ownership and actual text insertion. Determine whether it exposes stable partial events/API. Do not treat CLI claims alone as proof.
- [ ] Compare OpenWhispr only on material Handy gaps; document evidence/maintenance/license/security tradeoffs.
- [ ] Complete matrix with `PRESERVE/REUSE_EXTERNAL/PORT/GAP/OUT_OF_SCOPE`, concrete source path & test.

### P1.3 Accuracy/performance measurements
**Files:** `BENCHMARK.md` (only after measurement; no made-up values).
- [ ] Consent-based anonymized WAV corpus: VI short/long, VI+EN terms, punctuation, silence/noisy, early syllable, interrupted utterance.
- [ ] Local CPU model warm/cold repeat trials; p50/p95 release-to-final, WER/CER/terminology, CPU/RSS. Keep raw fixtures local unless approved.
- [ ] Compare streaming segmenter pre-roll, stable partial and stitching algorithms using deterministic fixtures.
- [ ] Optional A5000/2080Ti only if actual network/compute latency measured; Gemini only with explicit cloud/data authorization.

### P1.4 Architecture sign-off
**Files:** `SPEC.md`, `PLAN.md`, `TASKS.md`, `BASELINE.md`, `REFERENCE_MATRIX.md`.
- [ ] Review key target safety and clipboard regression risks, model privacy, fallback behavior and upstream legal notices.
- [ ] Demonstrate reference decision: Handy external vs sidecar port; avoid CLI toggle being mistaken for embed API.
- [ ] Confirm relevant Phase 1 research/architecture evidence and A01–A10 test design before source edits. Proceed to Phase 2 only when evidence gates support the chosen approach; ask the operator about material loss of parity, cloud-data authorization or destructive cleanup.

## Phase 2 — implementation tasks (deferred until Phase 1 acceptance)

### P2.1 Dictation lifecycle/hotkeys (Go)
**Candidate files:** `go-lite/voice_windows.go`, `go-lite/voice_windows_test.go`, `go-lite/internal/voice/session.go`, platform-specific hotkey module only if needed.
- [ ] Add failing tests for F5 start/stop, hold press/release, repeat, Esc, stale result, invalid transitions.
- [ ] Add distinct dictation mode and monotonically unique session ID to existing controller; no deterministic command/agent/TTS routing.
- [ ] Windows hotkey conflict controls, disabled-mode behavior; run focused and full `go test ./...`; commit with evidence.

### P2.2 Streaming/stabilization (Python + Go adapter)
**Candidate files:** `voice-sidecar/voice_sidecar.py`, `voice-sidecar/test_voice_sidecar.py`, `go-lite/voice_windows.go`.
- [ ] Failing synthetic tests for first-word pre-roll, partial/final order, overlap dedupe, missing/silent audio, cancellation.
- [ ] Versioned JSONL session-scoped provisional/stable/final protocol, bounded ASR queue, resident model with cancellation; expose partial UI-only.
- [ ] Validate real Vietnamese/English and CPU performance; Go/Python regression; commit.

### P2.3 Focus-safe paste and overlay (Win32)
**Candidate files:** new `go-lite/dictation_windows.go`, tests, `go-lite/voice_controls_windows.go`.
- [ ] Test original HWND/caret capture, switched app, elevated/secure field, clipboard update race, rich text restore and exactly-once insertion.
- [ ] Implement proven Windows focus/insertion approach only after comparing Handy/OSW source. No auto-submit or accidental command execution.
- [ ] Add non-focus-stealing DPI-aware status overlay/pet animation, run Notepad, browser/ChatGPT, VS Code, terminal real smoke; commit.

### P2.4 User-facing interaction / optional engine
**Candidate files:** existing pet input/menu/settings owners selected after reviewing source.
- [ ] Connect pet click chat/mic, F5/hold while preserving drag/right-click, modes and existing TTS.
- [ ] External Handy adapter only if tested control/API allows parity; otherwise sidecar is the sole capture/STT owner. Local default, GPU/cloud explicit opt-in.
- [ ] Independent final review; reproducible install/build, exact tests/latency and A01–A10 acceptance evidence, license notices.

## Release/stop rules

Stop if user edits endangered, insecure cross-process credentials, wrong-window paste, unverified remote/cloud audio, missing Windows GUI evidence, or unknown upstream license. No automatic merge/push/deploy. Each implementation task is independently testable/reviewable; document actual commands/exit codes rather than asserting equivalence.
