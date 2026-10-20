# kathy

Kathy is a lightweight intermediate representation for LaTeX-like documents. A page is written
as `.chw.tex`, a LaTeX subset whose `ky…` commands are Kathy primitives and whose
`\cwNewcommand` macros expand to them; the converter turns it into `.kathy.json`, the document
the browser renders (`webui/packages/kathy-ast` parses it, `webui/packages/kathy-renderer`
draws it).

This module is the Go converter. **TODO: it is a skeleton; the conversion is not written yet.**
The sample proposals under `projects/` carry both the `.chw.tex` sources and the `.kathy.json`
documents, so the browser side can be exercised before the converter exists.

| Package | What it will be |
| --- | --- |
| `cmd/kathy` | The executable: `kathy <page.chw.tex>` writes the Kathy document to stdout. |
| `document` | The Kathy document: the arena of rose and math nodes that `.kathy.json` serializes. |

The document's shape is fixed by the browser's schema in `webui/packages/kathy-ast/src/schema.generated.ts`;
the Go types follow it, never the other way round.
