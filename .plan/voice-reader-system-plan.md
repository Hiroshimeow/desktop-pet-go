# CPU-only Voice + Reader System Implementation Plan

Status: design plan for review. Do not implement runtime code until this plan is approved.

Requirement source of truth: `docs/VOICE_REQUIREMENTS.md`.

## 1. Requirement trace

This plan is governed by these requirement groups:

- Architecture boundary: `VOICE-REQ-001` to `VOICE-REQ-003`.
- CPU/offline constraints: `VOICE-REQ-010` to `VOICE-REQ-012`.
- Backend selection: `VOICE-REQ-020` to `VOICE-REQ-022`.
- Responsiveness/concurrency: `VOICE-REQ-030` to `VOICE-REQ-033`.
- Reader/input: `VOICE-REQ-040` to `VOICE-REQ-044`.
- STT safety: `VOICE-REQ-050` to `VOICE-REQ-052`.
- Command boundary: `VOICE-REQ-060` to `VOICE-REQ-062`.
- Cache/privacy: `VOICE-REQ-070` to `VOICE-REQ-073`.
- Audio UX: `VOICE-REQ-080` to `VOICE-REQ-082`.
- Build/test gates: `VOICE-REQ-090` to `VOICE-REQ-093`.
- Documentation consistency: `VOICE-REQ-100` to `VOICE-REQ-101`.

Any implementation PR must list the requirement IDs it satisfies.

## 2. Scope

Add an optional CPU-only voice/reader subsystem so the pet can:

- speak direct text using TTS;
- read clipboard text;
- read local `.txt` and `.md` files in Phase 1;
- later read text-based `.pdf` files in Phase 3 after a PDF-specific spike;
- pause, resume, stop, replay, and skip reading chunks;
- accept push-to-talk/listen-once STT commands after Phase 4;
- prioritize Vietnamese, Japanese, and English;
- keep rendering, drag, click, and multi-pet behavior responsive.

Out of scope for early phases:

- Cloud TTS/STT.
- GPU/CUDA-only inference.
- Always-listening microphone.
- OCR for scanned PDFs.
- Voice cloning.
- Asset-provided commands.
- Bundling model files into this repository.

## 3. Architecture decision

Use a sidecar-first, optional subsystem.

`go-lite` may own:

- CLI/profile parsing for voice options;
- `VoiceController` state machine;
- reader for small local text/markdown files;
- queue/cancel/timeout orchestration;
- adapter calls to local CPU backends;
- controllable Windows audio playback;
- pet animation events for voice state;
- right-click menu integration.

`go-lite` must not own:

- model weights;
- heavy ML runtime initialization inside render/message loop;
- backend package installation;
- benchmark logic;
- remote API calls;
- OCR engine;
- STT transcript to shell execution.

This reconciles the existing architecture boundary: runtime remains visual/interaction-first, while voice is an optional local orchestration layer.

## 4. Backend decision

### TTS default routing

- Vietnamese: Piper CPU voice.
- English: Piper CPU voice.
- Japanese: VOICEVOX local CPU engine through localhost API.
- OmniVoice: experimental opt-in only after CPU benchmark.
- NeuTTS: experimental opt-in only after CPU benchmark.

Piper is selected for VI/EN because it is a local neural TTS project with command-line usage suitable for CPU deployment. VOICEVOX is selected for Japanese because it provides a local engine/API suited for Japanese speech. whisper.cpp is selected for STT because it is an offline local Whisper implementation designed for CPU and quantized models. These are implementation assumptions to verify in Phase 0 before runtime integration.

### STT default routing

- whisper.cpp local CPU binary.
- Model path is explicit in profile/CLI.
- Phase 4 uses listen-once/push-to-talk only.

## 5. High-level architecture

```text
pet-lite.exe
  Win32 message loop + render loop
        |
        v
VoiceController                    # lightweight orchestration only
  - state machine
  - commands: speak/read/pause/resume/skip/stop/listen
  - emits pet animation events
  - owns global speaker lock
        |
        +--> ReaderService          # local txt/md first; pdf later
        |      - normalize text
        |      - split chunks
        |      - track chunk index
        |
        +--> TTSManager goroutine
        |      - timeout/cancel
        |      - cache lookup
        |      - adapter: piper/voicevox/command/experimental
        |
        +--> AudioPlayer
        |      - play/pause/resume/stop WAV
        |      - Windows MCI/waveOut target
        |
        +--> STTManager goroutine   # Phase 4
               - record listen-once WAV
               - call whisper.cpp
               - parse transcript to enum command
```

No long-running operation crosses into the Win32 callback or render critical path.

## 6. Voice state machine

```text
Idle
  -> Speaking        single direct text or current chunk synthesis/playback
  -> Reading         reader session has chunks remaining
  -> Listening       listen-once recording/transcription active
  -> Paused          audio or reader paused
  -> ErrorTransient  backend/read/playback error, then Idle or Reading
```

Rules:

- All transitions go through `VoiceController`.
- Every job uses `context.Context` with timeout/cancel.
- Stop/skip/pause/resume are idempotent.
- Phase 1 uses one global speaker lock across all pets.

## 7. User controls and CLI

### Phase 1 CLI

```powershell
.\pet-lite.exe -assets ..\assets -pet pet5 -voice -say "xin chào"
.\pet-lite.exe -assets ..\assets -pet pet5 -voice -read-file "D:\note.md" -read-lang vi
```

Flags planned for Phase 1:

- `-voice`: enable voice subsystem.
- `-say string`: speak text once after startup.
- `-read-file path`: read local `.txt`/`.md` file only in Phase 1.
- `-read-lang auto|vi|ja|en`: language hint.
- `-tts-backend auto|piper|voicevox|command|none`: default `auto` when `-voice` is set.
- `-tts-voice string`: backend voice ID/model alias.
- `-tts-speed float`: default `1.0`.
- `-tts-cache path`: optional cache root override with sentinel guard.
- `-tts-timeout-ms int`: default `45000`.
- `-tts-max-chars int`: default `350`.
- `-voice-profile path`: optional JSON config.

Phase 4 flags:

- `-stt`: enable listen-once STT.
- `-stt-backend whispercpp|command|none`.
- `-stt-model path`.
- `-stt-lang auto|vi|ja|en`.
- `-stt-timeout-ms int`.

### Runtime controls

Phase 2 adds a right-click popup menu while preserving current right-click animation semantics:

- Speak clipboard.
- Read file...
- Pause/Resume.
- Skip paragraph.
- Stop reading.
- Voice status.

Phase 4 adds:

- Listen once.

Left click remains normal pet interaction in Phase 1. Pause/resume on left click is not enabled unless explicitly approved later.

## 8. Reader design

### Phase 1 supported input

- `-say` direct text.
- Clipboard text through menu in Phase 2.
- `.txt`: UTF-8; UTF-16 BOM detection; reject binary.
- `.md`: UTF-8; strip frontmatter and optionally code fences; headings create section boundaries.

### Phase 3 PDF input

PDF is not Phase 1 support.

Before implementing PDF, run a spike and update this plan with:

- selected Go library or sidecar tool;
- max file size;
- max pages;
- max extracted chars;
- timeout;
- typed errors: `ErrPDFEncrypted`, `ErrPDFNoText`, `ErrPDFTooLarge`, `ErrPDFTimeout`, `ErrPDFMalformed`;
- fixtures: text PDF, scanned/no-text PDF, encrypted PDF, malformed PDF.

No OCR in early PDF support.

### Chunk model

```go
type ReadChunk struct {
    Index       int
    SourcePath  string
    Section     string
    Text        string
    LangHint    string
    StartOffset int
    EndOffset   int
}
```

Chunking rules:

- Split by paragraph first.
- Long paragraphs split by `.`, `!`, `?`, `。`, `！`, `？`, `…`.
- Hard cap by `tts_max_chars`.
- Preserve Vietnamese diacritics and Japanese punctuation.
- Deterministic output for tests.

## 9. TTS contract

```go
type TTSBackend interface {
    Name() string
    Check(ctx context.Context) error
    Synthesize(ctx context.Context, req TTSRequest) (TTSResult, error)
}

type TTSRequest struct {
    ID        string
    Text      string
    Lang      string // auto|vi|ja|en
    Voice     string
    Speed     float64
    OutputWAV string
}

type TTSResult struct {
    WAVPath    string
    DurationMS int
    Cached     bool
    Backend    string
}
```

### Piper adapter

- Used for Vietnamese/English default.
- Text is passed through stdin, never shell-concatenated.
- Output filename is hash/UUID under cache root.

Command shape:

```powershell
piper.exe --model <voice.onnx> --config <voice.json> --output_file <out.wav>
```

### VOICEVOX adapter

- Used for Japanese default.
- Localhost only: `http://127.0.0.1:50021`.
- If unavailable, log and fail the request without killing pet.
- No remote host support in default profile.

### Command adapter

- Explicit advanced option only.
- Executable/path comes from trusted CLI/profile, never from assets/transcript/file text.
- Request JSON on stdin, response JSON on stdout.
- No free-form shell command construction.

### Experimental adapters

- OmniVoice and NeuTTS remain explicit opt-in after CPU benchmark.
- They cannot become default without updating `docs/VOICE_REQUIREMENTS.md` and this plan.

## 10. Audio playback

Phase with pause/resume requires controllable player:

```go
type AudioPlayer interface {
    Play(ctx context.Context, wav string) error
    Pause() error
    Resume() error
    Stop() error
    IsPlaying() bool
}
```

Target Windows implementation: MCI or waveOut. `PlaySoundW` is allowed only as a simple fallback before pause/resume ships.

All controls must be idempotent.

## 11. STT design

Phase 4 only. No always-listening.

Flow:

```text
Listen once
  -> record N seconds or until silence
  -> write temp WAV under cache/stt
  -> call whisper.cpp with timeout
  -> parse transcript
  -> map transcript to VoiceCommand enum
  -> execute safe local action
```

Transcript cannot call `runHook`, PowerShell, shell, backend command template, or asset command.

Commands:

```go
type VoiceCommand string
const (
    CmdReadClipboard VoiceCommand = "read_clipboard"
    CmdPause         VoiceCommand = "pause"
    CmdResume        VoiceCommand = "resume"
    CmdSkip          VoiceCommand = "skip"
    CmdStop          VoiceCommand = "stop"
    CmdSpeakText     VoiceCommand = "speak_text"
)
```

Grammar examples:

- Vietnamese: `đọc clipboard`, `tạm dừng`, `tiếp tục`, `bỏ qua đoạn này`, `dừng đọc`.
- English: `read clipboard`, `pause`, `resume`, `skip paragraph`, `stop reading`.
- Japanese: `クリップボードを読んで`, `一時停止`, `再開`, `次へ`, `停止`.

Unknown transcript is safe no-op with log.

## 12. Language routing

Deterministic detector first:

- Japanese if Hiragana/Katakana or Japanese punctuation is present.
- Vietnamese if Vietnamese diacritics or `-read-lang vi`.
- English if Latin/ASCII without Vietnamese diacritics.
- Mixed text uses dominant language in Phase 1; span-level routing is later.

Routing:

```text
vi -> Piper Vietnamese voice
en -> Piper English voice
ja -> VOICEVOX local CPU endpoint
mixed -> dominant language Phase 1
```

No silent fallback across languages. Missing Japanese backend must not silently use English voice.

## 13. Pet animation integration

Voice events use best-effort animation fallback:

- `voice_listen_start`: `thinking`, `wave`, `idle`, current.
- `voice_listen_end`: `happy`, `idle`, current.
- `voice_speak_start`: `speaking`, `talk`, `wave`, `happy`, current.
- `voice_pause`: `sleepy`, `idle`, current.
- `voice_error`: `angry`, `cry`, `idle`, current.

Missing animation is not an error. No manifest write-back.

## 14. Data/config layout

```text
go-lite/
  voice.go
  voice_reader.go
  voice_tts.go
  voice_stt.go
  voice_audio_windows.go
  voice_audio_stub.go
  voice_menu_windows.go
  cache/                    # ignored
    voice-cache-manifest.json
    tts/
    stt/
models/                     # ignored
  tts/
    piper/
  stt/
    whisper/
scripts/
  voice/
    README.md
    check_voice_backends.ps1
```

`.gitignore` owns model/cache/audio ignore rules. Test audio fixtures must live under `go-lite/testdata/` and be allowlisted intentionally.

## 15. Cache and cleanup rules

Default cache root: `<exe-dir>/cache`.

If `-tts-cache` is provided:

- resolve absolute clean path;
- create `voice-cache-manifest.json` sentinel;
- write generated files only under `tts/` and `stt/` children;
- cleanup only if sentinel exists and manifest is valid;
- generated filenames are hash/UUID only;
- never recursive-delete arbitrary user path.

Logs must not dump full file content. Log path/hash/chunk index/length/error.

## 16. Voice profile format

```json
{
  "tts": {
    "backend": "auto",
    "default_lang": "vi",
    "speed": 1.0,
    "voices": {
      "vi": {
        "backend": "piper",
        "model": "models/tts/piper/vi.onnx",
        "config": "models/tts/piper/vi.onnx.json"
      },
      "en": {
        "backend": "piper",
        "model": "models/tts/piper/en.onnx",
        "config": "models/tts/piper/en.onnx.json"
      },
      "ja": {
        "backend": "voicevox",
        "endpoint": "http://127.0.0.1:50021",
        "speaker": 1
      }
    }
  },
  "stt": {
    "backend": "whispercpp",
    "model": "models/stt/whisper/ggml-small-q5_0.bin",
    "lang": "auto",
    "threads": 4,
    "record_seconds": 6
  },
  "reader": {
    "max_chars_per_chunk": 350,
    "pause_between_chunks_ms": 180,
    "strip_markdown_code_blocks": true
  }
}
```

## 17. Implementation phases

### Phase 0 — CPU backend verification only

Deliverables:

- `scripts/voice/README.md`.
- `scripts/voice/check_voice_backends.ps1`.
- documented install/run commands for Piper VI/EN, VOICEVOX JA, whisper.cpp STT.

Exit criteria:

- Vietnamese WAV generated with Piper on CPU.
- English WAV generated with Piper on CPU.
- Japanese WAV generated through local VOICEVOX endpoint on CPU.
- Short WAV transcribed with whisper.cpp CPU.
- No runtime integration yet.

### Phase 1 — Core TTS + TXT/MD reader

Deliverables:

- `VoiceController`, `TTSManager`, `ReaderService`, cache guard.
- Flags: `-voice`, `-say`, `-read-file`, `-read-lang`, `-tts-*`.
- Piper adapter for VI/EN.
- Text/Markdown reader only.
- Simple audio playback; pause/resume can wait until Phase 2 if player is not ready.

Exit criteria:

- Pet starts if backend/model missing, with clear log.
- `-say "xin chào"` speaks when backend exists.
- `-read-file test.md` reads chunks.
- Drag/click remains responsive.
- `go test ./...` passes.
- Debug and release Windows builds pass.

### Phase 2 — Controllable audio + menu controls

Deliverables:

- MCI/waveOut audio player with pause/resume/stop.
- Right-click popup menu reconciled with existing right-click animation.
- Clipboard read.
- Pause/resume/skip/stop commands.
- Voice animation events.

Exit criteria:

- Pause/resume/skip/stop work repeatedly without panic/deadlock.
- Dragging pet during speech does not close app.
- Multi-pet uses global speaker lock.

### Phase 3 — PDF reader spike and implementation

Deliverables:

- Selected PDF library/tool documented.
- Limits documented and enforced: file size, pages, chars, timeout.
- Typed errors for encrypted/no-text/too-large/timeout/malformed.
- Fixtures for text PDF, no-text/scanned PDF, encrypted PDF, malformed PDF.

Exit criteria:

- Text-layer PDF can be read and chunked.
- Scanned PDF reports no extractable text.
- No OCR dependency.

### Phase 4 — STT listen-once

Deliverables:

- Recorder implementation or reviewed equivalent.
- whisper.cpp adapter.
- Safe enum command parser for vi/en/ja.
- Menu item `Listen once`.

Exit criteria:

- `tạm dừng`, `tiếp tục`, `bỏ qua đoạn này`, `dừng đọc` work.
- English/Japanese equivalents map to same enum commands.
- Unknown transcript safe no-op.
- Transcript never invokes shell/hook.

### Phase 5 — Japanese TTS hardening

Deliverables:

- VOICEVOX adapter hardening.
- Voice profile speaker config.
- Japanese smoke tests.

Exit criteria:

- `-say "こんにちは" -read-lang ja` works through local VOICEVOX.
- Missing VOICEVOX logs clear error and pet stays alive.

### Phase 6 — OmniVoice/NeuTTS CPU benchmark

Deliverables:

- Optional command adapter scripts.
- Benchmark doc: install reproducibility, startup time, RAM, seconds per 100 chars for vi/ja/en.

Exit criteria:

- Promote only if CPU performance/install quality is acceptable.
- Otherwise remain experimental.

## 18. Automated test plan

Go unit tests:

- `VOICE-REQ-044`: chunking for Vietnamese, Japanese, English punctuation.
- `VOICE-REQ-040`: local path validation and unsupported extension rejection.
- `VOICE-REQ-050`: transcript maps only to enum commands.
- `VOICE-REQ-060`: assets cannot configure voice commands.
- `VOICE-REQ-071`: cache cleanup requires sentinel.
- `VOICE-REQ-072`: generated filenames are hash/UUID.
- `VOICE-REQ-033`: pause/resume/skip/stop idempotency.
- `VOICE-REQ-081`: missing voice animation fallback is non-fatal.

Review/audit commands, check-only:

```bash
cd go-lite
command -v gofmt >/dev/null
command -v go >/dev/null
test -z "$(gofmt -l *.go)"
go test ./...
GOOS=windows GOARCH=amd64 go build -o pet-lite-debug.exe .
GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -H=windowsgui" -o pet-lite.exe .
rm -f pet-lite-debug.exe pet-lite.exe
```

Build commands create ignored binaries. Remove them after verification or report them as generated artifacts; never commit binaries.

Developer format command, not audit gate:

```bash
cd go-lite
gofmt -w *.go
```

Script/backend check:

```powershell
scripts\voice\check_voice_backends.ps1 -CpuOnly
```

## 19. Manual Windows smoke tests

Phase 1:

```powershell
cd go-lite
.\pet-lite-debug.exe -assets ..\assets -pet pet5 -scale 2 -voice -say "xin chào"
.\pet-lite-debug.exe -assets ..\assets -pet pet5 -scale 2 -voice -read-file "D:\test.md" -read-lang vi
```

Phase 2:

```powershell
.\pet-lite-debug.exe -assets ..\assets -pet pet5 -count 2 -scale 2 -voice
```

Check right-click menu, clipboard read, pause/resume/skip/stop, dragging during speech.

Phase 3 PDF smoke is added only after PDF implementation:

```powershell
.\pet-lite-debug.exe -assets ..\assets -pet pet5 -scale 2 -voice -read-file "D:\text-layer.pdf" -read-lang en
```

Phase 4 STT smoke:

```powershell
.\pet-lite-debug.exe -assets ..\assets -pet pet5 -scale 2 -voice -stt -stt-lang auto
```

## 20. Review checkpoints

Before implementation, approve:

1. Requirement baseline in `docs/VOICE_REQUIREMENTS.md`.
2. Sidecar-first boundary in `docs/ARCHITECTURE.md`.
3. Phase 1 excludes PDF; PDF is Phase 3.
4. Piper VI/EN + VOICEVOX JA + whisper.cpp STT CPU backend direction.
5. Right-click menu behavior versus current right-click animation.
6. Cache override sentinel policy.
7. STT listen-once only, no always-listening.
