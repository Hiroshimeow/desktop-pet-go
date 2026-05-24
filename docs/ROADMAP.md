# Desktop Pet Lite Roadmap

Roadmap này gom các hướng phát triển tiếp theo cho `desktop-pet-lite`, chia theo phase để tránh project phình quá nhanh.

## Phase 0: Current baseline

Hiện trạng:

- Runtime Go/Win32.
- Python chỉ dùng làm asset tool.
- Có `assets/pet.json` làm config chung.
- Có `assets/pets/<pet_id>/pet.json` làm config riêng.
- Có auto-discovery pet folder.
- Có chọn pet bằng `-pet pet1`, `-pet pet1,pet2`, `-pet all`.
- Có import sheet 8x5 thành per-act animation strip.
- Có `act_blacklist`.
- Có movement ngang, không đi chéo.
- Có log crash vào `go-lite/pet-lite.log`.

## Phase 1: Stabilize runtime

Mục tiêu: pet chạy ổn, click/drag/right-click không crash, animation không lẫn movement/emotion.

Task đề xuất:

- Tách state machine rõ thành `Idle`, `Move`, `Drag`, `Reaction`, `Sleep`.
- Thêm unit test cho từng state transition.
- Thêm test cho `right_click`, `drag_start`, `drag_hold`, `drag_end`.
- Thêm guard không cho non-locomotion animation move.
- Thêm log level cơ bản: error/info/debug.
- Thêm command `-validate` để validate assets rồi thoát.

Definition of done:

- `go test ./...` pass.
- `pet-lite.exe -assets ..\assets -pet pet1 -catalog` pass.
- Click/drag/right-click 30 lần không force close.
- Không có case pet vừa khóc vừa tự đi.

## Phase 2: Asset quality tools

Mục tiêu: kiểm soát chất lượng animation trước khi đưa vào runtime.

Task đề xuất:

- `scripts/validate_animations.py`: kiểm tra mọi PNG có đúng `1280x256` không.
- `scripts/contact_sheet.py`: tạo ảnh preview toàn bộ act của một pet.
- `scripts/reorder_frames.py`: reorder frame trong một act, ví dụ walk/run.
- `scripts/trim_transparent_border.py`: trim/normalize alpha nhẹ nếu cần.
- `scripts/check_alpha_holes.py`: cảnh báo frame có lỗ alpha bất thường bên trong nhân vật.
- `scripts/preview_gif.py`: xuất GIF preview từng act.

Definition of done:

- Một lệnh validate được toàn bộ `assets/pets/*`.
- Có contact sheet để review nhanh pet1/pet2/pet3.
- Có tool sửa thứ tự frame walk/run mà không cần generate lại ảnh.

## Phase 3: Control panel

Mục tiêu: user chọn pet bật/tắt, scale, count, preview animation mà không dùng command line.

Thiết kế đề xuất:

```text
pet-control.exe
├─ list pets
├─ start/stop selected pets
├─ set scale/count
├─ preview animations
├─ open pet folder
├─ edit interaction mapping
└─ save user profile
```

Lưu ý: control panel nên là app riêng để runtime chính vẫn nhẹ.

Definition of done:

- Mở panel thấy danh sách pet.
- Tick pet nào thì runtime bật pet đó.
- Scale/count lưu được.
- Preview animation không ảnh hưởng runtime đang chạy.

## Phase 4: Plugin/API bridge

Mục tiêu: app khác có thể điều khiển pet mà không link trực tiếp vào code Go.

API event tối thiểu:

```json
{
  "pet": "pet1",
  "event": "music_start",
  "animation": "dance",
  "duration_ms": 5000
}
```

Candidate IPC:

1. Local HTTP cho dev/debug.
2. Named Pipe cho Windows production.
3. stdin/stdout JSON-RPC nếu pet là child process.
4. Windows Message cho event cực nhẹ.

Khuyến nghị:

- Implement local HTTP trước.
- Sau đó thêm named pipe.

Endpoint gợi ý:

```text
GET  /health
GET  /pets
POST /pets/{id}/play
POST /pets/{id}/say
POST /pets/{id}/effect
POST /shutdown
```

Definition of done:

- App khác gọi được pet play animation.
- Pet gửi được event click/right-click ra host app.
- Có sample client PowerShell hoặc Go.

## Phase 5: Assistant integration

Mục tiêu: dùng pet làm visual layer cho TTS, chat, OpenAI, vision.

Không nên nhúng model/API trực tiếp vào pet runtime. Thiết kế nên là:

```text
AI service / host app
  -> calls OpenAI/TTS/Vision
  -> sends visual events to pet runtime
  -> optionally opens float chat UI
```

Pet chỉ nhận event:

```text
tts_start       -> thinking/speaking
tts_end         -> happy/idle
music_start     -> dance + note effects
music_stop      -> idle
vision_detected -> surprised
api_error       -> dizzy/scared
```

Definition of done:

- Có sample host app gọi event `tts_start`, `tts_end`.
- Có sample music event làm pet `dance`.
- Float chat chạy riêng, không làm pet runtime nặng.

## Phase 6: Performance optimization

Mục tiêu: giảm RAM/CPU và ổn định khi bật nhiều pet.

Task đề xuất:

- Lazy-load animation strips.
- Chỉ decode current/next animation.
- Shared sprite store nếu nhiều instance cùng pet.
- Giảm frame size option: 256 -> 192 hoặc 160.
- Tick rate adaptive: idle pet update chậm hơn moving pet.
- Dirty rect render nếu có thể.
- Profile memory bằng Process Explorer/Windows Performance Recorder.

Definition of done:

- 1 pet idle CPU gần 0%.
- 3 pet vẫn mượt.
- RAM giảm đáng kể so với baseline hiện tại.

## Phase 7: Directional animation mode

Hiện tại runtime tự flip từ `walk.png`/`run.png`. Nếu sau này muốn animation riêng cho từng hướng, cần policy rõ:

```json
"directional_animation_mode": "flip" | "explicit"
```

Mode `flip`:

```text
walk.png + native_facing -> runtime flip
```

Mode `explicit`:

```text
walk_left.png
walk_right.png
run_left.png
run_right.png
```

Khi chưa có mode này, tiếp tục blacklist `walk_left`, `walk_right`, `run_left`, `run_right`.

## Priority ngắn hạn

Thứ tự nên làm tiếp:

1. `-validate` command.
2. `contact_sheet.py` để review asset.
3. `reorder_frames.py` cho walk/run.
4. State machine rõ hơn.
5. Local HTTP API cho plugin thử nghiệm.
6. Control panel sau khi runtime ổn.
