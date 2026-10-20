import { randomUUID } from 'node:crypto'

export interface PeekUiResult { error?: string }
export interface PeekUiEvent { type: 'ui'; id: string; command: string }

// A tool call belongs to one live response stream. Nothing is persisted or
// replayed when a page reconnects; only that stream's client receives its token.
export class PeekUiBridge {
  private readonly pending = new Map<string, (result: PeekUiResult) => void>()
  private closed = false

  private readonly send: (event: PeekUiEvent) => void

  constructor(send: (event: PeekUiEvent) => void) { this.send = send }

  request(command: string): Promise<PeekUiResult> {
    if (this.closed) return Promise.resolve({ error: 'UI disconnected.' })
    return new Promise((resolve) => {
      const id = randomUUID()
      const timer = setTimeout(() => this.reply(id, { error: 'UI did not respond.' }), 15_000)
      this.pending.set(id, (result) => {
        clearTimeout(timer)
        resolve(result)
      })
      try { this.send({ type: 'ui', id, command }) }
      catch { this.reply(id, { error: 'UI disconnected.' }) }
    })
  }

  reply(id: string, result: PeekUiResult): boolean {
    const resolve = this.pending.get(id)
    if (resolve == null) return false
    this.pending.delete(id)
    resolve(result)
    return true
  }

  close(): void {
    this.closed = true
    for (const id of this.pending.keys()) this.reply(id, { error: 'UI disconnected.' })
  }
}
