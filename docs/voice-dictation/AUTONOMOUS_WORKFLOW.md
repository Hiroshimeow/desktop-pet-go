# Autonomous delivery contract — DP-VOICE-001

Owner direction (2026-10-11): **finish the complete GOAL.md, not one checkpoint per user prompt**. This is an execution directive, not a new product feature or permission to change acceptance gates.

## Start and orchestration

1. **Single Pi TUI owner**, same named session in `GOAL.md`, one `develop` checkout. Read root `AGENTS.md`, `GOAL.md`, `SPEC.md`, `PLAN.md`, `TASKS.md`, pinned `SOURCE_AUDIT.md`/`G04_RELEASE_AUDIT.md`/`BENCHMARK.md`, and installed **`C:\Users\admin\.agents\skills\multi-model-orchestration\SKILL.md`**.
2. Use the already installed **`@tintinweb/pi-subagents` `SubagentWorkflow`** tool. Create and save a **real runnable workflow** at `<repo>/.agents/workflows/desktop-pet-delivery.js` (with static `export const meta`), then **actually invoke SubagentWorkflow for it** in this same Pi session. A prose plan, a TODO list, or ending a Pi reply is not an acceptable substitute. Do not start a second Pi process.
3. Coordinator is **pure orchestrator**: dispatch, progress, review, recovery and final verification; subagents do implementation/tests/research. Use a **single serial writer** on `develop`; independent review agents are strictly read-only. No alternate branches or worktrees. For actual implementation use `cliproxy-2080/gpt-6.1-sol` high, for independent deep review use `cliproxy-2080/claude-opus-4-6-thinking` high with `cliproxy-2080/claude-sonnet-4-6` fallback; `cliproxy-2080/gpt-6-luna` for QA when useful. These model IDs exist in the G6 Pi model catalog. All assistant/model outputs require code/runtime verification.
4. Source-specified extension behavior: workflow scripts are coordinator-only JavaScript and must use `agent()`, `phase()`, `pipeline()`, `parallel()`, with serial write stages. Use an `agentType: "Plan"` read-only reviewer for source/spec/review. Avoid `agentType: "Explore"` for full cross-file review (its prompt is for search excerpts). `SubagentWorkflow` is session-scoped; make the saved file restartable/idempotent by checking `GOAL.md` and Git before each write stage, rather than rerunning accepted work on resume. Do not use a raw `--print` process as the long-running owner.

## Required continuous pipeline (Gxx are acceptance gates, not invented subtasks)

| Phase | Delivery outcome | Independent internal review |
|---|---|---|
| **A: G04** | Close real Windows Handy/OpenWhispr/OSW-Windows code/runtime/licensing evidence; known API limitations truthfully marked | Opus architecture & provenance challenge; Sonnet cross-check as needed |
| **B: G05 + G06** | Select and document one proven sustainable runtime/IPC/insertion path; real CPU/microphone and representative Vietnamese/English benchmark | Independent design/safety reviewer refutes assumptions |
| **C: G07 + G08** | Implement *genuine* partial/stable/final STT, model lifetime, bounded cancellation/errors | Opus/sonnet review + deterministic suite + actual local runtime |
| **D: G09 + G10** | Implement F5/hold/cancel and original-target exactly-once paste/clipboard/focus security | Independent Windows/clipboard audit + cross-app checks |
| **E: G11 + G12** | Integrate pet chat/mic/dictation UI, privacy settings and performance with no regression | Cross-module UX/performance/privacy adversarial review |
| **F: G13 + G14** | Full Windows E2E + Go/Python/regression; evidence-backed release acceptance | Independent final reviewer verifies every remaining gate |

**Inner loop for each substantive phase:** writer implements/fixes the stage and runs tests -> read-only independent reviewer returns `APPROVE` or precise `REVISE` findings -> SAME one writer fixes confirmed issues -> review + regression again -> accept evidence and **automatically proceed to the next gate**. Group review work sensibly (about 2 implementation gates per review) rather than reviewing every tiny edit. Source evidence, test commands, actual revisions and runtime identity must appear in `GOAL.md` / local evidence documents. Commit proven checkpoints and push `develop` after accepted stages.

## Autonomy / anti-deadlock

- **Never end the main Pi turn with “G04 done; awaiting user review/continue”** while a further gate remains actionable. Dispatch the next workflow phase immediately. Treat G05 architecture and G14 final technical review as **internal independent reviewer sign-off**, not a human approval gate. User has already authorized this implementation workflow and technical decisions that preserve contract quality.
- If an implementation or review subagent fails, retry/root-cause-analyze or delegate a focused recovery subagent; do not accept a weak approximation or loop with no new evidence. A non-actionable blocker must be recorded with reproduction and next attempt. Continue other independent gates safely; never falsify PASS.
- `multi-model-orchestration` YAGNI/KISS means **no unnecessary new subsystem after full functional parity is preserved**, not “cheap solution”. Priority order and quality policy in `AGENTS.md` override any simplistic LOC/dependency/minimum-cost heuristic.
- Do not run an uncontrolled self-restarting goal loop. The workflow is a **finite, checkpointed delivery pipeline**, with failed-stage diagnosis and durable restartability. A scheduled **read-only** `Agent(schedule: "30m")` watchdog is acceptable if the extension supports it; it must never become a second writer or create another Pi process.
- Only interrupt for **real authorization boundaries**: non-opt-in cloud audio/API/secrets, unapproved destructive cleanup, materially diminished acceptance/capability, or unavoidable operator-specific data/action. Ordinary engineering choices, internal code review, test failures and phase-to-phase transitions do not require human sign-off.
- In privacy-sensitive scenarios, prefer existing local test fixtures and safe disposable foreground targets. Do not secretly record human audio or claim G06 human-corpus PASS from synthetic samples.

## What proves this workflow actually runs

- `.agents/workflows/desktop-pet-delivery.js` is committed on `develop`, syntactically valid and uses the real `SubagentWorkflow` API documented in installed `pi-subagents/docs/workflows.md`.
- Pi session shows a live **SubagentWorkflow task ID**, stage and subagent activity in the Windows Terminal `/agents -> Workflows`; mere textual mention of the tool is insufficient.
- G04 -> G05 -> ... -> G14 transitions produce verifiable GOAL scoreboard updates and commits automatically. Failed gates do not turn PASS without evidence. If Pi TUI exits/restarts, the same session can invoke the saved workflow again against current GOAL/Git evidence; no duplicate write ownership.
