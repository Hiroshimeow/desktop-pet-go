# Voice/Reader Requirements

Tài liệu này là requirement baseline cho mọi thay đổi TTS, STT, reader, audio, voice menu, model/cache và integration liên quan tới desktop pet. Mọi plan hoặc implementation voice phải trace về các requirement ID dưới đây.

## 1. Boundary kiến trúc

**VOICE-REQ-001 — Optional sidecar-first architecture**

Voice/reader là subsystem tùy chọn. Runtime `go-lite` chỉ giữ phần orchestration nhẹ: nhận lệnh UI/CLI, đổi pet animation state, quản lý queue/event, gọi backend local qua adapter, và phát audio. Model inference nặng, cài đặt model, benchmark CPU, hoặc server TTS/STT không được nhúng trực tiếp vào render/message loop.

**VOICE-REQ-002 — Pet runtime vẫn là visual/interaction runtime**

Không được biến `go-lite` thành AI suite nặng. Bất kỳ code TTS/STT/PDF nào đưa vào `go-lite` phải có lý do rõ: state machine, process orchestration, audio control, local-file reader, hoặc UI bridge. Backend model cụ thể phải nằm ngoài runtime chính dưới dạng binary/server/script local.

**VOICE-REQ-003 — Feature flag bắt buộc**

Voice không tự bật. Pet phải chạy như hiện tại nếu không truyền `-voice`, `-say`, `-read-file`, `-stt`, hoặc voice profile tương ứng.

## 2. CPU-only và offline

**VOICE-REQ-010 — CPU-only default path**

Tất cả backend supported mặc định phải chạy được CPU-only. Không có CUDA/GPU-required dependency trong path mặc định.

**VOICE-REQ-011 — Không cloud dependency**

Không dùng remote API cho TTS/STT/reader trong supported path. Network duy nhất được phép ở phase đầu là localhost service do user tự chạy, ví dụ VOICEVOX engine trên `127.0.0.1`.

**VOICE-REQ-012 — Missing backend không làm app chết**

Thiếu Piper/VOICEVOX/whisper.cpp/model/Python/script không được làm pet không hiện. Runtime phải log lỗi rõ, disable request/subsystem liên quan, rồi tiếp tục chạy.

## 3. Backend selection

**VOICE-REQ-020 — TTS routing mặc định**

Routing mặc định:

- Vietnamese: Piper CPU voice.
- English: Piper CPU voice.
- Japanese: VOICEVOX localhost CPU engine.
- OmniVoice/NeuTTS: experimental explicit opt-in sau benchmark CPU, không nằm trong default path.

**VOICE-REQ-021 — STT routing mặc định**

Phase 1 fixed-response tiếng Việt dùng duy nhất sidecar `faster-whisper-base` CPU/int8 đã pin revision và được chọn bằng benchmark Windows. Hướng `whisper.cpp` chỉ còn áp dụng cho reader/command phase sau nếu được đo lại và quyết định riêng.

**VOICE-REQ-022 — Ranh giới backend**

Phase 1 không cần provider registry hoặc interface một implementation: Go gọi trực tiếp một sidecar local đã pin, tách khỏi render loop và pet movement. Chỉ thêm hoặc thay backend khi có quyết định mới dựa trên benchmark thực tế.

## 4. Responsiveness và concurrency

**VOICE-REQ-030 — Không block Win32 message loop**

Không synthesis, transcribe, PDF parse, model load, audio wait, hoặc process wait nào được chạy trực tiếp trong window proc/message loop/render critical path.

**VOICE-REQ-031 — Context timeout/cancel bắt buộc**

Mọi job TTS/STT/PDF/read-file phải nhận timeout/cancel. Stop/skip/process exit phải cancel được job đang chạy.

**VOICE-REQ-032 — Global speaker lock mặc định**

Phase đầu chỉ cho một audio speaker toàn app để tránh nhiều pet nói đè nhau. Multi-speaker chỉ được thêm sau khi có policy rõ.

**VOICE-REQ-033 — Idempotent controls**

Pause/resume/skip/stop phải idempotent. Gọi lặp không được panic, deadlock, hoặc làm pet window đóng.

## 5. Reader/input contract

**VOICE-REQ-040 — Input local-only**

Reader chỉ nhận text trực tiếp, clipboard local, hoặc local file path. Không fetch URL/remote path trong phase đầu.

**VOICE-REQ-041 — Phase 1 chỉ TXT/MD**

Phase 1 reader chỉ support `.txt` và `.md`. `.pdf` là Phase 3 riêng, không được quảng cáo là supported trong Phase 1 CLI/test.

**VOICE-REQ-042 — PDF security gate**

Khi thêm PDF, phải có library/strategy được ghi rõ trước implementation, cùng giới hạn: max file size, max pages, max extracted chars, per-file timeout, và error typed cho encrypted/no-text/too-large/timeout/malformed.

**VOICE-REQ-043 — Không OCR trong scope đầu**

Scanned/image PDF không OCR. Runtime phải báo `pdf has no extractable text` hoặc error tương đương, không cố cài dependency OCR nặng.

**VOICE-REQ-044 — Chunking deterministic**

Chunking phải deterministic, giữ Unicode tiếng Việt/Nhật/Anh, split theo paragraph trước rồi sentence punctuation, và hard cap theo `tts_max_chars`.

## 6. STT safety

**VOICE-REQ-050 — STT chỉ emit enum command**

Transcript STT chỉ được map sang enum `VoiceCommand`. Transcript không bao giờ được đưa vào shell, `runHook`, PowerShell, hoặc command template.

**VOICE-REQ-051 — Unknown transcript safe no-op**

Transcript không khớp grammar phải được log và bỏ qua hoặc hỏi lại sau này. Không fallback sang command execution.

**VOICE-REQ-052 — Voice phải opt-in và có bounded listening**

Reader/command STT vẫn dùng listen-once/push-to-talk. Riêng Phase 1 fixed-response STS được phép wake-listening local khi user bật `-voice`: WebRTC VAD chỉ mở STT sau speech, không chạy transcript liên tục, không map transcript sang shell/hook, microphone bị bỏ qua khi TTS phát và có cooldown ngắn trước khi nghe lại. Visual pet phải tiếp tục chạy nếu voice setup/device/model lỗi.

## 7. Command execution boundary

**VOICE-REQ-060 — Asset không được điều khiển command**

Không đọc TTS/STT command từ asset manifest. Pet assets chỉ được chứa animation/config visual.

**VOICE-REQ-061 — Existing hook tách biệt voice**

`-click-cmd` và `-right-cmd` là trusted local CLI hook hiện hữu. Voice subsystem không được gọi các hook này từ transcript hoặc file content.

**VOICE-REQ-062 — Command backend explicit only**

Backend `command` chỉ dành cho advanced local config/CLI, nhận JSON stdin/stdout, không nhận free-form text làm shell command.

## 8. Cache, models, privacy

**VOICE-REQ-070 — Models/cache không commit**

`models/`, `go-lite/cache/`, generated WAV/MP3/FLAC/OGG, STT recordings, transcripts, và TTS cache không được commit.

**VOICE-REQ-071 — Cache cleanup sentinel**

Nếu cho phép `-tts-cache` override, cleanup chỉ được chạy trong cache root có sentinel/manifest do app tạo. Không recursive-delete path user tùy ý.

**VOICE-REQ-072 — Safe generated filenames**

Generated audio/transcript filenames phải dựa trên hash/UUID safe. Không dùng user text hoặc source path trực tiếp làm filename.

**VOICE-REQ-073 — Local privacy**

STT recordings/transcripts và generated speech phải ở local cache. Log không nên dump toàn bộ nội dung file dài; chỉ log path, chunk index, hash, length, và error.

## 9. Audio UX

**VOICE-REQ-080 — Controllable audio player**

Pause/resume/skip cần audio backend có control thật. `PlaySoundW` chỉ được dùng fallback đơn giản; phase có pause/resume phải dùng MCI/waveOut hoặc abstraction tương đương.

**VOICE-REQ-081 — Animation fallback không fatal**

Voice animation event chỉ dùng animation có sẵn. Thiếu `speaking`, `talk`, `thinking` không được fail; runtime fallback về `wave`/`happy`/current/idle.

**VOICE-REQ-082 — Voice menu không phá right-click hiện tại**

Nếu thêm popup menu right-click, phải reconcile với right-click animation/action hiện tại. Menu behavior phải có test/smoke rõ.

## 10. Build/test/review gates

**VOICE-REQ-090 — Audit command không sửa working tree**

Review gate dùng check-only command, ví dụ `test -z "$(gofmt -l *.go)"`. Command sửa file như `gofmt -w` chỉ được đặt ở developer fix workflow, không đặt trong audit gate.

**VOICE-REQ-091 — Debug và release build tách biệt**

Plan phải ghi cả debug console build và release GUI build. Debug dùng console/log thuận tiện; release dùng `-ldflags "-s -w -H=windowsgui"`.

**VOICE-REQ-092 — Tests phải trace requirement**

Test mới nên ghi trong tên hoặc comment requirement liên quan, ví dụ chunking Unicode trace `VOICE-REQ-044`, transcript no-shell trace `VOICE-REQ-050`.

**VOICE-REQ-093 — Dependency spike trước implementation nặng**

Phase 1 đã so sánh `faster-whisper-small` và `faster-whisper-base` trên Windows rồi chọn duy nhất `base` theo latency đo được; Piper cũng phải qua setup/load/playback check. Các hướng reader/command như whisper.cpp, cùng VOICEVOX, PDF library và OmniVoice/NeuTTS, vẫn cần spike/benchmark riêng trước khi được hỗ trợ.

## 11. Documentation consistency

**VOICE-REQ-100 — Một source of truth**

Plan voice phải reference tài liệu này. Nếu requirement thay đổi, cập nhật tài liệu này trước rồi mới sửa plan/README/docs khác.

**VOICE-REQ-101 — Phase support phải nhất quán**

Scope, CLI examples, implementation phases, automated tests, manual smoke tests phải nói cùng một mức support. Không được ghi Phase 1 support PDF nếu PDF nằm Phase 3.
