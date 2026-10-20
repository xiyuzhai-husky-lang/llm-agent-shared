// Data shapes for the expandable source-code viewer.

/** One highlighted token within a source line. `color` is a CSS colour supplied
 *  by the highlighter (semantic or syntactic); absent = default text colour. */
export interface SourceToken {
  content: string
  color?: string
}

/** A highlighted source file, ready to render with a line gutter. */
export interface SourceFile {
  /** the focused 1-based line (gets the emphasis). */
  focus: number
  /** per-line token arrays, in file order. */
  lines: SourceToken[][]
}
