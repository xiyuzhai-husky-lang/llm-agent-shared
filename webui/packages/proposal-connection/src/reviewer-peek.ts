import { peekApi, type PeekChat } from '@webui/peek/api'
import { hunkCommentMessage } from '@webui/peek/api'
import { peekFileReference } from '@webui/peek/api'
import type { PeekPanelView } from '@proposal/peek-panel'
import { json } from './decision'

type Request = Record<string, unknown>
type Subject = PeekPanelView['candidate']
interface Identity { proposal: string; candidate: string; label: string; draft_allowed: boolean }
interface State { request: Request; identity: Identity; view: PeekPanelView }
interface Action { thread_id: string; text: string; tab: string; model: string; effort: string; href: string; id: string; error: string | null; position: number; accept: boolean; timestamp: string; queued_id: string | null }

/** Adapts the shared Peek transport to the original panel. It retains chat drafts
 * and streaming state, while the existing helper persists conversations. */
export class PeekConnection {
  private readonly api = peekApi('/api/peek')
  private readonly states = new Map<string, State>()
  private readonly running = new Map<string, AbortController>()
  private readonly uiResults = new Map<string, (error: string | null) => void>()
  private catalog = this.api.models
  private models: ReturnType<typeof this.api.models> | undefined
  private publish: (request: Request, view: unknown) => void
  constructor(publish: (request: Request, view: unknown) => void) { this.publish = publish }
  private emit(state: State) { this.publish(state.request, structuredClone(state.view)) }
  private async state(request: Request): Promise<State> {
    const subject = (request.PeekPanel ?? request.DiffTab) as Subject
    const key = JSON.stringify(subject)
    const known = this.states.get(key)
    if (known) return known
    const identity = await json<Identity>('/api/reviewer/identity?' + new URLSearchParams({ subject: key }))
    this.models ??= this.catalog()
    const catalog = await this.models
    const state: State = { request: { PeekPanel: subject }, identity, view: { candidate: subject, round_label: identity.label,
      chats: [], listing: { threads: [] }, catalog, live: {}, queued: [], files: {}, draft_allowed: identity.draft_allowed, dismissed: [] } }
    this.states.set(key, state)
    await this.load(state)
    return state
  }
  private chat(chat: PeekChat): PeekPanelView['chats'][number] {
    const links: PeekPanelView['chats'][number]['links'] = []
    for (const message of chat.messages) for (const match of message.text.matchAll(/\[[^\]]*\]\(([^)]+)\)/g)) {
      const href = match[1]!
      try { const ref = peekFileReference(href, location.origin); if (ref) links.push({ href, target: { kind: 'repository_file', path: ref.path, start_line: ref.startLine, end_line: ref.endLine } }) } catch { /* External or malformed links retain normal Markdown navigation. */ }
    }
    return { ...chat, messages: chat.messages.map(m => ({ ...m, tab: m.tab ?? 'main' })), draftInstruction: draftInstruction(chat.candidate), verdictDraft: extractDraft(chat), links }
  }
  private async load(state: State) {
    const summaries = (await this.api.list(state.identity.proposal)).filter(t => t.candidate === state.identity.candidate)
    state.view.listing.threads = summaries.map(t => ({ id: t.id, title: t.title, running: t.running ?? false }))
    state.view.chats = await Promise.all(summaries.map(async t => this.chat(await this.api.load(state.identity.proposal, t.id))))
  }
  async read(request: Request) {
    const state = await this.state(request)
    state.identity = await json<Identity>('/api/reviewer/identity?' + new URLSearchParams({ subject: JSON.stringify(state.view.candidate) }))
    state.view.draft_allowed = state.identity.draft_allowed
    this.emit(state)
  }
  async queue(request: Request, input: { path: string; hunk: string; text: string }) {
    const state = await this.state(request)
    const hunk = await json<{ text: string }>('/api/reviewer/hunk?' + new URLSearchParams({ subject: JSON.stringify(state.view.candidate), path: input.path, hunk: input.hunk }))
    state.view.queued.push({ id: crypto.randomUUID(), text: hunkCommentMessage({ path: input.path, header: input.hunk, hunkText: hunk.text, comment: input.text }) })
    this.emit(state)
  }
  async act(request: Request, raw: unknown) {
    const state = await this.state(request)
    const operation = typeof raw === 'string' ? raw : Object.keys(raw as object)[0]!
    const action = (typeof raw === 'string' ? {} : (raw as Record<string, Action>)[operation]) as Action
    const folder = state.identity.proposal
    switch (operation) {
      case 'StartChat': await this.api.create(folder, state.identity.candidate); await this.load(state); break
      case 'Configure': await this.api.configure(folder, action.thread_id, { model: action.model, reasoningEffort: action.effort }); await this.load(state); break
      case 'Stop': this.running.get(action.thread_id)?.abort(); this.running.delete(action.thread_id); if (state.view.live[action.thread_id]) state.view.live[action.thread_id]!.running = false; await this.load(state); break
      case 'Send':
      case 'Draft': {
        const chat = state.view.chats.find(c => c.id === action.thread_id)
        if (!chat) throw new Error('Chat is unavailable.')
        const text = operation === 'Draft' ? draftInstruction(state.identity.candidate) : action.text
        if (this.running.has(chat.id)) {
          await this.api.steer({ folder, candidate: state.identity.candidate, threadId: chat.id, tab: action.tab, text })
          await this.load(state)
        } else {
          chat.messages.push({ role: 'user', text, tab: action.tab })
          const abort = new AbortController(); this.running.set(chat.id, abort)
          const live: PeekPanelView['live'][string] = { text: '', running: true, error: null, commands: [] }
          state.view.live[chat.id] = live
          if (operation === 'Draft') chat.draftInstruction = text
          const input = { folder, candidate: state.identity.candidate, threadId: chat.id, tab: action.tab, messages: chat.messages }
          void this.api.stream(input, delta => { live.text += delta; this.emit(state) }, abort.signal, command => new Promise<void>((resolve, reject) => {
            const id = crypto.randomUUID(), [kind, ...parts] = command.split(' '), target = parts.join(' ')
            if (kind !== 'tab' && kind !== 'diff') { reject(new Error('Use tab or diff.')); return }
            live.commands.push([id, { kind, target: kind === 'tab' ? target.toLowerCase() : { file: target } }])
            this.uiResults.set(id, error => error ? reject(new Error(error)) : resolve()); this.emit(state)
          }), message => { chat.messages.push({ ...message, tab: message.tab ?? action.tab }); this.emit(state) }).catch(error => {
            if (!abort.signal.aborted) live.error = String(error)
          }).finally(async () => {
            this.running.delete(chat.id); live.running = false
            await this.load(state)
            if (operation === 'Draft') { const saved = state.view.chats.find(c => c.id === chat.id); if (saved) { saved.draftInstruction = text; saved.verdictDraft = extractDraft(saved) } }
            this.emit(state)
          })
        }
        if (action.queued_id) state.view.queued = state.view.queued.filter(q => q.id !== action.queued_id)
        break
      }
      case 'UiResult': {
        this.uiResults.get(action.id)?.(action.error); this.uiResults.delete(action.id)
        const live = state.view.live[action.thread_id]; if (live) live.commands = live.commands.filter(([id]) => id !== action.id)
        break
      }
      case 'OpenFile': {
        const ref = peekFileReference(action.href, location.origin); if (!ref) throw new Error('Not a repository source link.')
        const response = await fetch('/candidate-file?' + new URLSearchParams({ request: JSON.stringify({ PeekPanel: state.view.candidate }), path: ref.path }))
        if (!response.ok) throw new Error(await response.text())
        state.view.files[action.href] = await response.text(); break
      }
      case 'AnswerDraft': {
        const chat = state.view.chats.find(c => c.id === action.thread_id)
        if (!chat?.verdictDraft || action.position !== chat.messages.length - 1 || this.running.has(chat.id) || state.view.dismissed.some(([id, position]) => id === chat.id && position === action.position)) throw new Error('This draft is no longer pending.')
        if (action.accept && !state.view.draft_allowed) throw new Error('This candidate cannot receive a verdict draft.')
        if (action.accept && chat.verdictDraft) await json('/api/reviewer/actions', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ id: `peek-draft-${chat.id}-${action.position}`, request: { ReviewPanel: state.view.candidate }, action: { ReviewPanel: { AppendEntry: { candidate: state.identity.candidate, permission: 'modify', comment: chat.verdictDraft, timestamp: action.timestamp, keep_signatures: false, diff_was_opened: false } } } }) })
        state.view.dismissed.push([action.thread_id, action.position]); break
      }
      default: throw new Error(`Unknown Peek action ${operation}`)
    }
    this.emit(state)
  }
  free() { for (const abort of this.running.values()) abort.abort(); this.running.clear() }
}

function draftInstruction(candidate: string) {
  return '根据当前线程至今的讨论，写一段发给 Worker 的话（candidate ' + candidate.slice(0, 7) + '）：中文叙述，identifier 保持代码中的写法，写清楚要改什么、改成什么以及为什么。这段话本身放在一个 ```draft 代码块里（以 ```draft 开头、以 ``` 结尾）；代码块之外只写你需要对 user 说的话，没有就不写。'
}
function extractDraft(chat: { messages: { role: string; text: string; interrupted?: boolean }[] }): string | undefined {
  const last = chat.messages.at(-1)
  if (last?.role !== 'assistant' || last.interrupted) return undefined
  return /^```draft[ \t]*\r?\n([\s\S]*?)^```[ \t]*\r?$/m.exec(last.text)?.[1]?.trim()
}
