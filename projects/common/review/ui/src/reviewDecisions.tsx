import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { parseDoubts, type DoubtDecision, type DoubtRuling } from './doubtModel'
import { parseNitpicks } from './nitpickModel'
import './review-tabs.css'

export const REVIEW_CHANGED_EVENT = 'proposal-review-changed'
export interface ReviewAnchor { path: string; range?: string }
export interface ReviewFinding {
  id: string
  kind: 'Doubt' | 'Nitpick'
  index: number
  text: string
  details?: string
  tag?: string
  anchors: ReviewAnchor[]
  comment: string
}
export interface ReviewEntry { candidate?: string; body: string; role?: string }

/** The four rulings and their buttons: y sustains (must act), n dismisses, a plain `followup` button marks the
item as handled in one of the next couple of batches, and a plain `defer` button sends it to the long-term todos. */
const RULING_BUTTONS = [['y', 'sustained', 'Sustain / must act'], ['n', 'dismissed', 'Dismiss'], ['followup', 'followup', 'Handled in one of the next couple of batches.'], ['defer', 'defer', 'Goes to the long-term todos.']] as const

/** Count findings once, regardless of anchors or worker fix/decline status. */
export function reviewDecisionTabLabel(kind: ReviewFinding['kind'], findings: readonly ReviewFinding[], rulings: ReadonlyMap<string, DoubtRuling>) {
  const items = findings.filter(item => item.kind === kind)
  const decided = items.filter(item => rulings.has(item.id)).length
  const label = kind === 'Doubt' ? 'Doubts' : 'Nitpicks'
  const progress = `${decided}/${items.length}`
  return {
    accessibleName: `${label} · ${progress}`,
    content: <span className="workspace-tab-label">
      <span>{label}</span><span> · </span>
      <span className={decided < items.length ? 'tab-stat-negative' : 'tab-stat-neutral'}>{progress}</span>
    </span>,
  }
}


export function normalizeReviewRange(range: string): string {
  return range.replace(/@@\s*-(\d+)(?:,(\d+))?\s*\+(\d+)(?:,(\d+))?\s*@@.*/, (_, a, b, c, d) => `@@ -${a},${b ?? '1'} +${c},${d ?? '1'} @@`)
}

/** Item-local paths and ranges use file and hunk headings.
 * Older location-only items remain file-wide; items with no location remain proposal-wide. */
export function reviewAnchors(text: string): ReviewAnchor[] {
  // Parsers join continuation lines; quote complete path headings before joining so spaces survive.
  text = text.replace(/^\s*##[ \t]+([^\n#]+)$/gm, (_, path: string) => `## \`${path.trim().replace(/^`|`$/g, '')}\``)
  const anchors: ReviewAnchor[] = []
  let path: string | undefined
  for (const match of text.matchAll(/(?:^|\s)##\s+(`[^`]+`|\S+)|###\s*(@@\s*-\d+(?:,\d+)?\s*\+\d+(?:,\d+)?\s*@@)/g)) {
    if (match[1]) {
      path = match[1].replace(/^`|`$/g, '')
      anchors.push({ path })
    } else if (path && match[2]) {
      const file = anchors.findIndex(anchor => anchor.path === path && anchor.range == null)
      if (file >= 0) anchors.splice(file, 1)
      anchors.push({ path, range: normalizeReviewRange(match[2]) })
    }
  }
  // Also read inline path and hunk headers.
  for (const match of text.matchAll(/`([^`]+)`\s*`?(@@\s*-\d+(?:,\d+)?\s*\+\d+(?:,\d+)?\s*@@)`?/g)) {
    anchors.push({ path: match[1], range: normalizeReviewRange(match[2]) })
  }
  if (anchors.length === 0) {
    for (const match of text.matchAll(/(?:位置[:：]\s*|`)([\w./-]+\.[\w-]+)(?::\d+(?:-\d+)?)?(?=[`\s:：。]|$)/g)) anchors.push({ path: match[1] })
  }
  return anchors.filter((anchor, index) => anchors.findIndex(other => other.path === anchor.path && other.range === anchor.range) === index)
}

function itemSources(markdown: string): Map<number, string> {
  const sources = new Map<number, string>()
  let index: number | undefined
  for (const line of markdown.split('\n')) {
    const start = /^\s*(\d+)[.)]\s+(.*)/.exec(line)
    if (start) { index = Number(start[1]); sources.set(index, start[2]) }
    else if (index != null) sources.set(index, `${sources.get(index)}\n${line}`)
  }
  return sources
}

/** Anchor headings are metadata; the location controls display them separately. */
export function reviewText(text: string): string {
  return text.replace(/(?:^|\s)##\s+(`[^`]+`|\S+)/g, '')
    .replace(/\s*###\s*@@\s*-\d+(?:,\d+)?\s*\+\d+(?:,\d+)?\s*@@/g, '').trim()
}

export function reviewFindings(doubts: string, nitpicks: string): ReviewFinding[] {
  const doubtSources = itemSources(doubts)
  const nitpickSources = itemSources(nitpicks)
  const withoutHeadings = (markdown: string) => markdown.replace(/^\s*#{2,3}[ \t]+[^\n]*$/gm, '')
  const doubtText = new Map(parseDoubts(withoutHeadings(doubts)).map(item => [item.index, item.text]))
  const nitpickText = new Map(parseNitpicks(withoutHeadings(nitpicks)).map(item => [item.index, item]))
  return [
    ...parseDoubts(doubts).map(item => ({ id: `Doubt ${item.index}`, kind: 'Doubt' as const, index: item.index,
      text: reviewText(doubtText.get(item.index) ?? item.text), anchors: reviewAnchors(doubtSources.get(item.index) ?? item.text), comment: item.text })),
    ...parseNitpicks(nitpicks).map(item => ({ id: `Nitpick ${item.index}`, kind: 'Nitpick' as const, index: item.index,
      text: reviewText(nitpickText.get(item.index)?.headline ?? item.headline), details: reviewText(nitpickText.get(item.index)?.details ?? item.details), tag: item.tag, anchors: reviewAnchors(nitpickSources.get(item.index) ?? `${item.headline} ${item.details}`),
      comment: `[${item.tag}] ${item.headline}${item.details ? ` ${item.details}` : ''}` })),
  ]
}

/** The user's rulings for this candidate, read in order with the last one winning; `revoked` takes the item's ruling
back, so it is undecided again and blocks Accept/Modify like any undecided item. */
export function reviewRulings(entries: ReviewEntry[], candidate: string): Map<string, DoubtRuling> {
  const result = new Map<string, DoubtRuling>()
  for (const entry of entries) {
    if (entry.candidate !== candidate || (entry.role != null && entry.role !== 'reviewer')) continue
    const match = /^(Doubt|Nitpick) (\d+) (sustained|dismissed|followup|defer|revoked):/.exec(entry.body.trim())
    if (match == null) continue
    if (match[3] === 'revoked') result.delete(`${match[1]} ${match[2]}`)
    else result.set(`${match[1]} ${match[2]}`, match[3] as DoubtRuling)
  }
  return result
}

export function reviewBlockedReason({ latest, latestLabel, diffReady, unsigned, findings, rulings, ready, error }: {
  latest: boolean; latestLabel: string; diffReady: boolean; unsigned: number
  findings: ReviewFinding[]; rulings: Map<string, DoubtRuling>; ready: boolean; error?: string
}): string | undefined {
  if (!latest) return `open ${latestLabel} first`
  const missing = [!diffReady ? 'open the Diff tab first' : unsigned > 0 ? `${unsigned} unsigned` : undefined]
  if (findings.length > 0 && !ready) missing.push(error ? 'review decisions unavailable' : 'loading review decisions')
  else for (const kind of ['Doubt', 'Nitpick'] as const) {
    const count = findings.filter(item => item.kind === kind && !rulings.has(item.id)).length
    if (count > 0) missing.push(`${count} undecided ${kind}${count === 1 ? '' : 's'}`)
  }
  return missing.filter(Boolean).join(' · ') || undefined
}

export function revisionActionBlockedLabel(permission: string, acceptLabel?: string): string | undefined {
  return permission === 'accept' ? acceptLabel : permission === 'modify' ? acceptLabel?.replace(/^Accept/, 'Modify') : undefined
}

interface ReviewDocument { folder: string; candidate: string; markdown: string }
interface ReviewClient {
  load: (folder: string, candidate: string, signal?: AbortSignal) => Promise<ReviewDocument>
  save: (folder: string, candidate: string, entry: { comment: string }) => Promise<ReviewDocument>
}

/** One candidate-scoped record drives the tabs, inline decisions and verdict gate. */
export function useReviewDecisions({ folder, candidate, doubts, nitpicks, client, parseEntries }: {
  folder: string; candidate: string; doubts: string; nitpicks: string; client: ReviewClient
  parseEntries: (markdown: string) => ReviewEntry[]
}) {
  const key = JSON.stringify([folder, candidate])
  const active = useRef(key)
  useLayoutEffect(() => { active.current = key }, [key])
  const sequence = useRef(0)
  const writes = useRef<Promise<void>>(Promise.resolve())
  const [record, setRecord] = useState<{ key: string; markdown: string }>()
  const [failure, setFailure] = useState<{ key: string; error: string }>()
  const [reload, setReload] = useState(0)
  const findings = useMemo(() => reviewFindings(doubts, nitpicks), [doubts, nitpicks])
  // A reload keeps the record on screen until the new one arrives, and a record equal to the one shown changes no
  // state, so a background change or the echo of the user's own ruling re-renders nothing; a change reloads without
  // counting itself in state either.
  const shown = useRef({ record, failure })
  shown.current = { record, failure }
  const keepRecord = useCallback((markdown: string) => {
    const current = shown.current.record
    if (current?.key === key && current.markdown === markdown) return
    shown.current = { ...shown.current, record: { key, markdown } }
    setRecord(shown.current.record)
  }, [key])
  const clearFailure = useCallback(() => {
    if (shown.current.failure == null) return
    shown.current = { ...shown.current, failure: undefined }
    setFailure(undefined)
  }, [])
  useEffect(() => {
    if (findings.length === 0) return
    let controller = new AbortController()
    const load = (controller: AbortController) => {
      const request = ++sequence.current
      void client.load(folder, candidate, controller.signal).then(document => {
        if (controller.signal.aborted || request !== sequence.current || active.current !== key) return
        if (document.folder !== folder || document.candidate !== candidate) throw new Error('Unexpected review response.')
        keepRecord(document.markdown)
        clearFailure()
      }).catch((reason: unknown) => {
        if (!controller.signal.aborted && request === sequence.current && active.current === key) setFailure({ key, error: describe(reason) })
      })
    }
    load(controller)
    const refresh = () => {
      controller.abort()
      controller = new AbortController()
      load(controller)
    }
    window.addEventListener(REVIEW_CHANGED_EVENT, refresh)
    return () => {
      window.removeEventListener(REVIEW_CHANGED_EVENT, refresh)
      controller.abort()
    }
  }, [folder, candidate, client, key, keepRecord, clearFailure, reload, findings.length])
  const rulings = useMemo(() => record?.key === key ? reviewRulings(parseEntries(record.markdown), candidate) : new Map<string, DoubtRuling>(), [record, key, candidate, parseEntries])
  const rule = useCallback((item: ReviewFinding, ruling: DoubtDecision) => {
    // Switching tabs can leave a save in flight; serialize writes so an older response cannot erase a ruling.
    const write = writes.current.catch(() => undefined).then(async () => {
      const document = await client.save(folder, candidate, { comment: `${item.id} ${ruling}: ${item.comment}` })
      if (document.folder !== folder || document.candidate !== candidate) throw new Error('Unexpected review response.')
      if (reviewRulings(parseEntries(document.markdown), candidate).get(item.id) !== (ruling === 'revoked' ? undefined : ruling)) throw new Error('Review response did not record the decision.')
      if (active.current === key) { ++sequence.current; keepRecord(document.markdown); clearFailure() }
      window.dispatchEvent(new Event(REVIEW_CHANGED_EVENT))
    })
    writes.current = write
    return write
  }, [folder, candidate, client, key, keepRecord, clearFailure, parseEntries])
  return { findings, rulings, rule, ready: findings.length === 0 || (record?.key === key && failure?.key !== key),
    error: failure?.key === key ? failure.error : undefined, retry: () => setReload(value => value + 1) }
}

export function ReviewFindingList({ findings, rulings, onRule, onLocate, onAction }: {
  findings: ReviewFinding[]; rulings: Map<string, DoubtRuling>
  onRule: (item: ReviewFinding, ruling: DoubtDecision) => Promise<void>
  onLocate?: (anchor: ReviewAnchor) => void
  onAction?: () => void
}) {
  const [busy, setBusy] = useState<string>()
  const [error, setError] = useState<string>()
  const decide = async (item: ReviewFinding, decision: DoubtDecision) => {
    setBusy(item.id); setError(undefined)
    try { await onRule(item, decision); onAction?.() } catch (reason) { setError(describe(reason)) } finally { setBusy(undefined) }
  }
  return <div className="review-findings">
    {error && <p role="alert">{error}</p>}
    {findings.map(item => <div className="review-finding" key={item.id} data-review-finding={item.id} data-ruling={rulings.get(item.id)}>
      <strong>{item.id}</strong>{item.tag && <span className={`review-nitpick-tag review-nitpick-${item.tag}`}>{item.tag}</span>}
      <p className={item.kind === 'Nitpick' ? 'review-nitpick-headline' : undefined}>{item.kind === 'Nitpick' ? <strong>{withCodeSpans(item.text)}</strong> : item.text}</p>{item.details && <p className="review-nitpick-details">{withCodeSpans(item.details)}</p>}
      <ReviewLocations anchors={item.anchors} onLocate={onLocate} />
      <div className="review-doubt-actions">
        {rulings.has(item.id) ? <>
          <span className={`review-doubt-ruling review-doubt-${rulings.get(item.id)}`}>{rulings.get(item.id)}</span>
          <button type="button" className="review-doubt-revoke" disabled={busy != null} aria-label={`revoke · ${item.id}`}
            title="Revoke this ruling" onClick={() => void decide(item, 'revoked')}>revoke</button>
        </> : RULING_BUTTONS.map(([key, ruling, title]) => <button key={ruling} type="button" disabled={busy != null}
            aria-label={key === ruling ? `${item.id} · ${ruling}` : `${key} · ${item.id} · ${ruling}`} title={title}
            onClick={() => void decide(item, ruling)}>{key}</button>)}
      </div>
    </div>)}
  </div>
}

export function ReviewLocations({ anchors, onLocate }: { anchors: ReviewAnchor[]; onLocate?: (anchor: ReviewAnchor) => void }) {
  return <div className="review-locations">{anchors.length === 0 ? <span>Proposal-wide</span> : anchors.map(anchor => {
    const label = `${anchor.range == null ? 'File-wide: ' : ''}${anchor.path}${anchor.range ? ` ${anchor.range}` : ''}`
    return onLocate ? <button type="button" key={label} onClick={() => onLocate(anchor)}>{label}</button> : <span key={label}>{label}</span>
  })}</div>
}

function describe(reason: unknown): string { return reason instanceof Error ? reason.message : String(reason) }

/** Structural diff-view bindings keep review persistence in the review layer. */
export function reviewHunkBindings(decisions: ReturnType<typeof useReviewDecisions>) {
  const { findings, rulings, rule } = decisions
  const atHunk = (path: string, range: string) => findings.filter(item => item.anchors.some(anchor => anchor.path === path && anchor.range === normalizeReviewRange(range)))
  const list = (items: ReviewFinding[], onAction?: () => void) => items.length === 0 ? null
    : <ReviewFindingList findings={items} rulings={rulings} onRule={rule} onAction={onAction} />
  return {
    needsAction: (path: string, range?: string) => findings.some(item => !rulings.has(item.id) && item.anchors.some(anchor =>
      anchor.path === path && anchor.range != null && (range == null || anchor.range === normalizeReviewRange(range)))),
    render: (path: string, range: string, onAction: () => void) => <div className="review-hunk-findings">
      {(['Doubt', 'Nitpick'] as const).map(kind => {
        const items = atHunk(path, range).filter(item => item.kind === kind)
        return <section key={kind} aria-label={`${kind}s`}><strong>{kind}s · {items.length}</strong>
          {items.length === 0 ? <p className="review-hunk-empty">No {kind.toLowerCase()}s.</p> : list(items, onAction)}
        </section>
      })}
    </div>,
    renderFile: (path: string) => {
      const items = findings.filter(item => item.anchors.some(anchor => anchor.path === path && anchor.range == null))
      return items.length === 0 ? null : <section aria-label="File-wide review"><strong>File-wide</strong>{list(items)}</section>
    },
    renderProposal: () => {
      const items = findings.filter(item => item.anchors.length === 0)
      return items.length === 0 ? null : <section aria-label="Proposal-wide review"><strong>Proposal-wide</strong>{list(items)}</section>
    },
  }
}

function withCodeSpans(text: string) {
  return text.split(/(`[^`]+`)/).map((piece, index) => piece.startsWith('`') && piece.endsWith('`') && piece.length > 2
    ? <code key={index}>{piece.slice(1, -1)}</code> : piece)
}
