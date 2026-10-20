import { useMemo } from 'react'
import type { DoubtDecision, DoubtRuling } from './doubtModel'
import { ReviewFindingList, reviewFindings, type ReviewAnchor, type ReviewFinding } from './reviewDecisions'
import './review-tabs.css'

/** Worker dispositions and the user's ruling are independent. Every tag requires a user decision. */
export function NitpicksTab({ markdown, rulings, onRule, onLocate }: {
  markdown: string
  rulings: Map<string, DoubtRuling>
  onRule: (item: ReviewFinding, ruling: DoubtDecision) => Promise<void>
  onLocate?: (anchor: ReviewAnchor) => void
}) {
  const items = useMemo(() => reviewFindings('', markdown), [markdown])
  if (/^Nitpicks:\s*\nunavailable\b/m.test(markdown)) return <p className="review-nitpicks-empty">{markdown.split('\n').slice(3).join('\n')}</p>
  if (items.length === 0) return <p className="review-nitpicks-empty">No nitpicks for this candidate.</p>
  return <div className="review-nitpicks"><ReviewFindingList findings={items} rulings={rulings} onRule={onRule} onLocate={onLocate} /></div>
}
