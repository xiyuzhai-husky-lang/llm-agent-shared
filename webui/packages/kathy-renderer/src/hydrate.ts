// Hydration: give a built `.ky-document` (build.ts, in this browser or on a server) its behaviour, without replacing or
// re-rendering any of it. DOM only; safe to run before the first paint. Returns a disposer.
//
//   - printed data clipped to a preview opens its full text in a dialog;
//   - a second press on the chosen option of a choice clears it (and fires `change` for whoever stores answers);
//   - each rendered-span window navigates, expands, goes fullscreen and resizes (span-window.ts);
//   - a source frame toggles between the two states the build rendered (the live one and its <template>).

import { attachSpanWindow } from './span-window.ts'

export function hydrateKyDocument(root: HTMLElement): () => void {
  const owner = root.ownerDocument
  const disposers: Array<() => void> = []
  const on = <K extends keyof HTMLElementEventMap>(type: K, listener: (event: HTMLElementEventMap[K]) => void): void => {
    root.addEventListener(type, listener)
    disposers.push(() => root.removeEventListener(type, listener))
  }

  // Printed data: open the clipped text in full.
  const longVerbatim = (event: Event) => (event.target as Element | null)?.closest?.<HTMLElement>('.ky-verbatim-inline-long[data-full]') ?? null
  on('click', (event) => {
    const code = longVerbatim(event)
    if (code && root.contains(code)) openVerbatimDialog(owner, code.getAttribute('data-full')!)
  })
  on('keydown', (event) => {
    const code = longVerbatim(event)
    if (!code || !root.contains(code) || (event.key !== 'Enter' && event.key !== ' ')) return
    event.preventDefault()
    openVerbatimDialog(owner, code.getAttribute('data-full')!)
  })

  // Choices: a radio group cannot otherwise be emptied. The press usually lands on the label (touch) rather than the
  // radio itself, so remember the state on the label's pointerdown/keydown; the radio's click then decides.
  const wasChecked = new WeakMap<HTMLInputElement, boolean>()
  const optionRadio = (event: Event) => {
    const label = (event.target as Element | null)?.closest?.('.ky-input-option')
    return label && root.contains(label) ? label.querySelector<HTMLInputElement>(':scope > input[type=radio]') : null
  }
  const remember = (event: Event) => { const radio = optionRadio(event); if (radio) wasChecked.set(radio, radio.checked) }
  on('pointerdown', remember)
  on('keydown', remember)
  on('click', (event) => {
    const radio = event.target
    if (!(radio instanceof owner.defaultView!.HTMLInputElement) || radio.type !== 'radio' || !radio.parentElement?.matches('.ky-input-option')) return
    if (wasChecked.get(radio) === true) {
      radio.checked = false
      radio.dispatchEvent(new Event('change', { bubbles: true }))
    }
    wasChecked.delete(radio)
  })

  // Source frames: swap the live frame with the other state kept in its template.
  on('click', (event) => {
    const button = (event.target as Element | null)?.closest?.('button')
    const frame = button?.parentElement
    const host = frame?.parentElement
    if (!button || !frame?.matches('.ks-frame') || !host?.matches('.ky-source') || !root.contains(host)) return
    const alternate = host.querySelector<HTMLTemplateElement>(':scope > template[data-ks-alternate]')
    const next = alternate?.content.firstElementChild
    if (!alternate || !next) return
    host.replaceChild(next, frame)
    alternate.content.replaceChildren(frame)
    next.querySelector<HTMLElement>(':scope > button')?.focus({ preventScroll: true })
  })

  for (const span of root.querySelectorAll<HTMLElement>('.ky-rendered-span')) disposers.push(attachSpanWindow(span))

  return () => { for (const dispose of disposers.splice(0)) dispose() }
}

function openVerbatimDialog(owner: Document, body: string): void {
  const dialog = owner.createElement('dialog')
  dialog.className = 'ky-verbatim-dialog'
  const pre = owner.createElement('pre')
  pre.className = 'ky-verbatim ky-verbatim-dialog-body'
  const full = owner.createElement('code')
  full.textContent = body
  pre.append(full)
  const close = owner.createElement('button')
  close.type = 'button'
  close.className = 'ky-verbatim-dialog-close'
  close.textContent = '关闭'
  const dismiss = (): void => {
    if (typeof dialog.close === 'function') dialog.close()
    else dialog.remove()
  }
  close.addEventListener('click', dismiss)
  dialog.addEventListener('click', (event) => { if (event.target === dialog) dismiss() })
  dialog.addEventListener('close', () => dialog.remove())
  dialog.append(pre, close)
  owner.body.append(dialog)
  if (typeof dialog.showModal === 'function') dialog.showModal()
  else dialog.setAttribute('open', '')
}
