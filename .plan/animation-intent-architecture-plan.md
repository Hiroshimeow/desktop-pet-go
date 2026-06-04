# Animation Intent Architecture Plan

## Purpose

This is the main blueprint for restructuring Desktop Pet into a professional, data-driven animation system. The runtime must stop hard-coding concrete animation names such as `happy`, `cry`, `angry`, `walk`, `run`, `wave`, or `dance`. Runtime code should reason about intent, tags, capabilities, state, priority, fallback, and playback policy. Individual pets should declare how their own clips satisfy those semantic intents.

This plan may justify deleting or rewriting large parts of the current runtime. Preserve existing code only when it fits the target architecture cleanly.


## Mandatory rewrite-first policy

The default assumption is that the current runtime architecture is disposable. Do not try to preserve old runtime structure merely to reduce the size of a diff.

Do not perform patch-stacking. A change is patch-stacking if it keeps mixed responsibilities, hard-coded animation names, unclear ownership, global state coupling, or lifecycle hacks in place and only adds another conditional, guard, timeout, fallback, or special case around the symptom.

Bug fixes must address root causes. It is forbidden to add a narrow fix whose only purpose is to pass a single test, quiet a crash, or satisfy one reproduction path while leaving the underlying design flaw intact.

Prefer replacement over preservation when existing code shows any of these traits:

- runtime branches based on concrete animation names;
- pet-specific behavior encoded in Go logic;
- input, brain, animation, rendering, and platform lifecycle mixed in one module;
- ambiguous ownership of mutable pet state or window resources;
- compatibility shims that would become permanent architecture;
- fixes that depend on timing luck, repeated safety checks, or scattered state guards.

Existing code may be copied only as raw reference for isolated low-level details such as Win32 constants, PNG decoding, frame sampling, asset paths, or logging patterns. It must not dictate the new architecture.

When uncertain, build a clean v2 path beside the old one, prove the new path with tests and smoke checks, then delete the old path. The goal is maintainable architecture, not minimum diff size.


## Core target flow

Input / Event -> Intent -> Pet Brain / State Machine -> Animation Query -> Animation Resolver -> Animation Player -> Renderer.

Runtime should understand concepts such as `positive_reaction`, `negative_reaction`, `idle`, `locomotion_slow`, `locomotion_fast`, `dragged`, `held`, `curious`, `sleepy`, and `celebrate`. Runtime should not require specific clip names.

## Architecture principles

1. Runtime must not know concrete animation names. It should call intent APIs such as `PlayIntent("drag_hold")`, not `PlayRandom("cry", "angry")`.
2. Pet packages are data, not logic branches. Do not add pet-specific runtime conditionals.
3. Brain and animation are separate. Brain decides what the pet wants to do; the animation system decides how to show it.
4. Fallback is a core feature. Missing optional clips must never crash the app.
5. Performance comes from compiled data. Parse manifests, validate, convert tags to bitmasks, load sprites, detect frames, and compile intent candidates at load time.

## Target module layout

Long-term Go layout:

```text
go-lite/
  cmd/pet-lite/main.go
  internal/app/{app.go,config.go,lifecycle.go}
  internal/platform/win32/{window.go,message_loop.go,timer.go,layered_window.go,input.go}
  internal/input/{event.go,gesture.go,mapper.go}
  internal/pet/{pet.go,brain.go,state.go,movement.go,context.go}
  internal/animation/{tag.go,manifest.go,intent.go,query.go,resolver.go,player.go,cooldown.go,transition.go}
  internal/assets/{loader.go,sprite_store.go,image_decode.go,validate.go}
  internal/render/{renderer.go,sprite.go,frame_sampler.go}
  internal/core/{clock.go,rng.go,math.go}
  internal/testutil/{fake_clock.go,fake_rng.go}
```

The existing monolithic `go-lite/main.go` should be broken apart. If untangling it is more expensive than a clean rewrite, create `cmd/pet-lite-v2` and migrate toward it.

## Data model direction

Introduce manifest schema v5. It should contain frame settings, scale, animation directory, known tags, animations, and per-animation metadata. Each animation has file, fps, loop, duration, tags, priority, optional movement data, optional cooldown, and playback hints.

Example animation semantics:

- idle: tags `idle`, `state`, `neutral`, `stationary`.
- walk: tags `locomotion`, `ground`, `slow`, with horizontal movement data.
- run: tags `locomotion`, `ground`, `fast`, with horizontal movement data.
- cry: tags `emotion`, `negative`, `sad`, `held`, `protest`, `stationary`.
- angry: tags `emotion`, `negative`, `angry`, `held`, `protest`, `stationary`.
- wave: tags `reaction`, `positive`, `friendly`, `stationary`.

Do not mix interaction logic directly into clip definitions. Keep intent profiles separate from clip inventory. Prefer `assets/default_intents.json` plus `assets/pets/<pet_id>/pet.json`. Pet manifests override common intent behavior only when needed.

## Intent profile direction

Every intent has fallback groups. Example intent fallback concepts:

- idle: `idle` -> `state + stationary` -> any `stationary`.
- left_click: `reaction + positive + friendly` -> `emotion + positive` -> any `reaction` -> `idle`.
- right_click: `reaction + curious/confused/negative` -> any `emotion` -> `idle`.
- drag_start: `emotion + stationary + negative/held/protest` -> `reaction + negative` -> `idle`.
- drag_hold: `emotion + stationary + negative/held/protest` -> `emotion + negative` -> `idle`.
- drag_end: `reaction + positive + friendly/recover` -> any `reaction` -> `idle`.
- locomotion_slow: `locomotion + slow` -> any `locomotion` -> `idle`.
- locomotion_fast: `locomotion + fast` -> `locomotion + slow` -> any `locomotion` -> `idle`.

## Runtime types

Use an `AnimationClip` model containing name, file, fps, frames, loop, duration, tags, priority, movement, and playback metadata.

Use `TagMask` bitmasks instead of string arrays at runtime. Required tag check is a bitwise contains-all operation. Preferred tag score is bit count over matching preferred tags.

Use `IntentQuery` with required, preferred, excluded, and weight fields. Compile each intent into fallback groups of animation candidates. Runtime should not scan and sort all clips on every tick.

Use `AnimationResolver` to resolve an intent plus context into a clip. It owns cooldown and recent-history logic.

## Selection algorithm

At load time:

1. For each intent, iterate fallback queries.
2. Find clips matching required tags and not matching excluded tags.
3. Score candidates by priority and preferred tag matches.
4. Sort and save fallback groups.
5. If an intent has no valid group, bind it to idle fallback.

At runtime:

1. Get compiled intent.
2. Choose the first fallback group with usable candidates.
3. Avoid cooldown candidates if alternatives exist.
4. Apply recent-history penalty.
5. Weighted-random by score.
6. Return selected clip.

Recommended score: base priority + preferred tag match bonus + context bonus - recent penalty - cooldown penalty.

## Pet Brain

Brain emits intents, not clip names. Valid examples: `drag_start`, `drag_hold`, `drag_end`, `left_click`, `right_click`, `idle`, `locomotion_slow`, `locomotion_fast`.

Use a minimal state machine: Idle -> Roaming -> ForcedReaction -> Dragged -> Returning. Avoid states named after clips such as HappyState, CryState, AngryState, or WaveState. Use generic reaction state with an intent field.

Resolver context may include intent, movement speed, facing, mood, dragged flag, distance to target, and current time. Actual clip may be walk, run, hop, float, crawl, or roll as long as tags match.

## AnimationPlayer

AnimationPlayer owns current clip, frame index, frame accumulator, loop, duration, forced-until, interrupt policy, and transitions. Pet should not manually advance animation frames.

Supported interrupt policies: none, soft, hard, after_loop, after_end. Suggested intent priorities: drag_start 100, drag_end 95, drag_hold 90, click reactions 70, idle 10, ambient 5.

## Renderer

Renderer receives only clip, frame rect, position, scale, facing, and alpha. Renderer must not know intent, emotion, drag, click, or behavior semantics.

SpriteStore loads PNGs, detects frame count, validates dimensions, and serves frame rectangles. Validation: height equals frame height; width is divisible by frame width; frames equal width divided by frame width.

## Input and gesture system

Win32 should produce raw events: PointerDown, PointerUp, PointerMove or poll, DoubleClick, RightClick, Cancel. GestureMapper maps raw input to pet events: click, drag_start, drag_hold, drag_end, right_click, double_click. GestureMapper must not call animation directly.

If SetCapture or ReleaseCapture caused re-entrancy or force-close behavior, avoid them. Poll button state in one centralized GestureMapper rather than scattering button-state checks throughout render code.

## Migration strategy

Preferred incremental commits. These are for reviewability only; they must not be used as an excuse to preserve flawed old architecture:

1. Add internal animation skeleton and tests.
2. Add schema v5 parser and validation.
3. Add resolver and tests.
4. Add player and tests.
5. Add brain intent API.
6. Route runtime through resolver.
7. Remove old hard-coded interactions.
8. Update docs and assets.

Greenfield option: create `cmd/pet-lite-v2` and `internal/v2/...`, run v1 and v2 side by side temporarily, then delete v1 after v2 passes smoke tests.

A temporary compatibility bridge may convert old `interactions` exact animation names into exact intent candidates, but this bridge must not define the final architecture.

## Test strategy

Unit tests required for TagMask, Resolver, AnimationPlayer, PetBrain, Asset validation, and GestureMapper.

Resolver tests must cover exact match, fallback group selection, no candidate to idle, cooldown avoiding repeat, and deterministic weighted random with fake RNG.

Player tests must cover loop frame advance, one-shot duration, soft/hard interrupt, and after_end behavior.

Brain tests must cover click intent, drag_start / drag_hold / drag_end intent, and distance-based locomotion intent.

Integration tests should not require Win32. Use fake input, fake clock, fake RNG, and fake sprite store.

Windows smoke test command: `pet-lite.exe -assets ..\\assets -pet pet5 -count 3`. Smoke checklist: click spam does not crash, drag hold does not crash, double click does not crash, multiple pets do not race, fallback works when optional clips are missing, and pets without locomotion clips fall back instead of crashing.

## Performance plan

Do not do JSON parsing, full animation scan, string tag comparison, image loading, buffer allocation, or candidate sorting every frame.

Do parse manifests, validate, convert tags to bitmasks, load PNGs, detect frames, compile intent candidates, and precompute fallback groups at load time.

Runtime hot path should process input, update brain, resolve only on state or intent change, advance frame, and draw current frame.

Multiple instances of the same pet should share immutable PetDefinition, SpriteStore, and compiled clip data. PetInstance stores only mutable position, state, and player data.

## Milestones

1. Animation package skeleton: TagMask, IntentQuery, Resolver, unit tests.
2. Manifest schema v5 parser: raw JSON structs, validation, compiled pet definition, sample asset.
3. Resolver: fallback groups, scoring, cooldown, recent penalty, fake RNG tests.
4. AnimationPlayer: frame advancement, loop, one-shot, interrupt policies, tests.
5. PetBrain: idle, roam, dragged, reaction states and tests.
6. GestureMapper: raw input to gesture events, thresholds, hold repeat timing, tests.
7. Runtime integration: Brain + Resolver + Player, no hard-coded animation names in runtime.
8. Docs and migration cleanup: schema v5 docs, asset authoring guide, default_intents.json, migration guide, legacy docs marked or removed.

## Non-goals

Do not implement AAA-style motion matching for this project. It is too heavy for 2D desktop pet sprites. Do not build a visual graph editor before the data model stabilizes. Do not keep adding hard-coded clip names to runtime code. Do not optimize prematurely before correctness, architecture boundaries, and tests are in place.

## Definition of done

The restructure is successful when runtime has no pet-specific animation branches, runtime does not hard-code clip names for interactions, new pets can be added by authoring metadata only, missing optional animations use fallback cleanly, resolver/player/brain have unit tests, multiple pet instances share immutable assets, and Windows smoke tests pass under click/drag spam.
