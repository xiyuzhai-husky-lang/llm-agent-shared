import { useMemo, useState, type ReactNode } from 'react'
import { ExplorerPanels } from './ExplorerPanels.tsx'
import { ExplorerTree } from './ExplorerTree.tsx'
import { filterExplorer, findExplorerLeaf, firstExpandedLeaf, type ExplorerNode } from './explorerModel.ts'

export function ReportExplorer<T>({ nodes, label, filterLabel, tree, header, renderDetails, empty = 'No matching records.',
  selectedKey, onSelect, detailClassName = '' }: {
  nodes: ExplorerNode<T>[]; label: string; filterLabel: string; tree: boolean; header?: ReactNode
  renderDetails: (value: T) => ReactNode; empty?: string
  selectedKey?: string | null; onSelect?: (key: string | null) => void; detailClassName?: string
}) {
  const [query, setQuery] = useState('')
  const [localKey, setLocalKey] = useState<string | null | undefined>()
  const [expansion, setExpansion] = useState<Map<string, boolean>>(() => new Map())
  const filtered = useMemo(() => filterExplorer(nodes, query), [nodes, query])
  const key = onSelect ? selectedKey : localKey
  const active = key === null ? undefined : key === undefined ? firstExpandedLeaf(nodes) : findExplorerLeaf(nodes, key)
  const explorer = <>
    {header}
    <label className="report-filter"><input type="search" aria-label={filterLabel} placeholder={filterLabel}
      value={query} onChange={event => setQuery(event.target.value)} /></label>
    <ExplorerTree nodes={filtered} label={label} inline={tree} selectedKey={active?.key}
      expansion={expansion} onToggle={(key, open) => setExpansion(current => new Map(current).set(key, open))}
      onSelect={onSelect ?? setLocalKey} renderDetails={renderDetails} filtering={query.trim() !== ''} />
    {!filtered.length && <p className="report-empty">{empty}</p>}
  </>
  return tree ? <div className="report-explorer report-explorer-inline">{explorer}</div>
    : <ExplorerPanels className="report-explorer" label={`Resize ${label} and details`} left={explorer}
      right={<div className={`report-detail ${detailClassName}`}>{active?.value !== undefined
        ? renderDetails(active.value) : <p className="report-empty">Select a leaf to view details.</p>}</div>} />
}
