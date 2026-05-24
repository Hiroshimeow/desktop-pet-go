# Desktop Pet Lite Docs

Bộ tài liệu này dùng để đọc lại kiến trúc, vận hành, case study và roadmap của project.

## Tài liệu chính

- `HANDBOOK.md`: hướng dẫn vận hành/phát triển hằng ngày, cách chạy, build, import pet, debug.
- `STUDYBOOK.md`: giải thích kỹ thuật theo hướng học lại project, module nào làm gì, vì sao chọn Go/Win32, asset pipeline, plugin model.
- `CASE_STUDIES.md`: các lỗi/decision record đã gặp như crop rỗ mặt, asset mix, config trùng, crash click, RAM target.
- `ARCHITECTURE.md`: kiến trúc module, data flow, config model, sprite contract, state machine concept.
- `ROADMAP.md`: kế hoạch phát triển tiếp theo theo phase.
- `ADD_NEW_PET.md`: hướng dẫn tự thêm pet mới từ sprite sheet 8x5.

## Thứ tự đọc đề xuất

1. `HANDBOOK.md`
2. `ARCHITECTURE.md`
3. `STUDYBOOK.md`
4. `CASE_STUDIES.md`
5. `ROADMAP.md`
6. `ADD_NEW_PET.md`

## Lệnh nhanh

Chạy pet:

```powershell
cd E:\git-project\desktop-pet-lite\go-lite
.\pet-lite.exe -assets ..\assets -pet pet1
```

Build/test:

```powershell
cd E:\git-project\desktop-pet-lite\go-lite
gofmt -w config.go config_test.go discovery.go host_api.go main.go pet.go sprite.go split_assets.go
go test ./...
go build -ldflags='-s -w' -o pet-lite.exe .
```

Import pet sheet:

```powershell
cd E:\git-project\desktop-pet-lite
py .\scripts\import_pet_sheet.py --src .\assets\pet2.png --pet pet2 --overwrite --remove-bg
```
