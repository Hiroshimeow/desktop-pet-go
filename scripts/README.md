# Asset tools

Các tool trong `scripts/` chỉ dùng lúc chuẩn bị asset. Runtime pet vẫn là Go executable trong `go-lite`.

## Cài dependency chung

```powershell
py -m pip install pillow
```

---

## `sprite_row_cutter_gui.py`

GUI nhẹ bằng Tkinter để cắt từng hàng animation thủ công. Tool này phù hợp khi sprite sheet có margin không đều, lưới 8x5 không khớp tuyệt đối, hoặc bạn muốn kéo vùng sáng tới đúng hàng như ảnh crop trên điện thoại.

### Chạy tool

```powershell
cd E:\git-project\desktop-pet-lite
py .\scripts\sprite_row_cutter_gui.py
```

### Cách dùng

1. Bấm `Open Image` và chọn ảnh source, ví dụ `assets\pet4.png`.
2. Bấm `Output Dir` và chọn thư mục output, ví dụ:

```text
E:\git-project\desktop-pet-lite\assets\pets\pet4\animations
```

3. Chọn số frame mỗi hàng ở `Frames`, mặc định là `5`, có thể tăng tới `10`.
4. Kéo vùng sáng 1x5 hoặc 1x10 tới đúng hàng animation.
5. Nhập `Act name`, ví dụ `idle`, `walk`, `run`, `happy`, `cry`, `angry`, `wave`, `sleepy`.
6. Bấm `Export Row`.
7. Di chuyển vùng sáng sang hàng tiếp theo, đổi act name, export tiếp.

### Phím/tương tác

```text
Drag chuột trái          di chuyển vùng crop
Kéo cạnh/góc             resize vùng crop
Mouse wheel              đổi chiều cao vùng crop
Ctrl + mouse wheel       đổi chiều rộng vùng crop
Arrow                    di chuyển 1px
Ctrl + Arrow             di chuyển 10px
Shift + Arrow            resize
PageUp/PageDown          nhảy lên/xuống 1 hàng theo chiều cao hiện tại
Auto Row                 nhảy tới row index đang nhập
```

### Output

Mỗi lần export tạo:

```text
<act>.png
```

Ví dụ với 5 frame và frame size 256:

```text
happy.png = 1280x256
```

Nếu bật `Save frames`, tool cũng tạo folder:

```text
happy_frames/
  happy_00.png
  happy_01.png
  happy_02.png
  happy_03.png
  happy_04.png
```

Tool cũng ghi lịch sử crop vào:

```text
_cut_history.jsonl
```

### Khi nào dùng tool GUI này?

Dùng `sprite_row_cutter_gui.py` khi:

- sheet có lề trên/dưới không đều;
- nhân vật hơi lệch cell;
- cắt tự động bị mất chân/dư đầu;
- muốn kiểm soát từng hàng bằng mắt;
- muốn export act thủ công như `smile.png`, `fight.png`, `dance.png`.

---

## `import_pet_sheet.py`

Tách tự động một sheet 8 hàng x 5 cột thành nhiều file animation riêng:

```text
assets/pets/<pet_id>/animations/<act>.png
```

Mỗi `<act>.png` là strip ngang 5 frame, frame chuẩn 256x256.

### Import sheet mặc định 8 act

```powershell
cd E:\git-project\desktop-pet-lite
py .\scripts\import_pet_sheet.py --src .\assets\pet2.png --pet pet2 --overwrite --remove-bg
```

Thứ tự act mặc định của 8 hàng:

```text
idle, walk, run, happy, cry, angry, wave, sleepy
```

### Import sheet thứ hai cho cùng pet

Dùng `--acts` để map lại 8 hàng:

```powershell
py .\scripts\import_pet_sheet.py `
  --src .\assets\pet1.1.png `
  --pet pet1 `
  --acts surprised,shy,thinking,cheer,scared,dizzy,dance,sit_idle `
  --overwrite `
  --remove-bg
```

### Vì sao tool không detect foreground bằng threshold nữa?

Tool cũ detect content bằng màu nền trắng/checkerboard rồi crop theo vùng nhân vật. Cách đó làm mất lỗ ở mặt, da và highlight vì các pixel sáng bị hiểu nhầm là background.

Tool mới mặc định crop theo geometry ổn định của sheet 5x8 rồi normalize về frame 256x256. Nếu bật `--remove-bg`, nó chỉ flood-fill background nối với viền ngoài, không xoá pixel trắng/sáng nằm bên trong nhân vật.

### Tham số hữu ích

```powershell
--frame 256          # kích thước frame xuất ra
--cols 5             # số frame mỗi hàng
--rows 8             # số act/hàng
--margin-x 0         # bỏ lề trái/phải cố định nếu sheet có lề ngoài
--margin-y 0         # bỏ lề trên/dưới cố định nếu sheet có lề ngoài
--remove-bg          # xoá nền bằng border flood-fill, tránh khoét lỗ trong mặt
--bg-threshold 42    # độ rộng màu nền khi flood-fill
--overwrite          # ghi đè animation đã có
```

## Import lại 3 pet hiện tại

```powershell
cd E:\git-project\desktop-pet-lite
py .\scripts\import_pet_sheet.py --src .\assets\pet1.png --pet pet1 --overwrite --remove-bg
py .\scripts\import_pet_sheet.py --src .\assets\pet1.1.png --pet pet1 --acts surprised,shy,thinking,cheer,scared,dizzy,dance,sit_idle --overwrite --remove-bg
py .\scripts\import_pet_sheet.py --src .\assets\pet2.png --pet pet2 --overwrite --remove-bg
py .\scripts\import_pet_sheet.py --src .\assets\pet3.png --pet pet3 --overwrite --remove-bg
```

Sau khi import, chạy:

```powershell
cd E:\git-project\desktop-pet-lite\go-lite
.\pet-lite.exe -assets ..\assets -pet pet1 -catalog
```
