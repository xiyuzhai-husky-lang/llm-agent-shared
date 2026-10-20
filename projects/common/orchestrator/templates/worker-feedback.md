# Worker delta — {{UID}}, round {{ROUND}}, candidate {{CANDIDATE}}

Continue your existing session and original assignment. This is only the new round's input.
{{FEEDBACK_SOURCE}}: `{{VERDICT_PATH}}`.

Follow the shared [Worker–Reviewer interaction](../../review/README.md#workerreviewer-interaction-the-one-specification-for-every-project).

{{REQUIRED_CHANGES}}

For each numbered item, investigate and fix it, ignore it or reject it with a concrete reason: ignore it when you judge it
unimportant and leave the candidate unchanged, reject it when you judge it not valid and leave the candidate unchanged. The numbered list maps back
to the source's own numbering; the full text remains at the path above. Write every disposition reason in Chinese.
Identifiers and file paths stay as they are written. The driver labels each item for the
user, an ignored or rejected conviction as a conviction; do not put labels such as `[conviction]` into your reason.
If you change the candidate, verify and commit the implementation under the original assignment's contract.
Do not handle the diff or proposal-folder explanation surfaces in this turn; the driver assigns those surfaces
only in the final wrap-up.
A fresh Git-mutation agent records any candidate as prescribed by the assignment.
Do not fabricate a new candidate when no files change. Never write user review entries.

End your report with exactly these sections (use `Dispositions: none` on separate lines when there are none):

```text
Dispositions:
1. fixed
2. ignored: <reason and evidence, in Chinese>
3. rejected: <reason and evidence, in Chinese>
Loop decision: review | present
Loop reason: <why another review is useful, or why no further action is needed>
```

Choose exactly one decision. `review` asks the driver to review the current candidate again; it
can be unchanged when new evidence warrants review. `present` means "no further action, present to
the user", whatever the Reviewer or the commissar raised. Your first `present` starts the Nitpicker for this
candidate; a Reviewer with no doubts starts it without a Worker round.
The Nitpicker's doubts come back to you in a round of their own, answered the same way.
Once the run's mission rank's Worker revisions are used, items come to you in a separate response-only turn, where you answer in words and change nothing. Items you ignored or rejected and your reasons remain visible.
After the reviewing exchanges, wrap-up supplies the explanation surfaces;
the final review-surfaces gate returns failures to wrap-up before PRESENT.

At the end of this work step, include exactly one `Step result:` section with the strict JSON defined in the shared Worker role. Incomplete / chow_problems go directly to wrap-up, skipping Outcome and reviewing exchanges; complete follows the usual loop. Write no proposal artifacts in this turn.
