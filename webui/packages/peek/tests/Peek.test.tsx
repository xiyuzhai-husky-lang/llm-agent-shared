import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { Peek } from '../src/Peek'
import { fakePeekApi, fakeModelCatalog } from './fake'
import type { PeekApi } from '../src/api'

afterEach(cleanup)
const candidate = 'a'.repeat(40)
const folder = 'basic/base'

it('renders the shared model defaults without any review action', async () => {
  const api = fakePeekApi(async () => {})
  vi.spyOn(api, 'models').mockResolvedValue({ ...fakeModelCatalog, defaults: { model: 'gpt-6-astra', reasoningEffort: 'medium' } })
  render(<Peek api={api} candidate={candidate} folder={folder} tab="diff" />)
  expect((await screen.findByRole('button', { name: 'Peek model' })).textContent).toContain('GPT-6-Astra')
  expect(screen.getByRole('button', { name: 'Peek reasoning effort' }).textContent).toContain('Medium')
  expect(screen.queryByRole('button', { name: /Draft|Modify|verdict|sign/i })).toBeNull()
})

it('sends a queued question only once the thread is ready', async () => {
  const stream = vi.fn<PeekApi['stream']>(async (_input, delta) => delta('Queued answer'))
  const api = fakePeekApi(stream)
  const consume = vi.fn()
  render(<Peek api={api} candidate={candidate} folder={folder} tab="diff"
    queuedMessage={{ id: 'queued', candidate, text: 'Question about the diff' }} onQueuedMessageConsumed={consume} />)
  await screen.findByText('Queued answer')
  await waitFor(() => expect(consume).toHaveBeenCalledExactlyOnceWith('queued'))
  expect(stream).toHaveBeenCalledTimes(1)
  expect(stream.mock.calls[0][0]).toMatchObject({ folder, candidate, messages: [{ role: 'user', text: 'Question about the diff' }] })
})

it('restores the saved transcript when the shared panel reopens', async () => {
  const api = fakePeekApi(async (_input, delta) => delta('Stored reply'))
  const view = render(<Peek api={api} candidate={candidate} folder={folder} tab="diff" />)
  await waitFor(() => expect(screen.getByRole<HTMLTextAreaElement>('textbox').disabled).toBe(false))
  fireEvent.change(screen.getByRole('textbox'), { target: { value: 'Stored question' } })
  fireEvent.click(screen.getByRole('button', { name: 'Send' }))
  await screen.findByText('Stored reply')
  view.unmount()
  render(<Peek api={api} candidate={candidate} folder={folder} tab="diff" />)
  expect(await screen.findByText('Stored reply')).toBeTruthy()
})
