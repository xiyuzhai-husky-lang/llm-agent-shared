import { useMemo, type ClipboardEvent } from 'react'
import { parseKyDocumentJson, type KyDocumentWire } from '@kathy/ast'
import { KyDocumentView } from '@kathy/renderer'
import { createKathyVfs, type KathyVfs } from '@kathy/vfs'
import './kathy-surface.css'

/** Create once in the host application and share across its surfaces. */
export function createKathySurfaceVfs({ folder, revision, loadSource }: {
  folder: string
  revision: string
  loadSource: (path: string, folder: string, revision: string, oid?: string) => Promise<string>
}): KathyVfs {
  return createKathyVfs((path, oid) => loadSource(path, folder, revision, oid))
}

export function KathySurface({ document, revision, label, vfs }: {
  document?: KyDocumentWire
  revision: string
  label: string
  vfs: KathyVfs
}) {
  const parsed = useMemo(() => document == null ? null : parseKyDocumentJson(document), [document])
  if (parsed == null) return <p>{label} is not available in this revision.</p>
  return <div className="kathy-surface proposal-diagnosis" role="region" aria-label={`${label} document`} onCopy={copyWithTexForMath}>
    <KyDocumentView document={parsed} profile="web-v1-source" vfs={vfs} oid={revision} />
  </div>
}

/**
 * KaTeX renders one glyph per positioned span, so a native copy yields one token per line.
 * Rebuild the selection text with each `.ky-math` element replaced by its `data-tex` source,
 * so a copy of prose + formulas pastes as a single readable line.
 */
function copyWithTexForMath(event: ClipboardEvent<HTMLDivElement>) {
  const selection = window.getSelection()
  if (selection == null || selection.rangeCount === 0 || selection.isCollapsed) return
  const fragment = selection.getRangeAt(0).cloneContents()
  const container = document.createElement('div')
  container.append(fragment)
  const partial = container.querySelector('.ky-math') == null && container.querySelector('.katex') != null
  for (const math of container.querySelectorAll<HTMLElement>('.ky-math')) {
    const tex = math.getAttribute('data-tex') ?? math.textContent ?? ''
    math.replaceWith(document.createTextNode(math.classList.contains('ky-math-display') ? `\n$$${tex}$$\n` : `$${tex}$`))
  }
  for (const clipped of container.querySelectorAll<HTMLElement>('.ky-verbatim-inline-long[data-full]')) {
    clipped.replaceWith(document.createTextNode(clipped.getAttribute('data-full') ?? ''))
  }
  for (const katexNode of container.querySelectorAll<HTMLElement>('.katex')) {
    katexNode.replaceWith(document.createTextNode(katexNode.textContent?.replace(/\s+/g, ' ') ?? ''))
  }
  const text = container.innerText.replace(/[ \t]+\n/g, '\n').replace(/\n{3,}/g, '\n\n').trim()
  if (partial && text.length === 0) return
  event.clipboardData.setData('text/plain', text)
  event.preventDefault()
}
