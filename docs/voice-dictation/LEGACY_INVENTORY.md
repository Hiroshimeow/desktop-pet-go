# Legacy Desktop Pet checkouts — read-only inventory (G6, 2026-10-11)

This inventory documents independently located checkouts. **No external folder has been deleted, moved, reset or cleaned.** They are not registered worktrees of the canonical `E:\git-project\desktop-pet-lite`.

| Sibling directory under `E:\git-project` | Git state observed | Preservation decision |
|---|---|---|
| `desktop-pet-assistant` | Standalone repository, branch `feat/p3-voice-pet-events`, HEAD `a24d249`, ~10 untracked files, no origin remote | **Keep** pending independent code/asset diff and owner attribution |
| `desktop-pet-phase3-acceptance` | Standalone/detached checkout `e5697e8`, 3 modified tracked, 4 untracked | **Keep** dirty acceptance evidence |
| `desktop-pet-phase8-acceptance` | Directory with assets, `.voice`, docs; no `.git` | **Keep**, could contain runtime/data artifacts |
| `desktop-pet-sts-acceptance` | Standalone/detached inconsistent HEAD, many modified tracked, ~20 untracked | **Keep**; do not prune on repository status alone |
| `desktop-pet-sts-phase1-acceptance-v2` | Separate clean clone at `61d96af`, branch `feat/phase1-vietnamese-sts` | Keep until all tests/assets validated and operator approves archive |
| `_desktop-pet-3d-dev-smoke-55c14d` | Unborn Git `master`, ~95 untracked files, no origin | **Keep**, not safe to infer orphan |

Canonical checkout `E:\git-project\desktop-pet-lite` now develops only on `develop`. Its July 2026 dirty state is preserved in local tag `archive/pre-develop-20261011`, commit `215ff2f`; this archived commit is not automatically part of current product behavior.

The only registered extra Git worktree was `E:\git-project\desktop-pet-lite\.wt\live-transcript-phase1-spec`, created by this workflow, clean, pointing to `04f10ae` and containing only a disposable `voice-sidecar/.venv` dependency installation. It may be removed after `develop` contains the docs commit.

## Deferred cleanup contract

1. Inspect each sibling's `git-common-dir`, remote/history, dirty diff, untracked inventory, assets and runtime ownership.
2. Compare to current `develop`; port missing legitimate behavior with tests only when still relevant; preserve evidence and user data before any deletion.
3. Do not delete/relocate independent repos automatically: changing paths may break tools and running workers. External repository cleanup needs explicit per-directory proof and a named backup destination.
4. Future new worktrees, if explicitly authorized, live **only** under the canonical repo's `.wt/`; standard work is performed directly on `develop`.
