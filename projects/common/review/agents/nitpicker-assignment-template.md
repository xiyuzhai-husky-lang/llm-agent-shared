# Nitpicker Assignment — proposal `{{UID}}`, candidate {{CANDIDATE}}

You are the nitpicker (Codex). Your rules are the assembled nitpicker document delivered
with this assignment; it is the only source of them. Build and run what you need to check a behaviour; change no
tracked file. Return your list as your final message; the driver saves it at `{{OUTPUT_PATH}}`.

Candidate `{{CANDIDATE}}` (merge base with `{{REF}}`: `{{MERGE_BASE}}`). Proposal artifacts will be written only during later wrap-up.
The Worker presented it; the Reviewer's doubts are at `{{VERDICT_PATH}}` — do not repeat a doubt already raised there.
Worker assignment: `{{ASSIGNMENT}}`.
{{PREVIOUS_DISPOSITIONS}}

Read the diff (`"{{TOOLS_REPO}}/scripts/proposal-tool.sh" workflow candidate diff "{{SLOT_DIR}}" {{MERGE_BASE}} {{CANDIDATE}}`) and the Worker assignment, report and relevant source, then write the `Doubts:` section per the contract. Time budget ≈ 10 minutes.
Each doubt's prose is Chow, written as the Worker writes its document surfaces (the project's authoring guidance
§ Chow Surfaces); the number, the `决定:` question and the anchor headings stay plain text.

Every doubt must identify all affected hunks of this exact candidate using the shared review anchor
format: an indented `## <repository-relative path>` followed by one or more indented
`### @@ -a,b +c,d @@` headings inside that numbered doubt.
Repeat for multiple files/hunks; use the new path for renames and old path for deletions.
For a file-wide concern use `## <path>` without a hunk; for a proposal-wide concern state that scope
and omit anchors. Never fabricate a range. Every doubt requires the user's ruling before Accept
or Modify, whatever the Worker's disposition. See the shared review norm.
