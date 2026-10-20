import { spawn, type ChildProcessByStdio } from 'node:child_process'
import type { IncomingMessage, ServerResponse } from 'node:http'
import { createInterface } from 'node:readline'
import type { Readable, Writable } from 'node:stream'
import { PEEK_DEFAULT_SETTINGS, readPeekModels, type PeekModelCatalog } from './models.ts'
import { PeekUiBridge } from './ui.ts'
import { PEEK_CHAT_ID_PATTERN, readPeekChat, writePeekChat, type SavedPeekChat, type PeekChatStore } from './chat.ts'

export const MAX_PEEK_BODY_BYTES = 128 * 1024

const CANDIDATE_OID = /^[0-9a-f]{40,64}$/
export interface PeekMessage {
  role: 'assistant' | 'user'
  text: string
  tab?: string
}

export interface PeekRequest {
  threadId: string
  candidate: string
  folder: string
  messages: PeekMessage[]
  tab: string
  ui?: boolean
}

type PeekProcess = ChildProcessByStdio<Writable, Readable, Readable>
export type StartPeekProcess = (repoRoot: string) => PeekProcess
export type PeekTarget = Pick<PeekRequest, 'candidate' | 'folder'>
export type PeekTargetExists = (repoRoot: string, input: PeekTarget) => Promise<boolean>
interface ActivePeek {
  done: Promise<void>
  closing: boolean
  ui?: PeekUiBridge
  steer?: (text: string, tab: string, candidate: string) => Promise<void>
}

export interface PeekMiddlewareOptions {
  repoRoot: string
  route: string
  store: PeekChatStore
  parseFolder: (folder: unknown) => string
  tabs: ReadonlySet<string>
  invalidTab: string
  targetUnavailable: string
  candidateMismatch: string
  targetExists: PeekTargetExists
  prompt: (input: PeekRequest) => string | Promise<string>
  clientInfo: { name: string; title: string; version: string }
  uiTool?: unknown
  startProcess?: StartPeekProcess
  readModels?: () => Promise<PeekModelCatalog>
}

export function peekMiddleware({ repoRoot, route, store, parseFolder, tabs, invalidTab, targetUnavailable,
  candidateMismatch, targetExists, prompt, clientInfo, uiTool,
  startProcess = defaultStartProcess, readModels = () => readPeekModels(startProcess(repoRoot)),
}: PeekMiddlewareOptions) {
  const parseTarget = (body: Record<string, unknown>) => parsePeekTarget(body, parseFolder)
  let pendingModels: Promise<PeekModelCatalog> | undefined
  const models = () => {
    pendingModels ??= readModels().finally(() => { pendingModels = undefined })
    return pendingModels
  }
  const activeChats = new Map<string, ActivePeek>()
  return (request: IncomingMessage, response: ServerResponse, next: () => void) => {
    const url = new URL(request.url ?? '/', 'http://localhost')
    if (url.pathname !== route) {
      next()
      return
    }
    void handle(request, response).catch((reason: unknown) => {
      const missing = (reason as NodeJS.ErrnoException)?.code === 'ENOENT'
      sendJson(response, reason instanceof PeekHttpError ? reason.status : missing ? 404 : 500, {
        error: missing ? 'Peek chat was not found.' : reason instanceof Error ? reason.message : 'Peek request failed.',
      })
    })
  }

  async function handle(request: IncomingMessage, response: ServerResponse): Promise<void> {
    if (request.method === 'GET') {
      const params = new URL(request.url ?? '/', 'http://localhost').searchParams
      if (params.get('action') === 'models') {
        sendJson(response, 200, await models())
        return
      }
      const folder = parseFolder(params.get('folder'))
      if (!params.has('threadId')) {
        const threads = (await store.list(folder)).map((thread) => ({
          ...thread, running: activeChats.get(store.path(folder, thread.id))?.closing === false }))
        sendJson(response, 200, { folder, threads })
        return
      }
      const threadId = parseThreadId(params.get('threadId'))
      const path = store.path(folder, threadId)
      // A closing stream flushes before a freshly opened panel reads the file.
      const active = activeChats.get(path)
      if (active?.closing) await active.done
      sendJson(response, 200, await readPeekChat(path, folder, threadId))
      return
    }
    if (request.method !== 'POST') {
      response.setHeader('Allow', 'GET, POST')
      throw new PeekHttpError(405, `Method ${request.method ?? 'unknown'} is not allowed.`)
    }
    const body = parseObject(await readBody(request))
    if (body.action === 'steer') {
      const { folder, candidate } = parseTarget(body)
      const threadId = parseThreadId(body.threadId)
      if (typeof body.text !== 'string' || body.text.trim().length === 0
        || typeof body.tab !== 'string' || !tabs.has(body.tab)) throw new PeekHttpError(400, 'A message and visible tab are required.')
      const active = activeChats.get(store.path(folder, threadId))
      if (active?.closing || active?.steer == null) throw new PeekHttpError(409, '当前回复已结束或尚未就绪；补充指令未发送，请重试。')
      await active.steer(body.text, body.tab, candidate)
      sendJson(response, 200, {})
      return
    }
    if (body.action === 'ui-result') {
      const folder = parseFolder(body.folder)
      const threadId = parseThreadId(body.threadId)
      if (typeof body.id !== 'string' || (body.error != null && typeof body.error !== 'string')) throw new PeekHttpError(400, 'Invalid UI result.')
      const ui = activeChats.get(store.path(folder, threadId))?.ui
      if (!ui?.reply(body.id, { error: typeof body.error === 'string' ? body.error.slice(0, 500) : undefined })) throw new PeekHttpError(409, 'UI call is no longer active.')
      sendJson(response, 200, {})
      return
    }
    if (body.action === 'create') {
      const input = parseTarget(body)
      if (!await targetExists(repoRoot, input)) throw new PeekHttpError(404, targetUnavailable)
      sendJson(response, 201, await store.create(input.folder, input.candidate))
      return
    }
    if (body.action === 'configure') {
      const folder = parseFolder(body.folder)
      const threadId = parseThreadId(body.threadId)
      if (typeof body.model !== 'string' || typeof body.reasoningEffort !== 'string') throw new PeekHttpError(400, 'Select a model and reasoning effort.')
      const catalog = await models()
      const model = catalog.models.find((entry) => entry.model === body.model)
      if (model == null) throw new PeekHttpError(400, 'The selected model is not available.')
      if (!model.supportedReasoningEfforts.some((entry) => entry.reasoningEffort === body.reasoningEffort)) throw new PeekHttpError(400, 'The selected reasoning effort is not supported by this model.')
      const path = store.path(folder, threadId)
      if (activeChats.has(path)) throw new PeekHttpError(409, 'Wait for the current reply before changing its model.')
      let release!: () => void
      activeChats.set(path, { done: new Promise<void>((resolve) => { release = resolve }), closing: true })
      try {
        const chat = await readPeekChat(path, folder, threadId)
        sendJson(response, 200, await writePeekChat(path, { ...chat, model: body.model, reasoningEffort: body.reasoningEffort }))
      } finally {
        activeChats.delete(path)
        release()
      }
      return
    }
    const input = parseBody(body, parseFolder, tabs, invalidTab)
    if (!await targetExists(repoRoot, input)) {
      throw new PeekHttpError(404, targetUnavailable)
    }
    const path = store.path(input.folder, input.threadId)
    if (activeChats.has(path)) throw new PeekHttpError(409, 'A reply is already running for this chat.')
    let release!: () => void
    const active: ActivePeek = {
      done: new Promise<void>((resolve) => { release = resolve }), closing: false,
    }
    activeChats.set(path, active)
    try {
      const chat = await readPeekChat(path, input.folder, input.threadId)
      if (chat.candidate !== input.candidate) throw new PeekHttpError(409, candidateMismatch)
      const previous = input.messages.slice(0, -1)
      if (JSON.stringify(previous) !== JSON.stringify(chat.messages.map(({ role, text }) => ({ role, text })))) {
        throw new PeekHttpError(409, 'This chat changed. Reopen Peek to load its latest messages.')
      }
      chat.messages.push({ ...input.messages.at(-1)!, tab: input.tab })
      await writePeekChat(path, chat)
      if (response.destroyed) return
      const context = await prompt({ ...input, messages: chat.messages })
      const child = startProcess(repoRoot)
      response.statusCode = 200
      response.setHeader('Content-Type', 'application/x-ndjson; charset=utf-8')
      response.setHeader('Cache-Control', 'no-store')
      response.setHeader('X-Accel-Buffering', 'no')
      response.flushHeaders()
      if (input.ui) active.ui = new PeekUiBridge((event) => {
        if (!response.destroyed && !response.writableEnded) response.write(`${JSON.stringify(event)}\n`)
      })
      await streamPeek(response, child, repoRoot, { ...input, messages: chat.messages }, chat,
        async (next) => { await writePeekChat(path, next) }, () => { active.closing = true }, active, { prompt: context, clientInfo, uiTool, candidateMismatch })
    } finally {
      active.ui?.close()
      activeChats.delete(path)
      release()
    }
  }
}

function defaultStartProcess(repoRoot: string): PeekProcess {
  return spawn('codex', ['app-server', '--stdio'], {
    cwd: repoRoot,
    stdio: ['pipe', 'pipe', 'pipe'],
  })
}

async function streamPeek(
  response: ServerResponse,
  child: PeekProcess,
  repoRoot: string,
  input: PeekRequest,
  chat: SavedPeekChat,
  save: (chat: SavedPeekChat) => Promise<void>,
  onClosing: () => void,
  active: ActivePeek,
  context: { prompt: string; clientInfo: PeekMiddlewareOptions['clientInfo']; uiTool?: unknown; candidateMismatch: string },
): Promise<void> {
  const ui = active.ui
  const model = chat.model ?? PEEK_DEFAULT_SETTINGS.model
  const reasoningEffort = chat.reasoningEffort ?? PEEK_DEFAULT_SETTINGS.reasoningEffort
  const lines = createInterface({ input: child.stdout })
  let finished = false
  let threadId: string | undefined
  let turnId: string | undefined
  let stderr = ''
  let answer = ''
  let dirty = false
  let saveTail = Promise.resolve()
  let nextRequestId = 4
  const steering = new Map<number, { text: string; tab: string; resolve: () => void; reject: (reason: Error) => void }>()
  let resolveFinished!: () => void
  const completed = new Promise<void>((resolve) => { resolveFinished = resolve })
  const persist = (interrupted: boolean) => {
    const next: SavedPeekChat = {
      ...chat,
      messages: answer.trim().length === 0 ? [...chat.messages] : [
        ...chat.messages, { role: 'assistant', text: answer, tab: input.tab, interrupted },
      ],
    }
    saveTail = saveTail.then(() => save(next))
    return saveTail
  }
  const flushTimer = setInterval(() => {
    if (!dirty || finished) return
    dirty = false
    void persist(true).catch((reason: unknown) => finish({ type: 'error', message: `Could not save chat: ${describe(reason)}` }))
  }, 500)

  const writeEvent = (event: unknown) => {
    if (!response.writableEnded && !response.destroyed) response.write(`${JSON.stringify(event)}\n`)
  }
  const finish = (event?: unknown) => {
    if (finished) return
    finished = true
    for (const request of steering.values()) request.reject(new PeekHttpError(409, '回复已结束，补充指令未发送。'))
    steering.clear()
    onClosing()
    ui?.close()
    clearInterval(flushTimer)
    lines.close()
    child.kill()
    const success = typeof event === 'object' && event != null && 'type' in event && event.type === 'done'
    void persist(!success).then(() => {
      if (event != null) writeEvent(event)
    }).catch((reason: unknown) => {
      writeEvent({ type: 'error', message: `Could not save chat: ${describe(reason)}` })
    }).finally(() => {
      response.end()
      resolveFinished()
    })
  }
  const writeMessage = (message: unknown) => {
    if (finished) return
    try {
      child.stdin.write(`${JSON.stringify(message)}\n`)
    } catch (reason) {
      finish({ type: 'error', message: describe(reason) })
    }
  }
  const writeRequest = (id: number, method: string, params: unknown) => {
    writeMessage({ id, method, params })
  }
  active.steer = (text, tab, candidate) => {
    if (finished || threadId == null || turnId == null) return Promise.reject(new PeekHttpError(409, '当前回复已结束或尚未就绪；补充指令未发送，请重试。'))
    if (candidate !== chat.candidate) return Promise.reject(new PeekHttpError(409, context.candidateMismatch))
    return new Promise<void>((resolve, reject) => {
      const id = nextRequestId++
      steering.set(id, { text, tab, resolve, reject })
      writeRequest(id, 'turn/steer', { threadId, expectedTurnId: turnId,
        input: [{ type: 'text', text: `Visible tab: ${tab}\n\nUser:\n${text}`, text_elements: [] }] })
    })
  }

  response.on('close', () => {
    if (!finished) finish()
  })
  child.stderr.on('data', (chunk: Buffer | string) => {
    stderr = `${stderr}${chunk.toString()}`.slice(-8_192)
  })
  child.stdin.on('error', (error) => finish({ type: 'error', message: error.message }))

  lines.on('line', (line) => {
    if (finished || line.trim().length === 0) return
    let message: AppServerMessage
    try {
      message = JSON.parse(line) as AppServerMessage
    } catch {
      finish({ type: 'error', message: 'Codex returned malformed streaming data.' })
      return
    }

    if (message.method == null && typeof message.id === 'number' && steering.has(message.id)) {
      const request = steering.get(message.id)!
      steering.delete(message.id)
      if (message.error != null) request.reject(new PeekHttpError(409, appServerError(message.error)))
      else if (directString(message.result, 'turnId') !== turnId) request.reject(new PeekHttpError(409, 'Codex did not accept the instruction for this reply.'))
      else {
        if (answer.trim().length > 0) chat.messages.push({ role: 'assistant', text: answer, tab: input.tab })
        answer = ''
        const message = { role: 'user' as const, text: request.text, tab: request.tab }
        chat.messages.push(message)
        input.tab = request.tab
        dirty = false
        writeEvent({ type: 'steered', message })
        void persist(true).then(request.resolve, (reason: unknown) => {
          request.reject(new Error(`Could not save chat: ${describe(reason)}`))
          finish({ type: 'error', message: `Could not save chat: ${describe(reason)}` })
        })
      }
      return
    }

    if (message.method === 'item/tool/call' && message.id != null) {
      const id = message.id
      const tool = directString(message.params, 'tool')
      const command = nestedString(message.params, 'arguments', 'command')
      const reply = ({ error }: { error?: string }) => writeMessage({ id, result: {
        success: error == null,
        contentItems: error == null ? [] : [{ type: 'inputText', text: error }],
      } })
      if (tool !== 'ui' || ui == null || !matchingNotification(message.params, threadId, turnId)) reply({ error: 'UI tool unavailable.' })
      else if (command == null || command.length > 2_048) reply({ error: 'Expected a short UI command.' })
      else void ui.request(command).then(reply)
      return
    }

    if (message.method == null && message.id === 1) {
      if (message.error != null) {
        finish({ type: 'error', message: appServerError(message.error) })
        return
      }
      writeMessage({ method: 'initialized' })
      writeRequest(2, 'thread/start', {
        approvalPolicy: 'never',
        cwd: repoRoot,
        ephemeral: true,
        model,
        allowProviderModelFallback: false,
        config: { model_reasoning_effort: reasoningEffort },
        ...(ui == null ? {} : { dynamicTools: [context.uiTool] }),
      })
      return
    }
    if (message.method == null && message.id === 2) {
      if (message.error != null) {
        finish({ type: 'error', message: appServerError(message.error) })
        return
      }
      if (directString(message.result, 'model') !== model) {
        finish({ type: 'error', message: 'Codex did not select the requested model.' })
        return
      }
      threadId = nestedString(message.result, 'thread', 'id')
      if (threadId == null) {
        finish({ type: 'error', message: 'Codex did not return a temporary thread.' })
        return
      }
      writeRequest(3, 'turn/start', {
        approvalPolicy: 'never',
        input: [{ type: 'text', text: context.prompt, text_elements: [] }],
        effort: reasoningEffort,
        threadId,
      })
      return
    }
    if (message.method == null && message.id === 3) {
      if (message.error != null) {
        finish({ type: 'error', message: appServerError(message.error) })
        return
      }
      turnId = nestedString(message.result, 'turn', 'id')
      return
    }
    if (message.method === 'item/agentMessage/delta') {
      if (matchingNotification(message.params, threadId, turnId)) {
        const delta = directString(message.params, 'delta')
        if (delta != null) {
          answer += delta
          dirty = true
          writeEvent({ type: 'delta', text: delta })
        }
      }
      return
    }
    if (message.method === 'turn/completed' && matchingCompletedTurn(message.params, threadId, turnId)) {
      const status = nestedString(message.params, 'turn', 'status')
      if (status === 'completed') {
        finish({ type: 'done' })
      } else {
        finish({ type: 'error', message: completedTurnError(message.params, status) })
      }
      return
    }
    if (message.method === 'error') {
      finish({ type: 'error', message: notificationError(message.params) })
    }
  })

  child.once('error', (error) => finish({ type: 'error', message: error.message }))
  child.once('close', (code) => {
    if (finished) return
    const detail = lastNonEmptyLine(stderr)
    finish({
      type: 'error',
      message: detail ?? `Codex exited before completing the reply${code == null ? '.' : ` (status ${code}).`}`,
    })
  })

  writeRequest(1, 'initialize', {
    capabilities: { experimentalApi: true, requestAttestation: false },
    clientInfo: context.clientInfo,
  })

  await completed
}

function parseObject(text: string): Record<string, unknown> {
  let parsed: unknown
  try {
    parsed = JSON.parse(text)
  } catch {
    throw new PeekHttpError(400, 'Request body must be JSON.')
  }
  if (typeof parsed !== 'object' || parsed == null || Array.isArray(parsed)) {
    throw new PeekHttpError(400, 'Request body must be a JSON object.')
  }
  return parsed as Record<string, unknown>
}

function parseBody(parsed: Record<string, unknown>, parseFolder: PeekMiddlewareOptions['parseFolder'], tabs: ReadonlySet<string>, invalidTab: string): PeekRequest {
  const { messages, tab } = parsed
  const { candidate, folder } = parsePeekTarget(parsed, parseFolder)
  const threadId = parseThreadId(parsed.threadId)
  if (typeof tab !== 'string' || !tabs.has(tab)) {
    throw new PeekHttpError(400, invalidTab)
  }
  if (!Array.isArray(messages) || messages.length === 0) {
    throw new PeekHttpError(400, '"messages" must contain at least one chat message.')
  }
  const normalized = messages.map((message) => {
    if (typeof message !== 'object' || message == null || Array.isArray(message)) {
      throw new PeekHttpError(400, 'Each Peek message must be an object.')
    }
    const { role, text: messageText } = message as Record<string, unknown>
    if ((role !== 'user' && role !== 'assistant') || typeof messageText !== 'string' || messageText.trim().length === 0) {
      throw new PeekHttpError(400, 'Each Peek message needs a valid role and non-empty text.')
    }
    return { role, text: messageText } satisfies PeekMessage
  })
  if (normalized.at(-1)?.role !== 'user') {
    throw new PeekHttpError(400, 'The last Peek message must come from the User.')
  }
  return { candidate, folder, threadId, messages: normalized, tab, ui: parsed.ui === true }
}

function parseThreadId(id: unknown): string {
  if (typeof id !== 'string' || !PEEK_CHAT_ID_PATTERN.test(id)) throw new PeekHttpError(400, '"threadId" must be a chat UUID.')
  return id
}

function parsePeekTarget({ candidate, folder: rawFolder }: Record<string, unknown>, parseFolder: PeekMiddlewareOptions['parseFolder']): PeekTarget {
  const folder = parseFolder(rawFolder)
  if (typeof candidate !== 'string' || !CANDIDATE_OID.test(candidate)) {
    throw new PeekHttpError(400, '"candidate" must be a full Git commit OID.')
  }
  return { candidate, folder }
}

interface AppServerMessage {
  error?: unknown
  id?: number | string
  method?: string
  params?: unknown
  result?: unknown
}

function matchingNotification(params: unknown, threadId?: string, turnId?: string): boolean {
  return threadId != null && directString(params, 'threadId') === threadId
    && (turnId == null || directString(params, 'turnId') === turnId)
}

function matchingCompletedTurn(params: unknown, threadId?: string, turnId?: string): boolean {
  return threadId != null && directString(params, 'threadId') === threadId
    && (turnId == null || nestedString(params, 'turn', 'id') === turnId)
}

function nestedString(value: unknown, parent: string, child: string): string | undefined {
  if (typeof value !== 'object' || value == null || Array.isArray(value)) return undefined
  const nested = (value as Record<string, unknown>)[parent]
  return directString(nested, child)
}

function directString(value: unknown, key: string): string | undefined {
  if (typeof value !== 'object' || value == null || Array.isArray(value)) return undefined
  const result = (value as Record<string, unknown>)[key]
  return typeof result === 'string' ? result : undefined
}

function appServerError(error: unknown): string {
  return directString(error, 'message') ?? 'Codex rejected the Peek request.'
}

function notificationError(params: unknown): string {
  const error = typeof params === 'object' && params != null && !Array.isArray(params)
    ? (params as Record<string, unknown>).error
    : undefined
  return appServerError(error)
}

function completedTurnError(params: unknown, status?: string): string {
  if (typeof params === 'object' && params != null && !Array.isArray(params)) {
    const turn = (params as Record<string, unknown>).turn
    if (typeof turn === 'object' && turn != null && !Array.isArray(turn)) {
      const message = directString((turn as Record<string, unknown>).error, 'message')
      if (message != null) return message
    }
  }
  return `Peek turn ${status ?? 'did not complete'}.`
}

function lastNonEmptyLine(text: string): string | undefined {
  return text.split('\n').map((line) => line.trim()).filter(Boolean).at(-1)
}

function describe(reason: unknown): string {
  return reason instanceof Error ? reason.message : String(reason)
}

function readBody(request: IncomingMessage): Promise<string> {
  return new Promise((resolveBody, rejectBody) => {
    let text = ''
    let bytes = 0
    request.setEncoding('utf8')
    request.on('data', (chunk: string) => {
      bytes += Buffer.byteLength(chunk, 'utf8')
      if (bytes > MAX_PEEK_BODY_BYTES) {
        rejectBody(new PeekHttpError(413, `Request body exceeds ${MAX_PEEK_BODY_BYTES} bytes.`))
        request.destroy()
        return
      }
      text += chunk
    })
    request.on('end', () => resolveBody(text))
    request.on('error', () => rejectBody(new PeekHttpError(400, 'Request body could not be read.')))
  })
}

export class PeekHttpError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

function sendJson(response: ServerResponse, status: number, payload: unknown): void {
  if (response.headersSent) {
    response.end()
    return
  }
  response.statusCode = status
  response.setHeader('Content-Type', 'application/json; charset=utf-8')
  response.setHeader('Cache-Control', 'no-store')
  response.end(JSON.stringify(payload))
}
