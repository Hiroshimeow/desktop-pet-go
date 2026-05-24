# Last update

- Added global default pet config at `assets/pet.json`.
- Added per-pet auto-discovery from `assets/pets/*`.
- Added sync logic that merges global defaults into each pet's `pet.json`.
- Added auto-detection for new `animations/<act>.png` files.
- Added lightweight cache `.petcache.json` using global config hash, pet config hash, and animation file size/mtime metadata.
- Added Python import tool at `scripts/import_pet_sheet.py`.
- Added tools documentation at `scripts/README.md`.
- Imported `assets/pet2.png` into `assets/pets/pet2/animations/*.png`.
- Imported `assets/pet3.png` into `assets/pets/pet3/animations/*.png`.
- Rebuilt Go executable at `go-lite/pet-lite.exe`.

Validation:

```text
go test ./... -> ok
pet-lite.exe size -> 3,012,608 bytes
```
