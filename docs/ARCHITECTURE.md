# Desktop Pet Lite Architecture

Tài liệu này mô tả kiến trúc kỹ thuật của `desktop-pet-lite` ở mức module và data flow.

## 1. Mục tiêu kiến trúc

Project hướng tới một desktop pet runtime trên Windows 11 với các đặc tính:

- executable nhỏ, không phụ thuộc Python runtime;
- có thể chạy một hoặc nhiều pet;
- mỗi pet có namespace asset/config riêng;
- animation được quản lý bằng file PNG strip;
- có thể thêm animation mới mà không sửa code;
- có thể tích hợp với app khác qua command/API/event;
- runtime giữ nhiệm vụ visual/interaction, không ôm AI/TTS/Vision nặng.

## 2. Layer tổng quan

```text
+----------------------------------------------------------+
| Host App / AI Service / Music App / Control Panel        |
| - optional                                               |
| - gửi event/command tới pet runtime                      |
+-----------------------------+----------------------------+
                              |
                              v
+----------------------------------------------------------+
| go-lite / pet-lite.exe                                  |
| - Go + Win32 runtime                                    |
| - load selected pets                                    |
| - state machine                                         |
| - transparent layered windows                           |
| - mouse interaction                                     |
+-----------------------------+----------------------------+
                              |
                              v
+----------------------------------------------------------+
| assets/                                                  |
| - pet.json default config                               |
| - pets/<pet_id>/pet.json                                |
| - pets/<pet_id>/animations/<act>.png                    |
+-----------------------------+----------------------------+
                              ^
                              |
+----------------------------------------------------------+
| scripts/                                                 |
| - Python asset tools                                    |
| - import sheet 8x5 -> per-act strips                    |
| - future validate/contact-sheet/reorder tools           |
+----------------------------------------------------------+
```

## 3. Runtime data flow

Khi chạy:

```powershell
pet-lite.exe -assets ..\assets -pet pet1,pet2
```

Flow:

```text
parse flags
  -> init log
  -> loadRuntimeProfile
  -> discover assets/pets/*
  -> filter by -pet
  -> for each selected pet:
       LoadPetManifestSynced(default, pet_dir)
       LoadSpriteStore(manifest)
       NewPet(...)
       create layered window
  -> start update/render loop
  -> enter Win32 message loop
```

## 4. Config model

### Default config

```text
assets/pet.json
```

Chứa default animation metadata, motion config, interactions, blacklist.

### Per-pet config

```text
assets/pets/<pet_id>/pet.json
```

Chứa override riêng. Có thể rất nhỏ. Runtime sẽ merge default vào.

### Merge rule

```text
final_manifest = merge(default_manifest, pet_manifest, scanned_animation_files)
```

Pet-specific value thắng default value. Animation mới trong folder được auto-add nếu không nằm trong `act_blacklist`.

## 5. Sprite contract

Runtime không đọc source sheet gốc. Runtime chỉ đọc animation strips:

```text
assets/pets/<pet_id>/animations/<act>.png
```

Contract:

```text
frame_width  = 256
frame_height = 256
columns      = 5
strip_width  = frame_width * columns = 1280
strip_height = frame_height = 256
```

Nếu thay đổi contract, phải đổi cả `assets/pet.json`, tool import, và test.

## 6. State machine concept

State hiện tại đang được cài trong `pet.go`. Concept target:

```text
Idle
  -> Move       khi roam target được chọn
  -> Drag       khi mouse down
  -> Reaction   khi click/right-click/event
  -> Sleep      khi idle lâu

Move
  -> Idle       khi đến target
  -> Drag       khi mouse down

Drag
  -> Reaction   khi thả chuột

Reaction
  -> Idle       khi hết duration
```

Animation policy:

```text
Move      -> chỉ dùng locomotion=true
Reaction  -> locomotion=false
Drag      -> locomotion=false, random emotion
Idle      -> idle/sleepy/sit_idle
```

## 7. Direction policy

Hiện tại dùng policy `flip`:

```text
walk.png / run.png là sprite gốc
native_facing cho biết sprite gốc nhìn trái hay phải
runtime flip khi facing ngược lại
```

Không dùng `walk_left.png`, `walk_right.png` trong mode hiện tại. Các file này bị blacklist để tránh conflict.

Future explicit mode:

```json
"directional_animation_mode": "explicit"
```

Khi đó runtime có thể chọn:

```text
walk_left.png
walk_right.png
run_left.png
run_right.png
```

## 8. Window/render model

Runtime dùng Win32 transparent/layered window cho pet. Mỗi pet instance có window riêng. Render loop update frame theo FPS animation và vị trí pet.

Ưu điểm:

- dễ kéo từng pet;
- dễ xử lý hitbox/window riêng;
- dễ spawn nhiều pet.

Nhược điểm:

- nhiều window có thể tăng RAM/GDI resources;
- layered window có overhead;
- cần careful crash/log handling trong Win32 callback.

## 9. Integration model

Không nhúng AI/TTS/Vision vào runtime. Runtime chỉ nên expose event bridge.

Host app gửi:

```json
{
  "pet": "pet1",
  "action": "play",
  "animation": "dance",
  "duration_ms": 5000
}
```

Runtime gửi lại:

```json
{
  "pet": "pet1",
  "event": "right_click",
  "x": 100,
  "y": 800
}
```

Candidate transports:

- local HTTP;
- named pipe;
- stdin/stdout JSON-RPC;
- Windows message.

## 10. Error handling

GUI app không nên fail silent. Hiện runtime có log file:

```text
go-lite/pet-lite.log
```

Các callback quan trọng có panic guard:

- render loop;
- drawPet;
- wndProc.

Khi debug nên build console mode, không dùng `-H=windowsgui`.

## 11. Boundaries cần giữ

- `scripts/` có thể dùng Pillow/Python, runtime không.
- `assets/` là contract giữa tool và runtime.
- `go-lite/` không đọc source sheet gốc.
- Pet runtime không gọi OpenAI trực tiếp trong phase hiện tại.
- Control panel nên là app riêng hoặc mode riêng, không làm runtime chính nặng.
