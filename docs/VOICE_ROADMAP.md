# Desktop Pet Voice Roadmap

Status: **Phases 1-9 are historical/accepted. Phase 10-12 are governed exclusively by `docs/PET_ZEROCLAW_SPEC.md`.**

Baseline Phase 1 commit: `fd292701b59e3b4093098f75a9ef7ed9036d3fd9` (`feat: add Vietnamese realtime voice phase 1`).

## Current goal

Build a lightweight personal Windows desktop pet whose final supported path is:

- Vietnamese + Japanese wake/conversation;
- existing local faster-whisper STT;
- local VI/JA TTS;
- Phase 9 deterministic fast local commands;
- **ZeroClaw v0.8.3 Daemon + localhost Gateway as the only general agent brain**;
- ZeroClaw-owned planning, memory/notes and reminders;
- expressive semantic 2D animation with an easier PNG authoring path and low steady-state CPU.

The canonical architecture, file allowlists, forbidden work, tests and ThinkBook acceptance for all post-Phase-9 work live in `docs/PET_ZEROCLAW_SPEC.md`. If this historical roadmap or `docs/VOICE_REQUIREMENTS.md` conflicts with that spec, the ZeroClaw spec wins for Phase 10-12.

## Keep it simple

- Personal app, not a platform.
- PET owns body/voice/reflexes; ZeroClaw owns the agent brain.
- Do not rebuild agent loop, provider routing, memory, planner, scheduler, generic tools or MCP framework in PET.
- Production transport is **Daemon + Gateway**, not ACP.
- Reuse the current voice sidecar, STT, animation intent architecture and Phase 9 local command path.
- Only three post-Phase-9 tasks exist: Phase 10, 11 and 12.
- No separate TEST/AUDIT role. Every phase uses `PLAN`, `DEV`, `REVIEW` and the route `PLAN -> DEV -> REVIEW -> PLAN -> DONE`.
- Work remains sequential on the same accepted working-tree lineage; do not introduce worktree/merge machinery only to parallelize it.

## Hard anti-overengineering contract

These limits are part of every Phase 2–8 task and are binding for PLAN, DEV and REVIEW.

### File budget

- Each phase may modify/create only the files explicitly listed in that phase's **Allowed files** section.
- A phase may touch at most **8 product/source files**, **3 test files**, and **2 documentation/setup files** unless the operator explicitly expands scope.
- Moving code into extra helper files does not bypass the cap.
- Generated `.plan/` reports do not count toward the cap and must never be treated as product changes.
- If implementation appears to require a file outside the allowlist, route back to PLAN with one concrete reason. Do not silently expand scope.

### Test budget

- Tests exist only to prove the requested behavior works and the existing pet still operates normally.
- Maximum new automated tests per phase: **8 focused test cases total** across Go/Python, unless the operator explicitly asks for more.
- Do not add fuzzing, property testing, matrix testing, soak testing, adversarial tests, policy tests, hypothetical race suites or platform combinations that the personal app does not use.
- Do not add tests for unsupported features, future providers, malformed states that cannot be produced by the current UI/runtime, or speculative security/product-policy cases.
- Reuse existing tests first. Prefer one table-driven test over many near-duplicate test functions.
- Full regression means the existing relevant Go/Python suite plus the bounded runtime smoke specified for that phase; it does not mean inventing new coverage targets.

### REVIEW boundary

REVIEW is a gate, not a second design phase.

REVIEW may reject only for:

1. a requested function does not work;
2. an existing Phase 1/current pet behavior regresses;
3. app/process lifecycle is unstable: crash, hang, deadlock, orphan process, blocked UI, or obvious resource runaway;
4. implementation violates an explicit phase constraint or file/test budget;
5. tests/build/runtime evidence claimed by DEV is false or missing.

REVIEW must **not**:

- invent new feature requirements;
- demand generic abstractions "for later";
- request provider/plugin/config frameworks;
- expand supported languages/formats;
- create speculative edge-case suites;
- require policy/compliance/security hardening unrelated to a real local-app failure path;
- restart DEV for style preferences when behavior is correct and code is maintainable.

If the phase works, existing relevant tests pass, and the bounded runtime smoke is stable, REVIEW should pass it.

## Dependency graph

```text
Phase 1-8 historical accepted voice work
  |
  v
Phase 9 deterministic click-to-command
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

Exact Phase 10-12 contracts are defined only in `docs/PET_ZEROCLAW_SPEC.md`.

---

## Phase 1 — DONE — Vietnamese fixed-response STS

CDPA team: `desktop-pet-sts-phase1`

Task ID: `cdpa-idem-593e2ebe2e10a6f96d24d9a0`

Delivered at `fd292701b59e3b4093098f75a9ef7ed9036d3fd9`:

- `-voice` opt-in runtime;
- WebRTC VAD + faster-whisper-base CPU/int8;
- wake + same-utterance and wake + follow-up flow;
- fixed Vietnamese intents/replies;
- Piper Vietnamese TTS;
- microphone mute/cooldown while speaking;
- semantic voice animation events;
- fail-soft process lifecycle;
- deterministic WAV sequence acceptance path;
- Windows end-to-end benchmark with p95 under 4 seconds.

Do not reimplement Phase 1 in later tasks.

---

## Phase 2 — Bilingual speech + TXT/MD reader core

CDPA team: `desktop-pet-voice-phase2-bilingual-reader`

### Outcome

Extend the existing Phase 1 path so the pet can speak and read Vietnamese or English without adding UI controls yet.

### Implement

1. Add CLI support:
   - `-say <text>`
   - `-read-file <path>`
   - `-read-lang auto|vi|en`
2. Add one English Piper voice to the existing setup/bootstrap beside the Vietnamese voice.
3. Extend the existing sidecar `speak` command with a simple language field; choose only VI or EN Piper model.
4. Add a small deterministic language detector:
   - explicit `-read-lang` wins;
   - Vietnamese diacritics => `vi`;
   - otherwise Latin text => `en`.
5. Add local reader support for only:
   - UTF-8 `.txt`;
   - UTF-8 `.md`.
6. Chunk text deterministically by paragraph, then sentence punctuation, then hard character cap.
7. Feed chunks through the existing voice/TTS lifecycle sequentially.
8. Existing Phase 1 wake/STT/fixed reply behavior must remain working.

### Do not add

- Windows popup/menu yet;
- pause/resume/skip controls yet;
- PDF/OCR;
- Japanese;
- provider/backend abstraction;
- another sidecar or audio pipeline.

### Allowed files

Maximum for this phase: **6 product/source + 2 test + 2 docs/setup files**.

Allowed paths only:

- `go-lite/main.go`
- `go-lite/voice_windows.go`
- `go-lite/internal/voice/reader.go` — new, reader/language/chunking only
- `voice-sidecar/voice_sidecar.py`
- `scripts/setup-voice.ps1`
- `go-lite/internal/voice/reader_test.go` — new
- `voice-sidecar/test_voice_sidecar.py`
- `README.md`
- `go-lite/README.md`

No other product file may be changed in Phase 2 without an explicit PLAN stop and operator-approved scope change.

### Tests

Phase 2 deliberately does **not** require ThinkBook runtime acceptance.

Required before DONE:

- focused Go tests for language detection, TXT/MD validation and deterministic chunking;
- focused sidecar tests for VI/EN model routing without real audio device dependency;
- existing Go voice/session tests remain green;
- `git diff --check`;
- no generated model/audio/cache files tracked.

Phase 3 performs the cumulative real Windows validation for Phase 2 + Phase 3.

---

## Phase 3 — Audio controls, clipboard/menu, Windows acceptance

CDPA team: `desktop-pet-voice-phase3-controls`

### Outcome

Make speech/reading controllable in the actual Windows pet and validate the cumulative Phase 2 functionality on ThinkBook.

### Implement

1. Reuse the existing audio output path; do not introduce MCI/waveOut or another player unless the current sidecar output cannot support the required controls cleanly.
2. Add idempotent runtime actions:
   - pause;
   - resume;
   - skip current reader chunk;
   - stop reading/speaking.
3. Add clipboard reading.
4. Add a minimal right-click menu for:
   - Read clipboard;
   - Pause/Resume;
   - Skip;
   - Stop.
5. Preserve current right-click animation/action behavior as much as possible; menu integration must not break click/drag/movement.
6. Keep one shared speaker at a time.

### Allowed files

Maximum for this phase: **6 product/source + 2 test + 1 doc file**.

Allowed paths only:

- `go-lite/main.go`
- `go-lite/pet.go`
- `go-lite/voice_windows.go`
- `go-lite/voice_controls_windows.go` — optional new file only if keeping controls in `voice_windows.go` would make it materially harder to maintain
- `voice-sidecar/voice_sidecar.py`
- `go-lite/voice_windows_test.go`
- `voice-sidecar/test_voice_sidecar.py`
- `go-lite/README.md`

Do not create a generic audio/player abstraction. Extend the existing path first.

### ThinkBook acceptance — mandatory in this phase

Use `@mcp-thinkbook` on a clean/current source checkout and verify:

- setup/bootstrap succeeds;
- Vietnamese `-say` works;
- English `-say` works;
- a Vietnamese `.txt` or `.md` file is read;
- an English `.txt` or `.md` file is read;
- clipboard read works;
- pause/resume works repeatedly;
- skip advances one chunk;
- stop terminates the current speech/read session;
- click, right-click, drag, movement and rendering remain responsive while voice runs;
- pet/sidecar processes exit cleanly;
- debug and release Windows builds succeed.

No subjective audio-quality benchmark is required; the goal is functional cumulative acceptance.

---

## Phase 4 — Bilingual deterministic voice commands

CDPA team: `desktop-pet-voice-phase4-commands`

### Outcome

Use the existing wake/VAD/STT loop to control pet voice/reader actions in Vietnamese and English.

### Implement

Add one small deterministic `VoiceCommand` parser for commands such as:

| Vietnamese | English | Action |
|---|---|---|
| `tạm dừng` | `pause` | pause |
| `tiếp tục` | `resume` | resume |
| `bỏ qua` / `đoạn tiếp` | `skip` / `next` | skip chunk |
| `dừng đọc` / `dừng lại` | `stop reading` / `stop` | stop |
| `đọc clipboard` | `read clipboard` | read clipboard |
| `trạng thái` | `status` | short local status reply |

Rules:

- reuse the Phase 1 wake flow;
- exact/normalized phrase matching only;
- unknown transcript does nothing special and remains available for later conversation routing;
- voice command dispatch calls existing Go actions directly;
- do not build tool/plugin/intent frameworks.

### Allowed files

Maximum for this phase: **4 product/source + 2 test files**.

Allowed paths only:

- `go-lite/voice_windows.go`
- `go-lite/internal/voice/commands.go` — new
- `go-lite/internal/voice/session.go` — only if needed to preserve wake flow
- `go-lite/internal/voice/commands_test.go` — new
- `go-lite/internal/voice/session_test.go`

No new command framework or registry file.

### Tests

- Go parser table tests for VI/EN commands;
- command actions use existing Phase 3 controls;
- existing Phase 1 and reader tests remain green;
- synthetic transcript/WAV evidence is enough for this phase; full bilingual Windows command acceptance is deferred to Phase 8.

---

## Phase 5 — Local-model bilingual conversation

CDPA team: `desktop-pet-voice-phase5-local-chat`

### Outcome

The pet can answer normal Vietnamese and English conversation through one local model running on the ThinkBook.

### Architecture

Use one localhost HTTP model runtime. Preferred first implementation: `llama.cpp` server because Go can call it with the standard `net/http` package and no model SDK is required.

Do not create a generic provider layer.

### Implement

1. Benchmark at most two small current GGUF instruct models on ThinkBook and choose one default based on:
   - acceptable Vietnamese and English responses;
   - RAM footprint;
   - model load time;
   - first-token latency;
   - generation speed.
2. Pin the selected model/runtime version or download artifact in the setup path.
3. Add a small Go localhost chat client using `net/http`.
4. Add a bounded conversation request containing:
   - system/persona text;
   - recent message history supplied by the caller;
   - latest user utterance.
5. Return plain reply text to the existing TTS path.
6. Missing model/server must fail soft: pet remains usable and existing fixed/command voice functionality remains available.
7. Keep process lifecycle simple: if reliable, pet starts/stops the local server itself; otherwise use one documented localhost startup command until Phase 8. Do not add a daemon manager framework.

### Allowed files

Maximum for this phase: **5 product/source + 2 test + 2 setup/docs files**.

Allowed paths only:

- `go-lite/main.go`
- `go-lite/voice_windows.go`
- `go-lite/internal/voice/chat.go` — new localhost client and bounded request only
- `go-lite/internal/voice/chat_test.go` — new
- `scripts/setup-voice.ps1` — only if model/runtime bootstrap belongs naturally in the existing setup
- `README.md`
- `go-lite/README.md`

The model benchmark may create ignored artifacts under `.voice/`; they are not source files and must not be committed.

### ThinkBook acceptance

This phase needs Windows because local model performance is hardware-dependent:

- model loads on the ThinkBook;
- one Vietnamese conversation works end-to-end text response -> TTS;
- one English conversation works end-to-end text response -> TTS;
- record RAM, load time, first-token latency and approximate tokens/sec;
- pet UI remains responsive during inference.

---

## Phase 6 — Command vs conversation routing

CDPA team: `desktop-pet-voice-phase6-router`

### Outcome

After wake/STT, the pet decides simply and predictably whether the utterance is a local command or ordinary conversation.

### Routing

```text
transcript
  -> normalized deterministic VoiceCommand match?
       yes -> execute command locally
       no  -> send to local conversation model
```

### Implement

1. Reuse the Phase 4 command matcher unchanged as the first check.
2. Route non-command speech to Phase 5 conversation.
3. Preserve Phase 1 fixed replies only as fallback when local chat is unavailable; do not maintain two competing conversational brains.
4. Wake-only timeout behavior remains unchanged.
5. TTS/microphone busy/cooldown remains one shared lifecycle.

### Allowed files

Maximum for this phase: **3 product/source + 2 test files**.

Allowed paths only:

- `go-lite/voice_windows.go`
- `go-lite/internal/voice/router.go` — new, only if a small pure routing function cannot live cleanly in an existing voice file
- `go-lite/internal/voice/router_test.go` — new
- `go-lite/internal/voice/commands_test.go`

No new classifier, intent framework or model-based router.

### Tests

- table-driven routing tests;
- command phrases never reach chat;
- ordinary VI/EN questions reach chat;
- local-model failure falls back cleanly without killing voice or UI.

No separate Windows benchmark is needed here; final cumulative acceptance is Phase 8.

---

## Phase 7 — Lightweight memory + personality

CDPA team: `desktop-pet-voice-phase7-memory`

### Outcome

Give the pet continuity without introducing a database or retrieval system.

### Implement

1. Add one editable local persona file, for example `.voice/persona.txt`.
2. Keep recent conversation history only; a small fixed number of recent turns is enough.
3. Persist history in one simple local JSON/JSONL file under `.voice/` so restart can restore recent context.
4. Trim oldest turns when the bounded history limit is exceeded.
5. Pass persona + bounded recent history to the Phase 5 chat request.
6. Add clear-history support through one simple local action/CLI/menu entry if it fits naturally; do not build a settings system just for this.

### Do not add

- SQLite;
- vector database;
- embeddings;
- RAG;
- automatic long-term fact extraction;
- multi-user profiles;
- cloud sync.

### Allowed files

Maximum for this phase: **4 product/source + 2 test + 1 doc file**.

Allowed paths only:

- `go-lite/main.go`
- `go-lite/voice_windows.go`
- `go-lite/internal/voice/memory.go` — new bounded JSON/JSONL persistence only
- `go-lite/internal/voice/memory_test.go` — new
- `go-lite/internal/voice/chat_test.go`
- `README.md`

No database package or retrieval layer.

### Tests

- history round-trip;
- bounded trimming;
- persona text enters the chat request;
- malformed/missing history file starts cleanly rather than breaking pet startup.

---

## Phase 8 — Final product hardening and bilingual Windows acceptance

CDPA team: `desktop-pet-voice-phase8-hardening`

### Outcome

Make the complete personal-app flow easy to install and verify on the ThinkBook without adding new product features.

### Work

1. Remove duplicated/dead voice code exposed by Phases 2–7.
2. Keep one setup command and one normal run command.
3. Ensure all required local assets/models are downloaded or checked by setup with useful errors.
4. Update README with only supported VI/EN behavior.
5. Measure cumulative CPU/RAM and latency enough to catch obvious regressions; no benchmark framework.
6. Verify clean process shutdown/restart and fail-soft behavior.

### Allowed files

Phase 8 is primarily verification and simplification.

Maximum: **6 existing product/source files + 2 existing test files + 2 docs/setup files**. **No new production source file** unless a reproduced acceptance defect cannot be fixed within the existing owning file.

Primary allowlist:

- `go-lite/main.go`
- `go-lite/voice_windows.go`
- existing files under `go-lite/internal/voice/` that were created by Phases 2–7
- `voice-sidecar/voice_sidecar.py`
- `scripts/setup-voice.ps1`
- existing relevant Go/Python voice tests
- `README.md`
- `go-lite/README.md`

Delete/simplify before adding new structure.

### Final ThinkBook acceptance

Verify on Windows:

- Vietnamese wake + conversation;
- English wake + conversation;
- Vietnamese command;
- English command;
- Vietnamese and English direct `-say`;
- TXT/MD reader;
- clipboard read;
- pause/resume/skip/stop;
- recent conversation memory survives restart;
- persona affects reply;
- click/drag/right-click/movement/render remain responsive;
- local model missing/unavailable does not kill visual pet or deterministic commands;
- debug/release build pass;
- full relevant Go/Python tests pass;
- no orphan pet/voice/model processes after exit.

Phase 8 must not add a new subsystem. It only fixes defects found by cumulative acceptance and simplifies where possible.

---

## CDPA execution rule

For every Phase 2–8 task:

- repository: `/home/ayumi/Workspace/git_project/desktop-pet-go`;
- read this roadmap and `docs/VOICE_REQUIREMENTS.md` first;
- operator scope in this roadmap wins over conflicting historical Japanese/PDF directions;
- use roles exactly `PLAN`, `DEV`, `REVIEW`;
- keep implementation minimal and reuse Phase 1 code;
- preserve unrelated dirty/untracked work;
- do not reset/clean/stash/discard operator files;
- phase must end with an independently reviewable commit/checkpoint before its dependent task proceeds;
- do not create additional CDPA tasks unless the operator explicitly asks.
