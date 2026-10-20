import { useEffect, useRef, useState, type CSSProperties } from 'react'
import type { OutcomeArtifacts, OutcomeFragment, OutcomeNode } from '../proposal/outcomeTreeModel'
import { OutcomeSetReport } from './OutcomeSetReport'

/** Left: the test sets. Right: the selected set's key figures and the items that matter. */
export function OutcomeSets({ nodes, artifacts }: { nodes: OutcomeNode[]; artifacts: OutcomeArtifacts }) {
  const root = useRef<HTMLDivElement>(null)
  const [width, setWidth] = useState(() => typeof window === 'undefined' ? 1000 : window.innerWidth)
  const [selected, setSelected] = useState<string | null>(nodes[0]?.key ?? null)
  useEffect(() => {
    const element = root.current!
    const measure = () => { if (element.clientWidth > 0) setWidth(element.clientWidth) }
    measure()
    if (typeof ResizeObserver === 'undefined') return
    const observer = new ResizeObserver(measure)
    observer.observe(element)
    return () => observer.disconnect()
  }, [])
  const inline = width < 720
  const current = nodes.find(node => node.key === selected)
  return <div ref={root} className={`outcome-sets${inline ? ' outcome-tree-mode' : ''}`}>
    <nav aria-label="Outcome test sets" className="outcome-set-list">
      <ul className="outcome-tree">{nodes.map(node => {
        const active = node.key === selected
        return <li key={node.key} data-outcome-key={node.key}>
          <div className="outcome-row outcome-set-row" data-selected={active || undefined} style={{ '--outcome-depth': 0 } as CSSProperties}>
            <button type="button" className="outcome-select" aria-label={node.label} aria-pressed={!inline ? active : undefined}
              aria-expanded={inline ? active : undefined} title={`${node.label} · ${node.token ?? ''} ${node.stats}`}
              onClick={() => setSelected(inline && active ? null : node.key)}>
              <span className="outcome-disclosure" aria-hidden>{inline ? active ? '▾' : '▸' : '·'}</span>
              <span className="outcome-label">{node.label}</span>
              {node.token && <span className={`outcome-token outcome-token-${node.tone ?? 'neutral'}`}>{node.token}</span>}
              <span className="outcome-counts">{node.stats}</span>
            </button>
          </div>
          {inline && active && <div className="outcome-inline-detail"><OutcomeSetReport fragment={node.key as OutcomeFragment} artifact={artifacts[node.key as OutcomeFragment]} /></div>}
        </li>
      })}</ul>
    </nav>
    {!inline && <section aria-label="Selected test set" className="outcome-set-body">
      {current ? <OutcomeSetReport key={current.key} fragment={current.key as OutcomeFragment} artifact={artifacts[current.key as OutcomeFragment]} /> : <p className="outcome-state">Select a test set.</p>}
    </section>}
  </div>
}

