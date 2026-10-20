import { useLayoutEffect, useRef, useState, type ReactNode } from 'react'
import './chat.css'

export interface ChatMessage {
  role: 'assistant' | 'user'
  text: string
}

// The consumer owns messages, transport and storage. This component owns only
// the chat interaction and layout; it has no project or model dependency.
export interface ChatViewProps {
  label: string
  header?: ReactNode
  composerControls?: ReactNode
  messages: readonly ChatMessage[]
  draft: string
  running?: boolean
  loading?: boolean
  sending?: boolean
  error?: string
  onDraftChange: (text: string) => void
  onSend: () => void
  onSteer?: () => void
  onStop?: () => void
  renderMessage?: (message: ChatMessage, index: number) => ReactNode
}

export function ChatView({
  label, header, composerControls, messages, draft, running = false, loading = false, sending = false, error,
  onDraftChange, onSend, onSteer, onStop, renderMessage,
}: ChatViewProps) {
  const historyRef = useRef<HTMLDivElement>(null)
  const [copiedIndex, setCopiedIndex] = useState<number>()
  const flashCopied = (index: number) => {
    setCopiedIndex(index)
    window.setTimeout(() => setCopiedIndex((current) => (current === index ? undefined : current)), 1200)
  }
  const inputRef = useRef<HTMLTextAreaElement>(null)
  const followsBottom = useRef(true)
  const canSend = (!running || onSteer != null) && !loading && !sending && draft.trim().length > 0

  useLayoutEffect(() => {
    const history = historyRef.current
    if (history != null && followsBottom.current) history.scrollTop = history.scrollHeight
  }, [messages])

  useLayoutEffect(() => {
    const input = inputRef.current
    if (input == null) return
    input.style.height = 'auto'
    input.style.height = `${Math.min(input.scrollHeight, 180)}px`
  }, [draft])

  useLayoutEffect(() => {
    if (!loading) inputRef.current?.focus()
  }, [loading])

  const send = () => {
    if (!canSend) return
    followsBottom.current = true
    if (running) onSteer?.()
    else onSend()
    inputRef.current?.focus()
  }

  return (
    <section aria-label={`${label} chat`} className="webui-chat">
      {header != null && <header className="webui-chat-header">{header}</header>}
      <div
        aria-busy={loading || running}
        aria-label={`${label} history`}
        className="webui-chat-history"
        onScroll={() => {
          const history = historyRef.current
          if (history != null) {
            followsBottom.current = history.scrollHeight - history.scrollTop - history.clientHeight < 80
          }
        }}
        ref={historyRef}
        role="log"
        tabIndex={0}
      >
        {loading && <p className="webui-chat-empty" role="status">Loading chat…</p>}
        {!loading && messages.length === 0 && <p className="webui-chat-empty">Start a conversation.</p>}
        {messages.map((message, index) => (
          <article aria-label={message.role === 'user' ? 'You' : 'Assistant'} className={`webui-chat-message ${message.role}`} key={index}>
            {message.text.length === 0
              ? <span aria-label={`${label} is replying`} className="webui-chat-typing">…</span>
              : renderMessage?.(message, index) ?? message.text}
            {message.text.length > 0 && (
              // Per-message tools (user 2026-09-17: "add copy etc to the peek chat"): copy the text; a user message can
              // also be put back into the composer to edit and resend.
              <div className="webui-chat-message-tools">
                <button
                  aria-label={copiedIndex === index ? `Copied message ${index + 1}` : `Copy message ${index + 1}`}
                  className={copiedIndex === index ? 'copied' : undefined}
                  onClick={() => { void copyText(message.text).then(() => flashCopied(index)) }}
                  title={copiedIndex === index ? 'Copied' : 'Copy'}
                  type="button"
                >
                  {copiedIndex === index
                    ? <svg aria-hidden="true" viewBox="0 0 16 16" width="13" height="13" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><path d="M3 8.5 6.5 12 13 4.5" /></svg>
                    : <svg aria-hidden="true" viewBox="0 0 16 16" width="13" height="13" fill="none" stroke="currentColor" strokeWidth="1.4"><rect x="5.5" y="5.5" width="8" height="8" rx="1.5" /><path d="M10.5 5.5V3.5A1.5 1.5 0 0 0 9 2H3.5A1.5 1.5 0 0 0 2 3.5V9a1.5 1.5 0 0 0 1.5 1.5h2" /></svg>}
                </button>
                {message.role === 'user' && (
                  <button aria-label={`Edit message ${index + 1} in the composer`} onClick={() => onDraftChange(message.text)} title="Edit and resend" type="button">
                    <svg aria-hidden="true" viewBox="0 0 16 16" width="13" height="13" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round"><path d="M11.5 2.5 13.5 4.5 5.5 12.5H3.5V10.5Z" /></svg>
                  </button>
                )}
              </div>
            )}
          </article>
        ))}
      </div>
      <footer aria-label={`${label} composer`} className="webui-chat-composer">
        {error != null && <div className="webui-chat-error" role="alert">{error}</div>}
        <div className="webui-chat-input-box">
          <textarea
            aria-label={`${label} message`}
            disabled={loading}
            onChange={(event) => onDraftChange(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing && event.keyCode !== 229) {
                event.preventDefault()
                send()
              }
            }}
            placeholder="Message…"
            ref={inputRef}
            rows={2}
            value={draft}
          />
          <div className="webui-chat-actions">
            {composerControls != null && <div className="webui-chat-composer-controls">{composerControls}</div>}
            {running && onStop != null && <button aria-label="Stop reply" onClick={onStop} type="button">■</button>}
            {(!running || (onSteer != null && draft.trim().length > 0)) &&
              <button aria-label={running ? 'Steer reply' : 'Send'} title={running ? 'Send to current reply' : 'Send'} disabled={!canSend} onClick={send} type="button">↑</button>}
          </div>
        </div>
      </footer>
    </section>
  )
}

async function copyText(text: string): Promise<void> {
  try { await navigator.clipboard.writeText(text) } catch {
    const area = document.createElement('textarea')
    area.value = text
    document.body.appendChild(area)
    area.select()
    document.execCommand('copy')
    area.remove()
  }
}
