# Animation runtime v2 migration note

The v2 path separates these responsibilities:

```text
Raw input -> GestureMapper -> Pet Brain -> Animation Resolver -> Animation Player -> Renderer
```

Completed packages:

- `internal/input`: raw pointer events to gesture events.
- `internal/pet`: brain state machine emits semantic intents.
- `internal/animation`: schema v5 compiler, resolver, cooldown/history, player.
- `internal/runtimev2`: headless integration engine for tests and smoke validation.

The legacy Windows runtime in `go-lite/*.go` is not deleted yet. It still hard-codes legacy animation names and remains the compatibility path. New runtime work should target `internal/runtimev2` first, then route the Win32 shell through that engine once asset loading and renderer adapters are complete.

## Smoke command

After installing Go:

```bash
cd go-lite
go run ./cmd/pet-lite-v2-smoke
```

Expected output includes resolved semantic flow for click, drag, and locomotion. This validates the v2 architecture without Win32.

## Current integration status

The legacy schema v4 manifests are now bridged into schema v5 definitions in `go-lite/v2_bridge.go`. The Windows runtime creates a `runtimev2.Engine` per `Pet` when the bridge compiles successfully. `Pet.Update`, click actions, drag start/hold/end, and the current renderer-facing `Pet.Animation` / `Pet.Frame` fields are synchronized from the v2 engine. This lets the existing renderer draw v2-selected clips before the Win32 shell is fully split into internal packages.

## Remaining integration work

1. Load authored real pet v5 manifests directly from disk instead of relying on the v4 bridge.
2. Add sprite store/frame validation under `internal/assets` or `internal/render`.
3. Replace the remaining Win32 pending/drag bookkeeping with direct `internal/input.GestureMapper` ownership.
4. Move renderer code behind an `internal/render` adapter that consumes `animation.AnimationPlayer.Snapshot()` and clip frame metadata directly.
5. After Windows smoke passes, delete legacy hard-coded interaction methods and old clip-name fallback helpers from `go-lite/pet.go`.
