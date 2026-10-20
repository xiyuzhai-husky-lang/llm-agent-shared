// The rendered-span excerpt window. `buildSpanWindow` writes its complete DOM (header, icon utilities, the
// scrollable viewport, the resize handle and the closed fullscreen dialog); `attachSpanWindow` gives that DOM its
// behaviour without rebuilding it. DOM only: no React, no math backend.

import type { KyCodespanSource } from '@kathy/ast'

const SVG = 'http://www.w3.org/2000/svg'
const icons = {
  previous: 'M15 5l-7 7 7 7',
  next: 'M9 5l7 7-7 7',
  expand: 'M14 4h6v6m0-6-7 7M10 20H4v-6m0 6 7-7',
  collapse: 'M20 4l-7 7m0-6v6h6M4 20l7-7m-6 0h6v6',
  fullscreen: 'M9 4H4v5m11-5h5v5M4 15v5h5m6 0h5v-5',
  close: 'M6 6l12 12M18 6 6 18',
}
type Icon = keyof typeof icons
type Action = 'previous' | 'next' | 'expand' | 'fullscreen' | 'close'

function setIcon(control: HTMLButtonElement, icon: Icon, label: string): void {
  const owner = control.ownerDocument
  const svg = owner.createElementNS(SVG, 'svg')
  svg.setAttribute('viewBox', '0 0 24 24')
  svg.setAttribute('width', '18')
  svg.setAttribute('height', '18')
  svg.setAttribute('fill', 'none')
  svg.setAttribute('stroke', 'currentColor')
  svg.setAttribute('stroke-width', '1.8')
  svg.setAttribute('stroke-linecap', 'round')
  svg.setAttribute('stroke-linejoin', 'round')
  svg.setAttribute('aria-hidden', 'true')
  svg.setAttribute('focusable', 'false')
  const shape = owner.createElementNS(SVG, 'path')
  shape.setAttribute('d', icons[icon])
  svg.append(shape)
  control.replaceChildren(svg)
  control.title = label
  control.setAttribute('aria-label', label)
}

/** Wrap `viewport` (already holding the excerpt and its labels) in the window DOM inside `element`. */
export function buildSpanWindow(element: HTMLElement, viewport: HTMLElement, source: KyCodespanSource, annotationCount: number): void {
  const owner = element.ownerDocument
  const frame = owner.createElement('div')
  frame.className = 'rs-window'
  const header = owner.createElement('div')
  header.className = 'rs-header'
  const path = owner.createElement('span')
  path.className = 'rs-path'
  path.textContent = `${source.path}:L${source.line}–L${source.end_line}`
  const utilities = owner.createElement('div')
  utilities.className = 'rs-utilities'
  const button = (action: Action, icon: Icon, label: string): HTMLButtonElement => {
    const control = owner.createElement('button')
    control.type = 'button'
    control.dataset.rsAction = action
    setIcon(control, icon, label)
    utilities.append(control)
    return control
  }
  if (annotationCount >= 2) {
    button('previous', 'previous', 'Previous span')
    button('next', 'next', 'Next span')
  }
  button('expand', 'expand', 'Expand excerpt').setAttribute('aria-expanded', 'false')
  button('fullscreen', 'fullscreen', 'Fullscreen excerpt')
  button('close', 'close', 'Close fullscreen').hidden = true
  const dialog = owner.createElement('dialog')
  dialog.className = 'rs-dialog'
  dialog.setAttribute('aria-label', path.textContent)
  const resize = owner.createElement('div')
  resize.className = 'rs-resize'
  resize.title = 'Drag to resize excerpt'
  resize.setAttribute('role', 'separator')
  resize.setAttribute('aria-label', 'Resize excerpt')
  resize.setAttribute('aria-orientation', 'horizontal')
  resize.tabIndex = 0
  resize.setAttribute('aria-valuemin', '120')
  viewport.tabIndex = 0
  viewport.setAttribute('role', 'region')
  viewport.setAttribute('aria-label', `Excerpt from ${path.textContent}`)
  element.dataset.rsAnnotations = String(annotationCount)
  header.append(path, utilities)
  frame.append(header, viewport, resize)
  element.append(frame, dialog)
}

/** Keep the excerpt's nodes under the same block, including while in the dialog's top layer. Scrolls the viewport to
 * the first span at once (callers attach before the first paint). Returns a disposer. */
export function attachSpanWindow(element: HTMLElement): () => void {
  const owner = element.ownerDocument
  const view = owner.defaultView!
  const frame = element.querySelector<HTMLElement>(':scope > .rs-window')!
  const viewport = frame.querySelector<HTMLElement>(':scope > .rs-viewport')!
  const resize = frame.querySelector<HTMLElement>(':scope > .rs-resize')!
  const dialog = element.querySelector<HTMLDialogElement>(':scope > dialog.rs-dialog')!
  const control = (action: Action) => frame.querySelector<HTMLButtonElement>(`:scope > .rs-header > .rs-utilities > button[data-rs-action="${action}"]`)
  const annotationCount = Number(element.dataset.rsAnnotations)
  const listeners: Array<() => void> = []
  const on = <K extends keyof HTMLElementEventMap>(target: HTMLElement, type: K, listener: (event: HTMLElementEventMap[K]) => void): void => {
    target.addEventListener(type, listener)
    listeners.push(() => target.removeEventListener(type, listener))
  }
  let expanded = false
  let current = 0
  let savedScroll = { top: 0, left: 0 }
  let viewportMarks: HTMLElement[] = []
  const clearViewportMark = (): void => {
    viewportMarks.forEach((block) => block.classList.remove('ky-rs-viewport-mark', 'ky-rs-viewport-mark-start'))
    viewportMarks = []
  }
  const markViewport = (): void => {
    clearViewportMark()
    const start = viewport.scrollTop
    const end = start + viewport.clientHeight
    if (end <= start) return
    const origin = viewport.getBoundingClientRect().top + viewport.clientTop
    // Select content blocks inside structural containers (proofs, quotes, lists),
    // while treating math and embedded widgets as single blocks.
    const selector = 'p, h2, h3, .ky-math-display, .ky-verbatim, .ky-diagram, .ky-source, .ky-input, .ky-rendered-span, .ky-quote-caption, .rs-label'
    viewportMarks = Array.from(viewport.querySelectorAll<HTMLElement>(selector)).filter((block) => {
      const enclosing = block.parentElement?.closest(selector)
      if (enclosing && viewport.contains(enclosing)) return false
      const bounds = block.getBoundingClientRect()
      const top = bounds.top - origin + start
      const bottom = bounds.bottom - origin + start
      return bounds.height > 0 && bottom > start && top < end
    })
    // Measure every block before adding styles; retain these nodes across reflow.
    viewportMarks.forEach((block) => block.classList.add('ky-rs-viewport-mark'))
    viewportMarks[0]?.classList.add('ky-rs-viewport-mark-start')
  }
  let emphasized: HTMLElement[] = []
  let emphasisTimer: number | undefined
  const clearEmphasis = (): void => {
    view.clearTimeout(emphasisTimer)
    emphasized.forEach((mark) => mark.classList.remove('rs-span-pulse'))
    emphasized = []
  }
  const scrollToSpan = (emphasize = false): void => {
    const marks = Array.from(viewport.querySelectorAll<HTMLElement>(`.rs-span[data-rs-annotation="${current}"]`))
      .filter((candidate) => candidate.closest('.ky-rendered-span') === element)
    const mark = marks[0]
    if (!mark) return
    if (expanded && !dialog.open) { mark.scrollIntoView({ block: 'center', inline: 'nearest' }); return }
    const bounds = viewport.getBoundingClientRect()
    const target = mark.getBoundingClientRect()
    viewport.scrollTop = Math.max(0, viewport.scrollTop + target.top - bounds.top - viewport.clientHeight / 3)
    if (target.left < bounds.left || target.right > bounds.right) {
      viewport.scrollLeft = Math.max(0, viewport.scrollLeft + target.left - bounds.left - 12)
    }
    if (emphasize) {
      clearEmphasis()
      emphasized = marks
      emphasized.forEach((target) => target.classList.add('rs-span-pulse'))
      emphasisTimer = view.setTimeout(clearEmphasis, 1200)
    }
  }
  const navigate = (step: number): void => { current = (current + step + annotationCount) % annotationCount; scrollToSpan(dialog.open) }
  const previous = control('previous')
  const next = control('next')
  if (previous) on(previous, 'click', () => navigate(-1))
  if (next) on(next, 'click', () => navigate(1))
  const expand = control('expand')!
  const fullscreen = control('fullscreen')!
  const close = control('close')!
  on(expand, 'click', () => {
    if (!expanded) markViewport()
    else clearViewportMark()
    expanded = !expanded
    frame.classList.toggle('rs-expanded', expanded)
    setIcon(expand, expanded ? 'collapse' : 'expand', `${expanded ? 'Collapse' : 'Expand'} excerpt`)
    expand.setAttribute('aria-expanded', String(expanded))
    if (!expanded) scrollToSpan()
  })
  on(fullscreen, 'click', () => {
    if (dialog.open) return
    if (!expanded) markViewport()
    savedScroll = { top: viewport.scrollTop, left: viewport.scrollLeft }
    dialog.append(frame)
    expand.disabled = true
    fullscreen.hidden = true
    close.hidden = false
    dialog.showModal()
    scrollToSpan(true)
    close.focus({ preventScroll: true })
  })
  on(close, 'click', () => dialog.close())
  on(dialog, 'close', () => {
    clearEmphasis()
    if (!expanded) clearViewportMark()
    element.prepend(frame)
    expand.disabled = false
    fullscreen.hidden = false
    close.hidden = true
    viewport.scrollTop = savedScroll.top
    viewport.scrollLeft = savedScroll.left
    fullscreen.focus({ preventScroll: true })
  })
  let drag: { id: number; y: number; height: number } | undefined
  const setHeight = (height: number): void => {
    const pixels = Math.round(Math.max(120, height))
    frame.style.setProperty('--rs-height', `${pixels}px`)
    resize.setAttribute('aria-valuenow', String(pixels))
  }
  on(resize, 'pointerdown', (event) => {
    if (event.pointerType === 'touch' || event.button !== 0) return
    event.preventDefault()
    drag = { id: event.pointerId, y: event.clientY, height: viewport.getBoundingClientRect().height }
    resize.setPointerCapture(event.pointerId)
  })
  on(resize, 'pointermove', (event) => {
    if (drag?.id === event.pointerId) setHeight(drag.height + event.clientY - drag.y)
  })
  const endDrag = (): void => {
    if (drag && resize.hasPointerCapture(drag.id)) resize.releasePointerCapture(drag.id)
    drag = undefined
  }
  on(resize, 'pointerup', endDrag)
  on(resize, 'pointercancel', endDrag)
  on(resize, 'lostpointercapture', () => { drag = undefined })
  on(resize, 'keydown', (event) => {
    if (event.key !== 'ArrowUp' && event.key !== 'ArrowDown') return
    event.preventDefault()
    setHeight(viewport.getBoundingClientRect().height + (event.key === 'ArrowUp' ? -32 : 32))
  })
  if (element.isConnected) scrollToSpan()
  return () => {
    if (dialog.open) dialog.close()
    for (const remove of listeners) remove()
    clearViewportMark()
    clearEmphasis()
    endDrag()
  }
}
