# GOAL CONTRACT — Desktop Pet / OSW-quality Local Dictation

**Contract:** `DP-VOICE-001` · **Owner:** user + one Pi Coding implementation session · **Version:** 1.0 · **Date:** 2026-10-11

**Single working branch:** `develop` · **Canonical checkout:** `E:\git-project\desktop-pet-lite` (md G6)

**Pi session ID:** `a0f8fe6c-0cf9-44be-965e-4df8cec8b6e8` · **Model:** `gpt-6.1-sol` / high

**Authority:** this file is the ONE progress board and scope contract. `AGENTS.md` governs quality/branch/safety; `docs/voice-dictation/SPEC.md` defines acceptance; `PLAN.md` is implementation sequence; `TASKS.md` holds fine-grained work. If they conflict on *progress or scope*, update/reconcile them here, not by silently redefining this contract.

## Live status — read this first

| Field | Verified value |
|---|---|
| **Overall** | **IN PROGRESS** — not product-complete |
| **Accepted gates** | **3 / 15** (count of evidenced gates, **not** percentage of elapsed work) |
| **Current gate (WIP=1)** | **G03** — full source-level OpenSuperWhisper dictation parity map |
| **Next gate** | G04 — evaluate Handy/OpenWhispr/Windows port and reusable streaming mechanisms |
| **Last accepted change** | `fe9d81f` — Go discovery test isolated from user-installed pet assets |
| **Confirmed baseline** | Go `go test -count=1 ./...` PASS; Python sidecar 18/18 PASS on G6 |
| **Last checked** | 2026-10-11 ~17:18 JST, G6 Git + test evidence |
| **Active blocker** | None confirmed. Real microphone, CPU latency and Windows target insertion remain UNVERIFIED, not PASS. |
| **Immediate next artifact** | `docs/voice-dictation/REFERENCE_MATRIX.md`: OSW source SHA + exact file/function/behavior + known missing features and licensing |

**Operator view:** `GOAL.md` for status → `docs/voice-dictation/TASKS.md` for detailed queue → `docs/voice-dictation/BASELINE.md` / `REFERENCE_MATRIX.md` for evidence. Update the table immediately whenever the current gate or blocker changes. Push verified milestone updates to `origin/develop` so the remote view stays useful.

## 1. Locked goal (do not drift)

Deliver **one real, responsive Windows Desktop Pet product** with three separate paths:

1. **Pet click → text chat** using existing agent route (without breaking pet click/drag).
2. **Pet mic click → voice chat**: mic → local STT → existing agent → local TTS.
3. **Global F5 toggle or configurable hold hotkey → OS-wide dictation**: mic → **genuine live provisional/stable transcript** → final accepted transcript pasted **exactly once** at the intended original caret. No automatic Enter, no agent/TTS/shell routing for dictation.

**Product grade:** behave as well as the relevant proven OpenSuperWhisper (OSW) dictation workflow, adapted for Windows, not a look-alike or stripped-down approximation. Perform **complete source-level mapping of every OSW behavior relevant to this goal** (including unsupported or TODO features explicitly). Research OSW, Handy, OpenWhispr, OSW-Windows and `local-live-caption-engine` before implementing a substitute. Reuse mature source/patterns where lawful and technically compatible. Copying code requires honoring license notices even if product README says only “Inspired by OpenSuperWhisper”.

**Default:** ThinkBook CPU, local speech recognition and local synthesis; optional authenticated LAN A5000/2080Ti only after measurement; Gemini/cloud only after user opt-in. Never upload audio silently. Preserve existing Go pet, animation, voice reader, command matcher and ZeroClaw integration.

**Definition of DONE:** all applicable G00–G14 PASS with commit, command/test, runtime and human-observable evidence; no unreviewed loss of quality, UX, safety or OSW-relevant parity. No claims of completed live dictation from mocks, source inspection alone or cloud-only smoke tests.

## 2. Acceptance gates — canonical scoreboard

Allowed states: `TODO` / `DOING` / `PASS` / `BLOCKED`. Only **one primary DOING gate** at a time. A gate is PASS only with a reproducible evidence link/path, test/command and revision. Do not equate activity, commits, files, lines or tokens with accepted progress.

| ID | Gate / operator-observable result | State | Evidence / next proof |
|---|---|---|---|
| **G00** | Single canonical `develop`; archived previous dirty changes; no competing worktree | **PASS** | `ecfd5a2`; backup tag `archive/pre-develop-20261011`; one registered Git worktree |
| **G01** | Go baseline robust on real G6 checkout, including extra ignored user assets | **PASS** | `fe9d81f`; `cd go-lite; go test -count=1 ./...` exit 0 (2026-10-11) |
| **G02** | Python voice sidecar baseline reproducible | **PASS** | `uv sync --frozen`; `uv run --no-sync python -m unittest -q test_voice_sidecar.py` → 18 PASS |
| **G03** | OSW relevant workflow/edge-case **source-level parity map**, pinned SHA/license; gaps honestly marked | **DOING** | Provide exact functions/files in `REFERENCE_MATRIX.md`; no README-only assertions |
| **G04** | Handy/OpenWhispr/OSW-Windows + local caption research; Windows Handy API/CLI limits demonstrated | TODO | Source/commit mapping + real native smoke and limitations |
| **G05** | One justified STT/GUI/insertion architecture based on parity, API limits and measured evidence | TODO | Decision record with reusable components, missing interfaces and failure handling |
| **G06** | G6 real microphone input + local Vietnamese/English/code-switch CPU accuracy/latency benchmark | TODO | Auditable WAV reference set (private), WER/CER, warm/cold p50/p95, CPU/RSS; no guessed numbers |
| **G07** | Live preview with correct unstable/stable/final events; no loss/duplication across overlapping speech | TODO | Deterministic segmentation/stitch tests + real partial event trace |
| **G08** | Resident model and bounded recording/inference; pause/cancel/error/restart have correct ownership | TODO | Lifecycle tests + repeated Windows speech / process checks |
| **G09** | F5 toggle + hold keydown/up + Esc cancel; no repeats, hotkey/IME/AltGr conflicts | TODO | Actual global input checks with target applications |
| **G10** | Original-target final-only **exactly-once** insertion without focus/clipboard corruption | TODO | Clipboard race and focus-change tests, safe copy-only fallback on unsafe targets |
| **G11** | Pet's 3 modes + non-focus-stealing live overlay, existing click/drag/chat/voice intact | TODO | Native GUI/animation/agent isolation smoke |
| **G12** | Correct user settings/privacy/resource behavior, local CPU default, no UI stalls or orphan processes | TODO | Repeated session profiling, opt-in network proof, failure diagnostics |
| **G13** | Cross-app Windows e2e: Notepad, VS Code, Chromium/ChatGPT, terminal; VI/EN speech | TODO | Actual run log, expected/actual output, screenshots or bounded UI evidence |
| **G14** | Independent acceptance: all existing Go/Python suites + app scenarios PASS, no regression | TODO | Release-grade evidence, resolved exceptions, sign-off |

**Progress count:** `PASS / 15` must be recomputed from this table whenever any gate changes. A gate may move from PASS back to DOING/BLOCKED if a regression invalidates its evidence. No gate may be quietly waived.

## 3. Anti-overengineering AND anti-cheap-solution contract

**Preserve quality without inventing a platform.** This feature is an integration into `desktop-pet-go`, not a new universal speech service, microservice architecture, plugin SDK, generic provider registry, alternative pet runtime or parallel rewrite. Features unrelated to the three user flows do **not** become mandatory just because OSW supports them.

For **every proposed implementation slice**, the agent must answer (briefly in commit/decision record):

1. **Requirement:** which Gxx / SPEC acceptance does this change directly satisfy?
2. **Reference:** how does mature upstream solve it? Pin file/function/commit when relevant.
3. **Boundary:** what is the narrowest integration that meets **full** required behavior and failure semantics?
4. **Proof:** which deterministic and real Windows checks will prove it?
5. **Non-goals:** what is intentionally left unchanged and why?

**Quality priority, not code-size priority:** correctness → mature reference → relevant parity → reliability/UX/latency/maintainability → reuse → simplicity → time/token/cost. KISS = simplest **correct, complete** design, not fastest plausible approximation. No arbitrary LOC, dependency or test quotas. No polling/respawning shortcuts or mocked results sold as production acceptance. When failure occurs, trace root cause and adopt durable fix, not a one-off workaround.

**Scope gate:** Before adding a new subsystem/tool/server, ask: “Which failing Gxx acceptance cannot be met by the existing pet/sidecar or proven reusable upstream?” If no evidence-backed answer, record `DEFERRED/OUT_OF_SCOPE` in the reference matrix; do **not** build it. Speculative features, cloud default, new model training, wake-word rewrite, premature GPU orchestration, generic agent infrastructure, unrelated repo cleanup and UI redesign outside the three flows are OUT OF SCOPE.

When two solutions satisfy **the same complete contract**, choose the proven simpler one. If the only easy solution drops stability/parity/security/performance, do NOT choose it for cost reasons; present the exact tradeoff for owner approval instead.

## 4. Agent execution and checkpoint contract

- **ONE owner, ONE checkout, ONE branch, ONE primary active gate.** Local work on `develop` only; create worktrees only by explicit user request, and then solely within `.wt/`.
- **Read this file at session start and every milestone.** Next gate selected from the scoreboard, not by ad-hoc ideas. `TASKS.md` may track detailed subtasks but must never redefine overall scope or falsely mark a gate PASS.
- **Update this file after each material evidence checkpoint** and at least whenever current gate/blocker/next action changes. Keep header's last-checked value, progress count and last proof current; add a short entry to the log below. State `BLOCKED` with root cause, real failure evidence and the next unblocking experiment; don't spin indefinitely.
- **Atomic checkpoints:** finish focused test → run regression relevant to risk → update GOAL status + proof → local commit; push `develop` on passed gate/checkpoint so operator sees the latest state remotely. If push fails, record that remote visibility is stale; no force push.
- **No fabricated PASS:** source inspection = SOURCE-VERIFIED, CLI smoke = RUNTIME-VERIFIED, UX acceptance = WINDOWS-E2E-VERIFIED. Mark missing checks TODO/BLOCKED, not PASS.
- **No plan churn:** do not repeatedly rewrite SPEC/PLAN/GOAL without newly verified information. Research only the source paths needed for the active gate, then ship a testable vertical slice. A task split must enable independent acceptance, not inflate task count.
- **Stop/ask user only for:** disclosure-sensitive cloud audio, destructive operations on legacy clones/assets, material reduction in acceptance/parity, ambiguous security boundary, or truly unavailable runtime/credential. Normal technical problems: research, solve root cause and continue.
- **Resource guard:** do not run an unbounded autonomous goal loop or silently spawn duplicate worker sessions. If an iteration produces no new proof, explicitly diagnose and record blockers before repeating. This is reliability control, **not** a reason to downgrade engineering quality.

## 5. Checkpoint log (append short evidence, no narrative spam)

| Time (JST) | Gate | Evidence / decision | Commit or artifact |
|---|---|---|---|
| 2026-10-11 17:03 | G00 | Single `develop`, historical WIP archived; scope/quality policy committed | `ecfd5a2`; tag `archive/pre-develop-20261011` |
| 2026-10-11 17:12 | G01 | Environment-sensitive pet discovery test fixed with isolated assets; full Go test PASS | `fe9d81f` |
| 2026-10-11 ~17:18 | G02 | Python sidecar 18/18 PASS on Windows G6; canonical Go suite independently rechecked PASS | `docs/voice-dictation/BASELINE.md` + local runs |

**Current next step:** complete **G03** OSW relevant dictation behavior map with pinned source and indicate which features require verified Windows adaptations; don't build a second independent voice core.

---

### Contract change rule

**Locked sections:** goal, scope, G00–G14 acceptance intent and quality policy. Agent may clarify wording without weakening acceptance, but cannot silently remove gates, redefine DONE or expand scope. User approves material changes. **Mutable sections:** live status, gate states/evidence, current/next focus, blockers and checkpoint log. One primary implementation owner updates them alongside actual evidence.
