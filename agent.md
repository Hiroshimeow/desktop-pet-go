# Agent Entry Point

This repository uses `agent.md` as the first file an AI coding agent should read before planning or editing.

When the user says: "đọc agent.md đi và làm theo", do this:

1. Read this file completely.
2. Read every required plan listed below.
3. Follow the active plan priorities unless the user gives a newer explicit instruction.
4. Prefer architectural cleanup over patch stacking.
5. Do not preserve existing code just because it exists; preserve only code that still fits the selected plan.
6. Default to rewrite-first for runtime architecture. Existing runtime code has no preservation priority unless it cleanly fits the active plan.
7. Bug fixes must address root causes. Do not add narrow patches merely to pass a symptom, a test, or a single reproduction case.
8. Before making code changes, check git status and avoid overwriting unrelated user changes.
9. After making code changes, report what changed, what was verified, and what could not be verified.

## Required plans

Read these files before doing substantive work:

- `.plan/animation-intent-architecture-plan.md`
- `.plan/voice-reader-system-plan.md`

## Current primary direction

The primary restructuring direction is the animation intent architecture plan:

- remove hard-coded animation names from runtime logic;
- introduce semantic tags, intents, fallback chains, resolver, player, and brain layers;
- compile animation selection data at load time for performance;
- keep renderer, input, brain, resolver, and player separated;
- allow deleting or rewriting large parts of the existing runtime if it produces a cleaner, more maintainable architecture.

## Rewrite-first and root-cause policy

- Do not perform patch-stacking refactors.
- Do not add bug fixes that only mask symptoms or only make one test/reproduction pass.
- For runtime architecture work, assume the old runtime is disposable until proven otherwise.
- Replace mixed-responsibility code instead of preserving it behind compatibility shims.
- Existing code may be reused only as reference or as isolated low-level implementation detail when it does not constrain the new architecture.
- When a bug exposes an architectural flaw, solve the architectural flaw first.

## Working discipline

- Keep plans in `.plan/`.
- Keep `agent.md` short and focused as an index and instruction entrypoint.
- If a new long plan is created, put it in `.plan/` and add its path here.
- Do not turn `agent.md` into a large design document.
- Prefer small reviewable commits, except when a clean greenfield v2 path is explicitly chosen.

## Verification expectations

For Go runtime work, prefer these checks when the toolchain is available:

```bash
cd go-lite
gofmt -w *.go
go test ./...
GOOS=windows GOARCH=amd64 go build -o /tmp/pet-lite.exe .
```

If the current machine does not have Go or cannot run Windows smoke tests, say so explicitly and provide the exact verification gap.
