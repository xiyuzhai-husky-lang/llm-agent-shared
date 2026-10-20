## Scope Consistency

The target named in Problem's title and first sentence, what Design/Implementation Plan/Diff actually change, and
Effect's `main` section are one and the same thing, named the same way. A Problem titled "codec slow" whose
measurements are all decode, an implementation that only restructures decode, and an Effect listing encode beside
decode is inconsistent and fails review. Measurements outside the target go to Effect `affected` with a rationale,
or nowhere.

## Effect (optional)

Effect is written only for work that changes a measurement — deferred or regressed tests fixed, a speedup and the
like; a slop overhaul has none. What it reports is basically specified in
advance by the assignment and Problem. The numbers are your own
measurements in your round, of what you judge important; never the Outcome, which you do not run. When it does not apply, write nothing: no Effect files and
no `Effect` entry in `revision.json`. When it applies it has TWO SECTIONS:
`main` — the measurements named in Problem, before → after; `affected` — measurements you judge important that are not
in Problem but moved, before → after, with one sentence saying why, left out when there are none. Never mix the two.
