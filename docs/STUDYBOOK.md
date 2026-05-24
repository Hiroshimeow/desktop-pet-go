# Desktop Pet Lite Studybook

Tài liệu này dùng để học lại toàn bộ project theo hướng kỹ thuật: vì sao chọn kiến trúc hiện tại, các module đang làm gì, điểm yếu hiện tại, và roadmap để biến pet runtime thành nền tảng plugin/assistant nổi trên desktop.

## 1. Bài toán

Yêu cầu sản phẩm:

- pet 2D chạy trên Windows 11;
- click exe là hiện pet;
- pet có animation mượt;
- có thể kéo bằng chuột;
- có thể bật nhiều pet;
- mỗi pet có kho animation riêng;
- sau này có thể tích hợp TTS, Vision, OpenAI API, float chat, music reaction;
- runtime càng nhẹ càng tốt;
- asset thêm mới không cần sửa code.

Điểm khó:

- Windows desktop pet cần transparent top-level window;
- animation PNG cần decode và render liên tục;
- mỗi pet có animation/logic riêng nhưng không được trộn asset;
- nếu dùng Python runtime sẽ dễ quản lý nhưng nặng;
- nếu dùng Go/Win32 nhẹ hơn nhưng code Win32 phức tạp hơn;
- RAM 2-3 MB là mục tiêu rất khó với Go + decoded PNG + layered window.

## 2. Kiến trúc hiện tại

```text
Go runtime
├─ load config
├─ discover selected pets
├─ sync per-pet manifest
├─ load sprite strips
├─ create transparent layered windows
├─ update pet state
├─ render frame
└─ handle mouse events

Python tools
└─ import source sheet -> per-act animation strips
```

Runtime không phụ thuộc Python. Python chỉ xử lý asset trước khi chạy.

## 3. Vì sao không dùng Python runtime?

Python phù hợp cho prototype, tool cắt ảnh, automation, test asset. Nhưng với desktop pet chạy nền lâu dài:

- interpreter overhead lớn;
- dependency GUI thường nặng;
- packaging exe lớn;
- RAM khó xuống thấp;
- nhiều pet/window càng tốn.

Go runtime có lợi:

- build thành một exe nhỏ;
- không cần Python trên máy người dùng;
- concurrency/event loop ổn;
- gọi Win32 API trực tiếp được;
- dễ nhúng/sidecar với app khác.

Nhược điểm:

- Win32 binding thủ công dài;
- xử lý alpha/layered window dễ lỗi;
- RAM vẫn không thể thấp như C/C++ thuần.

## 4. Module chính trong Go

### `main.go`

Vai trò:

- parse flag;
- init log;
- load profile runtime;
- chọn pet bằng `-pet`;
- tạo window;
- chạy update/render loop;
- xử lý Win32 message.

Flag quan trọng:

```text
-assets    đường dẫn assets root
-pet       pet1 hoặc pet1,pet2 hoặc all
-count     số instance cho pet đầu tiên
-scale     override scale
-catalog   in animation catalog rồi thoát
-click-cmd command khi click trái
-right-cmd command khi click phải
```

### `config.go`

Vai trò:

- định nghĩa schema config;
- `PetManifest`;
- `AnimationDef`;
- `MotionConfig`;
- `InteractionAction`;
- validate config.

Khái niệm quan trọng:

```text
locomotion = true   -> animation được phép di chuyển pet
locomotion = false  -> emotion/state/reaction, không tự di chuyển
native_facing       -> hướng gốc của sprite để runtime flip
act_blacklist       -> danh sách act không tự add vào manifest
```

### `discovery.go`

Vai trò:

- discover `assets/pets/*`;
- load `assets/pet.json`;
- load pet-specific `pet.json`;
- merge config;
- scan animation folder;
- auto-add animation mới;
- cache `.petcache.json`;
- sanitize interaction.

Điểm đáng chú ý: app không chạy tất cả pet mặc định nữa. Nó discover được tất cả, nhưng chỉ chạy pet được chọn bằng `-pet`.

### `pet.go`

Vai trò:

- state machine của pet;
- chọn idle/walk/run/emotion;
- random roam;
- drag state;
- direction/facing;
- frame advance.

Nguyên tắc:

- đi ngang, không đi chéo;
- nếu đang drag thì không auto roam;
- nếu move thì dùng act locomotion;
- nếu emotion thì không tự move;
- `walk/run` có thể flip theo hướng.

### `sprite.go`

Vai trò:

- load PNG strips;
- kiểm tra kích thước frame;
- trả về frame rect theo animation + frame index.

Mỗi animation file là strip 5 frame ngang:

```text
1280x256 = 5 * 256x256
```

### `host_api.go`

Vai trò hiện tại: nền cho tích hợp command/hook. Tương lai có thể mở rộng thành local API hoặc plugin bridge.

### `config_test.go`

Vai trò:

- test auto-discover;
- test manifest sync;
- test sprite store;
- test movement không đi chéo;
- test drag emotion không dùng locomotion.

## 5. Asset pipeline

Source sheet từ image generator thường là 5 cột x 8 hàng. Nhưng ảnh có vấn đề:

- margin không đều;
- nền có checkerboard/viền;
- nhân vật không nằm chính giữa;
- nếu threshold background thì mặt/da/highlight bị rỗ;
- nếu crop thô theo width/5 height/8 thì có thể dư đầu/mất chân.

Tool hiện tại chọn compromise:

- crop theo geometry sheet ổn định;
- normalize từng frame vào 256x256;
- optional `--remove-bg` bằng border flood-fill;
- không dùng threshold toàn ảnh để tránh khoét lỗ bên trong nhân vật.

## 6. Manifest và auto-sync

Config chung:

```text
assets/pet.json
```

Config riêng:

```text
assets/pets/<pet_id>/pet.json
```

Khi mở app:

```text
default manifest + pet manifest + animations folder -> final manifest
```

Nếu có file mới:

```text
assets/pets/pet2/animations/fly.png
```

Runtime tự thêm:

```json
"fly": {
  "file": "fly.png",
  "fps": 5,
  "kind": "action",
  "locomotion": false,
  "duration_ms": 1500
}
```

Sau đó developer chỉnh lại nếu cần:

```json
"fly": {
  "file": "fly.png",
  "fps": 8,
  "kind": "move",
  "locomotion": true,
  "speed_px_s": 70,
  "native_facing": "right"
}
```

## 7. Vì sao cần `act_blacklist`?

Auto-scan rất tiện nhưng nguy hiểm nếu folder chứa file không phải act thật:

```text
source.png
preview.png
walk_right.png
run_left.png
old_draft.png
```

Nếu runtime tự add hết, state machine có thể chọn nhầm animation. `act_blacklist` giải quyết bằng pattern đơn giản:

```json
["source", "preview", "*_draft", "*_tmp", "walk_left", "walk_right", "run_left", "run_right"]
```

Hiện tại engine tự flip direction, nên không dùng `walk_left/right`.

## 8. State machine tối thiểu

Trạng thái nên được hiểu như sau:

```text
IdleState
  -> RoamState nếu random roam
  -> DragState nếu mouse down
  -> ReactionState nếu click/right-click/event

RoamState
  -> IdleState khi tới target
  -> DragState nếu mouse down

DragState
  -> ReactionState khi drag end
  -> IdleState sau reaction

ReactionState
  -> IdleState khi hết duration
```

Hiện code chưa tách class/state rõ như diagram, nhưng logic đang đi theo hướng này.

## 9. Plugin/integration model

Không nên biến pet runtime thành app AI đầy đủ. Nên giữ pet là visual layer/event actor.

Mô hình tốt hơn:

```text
Host App
  ├─ owns business logic: music/TTS/chat/OpenAI/vision
  ├─ sends simple event to pet runtime
  └─ receives click/right-click/drag events from pet if needed

Pet Runtime
  ├─ renders pet
  ├─ handles interaction
  ├─ maps events -> animation
  └─ exposes tiny API
```

Ví dụ event:

```json
{
  "pet": "pet1",
  "event": "music_start",
  "animation": "dance",
  "duration_ms": 5000
}
```

## 10. IPC options

### Local HTTP

Ưu:

- dễ debug bằng curl/Postman;
- dễ gọi từ app khác;
- cross-language.

Nhược:

- có port;
- thêm HTTP server;
- cần security local-only.

### Named Pipe

Ưu:

- native Windows;
- không mở port;
- tốt cho plugin local.

Nhược:

- debug khó hơn HTTP;
- cross-language cần wrapper.

### Windows Message

Ưu:

- rất nhẹ;
- hợp Win32.

Nhược:

- payload hạn chế;
- khó tích hợp app non-Win32.

### stdin/stdout JSON-RPC

Ưu:

- pet là child process;
- dễ quản lý vòng đời;
- không port.

Nhược:

- parent app phải spawn pet;
- nếu người dùng tự click exe thì không có parent.

Khuyến nghị giai đoạn tiếp theo: local HTTP cho dev/debug, sau đó thêm named pipe cho production.

## 11. RAM/performance study

Nguồn RAM chính:

- Go runtime;
- decoded PNG RGBA buffer;
- per-window backing buffer;
- GDI/Win32 object;
- multiple pet windows;
- animation strips loaded sẵn.

Cách giảm RAM:

1. không auto load pet không được chọn;
2. lazy-load animation khi cần;
3. unload animation ít dùng;
4. giảm frame size từ 256 xuống 192 hoặc 160;
5. chỉ giữ RGBA cho current/next animation;
6. dùng one-process multi-pet nhưng shared sprite store theo pet;
7. hạn chế shadow/effect overlay.

Nếu mục tiêu bắt buộc là 2-3 MB RAM, cần nghiên cứu C/C++ thuần Win32, WIC/GDI+ decode streaming, hoặc custom RLE/compressed sprite format. Với Go, mục tiêu realistic hơn là vài chục MB.

## 12. Roadmap kỹ thuật

### Phase 1: Stabilize runtime

- fix crash click/right-click;
- tách state machine rõ;
- chỉ load selected pet;
- log lỗi rõ;
- test movement và drag.

### Phase 2: Asset correctness

- preview tool cho từng strip;
- validate frame size;
- validate loop first/last frame;
- reorder frame tool cho walk/run;
- detect transparent holes;
- generate contact sheet để review nhanh.

### Phase 3: Control panel

- UI chọn pet bật/tắt;
- scale từng pet;
- spawn multiple instance;
- preview animation;
- edit per-pet config.

### Phase 4: Plugin API

- local HTTP/named pipe;
- event mapping config;
- external command hooks;
- host app SDK sample.

### Phase 5: AI integration

- float chat panel là process riêng;
- TTS process riêng;
- OpenAI client nằm ở host/AI service;
- pet chỉ nhận event như `tts_start`, `tts_end`, `thinking`, `speaking`.

## 13. Quy tắc không phá flow

- Không thêm config trùng chức năng.
- Không hard-code pet id trong runtime.
- Không để source image trong `animations`.
- Không dùng emotion làm movement.
- Không dùng direction-specific animation khi engine đang tự flip.
- Không thêm Python vào runtime.
- Không auto chạy all pet nếu user chưa chọn.
- Không threshold-remove background toàn ảnh nếu có nguy cơ rỗ mặt.

## 14. Bài học chính

Project desktop pet tưởng đơn giản nhưng thực chất có 4 layer độc lập:

1. asset production;
2. animation metadata;
3. runtime state machine;
4. host/plugin integration.

Nếu trộn 4 layer này, project nhanh chóng rối: pet khác bị mix asset, animation sai hướng, click crash, config trùng, app khó nhúng. Hướng đúng là giữ mỗi layer có boundary rõ, dùng manifest làm contract giữa asset và runtime.
