# Task ledger — Desktop Pet live dictation

Scope: New dictation/live transcript initiative, separate from historical Phase 1 Vietnamese STS. Status as of 2026-10-11. **Canonical progress/scope contract is root `GOAL.md` (G00–G14).** This table tracks operational subtasks only; a TASK PASS never overrides a GOAL gate without evidence.

| ID | Deliverable | State | Proof/gate |
|---|---|---|---|
| P1-01 | Git branch/worktree identification; preserve dirty master | DONE | Canonical `develop`, archive `archive/pre-develop-20261011` (`215ff2f`); one owner, no new worktree; G00 evidence in GOAL.md |
| P1-02 | Baseline Go suite | PASS | Canonical develop: reproduced failure, isolated discovery tests with synthetic temp assets; targeted tests repeated 10 times and `go test -count=1 ./...` exit 0. See BASELINE.md remediation evidence; user assets unchanged |
| P1-03 | Python voice-sidecar test | PASS | Canonical root reverified: `uv sync --frozen` exit 0 (28 packages checked); `uv run --no-sync python -m unittest -q test_voice_sidecar.py` exit 0, 18 tests in 0.546s |
| P1-04 | Windows mic and GUI/voice smoke | PENDING | real microphone, response and process logs |
| P1-05 | Sibling worktree/clone inventory | DONE_INVENTORY | See LEGACY_INVENTORY.md; cleanup deferred because dirty copies must be preserved |
| P1-06 | OSW/Handy/OpenWhispr/Windows port source parity | IN_PROGRESS | G03 OSW01–21/test trace SOURCE PASS after nine review findings owner-rechecked/reconciled in REFERENCE_MATRIX.md; G04 sole active gate: G04_RELEASE_AUDIT.md H01–H06/O01–O09 release/API/native limit checkpoint; final GUI/candidate/legal review incomplete. SHA/license/semantics distinguished from runtime |
| P1-07 | Actual Handy CLI / offline UX experiment G6 | PARTIAL | BENCHMARK.md: released CLI start→real mic callback→cancel twice; settings/history/files/clipboard/foreground unchanged. Worker flag rejected exit2; public native0.2.4 strictCPU stream/abort/reset works. Stop/final/paste/hold/live GUI untested; G04 not accepted |
| P1-08 | ThinkBook CPU accuracy/latency benchmark | PRELIMINARY_ONLY | BENCHMARK.md: one synthetic sample, bounded batch/native stream trials/RSS; native stream VI output inaccurate, no tentative observed. No human corpus/WER/CER, real mic or release-to-final proof; G06 TODO |
| P1-09 | local-live-caption streaming comparator | PARTIAL | 27 existing tests PASS; actual probes expose stable/full invariant and false-overlap corruption; SOURCE_AUDIT.md/BENCHMARK.md. No live VI acceptance |
| P1-10 | Independent multi-model architecture gate | PENDING | Opus/Sonnet read-only adversarial review, acceptance A01-A10 coverage; no routine owner wait |
| P2-01 | Dictation mode, F5/hold/cancel | WAITING_P1_GATE | go unit + Windows keyboard |
| P2-02 | Resident STT, live partial/stable/final | WAITING_P1_GATE | voice-sidecar fixture+real speech |
| P2-03 | Safe target insertion/clipboard/overlay | WAITING_P1_GATE | real app target regression |
| P2-04 | Pet UI, settings/backend decision, final acceptance | WAITING_P1_GATE | end-to-end + privacy + legal audit |

Owner approved launching Pi Coding on 2026-10-11; product edits may begin after Phase 1 evidence/architecture gates. Material capability-reducing shortcuts, cloud audio or destructive cleanup require explicit approval. Completion status requires direct evidence, not repo/README claims.
