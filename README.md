# desktop-pet-lite

## Documentation

Tài liệu chi tiết nằm trong `docs/`:

- `docs/HANDBOOK.md`: handbook vận hành/phát triển hằng ngày.
- `docs/ARCHITECTURE.md`: kiến trúc module, data flow, config model.
- `docs/VOICE_REQUIREMENTS.md`: requirement baseline cho voice/reader/TTS/STT CPU-only; đọc trước khi review hoặc implement voice.
- `docs/STUDYBOOK.md`: studybook giải thích kỹ thuật và quyết định thiết kế.
- `docs/CASE_STUDIES.md`: case studies/decision records từ các lỗi đã gặp.
- `docs/ROADMAP.md`: roadmap phát triển tiếp theo.
- `docs/ADD_NEW_PET.md`: hướng dẫn tự thêm pet mới từ sprite sheet 8x5.
- `scripts/sprite_row_cutter_gui.py`: GUI cắt từng hàng animation 1x5 đến 1x10 thủ công.


Desktop Pet Lite là runtime pet Windows viết bằng Go/Win32. Voice/reader là tùy chọn; STT/TTS chạy local CPU, còn hội thoại `-voice` dùng ZeroClaw v0.8.3 Daemon + localhost Gateway làm brain.

## Voice / reader — supported VI/JA conversation

Từ thư mục gốc trên Windows 11, full voice/conversation chỉ cần một lệnh setup và một lệnh chạy PET:

```powershell
.\scripts\setup-voice.ps1
.\go-lite\pet-lite.exe -assets .\assets -pet pet5 -voice
```

Setup dùng `uv.lock` cho voice-sidecar, kiểm tra/tải faster-whisper base CPU/int8, Piper `vi_VN-vais1000-medium` + `en_US-lessac-medium`, cài Kokoro 0.9.4 + `misaki[ja]`/UniDic cho Japanese TTS và prewarm voice `jf_alpha`. Setup cũng tải đúng ZeroClaw Windows v0.8.3 prebuilt theo SHA-256, migrate config ZeroClaw hiện có, tạo agent `pet`, khóa Gateway ở `127.0.0.1:42617`, pair một bearer token riêng dưới `.voice/zeroclaw/`, rồi cài/start `ZeroClaw Daemon` bằng service lifecycle chính thức. Không tải hoặc start llama.cpp/Gemma trong flow Phase 10.

ZeroClaw chạy độc lập với PET. Đóng/restart `pet-lite.exe` hoặc voice-sidecar không dừng daemon; PET reconnect Gateway ở lần chạy tiếp theo. Provider/model/credentials vẫn do config ZeroClaw của user sở hữu, không được copy vào repo hoặc PET.

Luồng `-voice`:

```text
microphone -> VAD -> faster-whisper auto language
           -> wake/session
           -> deterministic command matcher
              -> matched: execute local, không gọi ZeroClaw
              -> unmatched: ZeroClaw /ws/chat agent=pet session_id=desktop-pet-voice
                            -> local TTS -> speaker
```

Wake Vietnamese hiện có vẫn giữ nguyên. Japanese hỗ trợ `ペット` và `ねえ ペット`; near-match như `ペットボトル` không wake. Faster-whisper tự detect Vietnamese/Japanese thay vì bị ép `vi`. Reply tiếng Việt dùng Piper; reply Japanese dùng Kokoro-82M `jf_alpha`.

`-command` vẫn là Phase 9 deterministic-only: click -> nghe đúng một utterance -> command matcher -> local action. Mode này không tạo ZeroClaw turn và không phụ thuộc daemon/model provider.

Các command local hỗ trợ:

| Vietnamese | English | Action |
|---|---|---|
| `tạm dừng` | `pause` | Pause playback |
| `tiếp tục` | `resume` | Resume playback |
| `bỏ qua` / `đoạn tiếp` | `skip` / `next` | Skip current reader chunk |
| `dừng đọc` / `dừng lại` | `stop reading` / `stop` | Stop current speech/read |
| `đọc clipboard` | `read clipboard` | Read Windows clipboard |
| `trạng thái` | `status` | Speak local status |

Direct speech và local TXT/MD reader cũ vẫn dùng VI/EN path:

```powershell
.\go-lite\pet-lite.exe -assets .\assets -pet pet5 -say "Xin chào"
.\go-lite\pet-lite.exe -assets .\assets -pet pet5 -say "Hello there"
.\go-lite\pet-lite.exe -assets .\assets -pet pet5 -read-file .\note.txt -read-lang auto
.\go-lite\pet-lite.exe -assets .\assets -pet pet5 -read-file .\note.md -read-lang auto
```

`-read-lang` nhận `auto|vi|en`; `auto` chọn `vi` khi có dấu tiếng Việt và `en` cho Latin text còn lại. Reader chỉ hỗ trợ UTF-8 `.txt`/`.md`, chunk deterministic theo paragraph -> sentence punctuation -> hard cap 350 ký tự. `-say`/`-read-file` không mở microphone nếu không có thêm `-voice`.

Right-click menu giữ đúng bốn control voice: `Read clipboard`, `Pause/Resume`, `Skip`, `Stop`. Clipboard dùng Windows clipboard thật và cùng reader/language path; click, drag, movement và render không chờ STT/TTS/Gateway inference.

PET không còn sở hữu conversation memory/persona trong Phase 10. `.voice/persona.txt` và `.voice/history.json` từ Phase 7 có thể còn trên máy nhưng normal `-voice` không đọc/ghi chúng; conversation state, tools và model routing thuộc ZeroClaw.

Phase 11 dùng chính agent `pet` và session `desktop-pet-voice` cho natural VI/JA planning, lưu/nhớ lại note ngắn qua ZeroClaw memory, và one-shot reminder qua ZeroClaw cron. `setup-voice.ps1` chỉ bật agentic runtime cho riêng `pet` khi profile hiện tại chưa bật; PET không có planner, note DB, scheduler hay MCP riêng. Khi normal `-voice` chạy, PET mở thêm một `/ws/chat` watcher bằng handshake `connect` (không tạo user/model turn) để nhận `cron_result`; reminder thành công đi vào đúng local speech queue/TTS hiện có. Restart riêng PET không dừng daemon nên reminder ZeroClaw đã lưu vẫn tồn tại và được nói nếu PET reconnect trước lúc job fire.

ZeroClaw `thinking` và `plan`/`tool_call`/`tool_result` được map vào semantic thinking/working reaction hiện có; final response vẫn đi qua local VI/JA TTS. Phase 9 deterministic command vẫn được match và execute trước ZeroClaw, nên không tạo agent turn.

Fail-soft là supported behavior: ZeroClaw/Gateway/provider unavailable không được làm visual pet hoặc Phase 9 `-command` chết; turn hội thoại lỗi được log và voice session quay lại listening. Reminder thất bại hoặc watcher lỗi chỉ log + semantic error, không biến output lỗi thành speech. Nếu STT/TTS, microphone hoặc speaker lỗi, visual pet vẫn tiếp tục chạy và lỗi local được ghi ở `go-lite\pet-lite.log`.

Acceptance tự động có thể đặt `DESKTOP_PET_VOICE_TEST_WAV_SEQUENCE` tới manifest WAV 16 kHz / 16-bit mono để vẫn chạy qua VAD/STT/session/router thật. Không đặt biến này khi dùng microphone thật.

Các note phase/benchmark cũ trong repo là historical evidence; phần này cùng phần animation bên dưới là runbook supported đến Phase 12 hiện tại.

## Chạy nhanh

Linux dev workflow:

```bash
cd go-lite
go test ./...
GOOS=windows GOARCH=amd64 go build -o pet-lite.exe .
```

Linux dùng để test core và cross-compile. GUI trong suốt vẫn là Win32-only, cần smoke test trên Windows thật.


```powershell
cd E:\git-project\desktop-pet-lite\go-lite
.\pet-lite.exe -assets ..\assets -pet pet5
```

Chọn nhiều pet:

```powershell
.\pet-lite.exe -assets ..\assets -pet all
```

Chỉnh kích cỡ pet trong `pet.json`, không cần truyền lệnh mỗi lần. Ví dụ global default 50%:

```json
// E:\git-project\desktop-pet-lite\assets\pet.json
{
  "scale": 0.5
}
```

Nếu muốn pet riêng khác kích cỡ, đặt trong file riêng của pet:

```json
// E:\git-project\desktop-pet-lite\assets\pets\pet5\pet.json
{
  "scale": 0.35
}
```

Với frame gốc 256x256:

```text
"scale": 1.0  => 256x256
"scale": 0.75 => 192x192
"scale": 0.5  => 128x128
"scale": 0.35 => khoảng 90x90
```

`-scale` vẫn còn nhưng chỉ dùng để override tạm khi test. Runtime chuẩn nên đọc scale từ `pet.json`.

Chạy toàn bộ pet đã import:

```powershell
.\pet-lite.exe -assets ..\assets -pet all
```

Tạo nhiều instance của pet đầu tiên:

```powershell
.\pet-lite.exe -assets ..\assets -pet pet5 -count 3
```

Liệt kê animation được load:

```powershell
.\pet-lite.exe -assets ..\assets -pet pet5 -catalog
```

## Build/test

Review/check-only, không sửa working tree:

```powershell
cd E:\git-project\desktop-pet-lite\go-lite
Get-Command gofmt -ErrorAction Stop | Out-Null
Get-Command go -ErrorAction Stop | Out-Null
if ((gofmt -l *.go).Length -ne 0) { throw "gofmt check failed" }
go test ./...
go build -o pet-lite-debug.exe .
go build -ldflags='-s -w -H=windowsgui' -o pet-lite.exe .
```

Các lệnh build tạo binary ignored; xóa sau khi verify nếu không cần giữ local.

Developer formatting/fix khi cần sửa format:

```powershell
gofmt -w *.go
```

## Cấu trúc pet

```text
assets/
  pet.json                    # cấu hình mặc định dùng chung
  pets/
    pet5/
      pet.json                # cấu hình riêng, được merge in-memory với assets/pet.json
      animations/
        idle.png
        walk.png
        run.png
        angry.png
```

Mỗi file trong `animations/<act>.png` là một horizontal strip gồm số frame tùy ý. Mỗi frame phải đúng `frame_width` x `frame_height`; runtime decode/validate PNG một lần khi load và tự phát hiện frame count từ `strip_width / frame_width`. `frames`/`columns` cũ trong manifest không còn là giới hạn số frame thực tế; `-catalog` load sprite store thật nên vừa hiển thị frame count đã detect vừa bắt lỗi strip sai chiều cao/chiều rộng.

### Author animation từ các PNG frame rời

Đặt các PNG frame đã vẽ vào một folder và dùng tên zero-padded để thứ tự lexical chính là thứ tự animation, ví dụ `0001.png`, `0002.png`, `0003.png`. Tool offline hiện có sẽ trim theo alpha bounds, giữ nguyên pixel (không scale), căn **visible bottom-center/feet** của mọi frame vào cùng `baseline-y`, đặt lên canvas trong suốt cố định rồi ghép thành một strip ngang:

```bash
cd go-lite
go run -tags tools split_assets.go \
  -frames /path/to/idle-frames \
  -out /path/to/output/idle.png \
  -width 256 -height 256 -baseline-y 224
```

Folder input chỉ cần các PNG frame; file không phải PNG bị bỏ qua. Frame fully-transparent, PNG hỏng, hoặc visible bounds không thể fit canvas/baseline sẽ làm command fail thay vì crop/scale ngầm. FPS, loop, `duration_ms`/`duration_s`, `native_facing`, kind và metadata animation vẫn nằm trong `pet.json`; `frames` nên ghi đúng N cho dễ đọc nhưng runtime vẫn detect N từ strip thật. Tool này chỉ pack/validate pixel art offline, không chạy image generation lúc PET runtime.

Với `pet5`, canvas chuẩn hiện tại là 256x256 và `baseline_y=224`. Khi author pose mới, giữ chân/đáy thân quanh bottom-center; không cần tự căn padding giống nhau giữa các source PNG vì packer chuẩn hóa anchor đó. Chỉ đưa strip kết quả vào `assets/pets/<pet>/animations/<clip>.png` khi đó là artwork thật cần thêm/cập nhật, rồi chạy `pet-lite.exe -assets ..\assets -pet <pet> -catalog` để decode/validate asset runtime.

Runtime Phase 12 cache BGRA đã scale/flip theo animation+frame+size+facing trong `SpriteStore`; các instance cùng pet dùng chung store/cache. Tick Win32 vẫn 16 ms cho input/brain/movement, nhưng pixel buffer chỉ copy khi visual frame thực sự đổi và `UpdateLayeredWindow` được bỏ qua khi cả frame, vị trí nguyên-pixel, facing và size đều không đổi.

ZeroClaw vẫn chỉ phát semantic state (`listening`, `thinking`, `working`, `speaking`, success/error); renderer không biết intent. `pet5` hiện chưa có artwork riêng cho các state agent này, nên resolver tiếp tục dùng fallback animation hiện có thay vì invent/generated art trong runtime.

Không cần hard-code tên pet trong code. Folder nào trong `assets/pets/<pet_id>` có `animations` thì được discover. App chỉ chạy pet được chọn qua `-pet`, nên không còn tự chạy tất cả pet ngoài ý muốn.

## Quy tắc animation

`walk` và `run` là locomotion duy nhất mặc định. Emotion như `cry`, `angry`, `happy`, `dizzy` không được dùng để di chuyển.

Runtime tự flip theo `native_facing`, nên không cần tạo `walk_left.png`/`walk_right.png`. Các act direction-specific này đang nằm trong `act_blacklist` để tránh lỗi pet đi trái nhưng hoạt cảnh nhìn sang phải hoặc ngược lại.

## Đồng bộ pet.json

`assets/pet.json` là nguồn cấu hình chung. Khi app mở, manifest được merge trong bộ nhớ với config chung, sau đó scan thư mục `animations`; runtime/catalog bình thường không tự ghi lại `pet.json`.

Nếu thêm `fly.png` hoặc `fight.png`, runtime/catalog nhận diện act tương ứng trong manifest in-memory với params default. Để persist metadata, chỉnh `pet.json` thủ công hoặc dùng sync path/tool explicit khi được expose; runtime/catalog bình thường không ghi `pet.json` hoặc `.petcache.json`.

## Interaction

Click giữ chuột trái: pet bị kéo và random emotion như `cry`, `angry`, `scared`, `dizzy` nếu pet có các act này.

Thả chuột trái: pet chạy `wave` nếu có.

Chuột phải: pet chạy `right_click`; riêng `pet5` random một trong `angry`, `cry`, `happy`, `wave` để luôn có phản hồi với asset hiện có. Có thể gắn command ngoài:

```powershell
.\pet-lite.exe -assets ..\assets -pet pet5 -right-cmd "Start-Process notepad"
```

Click trái cũng có thể gọi hook ngoài bằng `-click-cmd`, còn animation khi giữ chuột vẫn theo drag emotion.

Hook chạy bất đồng bộ, có timeout mặc định 15 giây và giới hạn concurrency để tránh click spam tạo quá nhiều process. Có thể chỉnh timeout bằng `-hook-timeout-ms`; đặt `<=0` để tắt timeout khi debug command local dài.

`-click-cmd` và `-right-cmd` chỉ dành cho command local đáng tin cậy. Không nhận shell command từ asset bundle không tin cậy; nếu sau này cần hook từ asset, nên dùng allowlist action thay vì command tự do.
