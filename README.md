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


Desktop Pet Lite là runtime pet Windows viết bằng Go/Win32. Voice là tùy chọn `-voice`; khi bật, pet tự quản lý một sidecar Python 3.11 đã khóa dependency để chạy VAD/STT/TTS local, không cần service thủ công.

## Voice Phase 1 — Vietnamese STS

Từ thư mục gốc trên Windows 11:

```powershell
.\scripts\setup-voice.ps1
.\go-lite\pet-lite.exe -assets .\assets -pet pet5 -voice
```

Bootstrap dùng `uv.lock`, tải đúng `faster-whisper-base` CPU/int8 và Piper `vi_VN-vais1000-medium` vào `.voice/`, kiểm tra SHA-256, kiểm tra thiết bị audio và build `go-lite\pet-lite.exe`. Model/cache/audio sinh ra không được commit.

Khi chạy, nói `pet ơi, em là ai` trong một câu; hoặc nói `pet ơi`, rồi hỏi trong 5 giây. Các biến thể wake `pét/bét/bết ơi` và `mèo ơi` được chấp nhận để chịu lỗi STT tiếng Việt. Intent cố định gồm chào hỏi, danh tính, trạng thái, tạm biệt và unknown. Microphone bị bỏ qua trong lúc TTS phát và thêm cooldown ngắn để pet không nghe chính nó.

Nếu `uv`, model, microphone hoặc speaker lỗi, pet hình ảnh vẫn tiếp tục chạy; xem `go-lite\pet-lite.log` để lấy lỗi có hành động khắc phục. Latency từng lượt được ghi tại `.voice\logs\turns.jsonl` và trong `pet-lite.log`.

Acceptance tự động có thể thay microphone bằng chuỗi WAV test mà vẫn đi qua VAD/STT thật. Đặt các WAV 16 kHz, 16-bit mono PCM và manifest dưới `.voice/`, ví dụ:

```json
{"items":[{"wav":"wake-question.wav","delay_ms":500},{"wav":"wake-only.wav"},{"wav":"follow-up.wav","delay_ms":500}]}
```

Path WAV là tương đối với manifest. Chạy trên PowerShell:

```powershell
$env:DESKTOP_PET_VOICE_TEST_WAV_SEQUENCE = (Resolve-Path .\.voice\acceptance\sequence.json)
.\go-lite\pet-lite-debug.exe -assets .\assets -pet pet5 -voice
Remove-Item Env:DESKTOP_PET_VOICE_TEST_WAV_SEQUENCE
```

Không đặt biến môi trường này khi dùng microphone thật; path production vẫn mở `RawInputStream` mặc định.

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

Mỗi file trong `animations/<act>.png` là một strip 5 frame ngang, mỗi frame 256x256. Runtime đọc theo quy tắc `frame_width=256`, `frame_height=256`, `columns=5`.

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
