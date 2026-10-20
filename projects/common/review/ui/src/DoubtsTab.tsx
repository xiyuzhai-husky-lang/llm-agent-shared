import { useMemo, useState } from 'react'
import { parseDoubts, rulingComment, type Doubt, type DoubtDecision, type DoubtRuling, type PublishedDoubt } from './doubtModel'
import { ReviewLocations, reviewFindings, reviewText, type ReviewAnchor } from './reviewDecisions'
import './review-tabs.css'

/** The Reviewer's doubts for the selected candidate; each takes one ruling that the owning reviewer writes
into its review record through `onRule`, and a ruling can be taken back (`revoked`). `rulings` are the rulings already recorded (see `doubtRulings`), keyed by
the Markdown list's numbering. With `published`, the loop driver's structured list, each doubt is shown as separate
parts: the doubt, the Worker's answer labelled by its disposition, and the decision asked of the user. */
export function DoubtsTab({ markdown, published, rulings, onRule, error, onLocate }: {
  markdown: string
  published?: PublishedDoubt[]
  rulings: Map<number, DoubtRuling>
  onRule: (item: Doubt, ruling: DoubtDecision, comment: string) => Promise<void>
  onLocate?: (anchor: ReviewAnchor) => void
  error?: string
}) {
  const items = useMemo(() => parseDoubts(markdown), [markdown])
  const findings = useMemo(() => reviewFindings(markdown, ''), [markdown])
  const [busy, setBusy] = useState<number>()
  const [failure, setFailure] = useState<string>()
  const rule = (item: Doubt, ruling: DoubtDecision) => {
    setBusy(item.index); setFailure(undefined)
    void onRule(item, ruling, rulingComment(item, ruling))
      .catch((reason: unknown) => setFailure(reason instanceof Error ? reason.message : String(reason)))
      .finally(() => setBusy(undefined))
  }
  const entries = published == null
    ? items.map(item => ({ item, published: undefined }))
    : published.map((doubt, position) => ({
      item: items.find(item => item.index === position + 1) ?? { index: position + 1, text: doubt.text }, published: doubt }))
  if (entries.length === 0) return <p className="review-doubts-empty">No doubts from the Reviewer for this candidate.</p>
  return <div className="review-doubts">
    {(error ?? failure) && <p className="review-doubts-error" role="alert">{error ?? failure}</p>}
    <ol>{entries.map(({ item, published }) => {
      const ruling = rulings.get(item.index)
      const finding = findings.find(finding => finding.index === item.index)
      return <li key={item.index} data-ruling={ruling}>
        <p className="review-doubt-text">{published ? reviewText(published.text) : finding?.text ?? item.text}</p>
        <ReviewLocations anchors={finding?.anchors ?? []} onLocate={onLocate} />
        {published && <>
          <div className="review-doubt-worker" data-disposition={published.worker.disposition}>
            <span className="review-doubt-worker-label">Worker · {published.worker.disposition}</span>
            <p>{published.worker.reason}</p>
          </div>
          {published.decision !== null && <p className="review-doubt-decision">{published.decision}</p>}
        </>}
        <div className="review-doubt-actions">
          {ruling ? <>
            <span className={`review-doubt-ruling review-doubt-${ruling}`}>{ruling}</span>
            <button type="button" className="review-doubt-revoke" disabled={busy != null} onClick={() => rule(item, 'revoked')} title="Revoke this ruling">Revoke</button>
          </> : <>
            <button type="button" disabled={busy != null} onClick={() => rule(item, 'sustained')}>Sustain</button>
            <button type="button" disabled={busy != null} onClick={() => rule(item, 'dismissed')}>Dismiss</button>
            <button type="button" disabled={busy != null} onClick={() => rule(item, 'followup')} title="Handled in one of the next couple of batches.">Followup</button>
            <button type="button" disabled={busy != null} onClick={() => rule(item, 'defer')} title="Goes to the long-term todos.">Defer</button>
          </>}
        </div>
      </li>
    })}</ol>
  </div>
}
