# Orchestrator pitfalls

Mistakes of the shared orchestration (driver, runner, gates, role wiring) that cost a run or a round, one file each in
`annals/YYYY-MM-DD-<short-description>.md`: what happened, what it cost, the cause, the fix, and how to notice it next time.
Read the index before changing the driver or the runner; add an entry the moment a run is lost to orchestration, before fixing.

- [Worker sandbox could not write the worktree's git dir; the candidate commit failed silently into a review of the old commit (2026-09-20)](annals/2026-09-20-worker-sandbox-cannot-write-worktree-git-dir.md)
- [Unstaged edits lost while a Git-mutation subagent worked in the same checkout (2026-09-20)](annals/2026-09-20-unstaged-edits-lost-under-a-git-subagent.md)
