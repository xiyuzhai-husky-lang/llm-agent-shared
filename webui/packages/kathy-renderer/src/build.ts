// Build: a checked Kathy document -> its complete, final DOM, in any Document (the browser's, or the server's in
// server.ts). Sources named by the document are read through the Kathy VFS here, and only here; the DOM that comes
// out holds every source frame in its loaded state and carries no event listeners. hydrate.ts gives it behaviour.

import type { KathyVfs } from '@kathy/vfs'
import type { KyDocument, KyRoseBoundaryClass, KyRoseAstIdxRangeWire, KyRenderedSpanAnnotation } from '@kathy/ast'
import { KY_VERDICTS, classifyKyRosePunctuation, roseBoundaryNeedsSpace } from '@kathy/ast'

import { KyRendererError, type KyRendererProfileId } from './errors.ts'
import { markNode } from './marks.ts'
import { renderMathEnvironment, renderMathRange } from './math/katex.ts'
import { KY_TIKZ_HAS_NO_RENDERER, validateRendererSupport } from './profiles.ts'
import { renderStaticSourceFrame } from './source-frame.ts'
import { buildSpanWindow } from './span-window.ts'
import { rosePunctuation, serializeEnvironment, serializeMathRange } from './tex.ts'

export interface KyDocumentRenderOptions {
  profile?: KyRendererProfileId
  recommendedLabel?: string
  vfs?: KathyVfs
  oid?: string
}

/** Printed data longer than this many characters is shown as a clipped preview; tapping it opens the full text in a dialog. */
export const KY_VERBATIM_INLINE_PREVIEW_CHARS = 48

/** Render `document` to its final `.ky-document` element, owned by `ownerDocument` and not yet attached. Rejects with a
 * KyRendererError when the document is outside the profile, a source cannot be read, or a source range or span does
 * not fit its file. */
export async function renderKyDocumentDom(ownerDocument: Document, document: KyDocument, options: KyDocumentRenderOptions = {}): Promise<HTMLElement> {
  return startKyDocumentDom(ownerDocument, document, options)
}

/** As renderKyDocumentDom, but a document outside the profile, or a source reference without a VFS or revision,
 * throws synchronously, before anything is read. */
export function startKyDocumentDom(ownerDocument: Document, document: KyDocument, options: KyDocumentRenderOptions): Promise<HTMLElement> {
  validateRendererSupport(document, options.profile)
  const requests = sourceRequests(document, options)
  return loadSources(requests, options).then((sources) => {
    const root = ownerDocument.createElement('div')
    root.className = 'ky-document'
    try {
      renderRoseRange(root, document, document.body, { options, sources })
    } catch (error) {
      if (error instanceof KyRendererError) throw error
      throw new KyRendererError('RenderFailed', 'body', {
        profile: options.profile,
        message: error instanceof Error ? error.message : String(error),
      })
    }
    return root
  })
}

interface SourceRequest {
  path: string
  oid: string
  line: number
  endLine: number
  annotations: Array<{ line: number; start: number; end: number }>
  at: string
}

type Sources = Map<string, string>
const sourceKey = (path: string, oid: string) => `${oid}\u0000${path}`

interface RenderContext {
  options: KyDocumentRenderOptions
  sources: Sources
  rendered?: { document: KyDocument; annotations: KyRenderedSpanAnnotation<KyRoseAstIdxRangeWire>[] }
}

/** Every source excerpt the document (and each document embedded in it) shows. */
function sourceRequests(document: KyDocument, options: KyDocumentRenderOptions, requests: SourceRequest[] = []): SourceRequest[] {
  for (const [index, node] of document.arena.rose.entries()) {
    const at = `arena.rose[${index}]`
    if (node.kind === 'rendered_span') sourceRequests(node.content as KyDocument, options, requests)
    const reference = node.kind === 'codespan'
      ? { path: node.source.path, line: node.source.line, endLine: node.source.end_line, oid: node.source.oid ?? undefined, annotations: node.annotations }
      : node.kind === 'verbatim_command' && options.profile === 'web-v1-source' ? parseSourceReference(node.body) : null
    if (reference == null) continue
    if (options.vfs == null) throw new KyRendererError('MissingSourceVfs', at)
    const oid = reference.oid ?? options.oid
    if (oid == null) throw new KyRendererError('MissingSourceOid', at)
    requests.push({ path: reference.path, oid, line: reference.line, endLine: reference.endLine, annotations: 'annotations' in reference ? reference.annotations : [], at })
  }
  return requests
}

async function loadSources(requests: SourceRequest[], options: KyDocumentRenderOptions): Promise<Sources> {
  const sources: Sources = new Map()
  await Promise.all(requests.map(async (request) => {
    const key = sourceKey(request.path, request.oid)
    let text: string
    try {
      text = await options.vfs!.read(request.path, request.oid)
    } catch (error) {
      throw new KyRendererError('SourceUnavailable', request.at, {
        profile: options.profile,
        message: `SourceUnavailable: ${request.path}@${request.oid}: ${error instanceof Error ? error.message : String(error)}`,
      })
    }
    const lines = text.split('\n')
    if (request.line < 1 || request.endLine < request.line || request.endLine > lines.length) {
      throw new KyRendererError('SourceRangeOutsideFile', request.at, {
        profile: options.profile,
        message: `SourceRangeOutsideFile: ${request.path}#L${request.line}-L${request.endLine}@${request.oid} (${lines.length} lines)`,
      })
    }
    for (const a of request.annotations) {
      const length = Array.from(lines[a.line - 1] ?? '').length
      if (![a.line, a.start, a.end].every(Number.isSafeInteger) || a.line < request.line || a.line > request.endLine || a.start < 1 || a.end < a.start || a.end > length) {
        throw new KyRendererError('SpanOutsideSourceLine', request.at, {
          profile: options.profile,
          message: `SpanOutsideSourceLine: ${request.path}:L${a.line} C${a.start}-${a.end}@${request.oid} (${length} characters)`,
        })
      }
    }
    sources.set(key, text)
  }))
  return sources
}

function renderRoseRange(
  root: HTMLElement,
  document: KyDocument,
  range: KyRoseAstIdxRangeWire,
  context: RenderContext,
): void {
  const owner = root.ownerDocument
  let block: HTMLParagraphElement | undefined
  let previous: KyRoseBoundaryClass | undefined
  const ensureBlock = (): HTMLParagraphElement => {
    if (block === undefined) {
      block = owner.createElement('p')
      root.append(block)
    }
    return block
  }
  let currentIndex = -1
  const markRose = (node: Node): Node => {
    if (context.rendered?.document === document) {
      context.rendered.annotations.forEach((a, i) => {
        if (a.targets.some((t) => t.kind === 'rose' && t.index === currentIndex)) node = markNode(owner, node, a.severity ?? 'note', i)
      })
    }
    return node
  }
  const appendVisible = (node: Node, boundary: KyRoseBoundaryClass): void => {
    node = markRose(node)
    const target = ensureBlock()
    if (previous !== undefined && roseBoundaryNeedsSpace(previous, boundary)) {
      target.append(owner.createTextNode(' '))
    }
    target.append(node)
    previous = boundary
  }

  for (let index = range.start; index < range.end; index += 1) {
    currentIndex = index
    const node = document.arena.rose[index]
    if (node === undefined) throw new KyRendererError('MissingAst', `arena.rose[${index}]`)
    switch (node.kind) {
      case 'word': appendVisible(owner.createTextNode(node.value), 'ordinary'); break
      case 'nat32': appendVisible(owner.createTextNode(String(node.value)), 'ordinary'); break
      case 'punctuation': appendVisible(owner.createTextNode(rosePunctuation(node.value)), classifyKyRosePunctuation(node.value)); break
      case 'math': {
        const tex = serializeMathRange(document, node.math_asts)
        const displayMode = node.left_delimiter === 'dollar_dollar' || node.left_delimiter === 'escaped_lbox'
        const element = owner.createElement(displayMode ? 'div' : 'span')
        element.className = displayMode ? 'ky-math ky-math-display' : 'ky-math ky-math-inline'
        element.setAttribute('data-tex', tex)
        renderMathRange(element, document, node.math_asts, tex, displayMode, context.rendered?.document === document ? context.rendered.annotations : [])
        if (displayMode) {
          block = undefined
          previous = undefined
          root.append(markRose(element))
        } else appendVisible(element, 'ordinary')
        break
      }
      case 'delimited': {
        const element = owner.createElement('span')
        element.append(owner.createTextNode('{'))
        renderRoseRange(element, document, node.asts, context)
        element.append(owner.createTextNode('}'))
        appendVisible(element, 'ordinary')
        break
      }
      case 'complete_command': {
        const argument = node.arguments[0]
        const heading = node.command_name === 'section' ? 'h2' : node.command_name === 'subsection' ? 'h3' : undefined
        const style = node.command_name === 'texttt' ? 'code'
          : context.options.profile === 'web-v1-source' && node.command_name === 'textbf' ? 'strong' : undefined
        if ((heading != null || style != null) && argument?.kind === 'required' && argument.data.kind === 'rose') {
          const element = owner.createElement(heading ?? style!)
          appendRoseRangeInline(element, document, argument.data.range, context)
          if (heading != null) { block = undefined; previous = undefined; root.append(element) }
          else appendVisible(element, 'ordinary')
          break
        }
        if (node.command_name === 'kySideEffects' && argument?.kind === 'required' && argument.data.kind === 'rose') {
          block = undefined; previous = undefined
          const element = owner.createElement('div')
          element.className = 'ky-side-effects'
          appendRoseRangeInline(element, document, argument.data.range, context)
          root.append(element)
          break
        }
        if (node.command_name !== 'label' || argument?.kind !== 'required' || argument.data.kind !== 'name') {
          throw new KyRendererError('UnsupportedByRenderer', `arena.rose[${index}]`)
        }
        const anchor = owner.createElement('span')
        anchor.className = 'ky-label'
        anchor.id = argument.data.value
        anchor.setAttribute('aria-hidden', 'true')
        ensureBlock().append(anchor)
        break
      }
      case 'verbatim_command': {
        const reference = context.options.profile === 'web-v1-source' ? parseSourceReference(node.body) : null
        if (reference != null) {
          const oid = reference.oid ?? context.options.oid!
          const element = owner.createElement(block == null ? 'div' : 'span')
          element.className = 'ky-source'
          renderStaticSourceFrame(element, {
            path: reference.path, line: reference.line, endLine: reference.endLine, oid,
            source: context.sources.get(sourceKey(reference.path, oid))!, open: false, compactCollapsed: true,
          })
          if (block == null) root.append(element)
          else appendVisible(element, 'ordinary')
          break
        }
        appendVisible(renderVerbatimInline(owner, node.body), 'ordinary')
        break
      }
      case 'codespan': {
        const oid = node.source.oid ?? context.options.oid!
        block = undefined; previous = undefined
        const element = owner.createElement('div')
        element.className = 'ky-source ky-codespan'
        const annotations = node.annotations.map((a) => {
          const label = owner.createElement('span')
          appendRoseRangeInline(label, document, a.label, context)
          return { line: a.line, start: a.start, end: a.end, severity: a.severity ?? undefined, label }
        })
        renderStaticSourceFrame(element, {
          path: node.source.path, line: node.source.line, endLine: node.source.end_line, oid,
          source: context.sources.get(sourceKey(node.source.path, oid))!, open: true, annotations,
        })
        root.append(element)
        break
      }
      case 'input': {
        // The document declares the control; whoever hosts the page stores the answer.
        block = undefined
        previous = undefined
        const element = owner.createElement('div')
        element.className = `ky-input ky-input-kind-${node.control.kind}`
        element.setAttribute('data-input-name', node.name)
        const prompt = owner.createElement('div')
        prompt.className = 'ky-input-prompt'
        prompt.id = `ky-input-prompt-${node.name}`
        appendRoseRangeInline(prompt, document, node.prompt, context)
        element.append(prompt)
        if (node.control.kind === 'text') {
          const textarea = owner.createElement('textarea')
          textarea.className = 'ky-input-text'
          textarea.name = node.name
          textarea.rows = 3
          textarea.setAttribute('aria-labelledby', prompt.id)
          element.append(textarea)
        } else if (node.control.kind === 'verdict') {
          // One widget: a row of verdict buttons (radio semantics) and a note below.
          const verdicts = owner.createElement('div')
          verdicts.className = 'ky-verdicts'
          verdicts.setAttribute('role', 'radiogroup')
          verdicts.setAttribute('aria-labelledby', prompt.id)
          for (const verdict of KY_VERDICTS) {
            const label = owner.createElement('label')
            label.className = `ky-verdict ky-verdict-${verdict}`
            const radio = owner.createElement('input')
            radio.type = 'radio'
            radio.name = `${node.name}.verdict`
            radio.value = verdict
            label.append(radio, owner.createTextNode(verdict))
            verdicts.append(label)
          }
          const note = owner.createElement('textarea')
          note.className = 'ky-input-text ky-verdict-note'
          note.name = `${node.name}.note`
          note.rows = 2
          note.placeholder = '备注'
          note.setAttribute('aria-label', '备注')
          element.append(verdicts, note)
        } else {
          const options = owner.createElement('div')
          options.className = 'ky-input-options'
          options.setAttribute('role', 'radiogroup')
          options.setAttribute('aria-labelledby', prompt.id)
          for (const [index, option] of node.control.options.entries()) {
            const label = owner.createElement('label')
            label.className = 'ky-input-option'
            const radio = owner.createElement('input')
            radio.type = 'radio'
            radio.name = node.name
            const content = owner.createElement('div')
            content.className = 'ky-choice-content'
            if (typeof option === 'string') {
              radio.value = option
              content.textContent = option
            } else {
              radio.value = String(index)
              renderRoseRange(content, document, option, context)
              if (index === 0) {
                label.classList.add('ky-choice-recommended')
                label.dataset.recommendedLabel = context.options.recommendedLabel ?? '推荐'
              }
            }
            label.append(radio, content)
            options.append(label)
          }
          element.append(options)
        }
        root.append(element)
        break
      }
      case 'rendered_span': {
        block = undefined; previous = undefined
        const element = owner.createElement('div')
        element.className = 'ky-rendered-span'
        const viewport = owner.createElement('div')
        viewport.className = 'rs-viewport'
        const previousRendered = context.rendered
        const content = node.content as KyDocument
        context.rendered = { document: content, annotations: node.annotations }
        try { renderRoseRange(viewport, content, content.body, context) }
        finally { context.rendered = previousRendered }
        const labels = owner.createElement('div')
        labels.className = 'rs-labels'
        node.annotations.forEach((a, i) => {
          const label = owner.createElement('div')
          label.className = `rs-label rs-${a.severity ?? 'note'}`
          label.dataset.rsAnnotation = String(i)
          appendRoseRangeInline(label, document, a.label, context)
          labels.append(label)
        })
        viewport.append(labels)
        buildSpanWindow(element, viewport, node.source, node.annotations.length)
        root.append(element)
        break
      }
      case 'table': {
        block = undefined; previous = undefined
        const wrapper = owner.createElement('div')
        wrapper.className = 'ky-table-wrap'
        const table = owner.createElement('table')
        table.className = 'ky-table'
        const hoareWatch = root.classList.contains('ky-hoare-watch')
        const head = table.createTHead()
        const body = table.createTBody()
        node.rows.forEach((row, rowIndex) => {
          const element = (hoareWatch && rowIndex === 0 ? head : body).insertRow()
          if (node.horizontal_rules.includes(rowIndex)) element.classList.add('ky-table-rule-before')
          row.forEach((cell, columnIndex) => {
            const cellElement = owner.createElement(hoareWatch && (rowIndex === 0 || columnIndex === 0) ? 'th' : 'td')
            if (cellElement.tagName === 'TH') cellElement.setAttribute('scope', rowIndex === 0 ? 'col' : 'row')
            cellElement.dataset.boundaryLeft = node.column_boundaries[columnIndex]
            cellElement.dataset.boundaryRight = node.column_boundaries[columnIndex + 1]
            appendRoseRangeInline(cellElement, document, cell, context)
            element.append(cellElement)
          })
        })
        if (node.horizontal_rules.includes(node.rows.length)) table.classList.add('ky-table-rule-after')
        wrapper.append(table)
        root.append(wrapper)
        break
      }
      case 'environment': {
        block = undefined
        previous = undefined
        if (node.asts.kind === 'tikz') {
          throw new KyRendererError('UnsupportedByRenderer', `arena.rose[${index}]`, { profile: context.options.profile, reason: KY_TIKZ_HAS_NO_RENDERER })
        }
        if (node.asts.kind === 'verbatim') {
          const pre = owner.createElement('pre')
          pre.className = `ky-verbatim ky-${node.environment_name}`
          const code = owner.createElement('code')
          // LaTeX discards the line break right after \begin{verbatim}; do the same so the box has no blank first line.
          let leadingNewline = true
          for (const token of node.asts.tokens) {
            const span = owner.createElement('span')
            span.className = `ky-code-${token.category}`
            span.textContent = leadingNewline ? token.text.replace(/^\r?\n/, '') : token.text
            if (token.text.length > 0) leadingNewline = false
            code.append(span)
          }
          pre.append(code)
          root.append(pre)
          break
        }
        switch (node.environment_name) {
          case 'kyHoareWatch': {
            if (node.asts.kind !== 'rose') throw new KyRendererError('UnsupportedByRenderer', `arena.rose[${index}].asts`)
            const watch = owner.createElement('div')
            watch.className = 'ky-hoare-watch'
            renderRoseRange(watch, document, node.asts.range, context)
            root.append(watch)
            break
          }
          case 'kyQuote': {
            if (node.asts.kind !== 'rose') throw new KyRendererError('UnsupportedByRenderer', `arena.rose[${index}].asts`)
            const quote = owner.createElement('blockquote')
            quote.className = 'ky-environment ky-quote'
            const title = node.arguments[0]
            if (title?.kind === 'optional' && title.data.kind === 'wild' && title.data.value !== '') {
              const caption = owner.createElement('div')
              caption.className = 'ky-quote-caption'
              caption.textContent = title.data.value
              quote.append(caption)
            }
            const body = owner.createElement('div')
            body.className = 'ky-quote-body'
            renderRoseRange(body, document, node.asts.range, context)
            quote.append(body)
            root.append(quote)
            break
          }
          case 'example':
          case 'theorem':
          case 'proof':
          case 'remark':
          case 'definition':
          case 'lemma':
          case 'corollary':
          case 'proposition': {
            if (node.asts.kind !== 'rose') throw new KyRendererError('UnsupportedByRenderer', `arena.rose[${index}].asts`)
            const section = owner.createElement('section')
            section.className = `ky-environment ky-${node.environment_name}`
            const heading = owner.createElement('h2')
            heading.append(environmentHeading(node.environment_name))
            const title = node.arguments[0]
            if (title?.kind === 'optional' && title.data.kind === 'rose') {
              heading.append(' (')
              appendRoseRangeInline(heading, document, title.data.range, context)
              heading.append(')')
            }
            section.append(heading)
            renderRoseRange(section, document, node.asts.range, context)
            root.append(section)
            break
          }
          case 'itemize':
          case 'enumerate': {
            if (node.asts.kind !== 'rose') throw new KyRendererError('UnsupportedByRenderer', `arena.rose[${index}].asts`)
            root.append(renderList(node.environment_name === 'enumerate' ? 'ol' : 'ul', document, node.asts.range, owner, context))
            break
          }
          default: {
            if (node.asts.kind !== 'math') throw new KyRendererError('UnsupportedByRenderer', `arena.rose[${index}]`)
            const element = owner.createElement('div')
            element.className = `ky-math ky-math-display ky-${node.environment_name}`
            const environmentTex = serializeEnvironment(document, node)
            element.setAttribute('data-tex', environmentTex)
            renderMathEnvironment(element, environmentTex)
            root.append(element)
            break
          }
        }
        break
      }
      case 'item': throw new KyRendererError('UnexpectedItem', `arena.rose[${index}]`)
      case 'new_paragraph': block = undefined; previous = undefined; break
      default: throw new KyRendererError('UnsupportedByRenderer', `arena.rose[${index}]`)
    }
  }
}

function renderVerbatimInline(owner: Document, body: string): HTMLElement {
  const code = owner.createElement('code')
  code.className = 'ky-verbatim-inline'
  if (body.length <= KY_VERBATIM_INLINE_PREVIEW_CHARS) {
    code.textContent = body
    return code
  }
  // The full text rides along in data-full; hydrate.ts opens it in a dialog.
  code.classList.add('ky-verbatim-inline-long')
  code.setAttribute('role', 'button')
  code.setAttribute('tabindex', '0')
  code.setAttribute('title', '点击展开全文')
  code.setAttribute('data-full', body)
  const preview = owner.createElement('span')
  preview.className = 'ky-verbatim-inline-preview'
  preview.textContent = body.slice(0, KY_VERBATIM_INLINE_PREVIEW_CHARS).trimEnd()
  const ellipsis = owner.createElement('span')
  ellipsis.className = 'ky-verbatim-inline-ellipsis'
  ellipsis.textContent = ' …'
  code.append(preview, ellipsis)
  return code
}

function appendRoseRangeInline(
  target: HTMLElement,
  document: KyDocument,
  range: KyRoseAstIdxRangeWire,
  context: RenderContext,
): void {
  const scratch = target.ownerDocument.createElement('span')
  renderRoseRange(scratch, document, range, context)
  while (scratch.firstChild !== null) {
    const child = scratch.firstChild
    if (child.nodeName === 'P') {
      while (child.firstChild !== null) target.append(child.firstChild)
      child.remove()
    } else target.append(child)
  }
}

function renderList(
  tag: 'ul' | 'ol',
  document: KyDocument,
  range: KyRoseAstIdxRangeWire,
  ownerDocument: Document,
  context: RenderContext,
): HTMLUListElement | HTMLOListElement {
  const list = ownerDocument.createElement(tag)
  let itemStart = range.start
  while (itemStart < range.end) {
    const nextItem = findNextItem(document, itemStart + 1, range.end)
    const item = ownerDocument.createElement('li')
    renderRoseRange(item, document, { start: itemStart + 1, end: nextItem }, context)
    list.append(item)
    itemStart = nextItem
  }
  return list
}

function environmentHeading(name: string): string {
  return name.charAt(0).toUpperCase() + name.slice(1)
}

function findNextItem(document: KyDocument, start: number, end: number): number {
  for (let index = start; index < end; index += 1) {
    if (document.arena.rose[index]?.kind === 'item') return index
  }
  return end
}

/** A profile-specific interpretation of ordinary verbatim text, outside Kathy. */
export function parseSourceReference(body: string): { path: string; line: number; endLine: number; oid?: string } | null {
  const match = body.match(/^([A-Za-z0-9_./-]+)#L([1-9][0-9]*)-L([1-9][0-9]*)(?:@([0-9a-fA-F]{7,64}))?$/)
  if (match == null) return null
  const [, path, start, end, oid] = match
  if (path!.split('/').some((part) => part === '' || part === '.' || part === '..')) return null
  const line = Number(start), endLine = Number(end)
  if (!Number.isSafeInteger(line) || !Number.isSafeInteger(endLine) || endLine < line) return null
  return { path: path!, line, endLine, oid }
}
