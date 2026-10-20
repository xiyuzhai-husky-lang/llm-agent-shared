import { PeekConnection } from './reviewer-peek'

type Request = Record<string, unknown>
interface Frame { epoch: string; cursor: number; reset: boolean; puts: Record<string, unknown> }
class HttpError extends Error { readonly status: number; constructor(status: number, message: string) { super(message); this.status = status } }
export async function json<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(url, init)
  if (!response.ok) throw new HttpError(response.status, await response.text())
  return await response.json() as T
}

/** Resolve the workflow's UID links before mounting the original numeric-key UI. */
export async function resolveDecisionLocation(): Promise<void> {
  const query = new URLSearchParams(location.search), proposal = query.get('proposal')
  if (proposal && (!/^\d+$/.test(proposal) || !query.has('candidate'))) {
    const resolved = await json<{ number: number; candidate: string }>('/api/reviewer/resolve?' + new URLSearchParams({ proposal }))
    query.set('proposal', String(resolved.number))
    if (!query.has('candidate')) query.set('candidate', resolved.candidate)
    history.replaceState(null, '', '?' + query)
  }
}

/** Transport for the existing reviewer components. The Go server owns projections,
 * navigation targets and decision gates. Browser work is limited to transport and UI state. */
export class DecisionConnection {
  private readonly abort = new AbortController()
  private readonly requests = new Map<string, Request>()
  private readonly signPresses = new Map<string, number>()
  private readonly peek = new PeekConnection((request, view) => this.deliver(request, view))
  private receive: (text: string) => void
  private fail: (message: string) => void
  constructor(_address: string, receive: (text: string) => void, fail: (message: string) => void) {
    this.receive = receive; this.fail = fail
    void this.watch()
    void this.watchWorkflow()
  }
  private deliver(request: Request, view: unknown) {
    if (this.abort.signal.aborted) return
    if ('DiffTab' in request) view = { ...(view as object), sign_presses: this.signPresses.get(JSON.stringify(request)) ?? 0 }
    this.receive(JSON.stringify({ View: [request, { [Object.keys(request)[0]!]: view }] }))
  }
  private refused(request: Request, error: unknown) {
    if (!this.abort.signal.aborted) this.receive(JSON.stringify({ Refused: [request, String(error)] }))
  }
  subscribe(requests: string[]) { const wanted = new Set(requests); for (const key of this.requests.keys()) if (!wanted.has(key)) this.requests.delete(key) }
  read(text: string) {
    const request = JSON.parse(text) as Request
    this.requests.set(text, request)
    void this.refresh(request)
  }
  private async refresh(request: Request) {
    try {
      if ('PeekPanel' in request) { await this.peek.read(request); return }
      const view = await json('/api/reviewer?' + new URLSearchParams({ request: JSON.stringify(request) }), { signal: this.abort.signal })
      this.deliver(request, view)
      if ('DiffTab' in request && !(view as { opened: boolean }).opened) {
        await this.command(request, { DiffTab: { OpenDiff: {} } }, false)
      }
    } catch (error) { this.refused(request, error) }
  }
  act(text: string, actionText: string) {
    const request = JSON.parse(text) as Request, action = JSON.parse(actionText) as Record<string, unknown>
    if ('PeekPanel' in request) {
      void this.peek.act(request, action.PeekPanel).catch(error => this.refused(request, error)); return
    }
    const diff = action.DiffTab as { SendToPeek?: { path: string; hunk: string; text: string } } | undefined
    if (diff?.SendToPeek) {
      void this.peek.queue(request, diff.SendToPeek).then(() => this.refresh(request)).catch(error => this.refused(request, error)); return
    }
    void this.command(request, action, true).catch(error => this.refused(request, error))
  }
  private async command(request: Request, action: unknown, deliver: boolean) {
    const payload = { id: crypto.randomUUID(), request, action }
    // Preserve the exact command ID when an HTTP outcome is uncertain. An explicit
    // next attempt retries the pending request before submitting a different action.
    const storageKey = 'proposal-reviewer-pending'
    const pending = deliver ? sessionStorage.getItem(storageKey) : null
    if (pending) {
      try { await json('/api/reviewer/actions', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: pending, signal: this.abort.signal }) } catch (error) { if (!(error instanceof HttpError && error.status >= 400 && error.status < 500)) throw error }
      sessionStorage.removeItem(storageKey)
    }
    const body = JSON.stringify(payload)
    if (deliver) sessionStorage.setItem(storageKey, body)
    try {
      const view = await json('/api/reviewer/actions', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body, signal: this.abort.signal })
      if (deliver) {
        const diff = (action as { DiffTab?: { Sign?: unknown; SignAllUnder?: unknown } }).DiffTab
        if (diff && ('Sign' in diff || 'SignAllUnder' in diff)) {
          const key = JSON.stringify(request)
          this.signPresses.set(key, (this.signPresses.get(key) ?? 0) + 1)
        }
        sessionStorage.removeItem(storageKey); this.deliver(request, view)
      }
    } catch (error) {
      // A definite server refusal is safe to retry as a new command. Transport
      // failures retain their original ID, which the journal deduplicates.
      if (deliver && error instanceof HttpError && error.status >= 400 && error.status < 500) sessionStorage.removeItem(storageKey)
      throw error
    }
  }
  private async watch() {
    let epoch = '', cursor = 0
    while (!this.abort.signal.aborted) {
      try {
        const query = new URLSearchParams({ prefix: 'revision/', epoch, after: String(cursor), wait: '20' })
        query.append('prefix', 'proposal/'); query.append('prefix', 'entry/')
        const frame = await json<Frame>('/api/views?' + query, { signal: this.abort.signal })
        const changed = epoch !== '' && (frame.reset || Object.keys(frame.puts).length > 0)
        epoch = frame.epoch; cursor = frame.cursor
        if (changed) {
          for (const request of this.requests.values()) {
            const component = Object.keys(request)[0]!
            if (['TitleBar', 'OverviewSidebar', 'BatchStackPage', 'TabStrip', 'DiffTab', 'DoubtsTab', 'ReviewPanel', 'PeekPanel'].includes(component)) void this.refresh(request)
          }
        }
      } catch (error) {
        if (!this.abort.signal.aborted) this.fail(String(error))
        return
      }
    }
  }
  private async watchWorkflow() {
    let epoch = '', cursor = 0
    while (!this.abort.signal.aborted) {
      try {
        const response = await fetch('/api/reviewer/workflow-events?' + new URLSearchParams({ epoch, after: String(cursor) }), { signal: this.abort.signal })
        if (response.status === 204) return
        if (!response.ok) throw new Error(await response.text())
        const frame = await response.json() as Frame
        const changed = epoch !== '' && (frame.reset || Object.keys(frame.puts).length > 0)
        epoch = frame.epoch; cursor = frame.cursor
        if (changed) for (const request of this.requests.values()) {
          if (['OverviewSidebar', 'BatchStackPage', 'WorkflowTab'].includes(Object.keys(request)[0]!)) void this.refresh(request)
        }
      } catch (error) { if (!this.abort.signal.aborted) this.fail(String(error)); return }
    }
  }
  free() { this.abort.abort(); this.peek.free() }
}
