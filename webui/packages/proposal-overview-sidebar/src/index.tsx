import { useEffect, useState } from 'react'
import type { SidebarView, SidebarRowView } from './types'
import './sidebar.css'
export type { SidebarView } from './types'
export const overviewSubject = () => null

/** The sidebar: the proposals under Open, In progress and Closed, in the order the server handed them over. */
export function ProposalOverviewSidebar({ view, navigate }: { view: SidebarView; navigate: (href: string) => void }) {
  const [folded, setFolded] = useState<Record<string, boolean>>({})
  const openBatch = () => { const query = new URLSearchParams(window.location.search); query.set('page', 'batch'); navigate(`?${query}`) }
  const current = new URLSearchParams(window.location.search).get('proposal')
  // With no proposal selected, the page opens the server's default row as a click on it would.
  useEffect(() => { if (current === null && view.defaultHref !== null) navigate(view.defaultHref) }, [current, view.defaultHref, navigate])
  return <aside className="proposal-explorer">
    <header className="proposal-explorer-header">
      <h1>{view.title}</h1>
      <button type="button" className="proposal-batch-link" onClick={openBatch}>{view.batchPageLabel}</button>
    </header>
    {view.stackNotice && <p className="proposal-explorer-empty proposal-explorer-stack-notice">{view.stackNotice}</p>}
    {view.errors.length > 0 && <ul aria-label="Proposal errors">{view.errors.map(error => <li key={error} role="alert">{error}</li>)}</ul>}
    {view.emptyNotice && <p className="proposal-explorer-empty">{view.emptyNotice}</p>}
    {view.parts.length > 0 && <nav aria-label="Proposals" className="proposal-explorer-list">
      {view.parts.map(part => {
        const key = part.heading.toLowerCase()
        const collapsed = part.foldable && (folded[key] ?? part.collapsed)
        const label = <>{part.heading}<span className="proposal-explorer-count"> {part.count}</span></>
        return <section aria-label={part.heading} className="proposal-explorer-part" data-part={key} data-collapsed={collapsed} key={key}>
          {part.foldable
            ? <h2><button type="button" className="proposal-explorer-part-toggle" aria-expanded={!collapsed} onClick={() => setFolded({ ...folded, [key]: !collapsed })}>
                <span className="proposal-explorer-chevron" aria-hidden="true">{collapsed ? '▸' : '▾'}</span>{label}
              </button></h2>
            : <h2>{label}</h2>}
          {collapsed ? null : part.rows.map(row => <SidebarRow key={row.folder} row={row} selected={current !== null && new URL(row.href, window.location.href).searchParams.get('proposal') === current} navigate={navigate} />)}
        </section>
      })}
    </nav>}
  </aside>
}

function SidebarRow({ row, selected, navigate }: { row: SidebarRowView; selected: boolean; navigate: (href: string) => void }) {
  return <a
    href={row.href}
    data-proposal={row.uid}
    aria-current={selected ? 'page' : undefined}
    onClick={event => {
      if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
      event.preventDefault()
      if (!selected) navigate(row.href)
    }}
  >
    <strong className={row.stop ? `candidate-stop-ground candidate-stop-ground-${row.candidateKind}` : undefined}>
      <span className="proposal-ordinal">{row.ordinalLabel}</span> {row.title}
      {row.stop && <span className="candidate-stop-mark" role="img" aria-label={stopName(row.candidateKind)} title={stopName(row.candidateKind)}>{row.stop}</span>}
    </strong>
    <span>{row.dateLabel}{row.workflowLabel && <> · {row.workflowLabel}</>}</span>
    {row.statusLabel && <em className={`proposal-status proposal-status-${row.statusLabel}`}>{row.statusLabel}</em>}
  </a>
}

const stopName = (kind: string) => kind === 'chow_problems' ? 'Chow problems' : 'incomplete'
