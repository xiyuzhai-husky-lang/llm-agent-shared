/** A rendered-span mark: `node` wrapped in `<mark class="rs-span rs-<severity>" data-rs-annotation="<i>">`. */
export function markNode(owner: Document, node: Node, severity: string, annotation: number): HTMLElement {
  const mark = owner.createElement('mark')
  mark.className = `rs-span rs-${severity}`
  mark.dataset.rsAnnotation = String(annotation)
  mark.append(node)
  return mark
}
