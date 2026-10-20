import type { ReactNode } from 'react'
import type { ExplorerNode } from './explorerModel.ts'

export function ExplorerTree<T>({ nodes, label, selectedKey, onSelect, inline, renderDetails, filtering = false, expansion, onToggle }: {
  nodes: ExplorerNode<T>[]; label: string; selectedKey?: string; onSelect: (key: string | null) => void
  inline: boolean; renderDetails: (value: T) => ReactNode; filtering?: boolean
  expansion: Map<string, boolean>; onToggle: (key: string, open: boolean) => void
}) {
  const renderNode = (node: ExplorerNode<T>): ReactNode => {
    const active = selectedKey === node.key
    const expansionKey = `${filtering ? 'filter' : 'tree'}:${node.key}`
    const open = expansion.get(expansionKey) ?? (filtering || node.defaultExpanded || false)
    const branch = node.children != null
    return <li key={node.key} className={node.tone ? `report-tone-${node.tone}` : undefined}>
      <button type="button" className="report-tree-row" title={`${node.label}${node.meta ? ` · ${node.meta}` : ''}`}
        aria-expanded={branch ? open : inline ? active : undefined}
        aria-pressed={!branch && !inline ? active : undefined}
        onClick={() => {
          if (branch) onToggle(expansionKey, !open)
          else onSelect(inline && active ? null : node.key)
        }}>
        <span className="report-triangle" aria-hidden="true">{branch ? open ? '▾' : '▸' : inline ? active ? '▾' : '▸' : '·'}</span>
        <span className="report-tree-label">{node.label}</span>{' '}
        {branch && <span className="report-count">{node.count}</span>}{' '}
        {node.meta && <small>{node.meta}</small>}
      </button>
      {branch && open && <ul>{node.children!.map(renderNode)}</ul>}
      {!branch && inline && active && node.value !== undefined && <div className="report-inline-detail">{renderDetails(node.value)}</div>}
    </li>
  }
  return <ul className="report-tree" aria-label={label}>{nodes.map(renderNode)}</ul>
}
