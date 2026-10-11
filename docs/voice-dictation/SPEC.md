# Desktop Pet: Local-First Dictation & Live Transcript — Specification

Status: **PROPOSED**, Phase 1 verification/design only. Date 2026-10-11. Source-of-truth product: `Hiroshimeow/desktop-pet-go`, baseline branch `origin/feat/phase1-vietnamese-sts`, SHA `61d96af7ca493a43fea236a8298435622a405eb8`.

## Product contract

One Windows on-screen pet offers three explicit modes, without creating another assistant:

- **Click pet → text chat**, preserving the existing drag behavior and agent route.
- **Click mic on pet → voice conversation**: microphone → STT → existing agent → TTS.
- **Global toggle F5 or configurable hold-to-talk → system-wide dictation**: microphone → on-device streaming STT → provisional/stable text visible → final text inserted at the original input caret. Dictation **never invokes** agent, TTS, shell, commands or auto-submit.

Reference: **OpenSuperWhisper** (OSW, `Starmel/OpenSuperWhisper`) as UX/behavioral reference; `cjpais/Handy`, `OpenWhispr/openwhispr`, OSW-Windows port, Whisper Local as implementation candidates. Reuse before reimplementation. Feature completeness, correctness, reliability and reference parity outrank token/time/LOC/dependency cost. Do not weaken functionality to make implementation easy. Preserve legal copyright/license notices when copying third-party code, even when README says only “Inspired by OpenSuperWhisper”.

## Baseline & invariants

Verified G6 branch: `origin/master` is **older** than `origin/feat/phase1-vietnamese-sts`; root `master` contains unsaved user edits and must not be reset. `go-lite/voice_windows.go` owns current sidecar process and JSON event handling; `voice-sidecar/voice_sidecar.py` owns `sounddevice` capture, WebRTC VAD and `faster-whisper`; `go-lite/voice_controls_windows.go` owns existing reader controls. Windows `go test ./...` PASS; Python sidecar `uv sync --frozen` and 18 unittest tests PASS. Real GUI/mic tests NOT proven.

Preserve pet animation/runtime, voice chat, deterministic voice command matcher, text chat, TTS and existing reader, including opt-in voice behavior. Never block Win32 render/message loop, execute untrusted transcript, leak private audio to cloud implicitly, or log secrets/audio text by default. Cancel/timeouts must be bounded; missing backend must not kill pet.

## Observable states and events

States: `idle → recording → transcribing → ready → idle`, or `recording/transcribing → error/cancel → idle`. Assign each activation a unique `session_id`; stale/out-of-order sidecar events must be discarded. Idempotent stop/cancel and exactly-once final insertion.

Event protocol (versioned): `record_started`, `speech_start`, `partial{stable,unstable}`, `final{text,language,session_id}`, `cancelled`, `error{code,reason}`, `done`. Partial is **visual only**; only final may trigger insertion.

### Input lifecycle

- Toggle F5: press once to start; again to finalize; Esc to discard. Configurable to avoid clashes with IDEs.
- Hold-to-talk: keydown starts; keyup finalizes. Ignore auto-repeat; accidental short presses discard. Do not break AltGr/IME.
- Record original target window/edit control/caret **before overlay steals focus**. Preview must not take focus. When target changed, secure/inaccessible, or different integrity level: don't paste into another app accidentally; safe fallback is copy-only plus visible warning. User can retry intentionally.
- Paste exactly once, preserve Unicode (Vietnamese accents + code-switch), caret/selection and clipboard contents when feasible. Restore clipboard only when clipboard sequence still belongs to insertion; do not overwrite another app's clipboard changes. No synthetic Enter.
- Overlay indicates recording, transcribing, unstable/stable partial, completion/error. Respect DPI, multiple monitors, pet animation fallback and reduced motion.
- History is local, opt-in with retention. No perpetual recording. Raw vs optional cleanup transcript clearly separated; never silently change code identifiers.

### Language/performance/privacy

Primary path: **ThinkBook CPU local**, existing faster-whisper base/int8 resident model initially; benchmark small/medium and alternatives on real Vietnamese/English technical-term audio. Measure cold/warm startup, release-to-final p50/p95, WER/CER/terminology, process CPU/RSS, first-word loss, stitch duplication and UI responsiveness. `local-live-caption-engine` may supply tested pre-roll, local agreement, overlap stitching and streaming scheduling; do not duplicate microphone/ASR ownership.

LAN A5000 / RTX2080Ti: optional only after real end-to-end benchmark and encrypted authenticated transport; cloud Gemini optional explicit opt-in only, never silent fallback. Use protected credentials. Free-tier availability does not authorize sending personal audio.

## Reuse decision gate

1. Inspect pinned upstream source (not marketing/readme alone), license and relevant paths.
2. Run Handy on G6 (installed app + hotkey + CLI) and compare to required F5/hold/partial/final/clipboard/target behavior. CLI toggle alone is not proof of hold-to-talk or an embeddable event protocol.
3. If Handy API/control meets contract, integrate by a supported interface. Otherwise port proven lifecycle/insertion patterns into existing Go/sidecar. No wholesale Electron/Tauri embedding without demonstrated advantage.
4. Independent review requires actual Windows GUI, app-target, mic, latency and safety evidence.

## Acceptance IDs

| ID | Evidence required |
|---|---|
| A01 | F5 toggle, hold down/up, Esc, key-repeat and conflicts tested on Windows |
| A02 | Captured streaming partial/stable/final event trace; **no provisional paste** |
| A03 | Final pasted exactly once into original intended target: Notepad, browser/ChatGPT, VS Code, terminal |
| A04 | Focus change, secure/elevated target, clipboard race fail safely with no wrong-window paste |
| A05 | VI/EN code-switch accuracy corpus with WER/CER, technical-name preservation |
| A06 | Resident/cold p50/p95 latency, idle/inference CPU & RSS; UI nonblocking |
| A07 | Cancel/restart/crash do not duplicate paste, hang pet or leak orphan processes |
| A08 | Default network/audio stays local; GPU/cloud explicit opt-in |
| A09 | Existing Go + Python tests and real pet click/drag/chat/voice/TTS unaffected |
| A10 | Handy/OSW parity and code reuse decisions linked to exact upstream source/commit/license |

## Non-goals

No new brain, no new TTS training, no Android app, no auto-submit, no wake-word rewrite, no speculative provider framework, no automatic microphone upload. **Phase 1 concludes with verified spec/plan/tasks; Phase 2 implementation needs approval.**
