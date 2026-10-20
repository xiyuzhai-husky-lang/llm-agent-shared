// @vitest-environment node
import { mkdtemp, readFile, rm } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { expect, it } from 'vitest'
import { createPeekChatStore, readPeekChat, writePeekChat } from '../node/chat'

it('restores the reviewer JSON shape from the host-selected directory', async () => {
  const root = await mkdtemp(join(tmpdir(), 'shared-peek-'))
  try {
    const store = createPeekChatStore(() => root)
    const chat = await store.create('basic/base', 'a'.repeat(40))
    const path = store.path(chat.folder, chat.id)
    const saved = await writePeekChat(path, { ...chat, model: 'gpt-6-astra', reasoningEffort: 'medium',
      messages: [{ role: 'user', text: 'Question', tab: 'diff' }, { role: 'assistant', text: 'Answer', interrupted: false }] })
    expect(JSON.parse(await readFile(path, 'utf8'))).toEqual(saved)
    expect(await readPeekChat(path, chat.folder, chat.id)).toEqual(saved)
    expect(await createPeekChatStore(() => root).list(chat.folder)).toEqual([expect.objectContaining({ id: chat.id, title: 'Question', hasMessages: true })])
    expect(Object.keys(saved).sort()).toEqual(['candidate', 'createdAt', 'folder', 'id', 'messages', 'model', 'reasoningEffort', 'updatedAt'])
  } finally { await rm(root, { recursive: true, force: true }) }
})
