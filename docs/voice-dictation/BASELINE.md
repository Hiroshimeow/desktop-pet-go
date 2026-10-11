# Baseline evidence (G6, 2026-10-11)

**Machine:** md G6 `desktop-b5e4iue-87cea2af`, Windows. Repo `E:\git-project\desktop-pet-lite`, remote `https://github.com/Hiroshimeow/desktop-pet-go`.

- `git fetch origin --prune --tags`: exit 0. `origin/master` at `87756bc4d46f6e561f89fa4e83a3612dd9bddfe2`, dated 2026-06-04. Voice-bearing `origin/feat/phase1-vietnamese-sts` at `61d96af7ca493a43fea236a8298435622a405eb8`, dated 2026-08-03. `git merge-base` returns `87756bc`, so master is an ancestor of the voice feature.
- Root checkout `master` at `3d45d4106633922a1cf54145d46bb49fee117e2f`, still DIRTY with modified `go-lite/main.go` and untracked `.plan/`, `go-lite/logging.go`, `go-lite/pet_transition_test.go`, `go-lite/validate.go`. Nothing reset, stashed, deleted or overwritten.
- `git worktree add -b feat/live-transcript-phase1-spec .wt/live-transcript-phase1-spec origin/feat/phase1-vietnamese-sts`: exit 0. Root `.git/info/exclude` locally excludes `/.wt/` (no tracked product change). New worktree starts clean.
- `cd go-lite; go test ./...`: exit **0**, all Go packages PASS including `internal/agent`, `internal/voice`, `internal/runtimev2`, Windows app package.
- Initial no-sync test: exit **1**, `ModuleNotFoundError: numpy` in empty venv (environment issue, not product failure). Resolved by `uv sync --frozen` exit **0**, 28 pinned packages installed; then `uv run --no-sync python -m unittest -q test_voice_sidecar.py` exit **0**, **18 tests PASS** in 0.573s.
- **Not tested yet:** real mic, live Windows GUI, F5, insertion, native audio/latency, Handy CLI, remote GPU, Gemini.

Code inspected: `go-lite/voice_windows.go` Go `VoiceController`, Python sidecar via uv and JSON lines, `listen`/command mode and `listen_once`; `voice-sidecar/voice_sidecar.py` uses `sounddevice`, `webrtcvad`, `faster_whisper.WhisperModel`; `go-lite/voice_controls_windows.go` handles current reader controls, not global dictation.

Historical `docs/VOICE_ROADMAP.md` includes phase-specific small-file/test budgets; this is a new initiative requiring owner sign-off for OSW parity rather than silent reuse of old phase limits.

## Later consolidation update (same date)

- The root `master` dirty files described above were **preserved**, tested (`cd go-lite; go test ./...` exit 0), and committed as `215ff2f480b799779f5b0906f75c993d4fd25006`. Annotated local recovery tag: `archive/pre-develop-20261011`.
- Canonical repository checkout `E:\git-project\desktop-pet-lite` was switched to **`develop`**, inheriting latest voice branch `61d96af` and this feature specification `04f10ae`. This supercedes the older “root master still DIRTY” description for current-state reporting; no WIP was discarded.
- The temporary clean `.wt/live-transcript-phase1-spec` worktree was removed only after its commit was proven reachable from `develop`; old `master` and temporary `feat/live-transcript-phase1-spec` **local branch refs** were then removed. The pre-develop legacy commit remains reachable via its archive tag.
- No scattered independent checkout under `E:\git-project` was deleted or moved: see `LEGACY_INVENTORY.md` for their dirty/ownership statuses. Only one active branch in the canonical local repo: `develop`.
- `AGENTS.md` specifies the single-branch operating contract and the owner's exact quality policy. Pi Coding implementation is owner-authorized **after** upstream mapping/Phase 1 evidence gates; unknown acceptance is not PASS.
- **Post-migration regression (G6 canonical root, 2026-10-11):** `cd go-lite; go test -count=1 ./...` exit 1. `TestAutoDiscoverAndMergedPetManifest` expects default `pet5` but actual default is `pet1` because ignored/untracked `assets/pets/pet1`, `pet2`, `pet3`, `pet4` coexist with tracked `pet5`. The clean worktree passed. Do not delete user assets to pass tests: isolate the test fixture and assert sorted default discovery deterministically. Other Go packages passed, so source regression vs environment dependence is bounded. `cd voice-sidecar; uv sync --frozen` exit 0 installed 28 packages in canonical root.
