import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import './title-bar.css'

interface CandidateEntry { label: string; stop: string | null; candidate: string; candidate_short: string; href: string; selected: boolean }
export interface TitleBarView {
  title: string
  uid: string
  candidate: string
  candidateShort: string
  candidateLabel: string
  candidateStop: string | null
  location: string
  candidates: CandidateEntry[]
}

export function titleBarSubject(location: URLSearchParams) {
  if (!location.has('proposal')) return undefined
  const proposal = location.get('proposal')
  if (!proposal || !/^\d+$/.test(proposal) || Number(proposal) > 0xffffffff) {
    throw new Error(`URL ?${location}: proposal expected a u32 number, found ${JSON.stringify(proposal)}`)
  }
  const candidate = location.get('candidate')
  if (candidate === null || !/^[0-9a-f]{40,64}$/.test(candidate)) throw new Error(`URL ?${location}: proposal ${proposal}, candidate expected a full commit SHA, found ${JSON.stringify(candidate)}`)
  return { proposal_number: Number(proposal), candidate_commit: candidate }
}

/** The title row. The revision picker stands in it while the review is shut, and in the review's top line while it is open. */
export function ProposalTitleBar({ view, navigate, reviewTopline }: { view: TitleBarView; navigate: (href: string) => void; reviewTopline: HTMLElement | null }) {
  const picker = <RevisionPicker view={view} navigate={navigate} />
  return <header className="proposal-titlebar">
    <div className="proposal-title-copy">
      <h1 id="proposal-title">{view.title}</h1>
      <code className="proposal-uid" title="Proposal UID">{view.uid}</code>    </div>
    {reviewTopline === null ? picker : createPortal(picker, reviewTopline)}
  </header>
}

const STOP_MARK: Record<string, { glyph: string; name: string }> = {
  incomplete: { glyph: '❓', name: 'incomplete' },
  'chow-problems': { glyph: '⚠', name: 'Chow problems' },
}

function StopMark({ stop }: { stop: string }) {
  const { glyph, name } = STOP_MARK[stop]
  return <span className="candidate-stop-mark" data-stop={stop} role="img" aria-label={name} title={name}>{glyph}</span>
}

const stopGround = (stop: string | null) => stop === null ? undefined : `candidate-stop-ground candidate-stop-ground-${stop}`

/** The candidate on screen as one compact control; the chain drops down on demand and closes on a click outside it. */
function RevisionPicker({ view, navigate }: { view: TitleBarView; navigate: (href: string) => void }) {
  const [open, setOpen] = useState(false)
  const root = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!open) return
    const close = (event: MouseEvent) => {
      if (!root.current?.contains(event.target as Node)) setOpen(false)
    }
    window.addEventListener('mousedown', close)
    return () => window.removeEventListener('mousedown', close)
  }, [open])
  useEffect(() => setOpen(false), [view.candidate])
  return <div aria-label="Revisions" className={open ? 'revision-picker open' : 'revision-picker'} ref={root}>
    <button
      type="button"
      className="revision-picker-current"
      aria-expanded={open}
      aria-haspopup="listbox"
      title={view.candidate}
      onClick={() => setOpen(!open)}
    >
      <strong className={stopGround(view.candidateStop)}>
        {view.candidateLabel}
        {view.candidateStop === null ? null : <StopMark stop={view.candidateStop} />}
      </strong>
      <code>{view.candidateShort}</code>
      <span aria-hidden="true">{open ? '▴' : '▾'}</span>
    </button>
    <nav aria-label="Proposal revisions" className="revision-picker-list">
      {view.candidates.map(entry => <a
        key={entry.candidate}
        href={entry.href}
        title={entry.candidate}
        aria-current={entry.selected ? 'page' : undefined}
        onClick={event => {
          if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
          event.preventDefault()
          setOpen(false)
          if (!entry.selected) navigate(entry.href)
        }}
      >
        <strong className={stopGround(entry.stop)}>
          {entry.label}
          {entry.stop === null ? null : <StopMark stop={entry.stop} />}
        </strong>
        <code>{entry.candidate_short}</code>
      </a>)}
    </nav>
  </div>
}
