import { expect, it, vi } from 'vitest'
import { peekApi } from '../src/api'
import { peekFileReference } from '../src/link'

it('uses the host route for streaming without changing the event protocol', async () => {
  const fetcher = vi.fn(async () => new Response('{"type":"delta","text":"Answer"}\n{"type":"done"}\n'))
  const delta = vi.fn()
  await peekApi('/api/diff-peek', fetcher).stream({ folder: 'basic/base', candidate: 'a'.repeat(40), threadId: 'thread', tab: 'diff',
    messages: [{ role: 'user', text: 'Question' }] }, delta)
  expect(fetcher).toHaveBeenCalledWith('/api/diff-peek', expect.objectContaining({ method: 'POST' }))
  expect(delta).toHaveBeenCalledExactlyOnceWith('Answer')
})

it('resolves root and nested citations without Proposal folder semantics', () => {
  expect(peekFileReference('README.md#L2-L4', 'http://localhost')).toEqual({ path: 'README.md', startLine: 2, endLine: 4 })
  expect(peekFileReference('src/a.ts:3', 'http://localhost')).toEqual({ path: 'src/a.ts', startLine: 3, endLine: 3 })
})
