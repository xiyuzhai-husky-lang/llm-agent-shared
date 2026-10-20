import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { ChatView } from './ChatView'

afterEach(cleanup)

it('sends steering with Enter while retaining a separate Stop control', () => {
  const steer = vi.fn()
  const send = vi.fn()
  render(<ChatView label="Chat" draft="Correction" messages={[]} running onDraftChange={() => {}} onSend={send} onSteer={steer} onStop={() => {}} />)
  fireEvent.keyDown(screen.getByLabelText('Chat message'), { key: 'Enter' })
  expect(steer).toHaveBeenCalledOnce()
  expect(send).not.toHaveBeenCalled()
  expect(screen.getByRole('button', { name: 'Steer reply' })).toBeTruthy()
  expect(screen.getByRole('button', { name: 'Stop reply' })).toBeTruthy()
})

it('sends on Enter, keeps Shift+Enter and IME composition as text input', () => {
  const send = vi.fn()
  render(<ChatView label="Chat" draft="Question" messages={[]} onDraftChange={() => {}} onSend={send} />)
  const input = screen.getByLabelText('Chat message')
  fireEvent.keyDown(input, { key: 'Enter', shiftKey: true })
  fireEvent.keyDown(input, { key: 'Enter', isComposing: true })
  fireEvent.keyDown(input, { key: 'Enter', keyCode: 229 })
  expect(send).not.toHaveBeenCalled()
  fireEvent.keyDown(input, { key: 'Enter' })
  expect(send).toHaveBeenCalledTimes(1)
})

it('lets the user draft while streaming and exposes Stop without another send', () => {
  const stop = vi.fn()
  const send = vi.fn()
  const change = vi.fn()
  render(<ChatView label="Chat" draft="Next" messages={[]} running onDraftChange={change} onSend={send} onStop={stop} />)
  const input = screen.getByLabelText('Chat message')
  fireEvent.change(input, { target: { value: 'Next question' } })
  expect(change).toHaveBeenCalledWith('Next question')
  fireEvent.keyDown(input, { key: 'Enter' })
  expect(send).not.toHaveBeenCalled()
  fireEvent.click(screen.getByRole('button', { name: 'Stop reply' }))
  expect(stop).toHaveBeenCalledOnce()
})

it('preserves reading position when streaming while scrolled up', () => {
  const props = { label: 'Chat', draft: '', onDraftChange: () => {}, onSend: () => {} }
  const view = render(<ChatView {...props} messages={[{ role: 'assistant', text: 'First' }]} />)
  const history = screen.getByRole('log')
  Object.defineProperties(history, {
    clientHeight: { value: 200 }, scrollHeight: { value: 1000 }, scrollTop: { value: 200, writable: true },
  })
  fireEvent.scroll(history)
  view.rerender(<ChatView {...props} messages={[{ role: 'assistant', text: 'First more' }]} />)
  expect(history.scrollTop).toBe(200)
  history.scrollTop = 800
  fireEvent.scroll(history)
  view.rerender(<ChatView {...props} messages={[{ role: 'assistant', text: 'First more again' }]} />)
  expect(history.scrollTop).toBe(1000)
})
