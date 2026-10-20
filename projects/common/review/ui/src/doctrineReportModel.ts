/** The commissar's report for one candidate, structured by the loop driver at
`.local/proposal-doctrines/<uid>/<oid>.json` beside the text it publishes at `<oid>.md`. The driver parses the
report; this side only renders it. */

/** One numbered conviction. The commissar assignment fixes the line's shape as
`<title> — <file:line> — doctrine: <name> § <the rule's words> — <the defect at that site>`. */
export interface PublishedConviction {
  number: string
  title: string
  location: string
  doctrine: string
  detail: string
  /** The item's full text as the commissar wrote it. */
  text: string
}

/** One doctrine the gadget selected for the candidate, whatever it found. `convictions` are the numbers of its convictions above; `reason` says why an
unfinished doctrine did not finish. */
export interface PublishedDoctrine {
  name: string
  status: 'convicted' | 'clean' | 'unfinished'
  convictions: string[]
  reason: string | null
  note: string
}

/** `halt` is the commissar's one sentence saying this candidate needs a decision rather than repairs. `coverage` holds any further report lines outside the
blocks, which older reports used for what ran, what went unchecked and how many files they started from. */
export interface PublishedDoctrines {
  candidate: string
  halt: string | null
  convictions: PublishedConviction[]
  doctrines: PublishedDoctrine[]
  starting_files: string | null
  coverage: string[]
}
