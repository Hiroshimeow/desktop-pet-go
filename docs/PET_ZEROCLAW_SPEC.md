# Desktop Pet + ZeroClaw Canonical Specification

Status: **active canonical specification for all work after Phase 9**
Repository: `/home/ayumi/Workspace/git_project/desktop-pet-go`
Target machine: Windows 11 ThinkBook
Architecture decision: **ZeroClaw Daemon + localhost Gateway**
Pinned ZeroClaw release for this roadmap: **v0.8.3**

## 1. Product outcome

Build one lightweight Windows desktop pet whose final supported experience is:

- wake and converse naturally in **Vietnamese and Japanese**;
- use the existing local STT path and local TTS path(s);
- use **ZeroClaw as the only general agent brain**;
- plan, remember useful context, create notes and schedule reminders through ZeroClaw capabilities rather than reimplementing them in the pet;
- preserve Phase 9 deterministic click-to-command actions as a fast local reflex/fallback;
- show semantic 2D states such as listening, thinking, working, speaking, success and error;
- make 2D animation assets easier to author/generate and keep rendering smooth with low CPU;
- survive PET UI restart without losing the ZeroClaw daemon, scheduled reminder state or agent memory.

The target system is a personal local application, not an extensible AI platform.

## 2. Canonical architecture

```text
                               Windows user session

                     ZeroClaw v0.8.3 daemon/service
                    (long-running independent brain)
                               |
                 localhost Gateway 127.0.0.1:42617
                   REST / WebSocket / SSE control plane
                               |
                         tiny Go adapter
                               |
        +----------------------v-----------------------+
        |               desktop-pet-go                 |
        |                                               |
        | Win32 UI / input / animation / renderer       |
        | local faster-whisper STT                      |
        | local VI + JA TTS                             |
        | Phase 9 deterministic fast commands           |
        | semantic voice/agent animation intents        |
        +-----------------------------------------------+
```

### Ownership

**ZeroClaw owns:**

- LLM/provider selection and routing;
- general conversation;
- agent reasoning/tool loop;
- long-lived sessions;
- memory;
- planning;
- cron/scheduling/reminders;
- built-in tools;
- MCP client behavior if a future capability actually needs it;
- restart-independent agent state.

**desktop-pet-go owns:**

- Win32 pet window/input lifecycle;
- click/drag/right-click behavior;
- Phase 9 deterministic local command parser/actions;
- microphone/STT orchestration through the existing sidecar;
- local speech playback;
- animation intent/state display;
- ZeroClaw Gateway transport adapter only.

The pet must never grow its own second agent loop, provider registry, memory database, scheduler, planner, generic tool registry or plugin runtime.

## 3. ZeroClaw runtime contract

### Version and install

Use the official **ZeroClaw v0.8.3 Windows prebuilt binary**. Do not compile Rust during the normal setup path.

The existing `scripts/setup-voice.ps1` remains the single setup entry point. It may download/check the pinned ZeroClaw release, configure the `pet` agent, and install/start the ZeroClaw user service. Do not add a second required setup command.

### Daemon, not ACP

Production integration is **not ACP**.

Run ZeroClaw as the long-lived daemon:

```text
zeroclaw daemon --host 127.0.0.1
```

and register it on Windows through ZeroClaw's own service command so it starts in the user session. The daemon owns Gateway + scheduler + heartbeat and remains alive independently of the pet UI.

ACP may be used manually for debugging ZeroClaw itself, but no PET production source may implement an ACP transport or support dual ACP/Gateway modes.

### Gateway

Use the pinned v0.8.3 Gateway surface only:

- default host: `127.0.0.1`;
- default port: `42617`;
- WebSocket agent chat: `/ws/chat`;
- REST health/status/config/session/cron/memory APIs when useful;
- SSE runtime observation stream: `/api/events` and recent history `/api/events/history` when useful;
- abort/cancel through the supported Gateway session/WS surface.

The v0.8.3 router source and the running daemon's `/api/openapi.json` / `/api/docs` are authoritative for exact payload fields. Do not invent a parallel PET protocol.

### Authentication

Keep Gateway localhost-only. Phase 10 must use the normal ZeroClaw pairing/bearer mechanism or another documented v0.8.3 localhost authentication path that does not require weakening the public-bind controls. Do not hard-code, commit or log bearer tokens/provider keys.

A local generated credential file under the existing ignored `.voice/` data directory is acceptable if it is the smallest reliable Windows solution. Do not add a credential manager abstraction.

### Provider

The PET does not call OpenAI/Anthropic/Gemini directly. Provider credentials and model selection belong to ZeroClaw configuration only.

The setup/acceptance may use the operator's configured provider. Do not hard-code one cloud model into PET source.

## 4. Language and voice contract

Final supported conversational languages for this roadmap are **Vietnamese and Japanese**.

### STT

Reuse the existing faster-whisper CPU/int8 sidecar and VAD/microphone lifecycle. Do not add a second STT engine unless the existing path demonstrably cannot recognize Japanese well enough for the bounded acceptance.

### Wake

- Vietnamese wake keeps the existing working wake behavior.
- Add one small Japanese wake alias set in the existing wake/session owner.
- Wake parsing remains deterministic normalization, not an LLM classifier.
- Phase 9 `-command` click-to-listen mode remains wake-free and deterministic-only.

### TTS

- Vietnamese keeps the accepted local Piper path unless regression evidence requires change.
- Japanese must use a local CPU-capable backend.
- Preferred first candidate: **Kokoro-82M Japanese** because it is small and has official Japanese voices.
- PLAN may compare at most one additional local Japanese candidate if Kokoro fails the ThinkBook latency/quality/runtime acceptance.
- Do not introduce cloud TTS into the supported path.
- Do not build a generic TTS provider framework; language dispatch may remain one direct branch in the existing sidecar.

English behavior already present may remain as compatibility behavior, but English is not part of the final acceptance target for Phase 10-12.

## 5. Routing contract

Normal conversational voice flow after Phase 10:

```text
wake/STT transcript
  -> existing deterministic VoiceCommand match?
       yes -> execute locally
       no  -> send transcript to ZeroClaw pet session
                -> agent reasoning/tools/memory
                -> final text
                -> local TTS
```

Phase 9 command flow stays:

```text
left click
  -> listen once
  -> STT
  -> deterministic parser only
  -> fixed local action or no-op
```

`-command` must never reach ZeroClaw.

The existing direct Gemma/llama.cpp chat path is legacy after Phase 10. It must not remain a competing supported default brain. PLAN/DEV may delete or leave dormant the smallest amount necessary for regression-safe migration, but final Phase 10 acceptance must prove normal agent conversation does not start llama-server.

## 6. Agent lifecycle -> animation intent contract

The pet displays semantic state only. ZeroClaw never selects a concrete clip/frame.

Required semantic states by final Phase 12:

```text
idle       -> normal idle/roam
listening  -> microphone listening
thinking   -> ZeroClaw reasoning / waiting on model
working    -> ZeroClaw tool/planning work
speaking   -> local TTS playback
success    -> short positive reaction
error      -> short existing error/confused fallback
```

Reuse current `IntentVoiceListening`, `IntentVoiceThinking`, `IntentVoiceSpeaking`, error/unknown intents and the existing Brain -> Intent -> Resolver -> AnimationPlayer architecture. Add only the minimum missing semantic intent if `working` cannot be represented clearly by an existing intent.

## 7. Personal-agent behavior contract

Phase 11 must expose useful ZeroClaw behavior through natural conversation without recreating ZeroClaw features.

Required bounded scenarios:

1. **Planning** — user asks in Vietnamese or Japanese for a simple plan; ZeroClaw returns a useful plan.
2. **Note/memory** — user asks the pet to remember a short fact/note; ZeroClaw stores it using its own memory/tool path, and a later conversation can recall it.
3. **Reminder** — user asks for a one-shot reminder; ZeroClaw schedules it using its own cron/scheduler path.
4. **Restart persistence** — create a reminder, restart only the PET before it fires, reconnect to the already-running ZeroClaw daemon, and the reminder still fires while the PET is connected.

Do **not** build a PET scheduler, note database, planner, reminder polling framework, calendar database or custom memory extraction.

Do not promise missed-reminder delivery when the PET is not running at the due instant. That is outside this roadmap unless ZeroClaw already provides it through a built-in channel with no PET code.

## 8. Animation/rendering contract

The final phase improves the body, not the brain.

Keep the current semantic animation architecture. Do not migrate to Live2D, Spine, Unity, Godot, Electron, WebView/canvas or another game/render engine.

### Authoring

Support an easy offline path from generated/drawn transparent PNG frames to runtime-ready animation data:

- arbitrary frame counts;
- per-clip FPS/loop/duration remains manifest metadata;
- normalize frames to one stable pet canvas;
- anchor rule: bottom-center/feet-stable unless existing pet metadata proves a better single rule;
- one small pack/validate command/tool only;
- no visual node editor, animation graph editor or asset-pipeline framework.

### Rendering

Steady-state draw must not keep doing expensive per-pixel scale/flip work on every 16 ms tick.

Target approach:

- decode/validate once;
- precompute or lazily cache render-ready scaled/flipped BGRA frames for the active scale/facing;
- direct buffer copy on draw;
- dirty redraw: skip `UpdateLayeredWindow` when frame, position, facing and visual state are unchanged;
- share immutable decoded/render-ready assets between instances of the same pet where practical.

Keep click/drag responsiveness and existing Win32 layered-window behavior.

## 9. Phase plan

Only **three new phases** exist after Phase 9.

```text
Phase 9 DONE/accepted click-command baseline
       |
       v
Phase 10 ZeroClaw brain + VI/JA voice
       |
       v
Phase 11 ZeroClaw personal-agent behavior
       |
       v
Phase 12 expressive 2D + performance + final acceptance
       |
       v
DONE
```

No Phase 13+ is implied by this specification.

---

# Phase 10 — ZeroClaw brain + Vietnamese/Japanese voice

CDPA team: `desktop-pet-zeroclaw-phase10-brain-voice`
Task ID: `cdpa-idem-46221f1edd34ace2ef737804`

## Outcome

Normal conversational PET voice uses the pinned ZeroClaw v0.8.3 daemon through localhost Gateway, with Vietnamese and Japanese wake/STT/reply/TTS. Phase 9 local deterministic commands remain local-first. The PET no longer owns the general LLM brain.

## Required implementation

1. Preserve the accepted Phase 9 working-tree checkpoint as baseline.
2. Extend the single existing setup path to install/check pinned ZeroClaw v0.8.3 prebuilt, configure one `pet` agent, bind Gateway to localhost, and install/start the ZeroClaw daemon service.
3. Add one tiny Go Gateway adapter. It owns only connection/auth/session/send/cancel/stream parsing. No provider/model/tool/memory logic.
4. Use v0.8.3 `/ws/chat` for actual agent turns. Use REST/SSE only where they reduce code for health/status/events. Do not use ACP.
5. Keep the Win32 message loop non-blocking; Gateway I/O runs off the UI thread and posts results through existing app/voice event ownership.
6. Normal `-voice` command-first routing becomes deterministic command -> ZeroClaw for non-command conversation.
7. `-command` stays deterministic-only and never initializes or calls ZeroClaw.
8. Remove/disable the direct llama/Gemma startup path from the supported normal conversational flow. Do not start llama-server in Phase 10 acceptance.
9. Reuse existing Phase 7 local persona/history only if needed during migration, but do not maintain it as a second authoritative long-term memory after ZeroClaw works. ZeroClaw owns agent memory.
10. Add Japanese wake aliases in the existing session/wake owner.
11. Reuse faster-whisper for Japanese STT.
12. Add Japanese local TTS through Kokoro-82M first. PLAN may benchmark one alternative local Japanese backend only if Kokoro fails bounded ThinkBook acceptance.
13. Map Gateway/agent lifecycle to existing listening/thinking/speaking/error semantic intents. Do not add artwork tooling in Phase 10.
14. Fail soft: ZeroClaw unavailable must not kill the visual pet or Phase 9 deterministic command mode.

## Allowed production/source files

Maximum: **7 product/source files + 4 test files + 2 setup/docs files**.

Primary allowlist:

- `go-lite/main.go`
- `go-lite/voice_windows.go`
- `go-lite/internal/agent/zeroclaw.go` — **NEW**, sole PET Gateway transport owner
- `go-lite/go.mod` — only for one WebSocket dependency required by `/ws/chat`
- `go-lite/go.sum` — dependency lock companion only
- `voice-sidecar/voice_sidecar.py`
- existing wake/session owner under `go-lite/internal/voice/` only if Japanese wake cannot be added cleanly through `voice_windows.go`
- `scripts/setup-voice.ps1`
- `README.md`

Allowed tests:

- `go-lite/internal/agent/zeroclaw_test.go` — **NEW**
- `go-lite/voice_windows_test.go`
- `voice-sidecar/test_voice_sidecar.py`
- one existing wake/session test file only if Japanese wake lives there

No other production file may be created without PLAN proving a deterministic compile/runtime blocker and routing back to PLAN before editing.

## Explicitly forbidden

- ACP transport in PET source;
- direct OpenAI/Anthropic/Gemini HTTP client in PET;
- provider registry/factory;
- PET agent loop;
- PET tool framework;
- PET memory DB/vector DB/RAG;
- PET scheduler;
- multi-agent framework;
- new daemon/process supervisor around ZeroClaw;
- cloud STT/TTS;
- broad voice rewrite;
- animation asset/editor work.

## Focused tests

- Gateway adapter connects/authenticates against a fake `httptest`/WebSocket server, sends one user turn, receives streamed/final reply, and cancels one in-flight turn.
- deterministic command path never calls Gateway;
- `-command` never calls Gateway;
- normal non-command accepted utterance calls Gateway;
- ZeroClaw unavailable fails soft;
- Japanese wake aliases accepted; unrelated near-match rejected;
- Japanese TTS sidecar path produces audio/event flow without changing Vietnamese Piper behavior.

Do not exceed **10 new focused automated test cases** total for this phase.

## ThinkBook acceptance

Use `@mcp-thinkbook` for one bounded native Windows acceptance:

- install/check ZeroClaw v0.8.3 prebuilt through the one setup path;
- `zeroclaw service status` shows daemon running under the user session;
- Gateway is localhost-only;
- PET starts normal `-voice` without manually started llama-server;
- no new llama-server is spawned;
- Vietnamese wake -> ordinary conversation -> ZeroClaw -> local spoken Vietnamese reply;
- Japanese wake -> ordinary Japanese conversation -> ZeroClaw -> local spoken Japanese reply;
- one deterministic command remains local and does not create a ZeroClaw turn;
- PET UI stays responsive while ZeroClaw works;
- restart only the PET and reconnect to the same daemon/session path;
- ZeroClaw unavailable/misconfigured keeps visual pet alive;
- shutdown PET leaves no PET/voice orphan and does not unnecessarily stop the independently installed ZeroClaw daemon.

## Dependency / route

Depends on Phase 9 task `cdpa-idem-25cece9621add3565c1708c5` reaching DONE.

Use exactly:

```text
PLAN -> DEV -> REVIEW -> PLAN -> DONE
```

No TEST/AUDIT.

---

# Phase 11 — ZeroClaw personal-agent behavior

CDPA team: `desktop-pet-zeroclaw-phase11-personal-agent`
Task ID: `cdpa-idem-20a83d9b97d579d46bc1a9af`

## Outcome

The PET uses ZeroClaw's existing reasoning, memory and scheduler to support bilingual VI/JA planning, notes/memory and reminders without implementing those subsystems in PET.

## Required implementation

1. Reuse the Phase 10 Gateway adapter; do not add a second transport.
2. Keep normal conversation going through the same ZeroClaw `pet` agent/session.
3. Confirm the configured ZeroClaw agent has the minimum built-in capabilities needed for planning, memory/note and one-shot cron reminder.
4. Use ZeroClaw memory/tool behavior for short notes/facts. Do not mirror them into Phase 7 JSON history as a second long-term store.
5. Use ZeroClaw scheduler/cron for reminders. Do not create Windows timers or PET scheduler persistence.
6. Consume only the minimum Gateway events/status needed to distinguish thinking/working/final/error for UI state.
7. `working` may reuse an existing semantic intent if one is visually suitable; add one small intent constant/profile only if necessary. Never map ZeroClaw tools directly to concrete clip names.
8. Preserve Phase 9 deterministic commands and Phase 10 VI/JA voice behavior.
9. Do not add MCP by default. Add one PET-specific MCP/tool bridge only if a required Phase 11 scenario cannot be satisfied by ZeroClaw v0.8.3 built-ins; such expansion requires PLAN evidence before DEV edits.

## Allowed production/source files

Maximum: **4 product/source files + 3 test files + 1 doc file**.

Primary allowlist:

- `go-lite/internal/agent/zeroclaw.go`
- `go-lite/voice_windows.go`
- existing semantic intent file under `go-lite/internal/pet/` only if one `working` intent is required
- `scripts/setup-voice.ps1` only for minimal ZeroClaw agent capability/config adjustment
- `README.md`

Allowed tests:

- `go-lite/internal/agent/zeroclaw_test.go`
- `go-lite/voice_windows_test.go`
- existing pet intent test file only if `working` is added

No new production file by default.

## Explicitly forbidden

- custom planner;
- custom reminder scheduler/timer DB;
- custom note DB/index/search;
- vector DB/RAG;
- custom memory extraction;
- generic PET tools registry;
- MCP server unless PLAN proves a required scenario impossible otherwise;
- calendar/Gmail integration;
- multi-agent PET orchestration;
- animation renderer overhaul.

## Focused tests

Use fake Gateway fixtures/events only. Prove:

- agent working event/state maps to semantic working/thinking intent without blocking UI;
- final response maps to speaking/TTS;
- Gateway failure maps to error/fail-soft;
- deterministic command still bypasses agent path.

Do not test ZeroClaw internals already covered by ZeroClaw itself. Do not exceed **6 new focused automated test cases**.

## ThinkBook acceptance

With ZeroClaw daemon already configured from Phase 10:

- Vietnamese planning request returns a useful plan;
- Japanese planning request returns a useful plan;
- store one short note/fact, then recall it in a later turn;
- schedule one short one-shot reminder through natural language;
- restart only the PET before the reminder fires;
- reconnect to the still-running ZeroClaw daemon;
- reminder still fires while PET is connected and is spoken locally;
- agent thinking/working/speaking states drive semantic pet reactions;
- Phase 9 local command still bypasses ZeroClaw;
- no PET scheduler/memory DB/new agent framework exists in the diff.

## Dependency / route

Depends on Phase 10 DONE.

Use exactly:

```text
PLAN -> DEV -> REVIEW -> PLAN -> DONE
```

No TEST/AUDIT.

---

# Phase 12 — Expressive 2D body, rendering efficiency and final acceptance

CDPA team: `desktop-pet-zeroclaw-phase12-animation-final`
Task ID: `cdpa-idem-ef46170c900a27d89a8f4cb5`

## Outcome

The completed ZeroClaw-powered VI/JA pet has materially more expressive, easier-to-author 2D animations with lower steady-state rendering cost, then passes one bounded final Windows acceptance across voice, agent behavior and animation.

## Required investigation before edits

PLAN/DEV must record a bounded ThinkBook baseline for one pet:

- idle CPU;
- ordinary continuously animating/moving CPU;
- obvious frame jitter/stutter observation;
- confirm current hot path: 16 ms UI tick, per-pixel scale/flip in `drawPet`, layered-window redraw each tick.

Do not build a profiling framework.

## Required implementation

1. Preserve Brain -> Intent -> Resolver -> AnimationPlayer. Do not rewrite agent/voice routing.
2. Add one small offline pack/validate mode that accepts ordered transparent PNG frames for one clip, normalizes them to the stable pet canvas/anchor, and emits the existing efficient strip format or an equally simple precompiled format.
3. Support arbitrary frame counts and existing per-clip FPS/loop/duration metadata.
4. Reuse `-catalog` or add one equally small preview/validate command; do not build an editor UI.
5. Cache render-ready scaled/flipped frames so steady-state drawing has no per-pixel resize/flip loop.
6. Add dirty redraw so unchanged frame/position/facing/visual state skips `UpdateLayeredWindow`.
7. Share immutable sprite/render cache data among instances of the same pet where practical.
8. Keep click/drag responsiveness and current layered-window behavior.
9. Provide distinct pet5 assets/config mapping for at least listening, thinking, working and speaking if usable artwork is available. Asset generation itself is not a runtime feature. If artwork is unavailable, complete the runtime/authoring path and document exact frame-folder requirements instead of adding image-generation tooling.
10. Run final cumulative acceptance; do not add new brain features.

## Allowed production/source files

Maximum: **6 product/source files + 4 test files + pet5 asset/config changes + 2 docs files**.

Primary allowlist:

- `go-lite/main.go`
- `go-lite/sprite.go`
- `go-lite/config.go`
- `go-lite/catalog.go`
- `go-lite/split_assets.go` — preferred owner to extend/replace for the one pack/validate path; do not create a second asset framework if this file can own it clearly
- one existing animation/runtime bridge file only if semantic final-state mapping needs a small adjustment
- `assets/pets/pet5/pet.json`
- `assets/pets/pet5/animations/*` only for actual new/updated pet5 animation art
- `README.md`
- `docs/PET_ZEROCLAW_SPEC.md` only for final factual completion notes; architecture must not be redefined here by DEV

Allowed tests:

- existing sprite/config tests if present, otherwise one focused new sprite test file;
- existing animation/player/runtime tests only where behavior changes;
- one focused dirty-redraw/cache test file only if no existing owner can test it.

## Explicitly forbidden

- Live2D;
- Spine;
- Unity/Godot;
- Electron/WebView/canvas renderer migration;
- GPU framework migration;
- skeletal animation engine;
- animation node/graph editor;
- plugin/theme system;
- runtime image-generation pipeline;
- ZeroClaw brain/provider changes;
- new personal-agent features.

## Performance acceptance

Use the same ThinkBook measurement method before and after.

Targets:

- idle PET process CPU approximately `<= 1%` if achievable with the native pipeline;
- ordinary one-pet animation/movement approximately `<= 3%` if achievable;
- otherwise REVIEW may accept a clearly substantial measured reduction if Windows sampling semantics make the absolute threshold misleading, but exact before/after numbers must be reported;
- no continuous PNG decode/JSON parse/per-pixel scale loop in the steady-state draw path;
- no visible anchor jitter;
- click/drag remains responsive.

## Final ThinkBook acceptance

One bounded final run must prove:

- ZeroClaw v0.8.3 daemon/service remains the independent brain;
- Gateway reconnect works after PET restart;
- Vietnamese wake + conversation + local TTS;
- Japanese wake + conversation + local TTS;
- deterministic Phase 9 command remains local-first;
- planning works;
- note/memory store + later recall works;
- one-shot reminder survives PET restart and fires while reconnected;
- listening/thinking/working/speaking/success-or-error semantic reactions are visible or correctly fall back when an art asset is intentionally absent;
- asset authoring/validation command works on one sample clip;
- CPU before/after evidence is recorded;
- PET/voice shutdown is clean;
- independently-running ZeroClaw daemon is not killed just because PET exits;
- full relevant Go/Python tests, vet, Windows debug/release build and `git diff --check` pass.

## Dependency / route

Depends on Phase 11 DONE.

Use exactly:

```text
PLAN -> DEV -> REVIEW -> PLAN -> DONE
```

No TEST/AUDIT.

---

## 10. Global CDPA execution rules for Phase 10-12

Every task must begin by reading, in order:

1. `docs/PET_ZEROCLAW_SPEC.md`;
2. relevant retained Phase 8/9 and immediately previous ZeroClaw phase PLAN/DEV/REVIEW reports;
3. current working tree and accepted checkpoint evidence.

The operator-approved spec is authoritative over older `docs/VOICE_ROADMAP.md`, `docs/VOICE_REQUIREMENTS.md`, Japanese/PDF historical documents, and broad architecture rewrite notes where they conflict.

Preserve accepted dirty baseline files. Do not reset, clean, stash, discard, branch-switch, rewrite history or opportunistically refactor unrelated accepted work.

### File discipline

- The phase-specific allowlist is binding.
- A new production file is allowed only where the spec explicitly marks it NEW, unless PLAN documents a deterministic compile/runtime blocker and routes back before DEV expands scope.
- Moving code to extra helpers does not bypass file budgets.
- `.plan/` reports are not product changes.

### Minimalism

Before adding code, workers must ask whether ZeroClaw v0.8.3 or the existing pet runtime already owns the capability.

Do not recreate:

- agent loop;
- memory system;
- provider/model abstraction;
- planner;
- scheduler/cron;
- generic tool/MCP framework;
- process supervisor;
- animation framework already present.

### Tests

Use TDD for the changed boundary only. Reuse existing tests. Prefer table tests/fake Gateway fixtures. Do not test ZeroClaw internals as if they were PET code.

### Windows evidence

Use `@mcp-thinkbook` for the phase-specific native acceptance only. Do not rerun broad cumulative acceptance until Phase 12.

### Routing

All three tasks use exactly roles:

```text
PLAN
DEV
REVIEW
```

and fast path:

```text
PLAN -> DEV -> REVIEW -> PLAN -> DONE
```

TEST/AUDIT are forbidden unless the operator explicitly expands the task after an evidence-based blocker.

### Stop condition

Phase 12 DONE is the end of this roadmap. Do not invent Phase 13+ or reopen completed phases for speculative platform/product hardening.

## 11. Authoritative external references

Pinned ZeroClaw facts for this spec were verified against the v0.8.3 release and official documentation on 2026-08-02:

- `https://github.com/zeroclaw-labs/zeroclaw/releases/tag/v0.8.3`
- `https://docs.zeroclawlabs.ai/master/en/reference/cli.html`
- `https://docs.zeroclawlabs.ai/master/en/setup/service.html`
- `https://docs.zeroclawlabs.ai/master/en/gateway/api.html`
- v0.8.3 gateway router: `crates/zeroclaw-gateway/src/lib.rs`

Kokoro Japanese voice reference:

- `https://huggingface.co/hexgrad/Kokoro-82M/blob/main/VOICES.md`
