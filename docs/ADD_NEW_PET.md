# Add New Pet Guide

Tài liệu này hướng dẫn thêm một pet mới từ ảnh sprite sheet 8x5 vào `desktop-pet-lite`.

## 1. Chuẩn ảnh đầu vào

Ảnh pet nên là sprite sheet có quy tắc:

```text
5 cột  = 5 frame cho mỗi animation
8 hàng = 8 animation/act
```

Mặc định tool đang map 8 hàng như sau:

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

Mỗi hàng phải là một loop độc lập. Frame cuối nên nối mượt về frame đầu. Với `walk/run`, thứ tự chân nên là chu kỳ hợp lý, ví dụ:

```text
contact -> down -> passing -> up -> contact
```

Không nên để 2-3 frame liên tiếp cùng một chân nếu đó là animation di chuyển.

## 2. Lưu ảnh nguồn

Ví dụ thêm pet4:

```text
E:\git-project\desktop-pet-lite\assets\pet4.png
```

## 3. Import pet

Chạy:

```powershell
cd E:\git-project\desktop-pet-lite
py .\scripts\import_pet_sheet.py --src .\assets\pet4.png --pet pet4 --overwrite --remove-bg
```

Output sẽ được tạo ở:

```text
assets\pets\pet4\animations\idle.png
assets\pets\pet4\animations\walk.png
assets\pets\pet4\animations\run.png
assets\pets\pet4\animations\happy.png
assets\pets\pet4\animations\cry.png
assets\pets\pet4\animations\angry.png
assets\pets\pet4\animations\wave.png
assets\pets\pet4\animations\sleepy.png
assets\pets\pet4\pet.json
```

## 4. Test catalog

```powershell
cd E:\git-project\desktop-pet-lite\go-lite
.\pet-lite.exe -assets ..\assets -pet pet4 -catalog
```

Nếu catalog in ra các act đúng, pet đã được runtime nhận.

## 5. Chạy pet4

```powershell
.\pet-lite.exe -assets ..\assets -pet pet4
```

Chạy pet mới cùng pet5:

```powershell
.\pet-lite.exe -assets ..\assets -pet pet5,pet_new
```

Chạy tất cả pet:

```powershell
.\pet-lite.exe -assets ..\assets -pet all
```

## 6. Nếu ảnh bị cắt sai

Sheet generated thường có margin không đều hoặc nhân vật tràn nhẹ sang cell kế bên. Khi đó thử chỉnh margin:

```powershell
py .\scripts\import_pet_sheet.py --src .\assets\pet4.png --pet pet4 --overwrite --remove-bg --margin-x 4 --margin-y 18
```

Giải thích:

- `--margin-x`: bỏ lề trái/phải của toàn sheet.
- `--margin-y`: bỏ lề trên/dưới của toàn sheet.
- `--remove-bg`: xóa nền bằng border flood-fill, an toàn hơn threshold vì không khoét lỗ mặt/da.

## 7. Nếu pet đi sai hướng

Runtime hiện dùng `walk.png`/`run.png` và tự flip theo `native_facing`.

Không cần tạo:

```text
walk_left.png
walk_right.png
run_left.png
run_right.png
```

Các file này đang nằm trong `act_blacklist` để tránh engine chọn nhầm hướng.

Nếu sprite gốc nhìn sang phải, dùng default:

```json
"native_facing": "right"
```

Nếu sprite gốc nhìn sang trái, sửa trong `assets/pets/pet4/pet.json` hoặc `assets/pet.json`:

```json
"walk": { "native_facing": "left" },
"run":  { "native_facing": "left" }
```

## 8. Nếu thêm animation mới

Ví dụ thêm:

```text
assets\pets\pet4\animations\dance.png
assets\pets\pet4\animations\fly.png
```

Khi mở app, runtime sẽ nhận diện `dance`, `fly` trong manifest in-memory với params default nếu các act này không nằm trong `act_blacklist`. Để persist metadata, chỉnh `assets/pets/pet4/pet.json` thủ công hoặc chạy sync manifest explicit khi được expose.

Sau đó chỉnh lại metadata nếu cần:

```json
"dance": {
  "file": "dance.png",
  "fps": 8,
  "kind": "reaction",
  "locomotion": false,
  "duration_ms": 3000,
  "description": "Nhảy khi có nhạc."
}
```

## 9. Checklist tự thêm pet

```text
[ ] Lưu source sheet vào assets/<pet_id>.png
[ ] Chạy import_pet_sheet.py
[ ] Kiểm tra assets/pets/<pet_id>/animations/*.png
[ ] Chạy -catalog
[ ] Chạy pet riêng bằng -pet <pet_id>
[ ] Kiểm tra walk/run có đúng hướng không
[ ] Kiểm tra click/drag/right-click không crash
[ ] Nếu ổn, commit source sheet + animations + pet.json
```
