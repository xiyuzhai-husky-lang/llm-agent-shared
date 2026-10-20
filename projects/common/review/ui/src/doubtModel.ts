/** The Reviewer's `Doubts:` section (formerly impeachments): a numbered list; "none" or an empty file means no items. */
export interface Doubt { index: number; text: string }
/** `followup` says the item stands and is handled right after this proposal, in one of its own; it counts as decided and does not block
Accept. `defer` sends the item to the long-term todos; it also counts as decided. */
export type DoubtRuling = 'sustained' | 'dismissed' | 'followup' | 'defer'
/** What the user can write about one item: a ruling, or `revoked`, which takes the item's ruling back so it is
undecided again. */
export type DoubtDecision = DoubtRuling | 'revoked'

/** One doubt as the loop driver publishes it structured, beside the Markdown list
(`.local/proposal-doubts/<uid>/<oid>.json`): the doubt as the reviewer wrote it, the decision asked of the user when there
is one, and the Worker's answer labelled by its disposition. The list is in the Markdown list's order. */
export interface PublishedDoubt {
  source: 'Reviewer' | 'Commissar'
  number: string
  text: string
  decision: string | null
  worker: { disposition: 'fixed' | 'agreed' | 'ignored' | 'rejected' | 'halted' | 'capped'; reason: string }
}

export function parseDoubts(markdown: string): Doubt[] {
  const items: Doubt[] = []
  for (const raw of markdown.split('\n')) {
    const line = raw.trimEnd()
    const start = /^\s*(\d+)[.)]\s+(.*)$/.exec(line)
    if (start) { items.push({ index: Number(start[1]), text: start[2] }); continue }
    if (items.length && line.trim() && !/^Doubts\b/i.test(line)) items[items.length - 1].text += ' ' + line.trim()
  }
  return items
}

/** The review-record comment that records one ruling or its revocation; the tab reads it back with `doubtRulings`. */
export function rulingComment(item: Doubt, decision: DoubtDecision): string {
  return `Doubt ${item.index} ${decision}: ${item.text}`
}

/** Rulings already written into the review record for this candidate, read in order with the last one winning; a
`revoked` entry leaves the item undecided. `entries` are the parsed review entries of the project's review record
(candidate + body); the project supplies the parser. */
export function doubtRulings(entries: { candidate?: string; body: string }[], candidate: string): Map<number, DoubtRuling> {
  const rulings = new Map<number, DoubtRuling>()
  for (const entry of entries) {
    if (entry.candidate !== candidate) continue
    const match = /^Doubt (\d+) (sustained|dismissed|followup|defer|revoked):/.exec(entry.body.trim())
    if (match == null) continue
    if (match[2] === 'revoked') rulings.delete(Number(match[1]))
    else rulings.set(Number(match[1]), match[2] as DoubtRuling)
  }
  return rulings
}
