import { EventEmitter } from 'node:events'
import { PassThrough } from 'node:stream'
import { createInterface } from 'node:readline'
import type { StartPeekProcess } from '../node/middleware'

export class PeekProcessFixture extends EventEmitter {
  stdin = new PassThrough()
  stdout = new PassThrough()
  stderr = new PassThrough()
  requests: Array<{ method?: string; params?: Record<string, unknown> }> = []
  constructor(answer = 'Saved answer') {
    super()
    const lines = createInterface({ input: this.stdin })
    lines.on('line', line => {
      const request = JSON.parse(line)
      this.requests.push(request)
      queueMicrotask(() => {
        const send = (value: unknown) => this.stdout.write(`${JSON.stringify(value)}\n`)
        if (request.method === 'initialize') send({ id: request.id, result: {} })
        if (request.method === 'thread/start') send({ id: request.id, result: { model: request.params.model, thread: { id: 'thread' } } })
        if (request.method === 'turn/start') {
          send({ id: request.id, result: { turn: { id: 'turn' } } })
          send({ method: 'item/agentMessage/delta', params: { threadId: 'thread', turnId: 'turn', delta: answer } })
          send({ method: 'turn/completed', params: { threadId: 'thread', turn: { id: 'turn', status: 'completed' } } })
        }
      })
    })
  }
  kill() { this.stdin.end(); this.stdout.end(); this.stderr.end(); return true }
  start: StartPeekProcess = () => this as unknown as ReturnType<StartPeekProcess>
}
