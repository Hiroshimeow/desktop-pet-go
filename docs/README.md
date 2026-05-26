# Desktop Pet Lite Docs

Bộ tài liệu này dùng để đọc lại kiến trúc, vận hành, case study và roadmap của project.

## Tài liệu chính

- `HANDBOOK.md`: hướng dẫn vận hành/phát triển hằng ngày, cách chạy, build, import pet, debug.
- `STUDYBOOK.md`: giải thích kỹ thuật theo hướng học lại project, module nào làm gì, vì sao chọn Go/Win32, asset pipeline, plugin model.
- `CASE_STUDIES.md`: các lỗi/decision record đã gặp như crop rỗ mặt, asset mix, config trùng, crash click, RAM target.
- `ARCHITECTURE.md`: kiến trúc module, data flow, config model, sprite contract, state machine concept.
- `VOICE_REQUIREMENTS.md`: requirement baseline cho voice/reader/TTS/STT CPU-only, dùng làm source of truth trước khi implement.
- `ROADMAP.md`: kế hoạch phát triển tiếp theo theo phase.
- `ADD_NEW_PET.md`: hướng dẫn tự thêm pet mới từ sprite sheet 8x5.

## Thứ tự đọc đề xuất

1. `HANDBOOK.md`
2. `ARCHITECTURE.md`
3. `VOICE_REQUIREMENTS.md` nếu đang review/implement voice hoặc reader
4. `STUDYBOOK.md`
5. `CASE_STUDIES.md`
6. `ROADMAP.md`
7. `ADD_NEW_PET.md`

## Lệnh nhanh

Chạy pet:

```powershell
cd E:\git-project\desktop-pet-lite\go-lite
.\pet-lite.exe -assets ..\assets -pet pet5
```

Review/check-only build/test:

```powershell
cd E:\git-project\desktop-pet-lite\go-lite
Get-Command gofmt -ErrorAction Stop | Out-Null
Get-Command go -ErrorAction Stop | Out-Null
if ((gofmt -l *.go).Length -ne 0) { throw "gofmt check failed" }
go test ./...
go build -o pet-lite-debug.exe .
go build -ldflags='-s -w -H=windowsgui' -o pet-lite.exe .
```

Developer formatting/fix:

```powershell
gofmt -w *.go
```

Import pet sheet:

```powershell
cd E:\git-project\desktop-pet-lite
py .\scripts\import_pet_sheet.py --src .\assets\pet2.png --pet pet2 --overwrite --remove-bg
```
