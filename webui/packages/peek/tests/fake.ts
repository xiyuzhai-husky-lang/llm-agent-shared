import type { PeekApi, PeekChat, PeekModelCatalog } from '../src/api'

export const fakeModelCatalog: PeekModelCatalog = {
  defaults: { model: 'gpt-5.6-sol', reasoningEffort: 'low' },
  models: [
    { model: 'gpt-6-astra', displayName: 'GPT-6-Astra', defaultReasoningEffort: 'medium', isDefault: true,
      supportedReasoningEfforts: ['low', 'medium', 'high', 'xhigh', 'max', 'ultra'].map((reasoningEffort) => ({ reasoningEffort, description: reasoningEffort })) },
    { model: 'gpt-5.4-mini', displayName: 'GPT-5.4-Mini', defaultReasoningEffort: 'medium', isDefault: false,
      supportedReasoningEfforts: ['low', 'medium', 'high', 'xhigh'].map((reasoningEffort) => ({ reasoningEffort, description: reasoningEffort })) },
    { model: 'gpt-5.6-sol', displayName: 'GPT-5.6-Sol', defaultReasoningEffort: 'low', isDefault: false,
      supportedReasoningEfforts: ['low', 'medium', 'high', 'xhigh', 'max', 'ultra'].map((reasoningEffort) => ({ reasoningEffort, description: reasoningEffort })) },
  ],
}

export function fakePeekApi(stream: PeekApi['stream']): PeekApi {
  const chats = new Map<string, PeekChat>()
  return {
    async steer() { throw new Error('No active reply.') },
    async models() { return structuredClone(fakeModelCatalog) },
    async configure(folder, id, settings) {
      const chat = chats.get(id)
      if (chat?.folder !== folder) throw new Error('Chat not found.')
      Object.assign(chat, settings)
      return structuredClone(chat)
    },
    async list(folder) {
      return [...chats.values()].filter((chat) => chat.folder === folder).map((chat) => ({
        id: chat.id, candidate: chat.candidate, createdAt: chat.createdAt, updatedAt: chat.updatedAt,
        hasMessages: chat.messages.length > 0,
        title: chat.messages.find((message) => message.role === 'user')?.text.trim() || 'New chat',
      }))
    },
    async create(folder, candidate) {
      const now = new Date().toISOString()
      const chat = { id: crypto.randomUUID(), folder, candidate, createdAt: now, updatedAt: now, messages: [] }
      chats.set(chat.id, chat)
      return structuredClone(chat)
    },
    async load(folder, id) {
      const chat = chats.get(id)
      if (chat?.folder !== folder) throw new Error('Chat not found.')
      return structuredClone(chat)
    },
    async stream(input, onDelta, signal, onUi, onSteered) {
      const chat = chats.get(input.threadId)!
      let text = ''
      chat.messages = input.messages
      try {
        await stream(input, (delta) => { text += delta; onDelta(delta) }, signal, onUi, (message) => {
          if (text.length > 0) chat.messages = [...chat.messages, { role: 'assistant', text }]
          chat.messages = [...chat.messages, message]
          text = ''
          onSteered?.(message)
        })
      } finally {
        chat.messages = text.length === 0 ? chat.messages : [
          ...chat.messages, { role: 'assistant', text, interrupted: signal?.aborted || undefined },
        ]
        chat.updatedAt = new Date().toISOString()
      }
    },
  }
}
