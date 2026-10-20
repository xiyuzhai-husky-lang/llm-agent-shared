# Plan workflow

The plan workflow produces design artifacts for the user to rule on; the proposal workflow produces a candidate. A plan is the step before a proposal, written to a correspondence form. The proposal opens only after the user accepts the plan. This light run spends its time on design. Role documents are assembled as [roles](../roles/README.md) describes.

## Responsibilities

The orchestrator writes the assignment with its target, sourced user words, correspondence class and instance directory, takes the assignment through Provost, and launches the roles through `plan-workflow`. It handles user answers and rule conflicts, preserves Protest text for the user, and accepts the UI before sending its link. UI construction and binding are separate from this driver.

The planner writes `plan.chw.tex` and runs Chow to produce `plan.kathy.json`, checks both artifacts against the project rules, and answers reviewer findings. Converter refusals belong to the planner while authoring; an inability to express the required content produces `chow_problems`.

The plan-reviewer changes nothing, checks the artifacts against the checklist, inherited role rules and user rulings, and returns violations, rule conflicts and Protest through separate typed fields.

The driver runs the proposal assignment source check before dispatch. It assembles current role documents at every launch, records the prompt actually sent, and keeps one agent session per role for the run, on Codex or Claude Code as the rank's model names (`mission-ranks.toml` beside this file). It manages the project gate, parsing, send-backs, snapshots, findings and answers; it does not build HTML.

## A run

1. The planner writes and checks both artifacts.
2. The configured project gate runs on the instance directory. Exit 0 passes; exit 1 blocks with one reason per stdout line; any other exit is a tool error. Gate repair goes back to the planner, outside the reviewing round caps. Two repair send-backs are allowed; the third failure ends the run `stuck`.
3. The plan-reviewer reports violations against the exact artifact snapshot for that round. The planner answers each item and revises; the gate and reviewer run again. Delivery requires a passing gate and zero violations, including when the planner declined an item with a reason.
4. Mission rank `role_round_cap` caps plan-reviewer rounds; `total_round_cap` caps reviewing rounds in total. The deadline records an event and the run continues. At either cap, last-round violations get a response-only planner turn. Both artifact digests must stay unchanged; a modification ends the run `stuck`. Answers are recorded and the capped run ends `stuck`.
5. `incomplete` carries one question and pauses the run. The orchestrator records the user's answer with `plan-workflow answer` and resumes the same planner session in a new round. `chow_problems` ends the run and delivers its problems list.
6. The user receives the clean plan after UI acceptance. An accepted plan is referenced by its proposal's assignment.

Planner malformed tails receive at most three send-backs; reviewer malformed tails receive one. The counters are separate and do not consume review rounds. Exhaustion ends the run `stuck`. Both roles retain strict JSON tails. Rule conflicts go to the orchestrator without becoming plan violations; Protest is delivered verbatim.

`status` returns typed per-round reports, findings, planner answers, rule conflicts, Protest, user answers and events. Instance records retain each reviewed `plan.chw.tex` and `plan.kathy.json` with their digests bound to that round. The UI can use these records to render the exact reviewed version.

## Project configuration

`projects/<org>/<project>/plan-workflow.toml` names:

- `correspondence_class`: the correspondence form under `llm-agent/correspondence/classes/`.
- `checks`: repository-relative files both roles read and apply.
- `project_gate`: a command relative to the project directory, invoked with `<instance-dir>` from the repository root.

Content rules and their mechanical checks belong to the project's role fragments and gate. The common workflow defines the interaction.
