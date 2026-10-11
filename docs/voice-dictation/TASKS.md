# Task ledger — Desktop Pet live dictation

Scope: New dictation/live transcript initiative, separate from historical Phase 1 Vietnamese STS. Status as of 2026-10-11.

| ID | Deliverable | State | Proof/gate |
|---|---|---|---|
| P1-01 | Git branch/worktree identification; preserve dirty master | DONE | remote feature 61d96af; .wt isolated |
| P1-02 | Baseline Go suite | FAIL_ENV_DEPENDENCY | Clean isolated worktree PASS; canonical root `go test -count=1 ./...` FAIL in `TestAutoDiscoverAndMergedPetManifest`: test assumes pet5 first despite ignored pet1..pet4 directories. Fix test isolation, do not delete assets |
| P1-03 | Python voice-sidecar test | PASS | `uv sync --frozen` exit 0; 18 unittest cases PASS |
| P1-04 | Windows mic and GUI/voice smoke | PENDING | real microphone, response and process logs |
| P1-05 | Sibling worktree/clone inventory | DONE_INVENTORY | See LEGACY_INVENTORY.md; cleanup deferred because dirty copies must be preserved |
| P1-06 | OSW/Handy/OpenWhispr/Windows port source parity | PENDING | exact SHA, paths, license, state transitions |
| P1-07 | Actual Handy CLI / offline UX experiment G6 | PENDING | launch, hotkey, partial and paste proof |
| P1-08 | ThinkBook CPU accuracy/latency benchmark | PENDING | VI/EN fixture, p50/p95, WER/CER, RSS |
| P1-09 | local-live-caption streaming comparator | PENDING | stable/unstable/final and stitch evidence |
| P1-10 | Owner architecture gate | PENDING | acceptance A01-A10 coverage |
| P2-01 | Dictation mode, F5/hold/cancel | WAITING_P1_GATE | go unit + Windows keyboard |
| P2-02 | Resident STT, live partial/stable/final | WAITING_P1_GATE | voice-sidecar fixture+real speech |
| P2-03 | Safe target insertion/clipboard/overlay | WAITING_P1_GATE | real app target regression |
| P2-04 | Pet UI, settings/backend decision, final acceptance | WAITING_P1_GATE | end-to-end + privacy + legal audit |

Owner approved launching Pi Coding on 2026-10-11; product edits may begin after Phase 1 evidence/architecture gates. Material capability-reducing shortcuts, cloud audio or destructive cleanup require explicit approval. Completion status requires direct evidence, not repo/README claims.
