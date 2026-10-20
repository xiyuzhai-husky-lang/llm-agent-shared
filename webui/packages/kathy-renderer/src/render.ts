// Render in this browser: build the final DOM (build.ts), attach it, hydrate it (hydrate.ts). The DOM a server
// renders (server.ts) is the same DOM; a page that ships it only hydrates.

import type { KyDocument } from '@kathy/ast'

import { startKyDocumentDom, type KyDocumentRenderOptions } from './build.ts'
import { hydrateKyDocument } from './hydrate.ts'
import './styles.ts'

export { renderKyDocumentDom, parseSourceReference, KY_VERBATIM_INLINE_PREVIEW_CHARS, type KyDocumentRenderOptions } from './build.ts'
export { hydrateKyDocument } from './hydrate.ts'
export { KyRendererError, type KyRendererProfileId } from './errors.ts'
export { validateRendererSupport } from './profiles.ts'

export interface KyDocumentRenderHandle {
  /** Settles once the document is in `container` and interactive; rejects with the KyRendererError that stopped it. */
  readonly finished: Promise<void>
  dispose(): void
}

/** Throws synchronously, before anything is read or written, when the document is outside the profile or a source
 * reference lacks a VFS or revision. */
export function renderKyDocument(
  container: HTMLElement,
  document: KyDocument,
  options: KyDocumentRenderOptions = {},
): KyDocumentRenderHandle {
  const building = startKyDocumentDom(container.ownerDocument, document, options)
  let disposed = false
  let root: HTMLElement | undefined
  let dehydrate: (() => void) | undefined
  return {
    finished: building.then((built) => {
      if (disposed) return
      root = built
      container.append(root)
      dehydrate = hydrateKyDocument(root)
    }),
    dispose() {
      if (disposed) return
      disposed = true
      dehydrate?.()
      root?.remove()
    },
  }
}
