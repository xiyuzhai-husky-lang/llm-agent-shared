import './review-tabs.css'

/** The mark of an incomplete proposal: a ❓ that never stops jiggling, next to the proposal in the list, the
candidate's revision chip and the tab that holds the Worker's question. */
export function IncompleteMark() {
  return <span className="incomplete-mark" role="img" aria-label="incomplete">❓</span>
}

/** The text a screen reader or a test reads for a label that carries the mark. */
export function incompleteLabel(label: string): string {
  return `${label} incomplete`
}
