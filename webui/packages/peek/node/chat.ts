import { createHash, randomUUID } from 'node:crypto'
import { mkdir, readFile, readdir, rename, rm, writeFile } from 'node:fs/promises'
import { dirname, join } from 'node:path'

export interface SavedPeekMessage {
  role: 'assistant' | 'user'
  text: string
  tab?: string
  interrupted?: boolean
}

export interface SavedPeekChat {
  id: string
  folder: string
  candidate: string
  createdAt: string
  updatedAt: string
  model?: string
  reasoningEffort?: string
  messages: SavedPeekMessage[]
}

export type PeekChatSummary = Pick<SavedPeekChat, 'id' | 'candidate' | 'createdAt' | 'updatedAt'> & { title: string; hasMessages: boolean; running?: boolean }

export const PEEK_CHAT_ID_PATTERN = /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/

export function peekScopeDirectory(root: string, repoRoot: string, folder: string): string {
  const key = createHash('sha256').update(JSON.stringify([repoRoot, folder])).digest('hex')
  return join(root, key)
}

export function peekChatPath(root: string, repoRoot: string, folder: string, id: string): string {
  if (!PEEK_CHAT_ID_PATTERN.test(id)) throw new Error('Invalid Peek chat ID.')
  return join(peekScopeDirectory(root, repoRoot, folder), `${id}.json`)
}

export async function createPeekChat(root: string, repoRoot: string, folder: string, candidate: string): Promise<SavedPeekChat> {
  return createPeekChatInDirectory(peekScopeDirectory(root, repoRoot, folder), folder, candidate)
}

async function createPeekChatInDirectory(directory: string, folder: string, candidate: string): Promise<SavedPeekChat> {
  const id = randomUUID()
  const now = new Date().toISOString()
  return writePeekChat(join(directory, `${id}.json`), {
    id, folder, candidate, createdAt: now, updatedAt: now, messages: [],
  })
}

export async function listPeekChats(root: string, repoRoot: string, folder: string): Promise<PeekChatSummary[]> {
  return listPeekChatsInDirectory(peekScopeDirectory(root, repoRoot, folder), folder)
}

async function listPeekChatsInDirectory(directory: string, folder: string): Promise<PeekChatSummary[]> {
  let entries: string[]
  try {
    entries = await readdir(directory)
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === 'ENOENT') return []
    throw error
  }
  const chats = await Promise.all(entries.filter((entry) => entry.endsWith('.json')
    && PEEK_CHAT_ID_PATTERN.test(entry.slice(0, -5))).map((entry) => (
    readPeekChat(join(directory, entry), folder, entry.slice(0, -5))
  )))
  return chats.map(({ id, candidate, createdAt, updatedAt, messages }) => ({
    id, candidate, createdAt, updatedAt,
    hasMessages: messages.length > 0,
    title: messages.find((message) => message.role === 'user')?.text.trim().replace(/\s+/g, ' ').slice(0, 80) || 'New chat',
  })).sort((left, right) => right.updatedAt.localeCompare(left.updatedAt) || right.id.localeCompare(left.id))
}

export async function readPeekChat(path: string, folder: string, id: string): Promise<SavedPeekChat> {
  const text = await readFile(path, 'utf8')
  const value: unknown = JSON.parse(text)
  if (typeof value !== 'object' || value == null || Array.isArray(value)) throw new Error('Saved Peek chat is invalid.')
  const chat = value as Record<string, unknown>
  if (chat.id !== id || chat.folder !== folder || typeof chat.candidate !== 'string' || !/^[0-9a-f]{40,64}$/.test(chat.candidate)
    || typeof chat.createdAt !== 'string' || !Number.isFinite(Date.parse(chat.createdAt))
    || typeof chat.updatedAt !== 'string' || !Number.isFinite(Date.parse(chat.updatedAt)) || !Array.isArray(chat.messages)
    || (chat.model != null && typeof chat.model !== 'string')
    || (chat.reasoningEffort != null && typeof chat.reasoningEffort !== 'string')
    || !chat.messages.every(isSavedMessage)) throw new Error('Saved Peek chat is invalid; the existing file was preserved.')
  return chat as unknown as SavedPeekChat
}

export async function writePeekChat(path: string, chat: SavedPeekChat): Promise<SavedPeekChat> {
  const saved = { ...chat, updatedAt: new Date().toISOString() }
  await mkdir(dirname(path), { recursive: true, mode: 0o700 })
  const temporary = `${path}.${randomUUID()}.tmp`
  try {
    await writeFile(temporary, `${JSON.stringify(saved, null, 2)}\n`, { flag: 'wx', mode: 0o600 })
    await rename(temporary, path)
    return saved
  } finally {
    await rm(temporary, { force: true })
  }
}

function isSavedMessage(value: unknown): value is SavedPeekMessage {
  if (typeof value !== 'object' || value == null || Array.isArray(value)) return false
  const { role, text, tab, interrupted } = value as Record<string, unknown>
  return (role === 'user' || role === 'assistant') && typeof text === 'string' && text.trim().length > 0
    && (tab == null || typeof tab === 'string') && (interrupted == null || typeof interrupted === 'boolean')
}

export function createPeekChatStore(directory: (folder: string) => string) {
  return {
    path(folder: string, id: string) {
      if (!PEEK_CHAT_ID_PATTERN.test(id)) throw new Error('Invalid Peek chat ID.')
      return join(directory(folder), `${id}.json`)
    },
    create: (folder: string, candidate: string) => createPeekChatInDirectory(directory(folder), folder, candidate),
    list: (folder: string) => listPeekChatsInDirectory(directory(folder), folder),
  }
}
export type PeekChatStore = ReturnType<typeof createPeekChatStore>
