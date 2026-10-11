# Desktop Pet — agent operating contract

## Repository and ownership

- **Canonical working directory:** `E:\git-project\desktop-pet-lite` on md G6 (`desktop-b5e4iue-87cea2af`).
- **One active development branch: `develop`.** All source, tests, docs, and verified changes are committed directly to `develop`. Never create a feature/review/plan branch or a new checkout without the operator explicitly asking.
- **One coding owner/session at a time.** A read-only audit or subagent may assist, but must not create competing implementation lineages.
- If an isolated Git worktree is explicitly requested in the future, use only `<repo>/.wt/<name>`. The `.wt/` tree is ignored; never create sibling worktrees in `E:\git-project`.
- At start, verify `git rev-parse --show-toplevel`, `git branch --show-current`, `git status --short`, `git remote -v`; if not root/develop, stop and diagnose rather than silently switching or resetting.
- **Commit locally after every independently verifiable slice.** Do not leave orphan work. Do not force-reset/push, discard WIP, clean user files, or remove old clones without inventory and preservation proof.
- The former dirty `master` from July 2026 is preserved at Git tag `archive/pre-develop-20261011` (commit `215ff2f`); reconcile genuinely useful deltas only after inspecting new source and passing regression. Do not blindly cherry-pick historical `go-lite/main.go` over newer voice code.
- Other sibling directories in `E:\git-project` include independent clones/acceptance snapshots, some with uncommitted work. They are not the canonical checkout, and may not be removed simply because `git worktree list` does not report them.
- Upstream remote branch consolidation is controlled by repository owner. Never recreate legacy `master` or `feat/phase1-vietnamese-sts` after migration to `develop`.

## Current objective

**Mandatory first read: `GOAL.md` — the canonical owner-facing GOAL CONTRACT, gate scoreboard and approved scope.** Refresh it at every verified milestone, commit it with the evidence and push `develop` so the owner can see progress. Do not redefine its locked acceptance gates, make up PASS evidence or add speculative subsystems. Keep one active gate at a time. Use `docs/voice-dictation/SPEC.md`, `PLAN.md`, `TASKS.md`, `REFERENCE_MATRIX.md`, `BASELINE.md` and `PI_HANDOFF.md` as the detailed dictation/live transcript references. Desktop Pet is one product: pet click for chat, pet mic for voice conversation, F5 toggle/configurable hold-to-talk for system-wide dictation with stable partial/final and safe final-only insertion. Preserve existing pet, animation, voice commands, ZeroClaw integration and local TTS.

**Reference parity policy:** OpenSuperWhisper upstream is the UX reference. Handy, OpenWhispr and local-live-caption-engine are concrete reuse candidates. Trace actual upstream source, commit, semantics, edge cases, license and tests before writing a substitute. Distinguish DOCUMENTED, SOURCE-VERIFIED and WINDOWS-RUN-VERIFIED. No claims of full feature parity without real evidence.

## SOLUTION QUALITY POLICY

Do not optimize for implementation effort, token usage, coding time, or
dependency count when those optimizations reduce product quality,
correctness, reliability, UX, performance, or parity with a mature
reference implementation.

Optimization priority:

1. Correct behavior and acceptance criteria.
2. Proven/mature existing implementations and upstream techniques.
3. Functional parity with the chosen reference.
4. Reliability, latency, UX, and maintainability.
5. Reuse before custom implementation.
6. Simplicity only after the above are satisfied.
7. Token/time/implementation cost is the lowest-priority concern.

Do not invent a simpler substitute merely because it is easier to build.

Before implementing a custom solution:
- inspect the mature reference implementation;
- understand how it solves the same problem;
- map its behavior and edge cases;
- reuse or port the proven technique where appropriate.

If proposing a shortcut that drops behavior, robustness, UX, or
performance, explicitly identify the lost capability and obtain approval
before proceeding.

KISS means:
"the simplest implementation that fully satisfies the target behavior"

KISS does NOT mean:
"the easiest or fastest approximation of the requested product."

## Autonomous execution (owner-authorized 2026-10-11)

- **Read `docs/voice-dictation/AUTONOMOUS_WORKFLOW.md` and use the installed global `multi-model-orchestration` skill.** Launch a durable, actual `SubagentWorkflow` inside the one existing interactive Pi session and carry all G04-G14 gates through internal multi-model review, correction, proof, commit and automatic next-stage dispatch. No per-gate user approval or end-of-turn wait. Reviews are read-only; one serial coding subagent owns `develop` writes.
- The workflow must **actually run**, not just be planned. Keep the progress gate in `GOAL.md`. Only seek owner authorization for sensitive data/cloud, destructive changes, genuine contract downgrade or blocked external prerequisites. Preserve the quality policy below over any simplistic YAGNI shortcut.

## Verification and reliability

- Choose a durable, root-cause solution after tracing the actual failure and mature reference. Do not ship mocks, unbounded polling, fragile shell/process spawning on every hotkey, silent fallback to cloud, or incomplete focus insertion as if they met acceptance.
- Default STT/TTS path stays on local ThinkBook CPU. A5000/RTX2080Ti inference is opt-in and measured end to end. Gemini/cloud requires explicit opt-in, privacy disclosure and protected credentials.
- Preserve non-blocking Win32 UI, bounded cancellation/process lifetime, final-only text insertion, no transcript→shell execution, UTF-8/VI-EN terminology and clipboard/focus safety.
- Run targeted tests and the relevant full regression, recording commands, environment, exit codes and real Windows input/GUI evidence. No passing a skipped, mocked or unrun acceptance check.
- Keep expensive iterative agents **bounded by explicit task/checkpoint progress**, not infinite self-restarts. This protects session integrity without treating costs as a reason to accept a weaker product.
- Commit evidence/spec updates with related code; no push of credentials, recordings, models or private local user files.
