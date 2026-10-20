export type ExplorerTone = 'error' | 'warning' | 'success'

export interface ExplorerNode<T> {
  key: string
  label: string
  count: number
  meta?: string
  tone?: ExplorerTone
  search?: string
  defaultExpanded?: boolean
  children?: ExplorerNode<T>[]
  value?: T
}

/** Prune without changing identities; a search reveals all matching ancestors. */
export function filterExplorer<T>(nodes: ExplorerNode<T>[], query: string): ExplorerNode<T>[] {
  const needle = query.trim().toLowerCase()
  if (!needle) return nodes
  const visit = (node: ExplorerNode<T>, parentMatches = false): ExplorerNode<T>[] => {
    const text = node.children ? node.label : `${node.label} ${node.meta ?? ''} ${node.search ?? ''}`
    const matches = parentMatches || text.toLowerCase().includes(needle)
    if (!node.children) return matches ? [node] : []
    const children = node.children.flatMap(child => visit(child, matches))
    return children.length ? [{ ...node, children, count: children.reduce((sum, child) => sum + child.count, 0), defaultExpanded: true }] : []
  }
  return nodes.flatMap(node => visit(node))
}

export function firstExpandedLeaf<T>(nodes: ExplorerNode<T>[]): ExplorerNode<T> | undefined {
  for (const node of nodes) {
    if (!node.children) return node
    if (node.defaultExpanded) {
      const leaf = firstExpandedLeaf(node.children)
      if (leaf) return leaf
    }
  }
}

export function findExplorerLeaf<T>(nodes: ExplorerNode<T>[], key: string): ExplorerNode<T> | undefined {
  for (const node of nodes) {
    if (node.key === key && !node.children) return node
    const found = node.children && findExplorerLeaf(node.children, key)
    if (found) return found
  }
}
