# Desktop Pet Lite Case Studies

Tài liệu này ghi lại các case study/decision record trong quá trình phát triển `desktop-pet-lite`. Mục tiêu là để sau này đọc lại hiểu vì sao project chọn hướng hiện tại, lỗi nào đã gặp, và cách tránh lặp lại.

## Case 1: Python prototype sang Go runtime

### Vấn đề

Ban đầu app dễ quản lý bằng Python, nhưng user yêu cầu runtime nhẹ, exe nhỏ, có thể nhúng như plugin/sidecar cho app khác. Python runtime không phù hợp với mục tiêu RAM thấp và phát hành exe gọn.

### Quyết định

- Runtime chuyển sang Go + Win32.
- Python giữ vai trò tool offline trong `scripts/`.
- Không đưa Python dependency vào app chạy thật.

### Kết quả

- EXE nhỏ hơn nhiều so với Python package.
- Runtime vẫn không đạt 2-3 MB RAM, nhưng nhẹ hơn hướng Python đáng kể.
- Code Win32 phức tạp hơn, cần log/test tốt hơn.

### Bài học

Dùng Python cho asset pipeline là đúng. Dùng Python cho runtime desktop pet lâu dài là sai mục tiêu nếu cần nhẹ.

## Case 2: Một sheet lớn chứa nhiều hàng animation

### Vấn đề

Ảnh source là sheet 8x5, kích thước khoảng `992x1586`, margin bốn cạnh không đều. Cắt thô bằng `width/5` và `height/8` khiến frame bị mất chân hoặc dư đầu.

### Thử nghiệm sai

Tool từng detect content bằng threshold nền trắng/checkerboard rồi crop. Kết quả: mặt, da và highlight bị rỗ vì các pixel sáng bị hiểu nhầm là background.

### Quyết định

- Không dùng threshold foreground toàn ảnh.
- Crop theo geometry sheet ổn định.
- Normalize frame về `256x256`.
- Nếu cần xóa nền, dùng border flood-fill với `--remove-bg`, chỉ xóa nền nối với viền ngoài.

### Bài học

Background removal phải bảo toàn nội bộ nhân vật. Với anime/chibi màu sáng, threshold theo màu nền rất nguy hiểm.

## Case 3: Asset của nhiều pet bị mix

### Vấn đề

Khi thêm pet mới có hình dạng khác, nếu runtime dùng chung folder hoặc hard-code animation name, asset các pet dễ bị trộn.

### Quyết định

Mỗi pet có folder riêng:

```text
assets/pets/<pet_id>/animations/*.png
assets/pets/<pet_id>/pet.json
```

Global default config nằm ở:

```text
assets/pet.json
```

Runtime chỉ chạy pet được chọn bằng `-pet`.

### Kết quả

Pet độc lập hơn. Sau này thêm `pet4` chỉ cần import asset vào folder mới và chạy:

```powershell
.\pet-lite.exe -assets ..\assets -pet pet4
```

### Bài học

Folder boundary quan trọng hơn naming convention. Pet id phải là namespace cho toàn bộ animation/config.

## Case 4: Config trùng chức năng

### Vấn đề

Có nhiều file config như `default.json`, `animations.json`, `pet.json`, làm flow khó hiểu và dễ mâu thuẫn.

### Quyết định

- Xóa `animations.json`.
- Xóa `profiles/default.json` trong flow mặc định.
- Dùng `assets/pet.json` làm default config duy nhất.
- Pet riêng dùng `assets/pets/<pet_id>/pet.json`.

### Bài học

Config layer phải ít và rõ. Default chung + override riêng là đủ cho giai đoạn này.

## Case 5: Auto-scan animation tạo lỗi sai hướng

### Vấn đề

Nếu folder có `walk_right.png`, `walk_left.png`, runtime auto-add thành act mới. Khi kết hợp với logic flip, pet có thể đi trái nhưng animation nhìn phải hoặc ngược lại.

### Quyết định

Thêm `act_blacklist`:

```json
["source", "preview", "*_draft", "*_tmp", "walk_left", "walk_right", "run_left", "run_right"]
```

Runtime hiện dùng `walk.png`/`run.png` + `native_facing` + flip.

### Bài học

Auto-discovery phải có blacklist/allowlist. Không thể scan mọi PNG và coi là act hợp lệ.

## Case 6: Click pet bị force close

### Vấn đề

Click/drag/right-click đi qua Win32 `wndProc`. Nếu panic xảy ra trong message handler hoặc render loop, app GUI có thể tắt im lặng.

### Quyết định

- Thêm panic guard quanh render loop, draw frame, wndProc.
- Ghi lỗi vào `go-lite/pet-lite.log`.
- Sanitize interaction nếu trỏ tới animation không tồn tại.

### Bài học

App GUI cần log file ngay từ đầu. Nếu build `-H=windowsgui`, lỗi stdout/stderr không còn dễ thấy.

## Case 7: Auto chạy tất cả pet gây khó kiểm soát

### Vấn đề

Khi runtime auto-discover rồi chạy tất cả pet, user không kiểm soát được pet nào bật. Với nhiều pet, RAM và UI rối hơn.

### Quyết định

- Mặc định `-pet pet1`.
- Chọn nhiều pet bằng `-pet pet1,pet2`.
- Chỉ khi user muốn mới dùng `-pet all`.

### Bài học

Discovery không đồng nghĩa activation. Runtime có thể biết tất cả pet, nhưng chỉ nên chạy pet được chọn.

## Case 8: Emotion bị dùng làm movement

### Vấn đề

Có lúc pet vừa khóc vừa đi hoặc đang dùng emotion nhưng logic movement vẫn chạy. Điều này làm hành vi thiếu tự nhiên.

### Quyết định

`AnimationDef` có field:

```json
"locomotion": true/false
```

Chỉ `walk` và `run` mặc định là locomotion. Emotion như `cry`, `angry`, `happy` là non-locomotion.

### Bài học

Animation metadata không chỉ là FPS/file. Nó cần mô tả semantics: move/state/emotion/reaction.

## Case 9: RAM mục tiêu 2-3 MB

### Vấn đề

User muốn RAM chỉ khoảng 2-3 MB. Runtime Go + decoded PNG + layered window thực tế lên vài chục MB.

### Đánh giá

2-3 MB RAM là không realistic cho Go runtime hiện tại. EXE vài MB không đồng nghĩa working set vài MB.

### Hướng tối ưu nếu bắt buộc

- C/C++ Win32 thuần;
- WIC/GDI+ decode lazy;
- custom sprite compression;
- load current animation only;
- giảm frame size;
- hạn chế số window;
- shared atlas/buffer.

### Bài học

Phải tách rõ exe size, private memory, working set, GPU/GDI resource. Tối ưu RAM là một workstream riêng, không thể giải quyết chỉ bằng `go build -ldflags`.

## Case 10: Tích hợp TTS/Vision/OpenAI/float chat

### Vấn đề

Pet có thể cần TTS, Vision model, OpenAI completion, float chat UI. Nếu nhồi hết vào pet runtime, app sẽ nặng và coupling cao.

### Quyết định đề xuất

Pet runtime là visual sidecar. AI/TTS/Vision nằm ở service/app khác.

```text
Host app / AI service -> event -> pet runtime -> animation/effect
```

Ví dụ:

```json
{"event":"music_start","pet":"pet1","animation":"dance"}
{"event":"tts_start","pet":"pet1","animation":"thinking"}
{"event":"api_error","pet":"pet1","animation":"dizzy"}
```

### Bài học

Pet runtime nên là actor/render layer, không phải AI monolith.

## Case 11: Hướng phát triển control panel

### Bài toán

Khi có nhiều pet, user cần chọn pet nào bật, scale, animation preview, và config override.

### Thiết kế đề xuất

Control panel có thể là app riêng hoặc mode riêng:

```text
pet-control.exe
├─ list pets
├─ preview animations
├─ enable/disable pet
├─ set scale/count
├─ edit interaction mapping
└─ save selected profile
```

Runtime vẫn giữ lightweight. Control panel chỉ chạy khi cần.

### Bài học

Không nên nhét panel vào runtime nếu mục tiêu là nhẹ. Panel là management tool riêng.

## Case 12: Contract giữa asset tool và runtime

### Contract hiện tại

Asset tool xuất:

```text
assets/pets/<pet_id>/animations/<act>.png
```

Mỗi file:

```text
width  = 5 * frame_width
height = frame_height
frame_width = 256
frame_height = 256
```

Runtime đọc:

```json
{
  "frame_width": 256,
  "frame_height": 256,
  "columns": 5
}
```

### Bài học

Miễn giữ contract này, runtime không quan tâm source image ban đầu là 992x1586 hay kích thước khác. Tool có thể thay đổi, runtime vẫn ổn.

## Tổng kết

Project đang đi đúng hướng khi tách thành ba layer:

1. `scripts/`: chuẩn hóa asset;
2. `assets/`: manifest + animation contract;
3. `go-lite/`: runtime Win32 nhẹ.

Các lỗi đã gặp chủ yếu đến từ việc boundary chưa rõ: config trùng, asset mix, auto-scan quá rộng, emotion/movement lẫn nhau. Các thay đổi gần đây đang đưa project về hướng có thể mở rộng: pet độc lập, chọn pet rõ, sync config tự động, và integration qua event thay vì nhúng AI trực tiếp vào runtime.
