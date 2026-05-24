# Desktop Pet Lite Handbook

Tài liệu này là handbook vận hành và phát triển cho `desktop-pet-lite`. Mục tiêu của project là tạo một runtime pet desktop Windows nhẹ, có thể bật một hoặc nhiều pet, có kho animation độc lập cho từng pet, và có thể tích hợp dần với các app khác qua event/hook/plugin.

## 1. Nguyên tắc kiến trúc

Runtime chính là Go/Win32. Python chỉ dùng cho tool xử lý asset trong `scripts/`, không chạy cùng app pet.

Thiết kế hiện tại ưu tiên:

- runtime đơn giản, ít dependency;
- mỗi pet độc lập trong `assets/pets/<pet_id>`;
- config mặc định chung nằm ở `assets/pet.json`;
- pet riêng có thể override bằng `assets/pets/<pet_id>/pet.json`;
- không hard-code tên pet trong code;
- chỉ chạy pet được chọn qua flag `-pet`;
- animation mới trong folder `animations` được auto-sync vào pet json;
- asset import/cắt sheet để riêng trong script Python.

## 2. Cấu trúc repo

```text
E:\git-project\desktop-pet-lite
├─ README.md
├─ LAST_UPDATE.md
├─ assets
│  ├─ pet.json                  # default config chung
│  ├─ pet1.png                  # source sheet gốc
│  ├─ pet1.1.png                # source sheet phụ của pet1
│  ├─ pet2.png
│  ├─ pet3.png
│  └─ pets
│     ├─ pet1
│     │  ├─ pet.json            # config riêng đã sync
│     │  ├─ .petcache.json      # cache fingerprint
│     │  └─ animations
│     │     ├─ idle.png
│     │     ├─ walk.png
│     │     └─ ...
│     ├─ pet2
│     └─ pet3
├─ go-lite
│  ├─ main.go
│  ├─ config.go
│  ├─ discovery.go
│  ├─ pet.go
│  ├─ sprite.go
│  ├─ host_api.go
│  ├─ config_test.go
│  └─ pet-lite.exe
└─ scripts
   ├─ import_pet_sheet.py
   └─ README.md
```

## 3. Cách chạy

Chạy một pet:

```powershell
cd E:\git-project\desktop-pet-lite\go-lite
.\pet-lite.exe -assets ..\assets -pet pet1
```

Chạy nhiều pet:

```powershell
.\pet-lite.exe -assets ..\assets -pet pet1,pet2
```

Chạy tất cả pet đã import:

```powershell
.\pet-lite.exe -assets ..\assets -pet all
```

Tạo nhiều instance của pet đầu tiên:

```powershell
.\pet-lite.exe -assets ..\assets -pet pet1 -count 3
```

In catalog animation rồi thoát:

```powershell
.\pet-lite.exe -assets ..\assets -pet pet1 -catalog
```

## 4. Build và test

Build bản console, dễ debug:

```powershell
cd E:\git-project\desktop-pet-lite\go-lite
gofmt -w config.go config_test.go discovery.go host_api.go main.go pet.go sprite.go split_assets.go
go test ./...
go build -ldflags='-s -w' -o pet-lite.exe .
```

Build bản GUI không mở console:

```powershell
go build -ldflags='-s -w -H=windowsgui' -o pet-lite.exe .
```

Khi debug crash, dùng bản console hoặc xem log:

```text
go-lite/pet-lite.log
```

## 5. Config chung: `assets/pet.json`

`assets/pet.json` là default config. Nó định nghĩa frame size, columns/frame count, animation directory, hitbox, motion config, interactions, act blacklist, default params cho animation đã biết và params cho animation lạ.

Ví dụ nhóm quan trọng:

```json
{
  "frame_width": 256,
  "frame_height": 256,
  "columns": 5,
  "default_animation": "idle",
  "animation_dir": "animations",
  "act_blacklist": ["source", "preview", "*_draft", "*_tmp", "walk_left", "walk_right", "run_left", "run_right"]
}
```

Không nên tạo lại toàn bộ config cho mỗi pet. Pet riêng chỉ cần override phần khác biệt.

## 6. Config riêng từng pet

Mỗi pet có thể có:

```text
assets/pets/<pet_id>/pet.json
```

Nếu pet json thiếu act/config, runtime merge từ `assets/pet.json`. Nếu có animation mới trong folder `animations`, runtime tự thêm vào `pet.json` của pet đó.

Ví dụ override tối thiểu:

```json
{
  "schema": 4,
  "id": "pet2",
  "name": "Pink Pet",
  "animation_dir": "animations",
  "animations": {}
}
```

## 7. Cơ chế sync và cache

Khi app mở, runtime làm:

1. đọc `assets/pet.json`;
2. đọc `assets/pets/<pet_id>/pet.json` nếu có;
3. merge default vào pet config;
4. scan `assets/pets/<pet_id>/animations/*.png`;
5. bỏ qua act trong `act_blacklist`;
6. thêm act mới vào pet config;
7. sanitize interaction nếu interaction trỏ tới animation không tồn tại;
8. ghi lại `pet.json` nếu có thay đổi;
9. ghi `.petcache.json`.

Cache không hash toàn bộ PNG mỗi lần mở app. Nó dùng hash của `assets/pet.json`, hash của pet `pet.json`, và size + mtime của từng PNG trong `animations`.

## 8. Quy tắc animation

Mỗi animation là một strip PNG ngang:

```text
width  = 1280 px
height = 256 px
frames = 5
frame  = 256x256 px
```

Tên file chính là tên act:

```text
idle.png    -> act idle
walk.png    -> act walk
run.png     -> act run
angry.png   -> act angry
fly.png     -> act fly
fight.png   -> act fight
```

Quy tắc runtime:

- `walk` và `run` là locomotion mặc định;
- emotion như `cry`, `angry`, `happy`, `dizzy` không di chuyển;
- pet chỉ tự di chuyển ngang, không đi chéo;
- `native_facing` cho biết sprite gốc đang nhìn hướng nào;
- runtime tự flip khi pet đổi hướng;
- không cần tạo `walk_left.png`/`walk_right.png` nếu chỉ khác hướng nhìn.

## 9. `act_blacklist`

`act_blacklist` giúp runtime không tự nhặt file không nên dùng làm act.

```json
"act_blacklist": [
  "source",
  "preview",
  "*_draft",
  "*_tmp",
  "walk_left",
  "walk_right",
  "run_left",
  "run_right"
]
```

Lý do blacklist `walk_left`, `walk_right`: hiện runtime đã có flip theo hướng. Nếu vừa có `walk_right.png`, vừa flip logic, rất dễ xảy ra lỗi pet đi trái nhưng animation vẫn nhìn phải, hoặc ngược lại.

## 10. Import asset từ sheet 8x5

Tool:

```text
scripts/import_pet_sheet.py
```

Cài dependency:

```powershell
py -m pip install pillow
```

Import pet cơ bản:

```powershell
cd E:\git-project\desktop-pet-lite
py .\scripts\import_pet_sheet.py --src .\assets\pet2.png --pet pet2 --overwrite --remove-bg
```

Mapping mặc định:

```text
row 0 -> idle
row 1 -> walk
row 2 -> run
row 3 -> happy
row 4 -> cry
row 5 -> angry
row 6 -> wave
row 7 -> sleepy
```

Import sheet phụ cho pet1:

```powershell
py .\scripts\import_pet_sheet.py `
  --src .\assets\pet1.1.png `
  --pet pet1 `
  --acts surprised,shy,thinking,cheer,scared,dizzy,dance,sit_idle `
  --overwrite `
  --remove-bg
```

Tool hiện tại không dùng threshold để khoét foreground toàn ảnh nữa, vì cách đó làm rỗ mặt/da/highlight. `--remove-bg` dùng border flood-fill: chỉ xóa nền nối với viền ngoài, không xóa pixel sáng bên trong nhân vật.

## 11. Interaction hiện tại

- Click trái/giữ: bắt đầu drag, pet random emotion nếu có act phù hợp.
- Drag hold: định kỳ đổi emotion như `cry`, `angry`, `scared`, `dizzy` nếu pet có.
- Thả chuột: chạy `wave` nếu có.
- Chuột phải: chạy animation cấu hình ở `right_click`, mặc định là `thinking` nếu pet có.
- `-click-cmd` và `-right-cmd` cho phép gọi command ngoài.

Ví dụ:

```powershell
.\pet-lite.exe -assets ..\assets -pet pet1 -right-cmd "Start-Process notepad"
```

## 12. Tích hợp với app khác

Hướng nên dùng là sidecar process:

```text
main-app.exe        # app chính
pet-lite.exe        # pet runtime Go/Win32
audio-service.exe   # TTS/music nếu cần
chat-panel.exe      # float chat UI nếu cần
ai-service.exe      # OpenAI/TTS/Vision worker nếu cần
```

Pet runtime không nên ôm TTS, Vision, OpenAI trực tiếp nếu muốn nhẹ. Pet chỉ nên nhận event:

```text
music_start -> dance
music_stop  -> idle
tts_start   -> thinking / wave
vision_seen -> surprised
api_error   -> dizzy / scared
```

Cách giao tiếp tương lai:

- local HTTP server;
- named pipe;
- Windows message;
- stdin/stdout JSON-RPC;
- file/event queue đơn giản.

Đề xuất thực tế: bắt đầu với local HTTP hoặc named pipe, vì dễ debug và đủ nhanh.

## 13. Tối ưu RAM

EXE khoảng vài MB không đồng nghĩa RAM chỉ vài MB. Go runtime + Win32 layered window + decoded PNG frame buffer có thể lên vài chục MB. Mục tiêu 2-3 MB RAM là rất khó với hướng hiện tại.

Các hướng tối ưu:

1. chỉ load pet đang chạy, không load toàn bộ pet;
2. chỉ decode animation cần dùng thay vì toàn bộ strip;
3. downscale asset trước khi load;
4. dùng một shared frame buffer;
5. tránh tạo nhiều layered window nếu không cần;
6. build native C/C++ nếu bắt buộc RAM cực thấp;
7. dùng atlas một lần và cache frame rect thay vì copy frame nhiều lần.

## 14. Checklist thêm pet mới

1. Đặt source sheet vào `assets`, ví dụ `assets/pet4.png`.
2. Import:

```powershell
py .\scripts\import_pet_sheet.py --src .\assets\pet4.png --pet pet4 --overwrite --remove-bg
```

3. Test catalog:

```powershell
cd .\go-lite
.\pet-lite.exe -assets ..\assets -pet pet4 -catalog
```

4. Chạy pet:

```powershell
.\pet-lite.exe -assets ..\assets -pet pet4
```

5. Nếu pet sai scale/hitbox, chỉnh `assets/pets/pet4/pet.json`.

## 15. Checklist debug lỗi

App không hiện:

- chạy bản console, không dùng `-H=windowsgui`;
- kiểm tra `go-lite/pet-lite.log`;
- chạy `-catalog` xem pet có load animation không;
- kiểm tra flag `-pet` có đúng id folder không;
- kiểm tra `assets/pet.json` có valid JSON không.

Click bị crash:

- xem `pet-lite.log`;
- kiểm tra interaction có trỏ tới animation không tồn tại không;
- chạy `-catalog` để xác nhận act tồn tại;
- test lại bằng một pet duy nhất: `-pet pet1`.

Animation sai hướng:

- kiểm tra `native_facing` trong config;
- tránh dùng `walk_left.png`/`walk_right.png` khi chưa bật directional mode;
- để runtime tự flip từ `walk.png`/`run.png`.

Asset bị rỗ mặt:

- import lại bằng tool mới;
- không dùng threshold foreground crop;
- dùng `--remove-bg` vì nó chỉ xóa nền nối với border.
