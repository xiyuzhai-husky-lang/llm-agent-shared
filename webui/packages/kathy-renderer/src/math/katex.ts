// The math backend: the only module that knows KaTeX. It turns a Kathy math node into DOM inside a given
// element (any Document: the browser's or the server's); its stylesheet and fonts are `./styles.ts`.
// Replacing KaTeX replaces this module and `./styles.ts`, nothing else.

import type { KyDocument, KyRenderedSpanAnnotation, KyRoseAstIdxRangeWire } from '@kathy/ast'
import katex from 'katex'

import { markNode } from '../marks.ts'
import { serializeMathRange } from '../tex.ts'

const baseOptions = { throwOnError: true, strict: 'error' as const, output: 'html' as const }

/** A math environment node (align, cases, …) whose TeX is already serialized. */
export function renderMathEnvironment(element: HTMLElement, tex: string): void {
  element.innerHTML = katex.renderToString(tex, { ...baseOptions, displayMode: true, trust: false })
}

/** Inline or display math over `range`, with the rendered-span annotations that target its nodes marked. */
export function renderMathRange(element: HTMLElement, document: KyDocument, range: { start: number; end: number }, tex: string,
  displayMode: boolean, annotations: KyRenderedSpanAnnotation<KyRoseAstIdxRangeWire>[]): void {
  const applied = new Set<number>()
  const groups = mathDelimiterGroups(document, range)
  const expanded = annotations.map((a) => groups.filter((g) => a.targets.some((t) => t.kind === 'math' && g.delimiters.includes(t.index))))
  const decorated = serializeMathRange(document, range, (index, text) => {
    for (const [i, a] of annotations.entries()) {
      const group = expanded[i]?.find((g) => g.start === index || g.end === index)
      if (group) {
        applied.add(i)
        if (group.start === index) text = `\\htmlClass{rs-group-${i}}{${text}`
        if (group.end === index) text += '}'
        continue
      }
      if (expanded[i]?.some((g) => g.delimiters.includes(index))) continue
      if (!a.targets.some((t) => t.kind === 'math' && t.index === index)) continue
      applied.add(i)
      // Keep the required braces of a script or command argument outside the
      // KaTeX group wrapper. Every wrapper is parsed by KaTeX before acceptance.
      text = document.arena.math[index]?.kind === 'delimited' && text.startsWith('{') && text.endsWith('}')
        ? `{\\htmlClass{rs-group-${i}}{${text.slice(1, -1)}}}` : `\\htmlClass{rs-group-${i}}{${text}}`
    }
    return text
  })
  const options = { ...baseOptions, displayMode }
  if (!applied.size) { element.innerHTML = katex.renderToString(tex, { ...options, trust: false }); return }
  try {
    element.innerHTML = katex.renderToString(decorated, { ...options, strict: (code: string) => code === 'htmlExtension' ? 'ignore' : 'error',
      trust: (context) => context.command === '\\htmlClass' && /^rs-group-\d+$/.test(String(context.class)) })
    for (const i of applied) {
      const groups = element.querySelectorAll(`.rs-group-${i}`)
      if (!groups.length) throw new Error('No splittable KaTeX group')
      for (const group of groups) {
        const mark = markNode(element.ownerDocument, element.ownerDocument.createTextNode(''), annotations[i]!.severity ?? 'note', i)
        mark.replaceChildren(...Array.from(group.childNodes))
        mark.className += ` ${group.className}`
        group.replaceWith(mark)
      }
    }
  } catch {
    element.innerHTML = katex.renderToString(tex, { ...options, trust: false })
    for (const i of applied) {
      const mark = markNode(element.ownerDocument, element.ownerDocument.createTextNode(''), annotations[i]!.severity ?? 'note', i)
      mark.dataset.rsFallback = 'whole-formula'
      mark.replaceChildren(...Array.from(element.childNodes))
      element.append(mark)
    }
  }
}

// A scalable delimiter and its argument are one KaTeX leftright group. A mark
// touching either delimiter expands to that group, keeping surrounding math out.
function mathDelimiterGroups(document: KyDocument, range: { start: number; end: number }) {
  const groups: Array<{ start: number; end: number; delimiters: number[] }> = []
  const seen = new Set<number>()
  const boundary = (index: number): { side: 'left' | 'right'; ids: number[] } | undefined => {
    const node = document.arena.math[index]
    if (node?.kind === 'attach') {
      const base = boundary(node.base)
      return base ? { side: base.side, ids: [index, ...base.ids] } : undefined
    }
    if (node?.kind === 'lefted' || node?.kind === 'righted') return {
      side: node.kind === 'lefted' ? 'left' : 'right', ids: [index, node.argument],
    }
  }
  const visitRange = (r: { start: number; end: number }) => {
    const stack: Array<{ index: number; ids: number[] }> = []
    for (let index = r.start; index < r.end; index++) {
      const edge = boundary(index)
      if (edge?.side === 'left') stack.push({ index, ids: edge.ids })
      if (edge?.side === 'right') {
        const left = stack.pop()
        if (left) groups.push({ start: left.index, end: index, delimiters: [...left.ids, ...edge.ids] })
      }
      visitNode(index)
    }
  }
  const visitNode = (index: number) => {
    if (seen.has(index)) return
    seen.add(index)
    const node = document.arena.math[index]
    if (node?.kind === 'delimited') visitRange(node.asts)
    if (node?.kind === 'attach') {
      visitNode(node.base)
      for (const s of node.scripts) if (s.script.kind !== 'quotes') visitNode(s.script.ast)
    }
    if (node?.kind === 'complete_command') for (const a of node.arguments) {
      if (a.kind === 'math_ast') visitNode(a.ast)
      if ((a.kind === 'curled' || a.kind === 'optional') && a.data.kind === 'math') visitRange(a.data.range)
    }
    if (node?.kind === 'environment' && node.asts.kind === 'math') visitRange(node.asts.range)
  }
  visitRange(range)
  return groups
}
