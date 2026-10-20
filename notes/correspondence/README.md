# Correspondence system

A correspondence is a standalone web page built for communicating with the user remotely, over Telegram or a similar channel, when the material does not fit in a chat message: a diff whose critical hunks must be signed one by one, a bug diagnosis with annotated code spans and call stacks, or a set of source pieces laid out in reading order.

Every page belongs to a class named `<genre>/<class>`, where the genre is the kind of page and the class is one concrete form of it (for example `diagnosis/basic`, `show/code-pieces`, or `diff/advanced`). A page places its primary content first—such as the rule a plan proposes or the side-by-side output of competing designs—and leaves secondary context for later sections.

- `llm-agent/correspondence/README.md` is the entry point describing how classes and examples are organized and how primary content is separated from secondary material.
- `llm-agent/correspondence/classes/<genre>/<class>/` defines a class; the classes here are `diagnosis/basic`, `diff/basic`, `diff/advanced` and `show/code-pieces`. In each class directory, `guideline.md` fixes the section order, content rules, and input schema, and `ui/` builds the page.
- `go-src/llm-agent/correspondence/diff-advanced/` is the Go server behind `diff/advanced`. Unlike a plain diff, an advanced diff includes a `must-sign-additions.json` file that marks the candidate line ranges of critical changes and explains what signing each range confirms; the viewer highlights hunks through Tree-sitter grammars and records the user's per-hunk signatures under `signatures/`.
