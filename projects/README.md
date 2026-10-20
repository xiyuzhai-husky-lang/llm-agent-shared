# Projects

A project lives at `projects/<group>/<project>/`: a `README.md` naming its branch and
its goal, a `proposal-workflow.toml` read by `go-src/llm-agent/workflows/proposal-workflow`,
and an `outcome_fragments.txt` listing the Outcome fragments its candidates report. The
shared skeleton every project builds on is `projects/common/`.

The projects here are dummies that show the layout; nothing in them is real work. Each
carries one fake proposal under `proposals/<year>/<month>/<day>/<name>_<number>/`, and
`ai-infra/llm-inference/CLAUDE.md` is an example of the file a project's orchestrator reads
at the start of every session.
