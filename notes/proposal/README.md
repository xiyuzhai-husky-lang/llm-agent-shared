# Proposal system

The proposal system turns a scoped assignment into a landed commit without requiring the user to read agent transcripts. Each proposal is a commit-sized change developed in an isolated git worktree slot, checked by mechanical gates, measured by the project's Outcome suite, and examined by three review roles before reaching the decision browser for the user's ruling.

All workflow state is persisted in the SQLite journal of the resident Go service `proposal-workflow`. An orchestrator agent forms batches, writes sourced assignments, starts and monitors runs, delivers candidates, and lands accepted proposals, without making technical judgments for any role or recording a verdict on the user's behalf.

## Roles

When `proposal-workflow` launches a role, it assembles the role's prompt from `projects/common/roles/` and the project's own role fragments rather than reading a repository-root `AGENTS.md`. Each role runs on the agent runtime and reasoning effort specified by the run's mission rank (`projects/common/orchestrator/mission-ranks.toml`):

| Role | Responsibility | Session scope |
| --- | --- | --- |
| **Orchestrator** | Selects independent tasks for a batch, exports the user conversation, writes the sourced assignment, runs it through the Provost, obtains the user's one-time batch approval, starts and monitors runs, delivers candidates, and lands accepted changes. | External coordinator |
| **Provost** | Checks an assignment or user-facing document against the conversation log before dispatch, marking every requirement `sourced`, `unsourced`, `distorted`, or `missing`. | One call per check |
| **Worker** | Implements the assignment in its worktree slot, commits the candidate, and reports `complete`, `incomplete`, or `chow_problems`. Answers every review finding as `fixed`, `ignored`, or `rejected` with a reason while it may still revise, and as `agreed` or `declined` once its revisions are spent, and writes the proposal directory's explanation pages during wrap-up. | One session for the run (receives deltas after turn 1) |
| **Commissar** | Computes the doctrines touched by the diff, dispatches a check per doctrine, raises convictions, and can halt the review loop straight to wrap-up. | Fresh session per candidate |
| **Reviewer** | Reviews the candidate independently with no access to the Worker's session, raises Doubts strictly within the confirmed scope line, and rules on every changed MUST SIGN file. | One session for the run |
| **Nitpicker** | Performs a final review pass after the Reviewer passes settle; its Doubts also go to the Worker first. | Fresh session per pass |
| **User** | Approves the batch's scope lines and expected bypasses once before launch, answers `incomplete` questions when existing rulings do not cover them, and rules on delivered candidates in the decision browser. | Decision browser and chat |

## Batches, assignments, and mission ranks

Proposals run in batches of independent tasks that do not touch the same files or shared baselines. Batches form a **stack**, not a queue: the top batch is active, and when urgent work arrives a new batch can be pushed on top, suspending the lower batch in place without resetting its counters or invalidating candidates already awaiting a verdict.

Before a batch starts, the orchestrator writes each proposal's assignment from four tagged block kinds verified mechanically before dispatch:

- `[SCOPE, confirmed by user <date> <time> "<quote>"]` — the one-sentence scope line, whose confirmation quote must appear verbatim in the exported conversation.
- `[USER, <date>.md <time>]` — verbatim quotes from the user.
- `[MECHANICS, <path> § <section>]` — references to existing project rules.
- `[ORCHESTRATOR]` — the orchestrator's framing; any sentence that constrains the Worker must cite a source or the checker rejects the assignment.

Once every assignment in the batch passes the Provost and the user confirms the batch's scope lines and any `bypass.json` in a single prompt, the orchestrator records the batch (`batch open`, `batch add`, `batch approve`), prepares each slot on a `temp_<project>_proposal_...` branch, and starts the runs with `--slot`, `--uid`, `--assignment`, and `--rank`.

The mission rank (`D`, `C`, `B`, `A`, `S`, `SS`, `SSS` in `projects/common/orchestrator/mission-ranks.toml`) fixes the run's deadline, the agent runtime configuration, and `worker_revision_cap`—the number of times per revision the Worker may modify the candidate in response to review findings (from 1 in `D` to 7 in `SSS`, excluding the initial Worker turn and gate repairs). Once that cap is spent, the Worker still answers new findings in `response-only` turns, where any code change marks the run `stuck`, and the unchanged candidate finishes the remaining review roles and delivers. Passing the deadline records a `deadline` event without stopping the run.

When a task requires design sign-off before implementation, `plan-workflow` (`projects/common/plan-workflow/README.md`) runs first: a planner authors `plan.chw.tex` and `plan.kathy.json` for a correspondence form, a plan-reviewer checks them against project rules and user rulings, and the proposal opens only after the user accepts the plan.

## Candidate kinds and phases of a run

A Worker turn returns one of three candidate kinds:

- **`complete`** — implementation is finished; the candidate proceeds through all mechanical gates, Outcome measurement, and the three review roles.
- **`incomplete`** — blocked on a decision only the user can make; skips Outcome and review roles, writes `question.md` in wrap-up, and presents without verdict buttons until `answer` records the user's reply and opens a new revision.
- **`chow_problems`** — blocked by document converter issues recorded in `chow-problems.md`; also skips review and goes straight to wrap-up.

Phase transitions are defined in `go-src/llm-agent/workflows/proposal-workflow/internal/workflow/transition.go`. A `complete` candidate passes through twelve phases:

1. **`preflight`** — verifies that the slot is a clean worktree root on the expected branch with its merge-base equal to the pinned base commit.
2. **`worker`** — the Worker implements the assignment, commits the change, and returns the candidate.
3. **`worker-step`** — runs mechanical gates and guards, and forbids writing proposal explanation pages before `wrapup` (`incomplete` and `chow_problems` branch directly from here to `wrapup`).
4. **`project`** — runs the project's optional mechanical check.
5. **`outcome`** — runs the project's selected Outcome fragments (`projects/common/outcome/README.md`) on the candidate.
6. **`commissar`** — checks diff-triggered doctrines and raises convictions.
7. **`reviewer`** — performs independent scoped review, raising Doubts and ruling on changed MUST SIGN files.
8. **`nitpicker`** — performs the final review pass and raises Doubts.
9. **`wrapup`** — the Worker writes the proposal directory's explanation pages and `revision.json` without modifying the reviewed code or candidate kind.
10. **`wrap-gate`** — verifies that `wrapup` touched only the proposal directory and left settled code hunks intact.
11. **`surfaces`** — runs the review-surfaces gate, opening every explanation page with the reviewer's own reader.
12. **`deliver`** — delivers the immutable candidate to `proposal-decision` and sets the proposal status to `present`.

Whenever `commissar`, `reviewer`, or `nitpicker` raises findings, the workflow sends them back to the Worker. If the Worker revises the candidate, the new candidate restarts from `worker-step` and passes every gate, Outcome, and all three review roles from scratch; if the Worker keeps the candidate unchanged, its `agreed`/`disagreed` answers attach to the Doubts and the run advances to the next role. Gate failures in `worker-step`, `project`, or `outcome` return to the Worker for repair (up to two repair turns before becoming `stuck`); failures in `wrapup`, `wrap-gate`, or `surfaces` retry `wrapup` (up to two times before becoming `aborted`). Stopped, stuck, or aborted runs require the orchestrator to diagnose the cause and explicitly invoke `resume`.

## Decision browser and post-verdict flow

In `proposal-decision`, inspection and ruling actions (`Accept`, `Modify`, `Reject`, `Interact`, `Peek`) belong exclusively to the user:

- **MUST SIGN hunks**: Files matched by the project's must-sign patterns require the user to sign every critical hunk before `Accept` is enabled.
- **Doubts**: The user inspects findings from the Commissar, Reviewer, and Nitpicker alongside the Worker's disposition and reason, and can rule a Doubt as `followup` (re-surfaced in subsequent batches until scheduled) or `defer` (sent to long-term todos).
- **Verdicts and landing**:
  - **`Modify`** (or answering an `incomplete` question) opens a new revision and resets the Worker's revision counter to zero.
  - **`Accept`** triggers `land` followed by `land-outcome`: the candidate is landed onto the project's target branch, Outcome is re-measured on the new tip to check for regressions, and the proposal status becomes `landed`.
  - **`Reject`** (or retiring a run with `end-early`) runs `retire-check` and `retire-clean`, settles the batch ledger, cleans the worktree slot, and renames the `temp_...` branch to a local `reference_...` branch.

## Snapshot of a proposal directory

`projects/common/` holds the shared project skeleton, while `projects/games/autodiablo/` and `projects/ai-infra/llm-inference/` are dummy projects with sample proposals under `proposals/2026/10/20/` that illustrate the on-disk layout.

`projects/games/autodiablo/proposals/2026/10/20/autodiablo_pathfinding_000001/`:

```text
autodiablo_pathfinding_000001/
├── criteria.md
├── design.chw.tex
├── diagnosis.chw.tex
├── evidence/
│   ├── user-rulings.md
│   └── verification.md
├── implementation-plan.chw.tex
├── must-sign-justifications.md
├── problem.chw.tex
├── related.md
├── review.md
├── revision.json
├── scope.chw.tex
└── tests.md
```

`revision.json` records the base commit, candidate kind, and review surfaces written during `wrapup`:

```json
{
  "format": 4,
  "base": "3f9c2a1d7e8b4c6f0a2d9e1b5c7f8a3d4e6b9c01",
  "kind": "complete",
  "surfaces": {
    "Problem": "updated",
    "Scope": "updated",
    "Diagnosis": "updated",
    "Related": "updated",
    "Criteria": "updated",
    "Design": "updated",
    "Implementation Plan": "updated",
    "Tests": "updated",
    "Diff": "updated"
  }
}
```

Each entry in `criteria.md` links directly to the user's verbatim words in `evidence/user-rulings.md`:

```markdown
- [One package, nothing outside the movement loop](evidence/user-rulings.md#L5-L6)
  > Only `pathfinding` is new; the movement loop is the one caller changed.

- [The bot never steps on lava](evidence/user-rulings.md#L9-L10)
  > Lava cells cost infinity; the search never returns a path through one.
```

And `review.md` records the candidate digest and the user's verdict:

```markdown
### 2026-10-20T09:12:40.118Z · Candidate: 8d1e4b7a2c9f0e3d6a5b1c4f7e8d9a0b2c3d4e5f · Permission: accept
```

## Programs, browsers, and project paths

Two independent Go programs share a single HTTP value contract and entry script:

| Path | What it is |
| --- | --- |
| `go-src/llm-agent/workflows/proposal-workflow/` | Runs the roles, mechanical gates, response turns, wrap-up, delivery, and landing, and owns the batch ledger. Its second command, `plan-workflow`, drives the pre-proposal design workflow. |
| `go-src/llm-agent/workflows/proposal-decision/` | Serves the decision browser and records the user's hunk signatures, Doubt rulings, and verdicts in its own SQLite journal; never runs a role. Also provides `proposal-diff`. |
| `go-src/llm-agent/workflows/proposal-platform/` | The version-one HTTP value contract (`api`), SQLite journal (`journal`), and keyed view synchronization (`syncview`) shared by both programs, with no executable and no business transitions. |
| `scripts/proposal-tool.sh` | The single CLI entry point: `proposal-tool.sh workflow\|decision\|plan\|diff …` builds the selected binary through Go's cache and executes it. |

The browser applications and shared packages under `webui/` and `projects/`:

| Path | What it is |
| --- | --- |
| `webui/apps/proposal-workflow/` | The workflow browser showing the batch stack, proposals, and phase progress. |
| `webui/apps/proposal-decision/` | The decision browser where the user inspects candidate surfaces, signs MUST SIGN hunks, rules on Doubts, and records a verdict. |
| `webui/apps/plan-workflow/` | The plan workflow browser for reviewing pre-proposal design artifacts. |
| `webui/packages/proposal-*/` | The tabs, panels, and client packages composing the browsers: `proposal-batch-stack-page`, `proposal-connection`, `proposal-title-bar`, `proposal-overview-sidebar`, `proposal-main-tab`, `proposal-diff-tab`, `proposal-doubts-tab`, `proposal-effect-tab`, `proposal-tests-tab`, `proposal-outcome-tab`, `proposal-review-panel`, `proposal-peek-panel`, `proposal-quote-notebook`, and `proposal-kathy-document`. |
| `webui/packages/{peek,chat-view,review-surface,kathy-*}/` | Shared UI primitives for source peeking, chat transcripts, review surfaces, and Kathy document rendering (`kathy-ast`, `kathy-renderer`, `kathy-source`, `kathy-vfs`). |
| `projects/common/` | The shared project skeleton: role prompt prefixes and fragments (`roles/`), orchestrator templates and mission ranks (`orchestrator/`), pre-proposal plan workflow docs and ranks (`plan-workflow/`), review templates and UI (`review/`), Outcome runner and fragments (`outcome/`), shared diff UI (`diff/`), decision and batch-stack services (`proposal/`, `batch_stack/`), Kathy surface components (`kathy-surface/`), and coding standards (`standards/`). |
| `projects/games/autodiablo/`, `projects/ai-infra/llm-inference/` | Dummy projects illustrating `proposal-workflow.toml`, `outcome_fragments.txt`, and sample proposals under `proposals/2026/10/20/`. |
