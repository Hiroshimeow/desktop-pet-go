# desktop-pet-lite-go

Go/Win32 desktop pet runner. Đây là runtime chính. Python đã bị loại khỏi runtime; chỉ còn Go exe.

## Runtime files

```text
E:\git-project\desktop-pet-lite\go-lite\pet-lite.exe
E:\git-project\desktop-pet-lite\animations.json
E:\git-project\desktop-pet-lite\assets\animations\*.png
```

Mỗi file PNG trong `assets\animations` là đúng 1 animation, dễ thay thế độc lập.

## Asset format

Mỗi animation là một strip PNG:

```text
width  = 1280 px
height = 256 px
frames = 5
frame  = 256x256 px
order  = left -> right
loop   = frame 5 nối lại frame 1
```

Danh sách hiện có:

```text
idle.png        đứng yên
walk.png        đi ngang
run.png         chạy ngang
happy.png       vui/cười
cry.png         khóc
angry.png       tức giận
wave.png        vẫy tay
sleepy.png      buồn ngủ
surprised.png   ngạc nhiên
shy.png         ngại
thinking.png    suy nghĩ
cheer.png       cổ vũ/vỗ tay
scared.png      sợ
dizzy.png       chóng mặt
dance.png       nhảy theo nhạc
sit_idle.png    ngồi nghỉ
```

## Animation logic

- `walk` và `run` là locomotion duy nhất.
- Emotion như `cry`, `angry`, `happy`, `wave` không được dùng để di chuyển.
- Pet không còn vừa khóc vừa đi.
- Khi cần move: engine chỉ chọn `walk` hoặc `run`.
- Khi emotion/state đang chạy: giữ nguyên vị trí.
- Tự di chuyển chỉ theo trục X, không đi chéo.
- Facing trái/phải được xử lý bằng mirror trong renderer.

## Interaction

- Giữ chuột trái để kéo pet đi khắp màn hình.
- Khi kéo: random `cry`, `angry`, `sleepy`, `wave`.
- Thả chuột trái: `wave`.
- Chuột phải: random `angry`, `cry`, `wave`, `happy`.
- Để im: pet tự `idle`, `walk`, `run`, `sleepy`, `sit_idle`, `thinking`, `wave`, `happy`.

## Run

```powershell
cd E:\git-project\desktop-pet-lite\go-lite
.\pet-lite.exe -assets ..\assets -pet pet4
```

Kích cỡ pet được đọc từ `pet.json`, không cần truyền lệnh mỗi lần.

Global default:

```json
// E:\git-project\desktop-pet-lite\assets\pet.json
{
  "scale": 0.5
}
```

Override riêng từng pet:

```json
// E:\git-project\desktop-pet-lite\assets\pets\pet4\pet.json
{
  "scale": 0.35
}
```

Nếu asset là `256x256`:

```text
"scale": 1.0  => window 256x256
"scale": 0.5  => window 128x128
"scale": 0.35 => window khoảng 90x90
"scale": 0.75 => window 192x192
```

Có thể kết hợp với nhiều pet:

```powershell
.\pet-lite.exe -assets ..\assets -pet pet1,pet4
```

Hoặc nhiều instance của pet đầu tiên:

```powershell
.\pet-lite.exe -assets ..\assets -pet pet4 -count 3
```

`-scale` vẫn còn nhưng chỉ dùng để test tạm thời. Runtime chuẩn nên để scale trong `pet.json`.

## Build / test

```powershell
cd E:\git-project\desktop-pet-lite\go-lite
go test ./...
go build -ldflags "-s -w -H=windowsgui" -o pet-lite.exe .
```

## Host app integration

Hiện có 2 kiểu nhúng:

### 1. Sidecar process

App khác chạy `pet-lite.exe` và truyền command hook:

```powershell
.\pet-lite.exe -click-cmd "your command" -right-cmd "your command"
```

Cách này dễ tích hợp nhất cho app C#, Go, Python, Rust, Node, Electron hoặc app nghe nhạc.

### 2. In-process Go package

Có thể tách các file core thành package:

```text
config.go
sprite.go
pet.go
host_api.go
```

rồi app Go khác gọi trực tiếp logic animation.

## Music app example

Với app nghe nhạc, nên dùng:

```text
music_start -> dance
music_stop  -> idle
volume_up   -> cheer
volume_mute -> thinking hoặc sad/cry
```

Hiệu ứng nốt nhạc nên là FX layer riêng, không nên gộp vào sprite nhân vật. Pet layer và FX layer tách riêng sẽ dễ mở rộng hơn.
