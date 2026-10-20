# Reviewer delta — {{UID}}, round {{ROUND}}

Continue your existing independent Reviewer session. The current base assignment is `{{ASSIGNMENT}}`: read it and check this revision's user words, scope changes and acceptance criteria against your earlier context. Apply your review contract to that current assignment.
Candidate `{{CANDIDATE}}`; parent `{{PARENT}}`; merge base with `{{REF}}`: `{{MERGE_BASE}}`.
New Worker report and dispositions: `{{REPORT}}`.
{{PREVIOUS}}
{{DECLINED}}
{{MUST_SIGN}}

Review the current candidate and new evidence in the context of your previous rounds.
Your doubts go to the Worker, which fixes, ignores or rejects each with a reason and decides whether
another round is useful; an ignored or rejected doubt reaches the user. Return the complete `Doubts:`, `Rule conflicts:`
and `Must sign:` sections as your final message; the driver saves them at `{{VERDICT_PATH}}`.
`Doubts:` and `Rule conflicts:` each appear exactly once on their own line, followed by exactly `none`
or sequential numbered items starting at `1. `. Rule conflicts describe both rules (file:line or document
§ section) and their contradiction in Chinese. Never turn a contradiction between rules into a doubt,
decide it, or ask the Worker to change anything for it. The orchestrator fixes the rules on projects-refactor;
rule conflicts do not block the candidate and appear only in their own read-only UI section.
