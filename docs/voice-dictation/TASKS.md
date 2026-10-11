# Task ledger — Desktop Pet live dictation

Scope: New dictation/live transcript initiative, separate from historical Phase 1 Vietnamese STS. Status as of 2026-10-11.

| ID | Deliverable | State | Proof/gate |
|---|---|---|---|
| P1-01 | Git branch/worktree identification; preserve dirty master | DONE | remote feature 61d96af; .wt isolated |
| P1-02 | Baseline Go suite | PASS | `cd go-lite; go test ./...` exit 0 |
| P1-03 | Python voice-sidecar test | PASS | `uv sync --frozen` exit 0; 18 unittest cases PASS |
| P1-04 | Windows mic and GUI/voice smoke | PENDING | real microphone, response and process logs |
| P1-05 | All sibling worktree inventory | PENDING | ownership/dirty manifest; do not delete |
| P1-06 | OSW/Handy/OpenWhispr/Windows port source parity | PENDING | exact SHA, paths, license, state transitions |
| P1-07 | Actual Handy CLI / offline UX experiment G6 | PENDING | launch, hotkey, partial and paste proof |
| P1-08 | ThinkBook CPU accuracy/latency benchmark | PENDING | VI/EN fixture, p50/p95, WER/CER, RSS |
| P1-09 | local-live-caption streaming comparator | PENDING | stable/unstable/final and stitch evidence |
| P1-10 | Owner architecture gate | PENDING | acceptance A01-A10 coverage |
| P2-01 | Dictation mode, F5/hold/cancel | NOT_AUTHORIZED | go unit + Windows keyboard |
| P2-02 | Resident STT, live partial/stable/final | NOT_AUTHORIZED | voice-sidecar fixture+real speech |
| P2-03 | Safe target insertion/clipboard/overlay | NOT_AUTHORIZED | real app target regression |
| P2-04 | Pet UI, settings/backend decision, final acceptance | NOT_AUTHORIZED | end-to-end + privacy + legal audit |

Never change product code while Phase 1 unaccepted. Completion status requires direct evidence, not repo/README claims.
