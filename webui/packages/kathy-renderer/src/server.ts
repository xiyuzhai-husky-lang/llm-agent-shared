// Server rendering (Node): the same renderer (build.ts) running over a server-side DOM, so a page can ship the final
// document as HTML and only hydrate it in the browser (hydrate.ts). Build tools use this; browsers never load it.

import { JSDOM } from 'jsdom'
import type { KyDocument } from '@kathy/ast'

import { renderKyDocumentDom, type KyDocumentRenderOptions } from './build.ts'

/** An empty HTML document for server rendering: pass it to renderKyDocumentDom, or to page code that decorates the
 * rendered DOM before serializing it. */
export function createKyServerDocument(): Document {
  return new JSDOM('<!doctype html><html><head></head><body></body></html>').window.document
}

/** The final `.ky-document` element of `document` as HTML: every source frame loaded, every formula typeset. */
export async function renderKyDocumentHtml(document: KyDocument, options: KyDocumentRenderOptions = {}): Promise<string> {
  const root = await renderKyDocumentDom(createKyServerDocument(), document, options)
  preserveTextBoundaries(root)
  return root.outerHTML
}

export { renderKyDocumentDom, type KyDocumentRenderOptions } from './build.ts'

/** HTML parsing would merge adjacent text nodes. Keep their boundaries so browser
 * shaping and line breaking match the DOM renderer, including CJK punctuation. */
function preserveTextBoundaries(root: Node): void {
  let previous: Node | null = null
  for (const child of Array.from(root.childNodes)) {
    if (previous?.nodeType === 3 && child.nodeType === 3) {
      root.insertBefore(root.ownerDocument!.createComment('ky-text'), child)
    }
    preserveTextBoundaries(child)
    if (child.nodeType === 1 && (child as Element).tagName === 'TEMPLATE') {
      preserveTextBoundaries((child as HTMLTemplateElement).content)
    }
    previous = child
  }
}
