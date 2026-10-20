# Outcome fragments

A project's `outcome_fragments.txt` selects fragments from
`projects/common/outcome/fragments/<name>/fragment.toml`. Lines are ordered;
blank lines and `#` comments are ignored. A name consists of nonempty slash-separated
segments containing ASCII letters, digits, underscores or hyphens. Names cannot repeat.
The definition supplies `title`, `report` and the producer `command`; `run.py` owns execution.

The Outcome tab of the shared Go reviewer (`webui/packages/proposal-outcome-tab`)
reads the selection and definitions from its repository checkout when it loads a measured
candidate, together with the run index `run.json` and the reports the workflow's Outcome gate
recorded under `.local/proposal-outcome/<uid>/<candidate>/`. A run can contain a subset of the selected fragments;
its entries retain their recorded order. Every entry must name a selected fragment, and
its report filename must equal the definition's `report`. Missing definitions, blank titles,
duplicate fragments and malformed reports are errors. Navigation labels come only from
`title`; no reviewer-specific list of accepted names or labels exists.

Dedicated readers remain selected by fragment name. All other selected fragments must
produce the statistics report below. A dedicated reader's error is never retried as statistics.
TypeScript-based reviewers have not adopted this Go reader.

## Statistics report, format 1

The schema is defined by `go-src/llm-agent/workflows/proposal-workflow/internal/material/outcome_validation.go`. The full measured report is:

```json
{
  "format": 1,
  "title": "Natural Number Game coverage",
  "status": "passed",
  "commit": "0123456789abcdef0123456789abcdef01234567",
  "counts": [
    { "name": "Solved", "value": 12 },
    { "name": "Total", "value": 15 }
  ],
  "table": {
    "columns": ["World", "Solved", "Total"],
    "rows": [["Addition", 7, 8], ["Multiplication", 5, 7]]
  }
}
```

All displayed fields above are required, with no unknown fields accepted in the report,
count objects or table. The producer supplies everything except `commit`, which the runner
stamps with the measured HEAD. `format` is the integer `1`; `title` is a nonblank string;
`status` is exactly `passed` or `failed`. The reviewer validates `commit` with its shared
Git object-id reader and shows a mismatch against the candidate explicitly.

`counts` is an ordered array of objects with a nonblank, unique `name` and an unsigned
64-bit integer `value`. `table.columns` is an ordered array of nonblank, unique strings.
Each row has exactly one cell per column. A cell is a string or a JSON number; null,
booleans, arrays and objects are rejected. Empty `counts` and empty `rows` are allowed.
For a counts-only report, use `"table": {"columns": [], "rows": []}`; rows without columns
are invalid. Counts do not imply pass/fail: the report's explicit `status` determines it.

Go formats the counts and table cells into strings before sending the view to the
browser, preserving integer precision. The shared TSX component renders that view only.
Invalid JSON, missing fields, unknown fields, wrong types, duplicate names and row-width
mismatches fail with the report path and reason. There is no raw-JSON or legacy-format fallback.
