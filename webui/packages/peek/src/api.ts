import type { PeekModelCatalog, PeekModelSettings } from '../node/models'
import type { PeekChatSummary, SavedPeekChat } from '../node/chat'

export type { PeekChatSummary, PeekModelCatalog, PeekModelSettings }

export interface PeekMessage {
  role: 'assistant' | 'user'
  text: string
  tab?: string
  interrupted?: boolean
}

export type PeekChat = SavedPeekChat

export interface PeekRequest {
  threadId: string
  candidate: string
  folder: string
  messages: PeekMessage[]
  tab: string
}

export interface PeekApi {
  steer(input: Omit<PeekRequest, 'messages'> & { text: string }, signal?: AbortSignal): Promise<void>
  models(signal?: AbortSignal): Promise<PeekModelCatalog>
  configure(folder: string, threadId: string, settings: PeekModelSettings, signal?: AbortSignal): Promise<PeekChat>
  list(folder: string, signal?: AbortSignal): Promise<PeekChatSummary[]>
  create(folder: string, candidate: string, signal?: AbortSignal): Promise<PeekChat>
  load(folder: string, threadId: string, signal?: AbortSignal): Promise<PeekChat>
  stream(input: PeekRequest, onDelta: (text: string) => void, signal?: AbortSignal, onUi?: (command: string) => void | Promise<void>, onSteered?: (message: PeekMessage) => void): Promise<void>
}

export type FetchLike = (input: string, init?: RequestInit) => Promise<Response>

export function peekApi(route: string, fetchLike: FetchLike = (input, init) => fetch(input, init)): PeekApi {
  async function readJson(url: string, init: RequestInit): Promise<unknown> {
    const response = await fetchLike(url, init)
    const payload: unknown = await response.json()
    if (!response.ok) throw new Error(errorMessage(payload) ?? `Chat request failed with status ${response.status}.`)
    return payload
  }
  return {
    async steer(input, signal) {
      await readJson(route, { method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action: 'steer', ...input }), signal })
    },
    async models(signal) {
      const payload = await readJson(`${route}?action=models`, { signal })
      if (typeof payload !== 'object' || payload == null || !('models' in payload) || !Array.isArray(payload.models)
        || !('defaults' in payload) || typeof payload.defaults !== 'object' || payload.defaults == null
        || !('model' in payload.defaults) || typeof payload.defaults.model !== 'string'
        || !('reasoningEffort' in payload.defaults) || typeof payload.defaults.reasoningEffort !== 'string') {
        throw new Error('Peek returned invalid model settings.')
      }
      return payload as PeekModelCatalog
    },
    async configure(folder, threadId, settings, signal) {
      return parseChat(await readJson(route, {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action: 'configure', folder, threadId, ...settings }), signal,
      }))
    },
    async list(folder, signal) {
      const payload = await readJson(`${route}?${new URLSearchParams({ folder })}`, { signal })
      if (typeof payload !== 'object' || payload == null || !('threads' in payload) || !Array.isArray(payload.threads)
        || !payload.threads.every((thread: unknown) => typeof thread === 'object' && thread != null
          && 'id' in thread && typeof thread.id === 'string'
          && 'candidate' in thread && typeof thread.candidate === 'string'
          && 'title' in thread && typeof thread.title === 'string'
          && 'updatedAt' in thread && typeof thread.updatedAt === 'string')) {
        throw new Error('Peek returned an invalid chat list.')
      }
      return payload.threads as PeekChatSummary[]
    },
    async create(folder, candidate, signal) {
      return parseChat(await readJson(route, {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action: 'create', folder, candidate }), signal,
      }))
    },
    async load(folder, threadId, signal) {
      return parseChat(await readJson(`${route}?${new URLSearchParams({ folder, threadId })}`, { signal }))
    },
    async stream(input, onDelta, signal, onUi, onSteered) {
      const response = await fetchLike(route, {
        body: JSON.stringify({ ...input, ...(onUi == null ? {} : { ui: true }) }),
        headers: { 'Content-Type': 'application/json' },
        method: 'POST',
        signal,
      })
      if (!response.ok) {
        const payload: unknown = await response.json().catch(() => undefined)
        throw new Error(errorMessage(payload) ?? `Peek request failed with status ${response.status}.`)
      }
      if (response.body == null) throw new Error('Peek response did not include a stream.')
      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      let done = false
      const executeUi = async (id: string, command: string) => {
        let error: string | undefined
        signal?.throwIfAborted()
        try {
          if (onUi == null) throw new Error('UI is unavailable.')
          await onUi(command)
        } catch (reason) { error = (reason instanceof Error ? reason.message : String(reason)).slice(0, 500) }
        signal?.throwIfAborted()
        await readJson(route, {
          method: 'POST', headers: { 'Content-Type': 'application/json' }, signal,
          body: JSON.stringify({ action: 'ui-result', folder: input.folder, threadId: input.threadId, id, error }),
        })
      }
      try {
        while (true) {
          const next = await reader.read()
          buffer += decoder.decode(next.value, { stream: !next.done })
          const lines = buffer.split('\n')
          buffer = lines.pop() ?? ''
          for (const line of lines) done = await consumeEvent(line, onDelta, executeUi, onSteered) || done
          if (next.done) break
        }
        if (buffer.trim().length > 0) done = await consumeEvent(buffer, onDelta, executeUi, onSteered) || done
        if (!done) throw new Error('Peek stream ended before the reply completed.')
      } catch (error) {
        await reader.cancel().catch(() => undefined)
        throw error
      } finally { reader.releaseLock() }
    },
  }
}

function parseChat(payload: unknown): PeekChat {
  if (typeof payload !== 'object' || payload == null || !('messages' in payload) || !Array.isArray(payload.messages)
    || !('id' in payload) || typeof payload.id !== 'string'
    || !('candidate' in payload) || typeof payload.candidate !== 'string'
    || !('folder' in payload) || typeof payload.folder !== 'string') {
    throw new Error('Peek returned an invalid saved chat.')
  }
  return payload as PeekChat
}

async function consumeEvent(line: string, onDelta: (text: string) => void, onUi: (id: string, command: string) => Promise<void>, onSteered?: (message: PeekMessage) => void): Promise<boolean> {
  if (line.trim().length === 0) return false
  let event: unknown
  try {
    event = JSON.parse(line)
  } catch {
    throw new Error('Peek returned malformed streaming data.')
  }
  if (typeof event !== 'object' || event == null || Array.isArray(event)) {
    throw new Error('Peek returned an unexpected streaming event.')
  }
  const { command, id, message, text, type } = event as Record<string, unknown>
  if (type === 'steered') {
    if (typeof message !== 'object' || message == null || !('text' in message) || typeof message.text !== 'string') throw new Error('Invalid steering event.')
    onSteered?.(message as PeekMessage)
    return false
  }
  if (type === 'ui') {
    if (typeof id !== 'string' || typeof command !== 'string') throw new Error('Invalid UI command.')
    await onUi(id, command)
    return false
  }
  if (type === 'delta' && typeof text === 'string') {
    onDelta(text)
    return false
  }
  if (type === 'done') return true
  if (type === 'error') {
    throw new Error(typeof message === 'string' ? message : 'Peek failed.')
  }
  return false
}

function errorMessage(payload: unknown): string | undefined {
  return typeof payload === 'object' && payload != null && 'error' in payload
    && typeof payload.error === 'string'
    ? payload.error
    : undefined
}
