import { useState } from 'react'
import type {
  NewReviewComment,
  ReviewComment,
  ReviewDocument,
  ReviewDocumentState,
  ReviewDocumentSummary,
  ReviewStatus,
} from './types'
import { MarkdownView } from './MarkdownView'
import './reviewSurface.css'

export function ReviewSurface({
  documents,
  selectedId,
  document,
  comments,
  state,
  loading,
  error,
  onSelect,
  onPostComment,
  onSetStatus,
}: {
  documents: ReviewDocumentSummary[]
  selectedId?: string
  document?: ReviewDocument
  comments: ReviewComment[]
  state?: ReviewDocumentState
  loading: boolean
  error?: string
  onSelect: (id: string) => void
  onPostComment: (comment: NewReviewComment) => Promise<void>
  onSetStatus: (status: ReviewStatus) => Promise<void>
}) {
  return (
    <div className="rs-shell">
      <aside className="rs-sidebar">
        <div className="rs-brand">
          <div className="rs-brand-title">review surface</div>
          <div className="rs-brand-sub">markdown-backed local review</div>
        </div>
        <div className="rs-doc-list">
          {documents.map((doc) => (
            <button
              key={doc.id}
              className={doc.id === selectedId ? 'rs-doc-link active' : 'rs-doc-link'}
              type="button"
              onClick={() => onSelect(doc.id)}
            >
              <span className="rs-doc-kind">{doc.kind}</span>
              <span className="rs-doc-title">{doc.title}</span>
              <span className="rs-doc-path">{doc.path}</span>
            </button>
          ))}
          {documents.length === 0 && <div className="rs-empty">No documents configured.</div>}
        </div>
      </aside>
      <main className="rs-main">
        {error ? (
          <div className="rs-placeholder">{error}</div>
        ) : loading ? (
          <div className="rs-placeholder">Loading...</div>
        ) : document ? (
          <DocumentReview
            document={document}
            comments={comments}
            state={state}
            onPostComment={onPostComment}
            onSetStatus={onSetStatus}
          />
        ) : (
          <div className="rs-placeholder">Select a document.</div>
        )}
      </main>
    </div>
  )
}

function DocumentReview({
  document,
  comments,
  state,
  onPostComment,
  onSetStatus,
}: {
  document: ReviewDocument
  comments: ReviewComment[]
  state?: ReviewDocumentState
  onPostComment: (comment: NewReviewComment) => Promise<void>
  onSetStatus: (status: ReviewStatus) => Promise<void>
}) {
  return (
    <div className="rs-review">
      <section className="rs-document">
        <header className="rs-document-head">
          <div>
            <div className="rs-document-kind">{document.kind}</div>
            <h1>{document.title}</h1>
          </div>
          <code>{document.path}</code>
        </header>
        <article className="rs-document-body">
          <MarkdownView text={document.body} />
        </article>
      </section>
      <CommentPanel
        comments={comments}
        state={state}
        onPostComment={onPostComment}
        onSetStatus={onSetStatus}
      />
    </div>
  )
}

function CommentPanel({
  comments,
  state,
  onPostComment,
  onSetStatus,
}: {
  comments: ReviewComment[]
  state?: ReviewDocumentState
  onPostComment: (comment: NewReviewComment) => Promise<void>
  onSetStatus: (status: ReviewStatus) => Promise<void>
}) {
  const [author, setAuthor] = useState('reviewer')
  const [body, setBody] = useState('')
  const [posting, setPosting] = useState(false)
  const [error, setError] = useState<string | undefined>(undefined)

  async function submit() {
    const trimmed = body.trim()
    if (!trimmed) return
    setPosting(true)
    setError(undefined)
    try {
      await onPostComment({ author: author.trim() || 'reviewer', body: trimmed })
      setBody('')
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setPosting(false)
    }
  }

  return (
    <aside className="rs-comments">
      <div className="rs-comments-head">
        <span>comments</span>
        <b>{comments.length}</b>
      </div>
      <StatusControl state={state} onSetStatus={onSetStatus} />
      <div className="rs-comment-list">
        {comments.map((comment) => (
          <div key={comment.id} className="rs-comment">
            <div className="rs-comment-meta">
              <b>{comment.author}</b>
              <span>{new Date(comment.timestamp).toLocaleString()}</span>
            </div>
            <MarkdownView text={comment.body} />
          </div>
        ))}
        {comments.length === 0 && <div className="rs-empty">No comments yet.</div>}
      </div>
      <div className="rs-composer">
        <input
          value={author}
          onChange={(e) => setAuthor(e.target.value)}
          disabled={posting}
          placeholder="author"
        />
        <textarea
          value={body}
          onChange={(e) => setBody(e.target.value)}
          disabled={posting}
          placeholder="comment with markdown + $math$"
          rows={5}
        />
        <button type="button" disabled={posting || body.trim().length === 0} onClick={submit}>
          {posting ? 'posting...' : 'post'}
        </button>
        {error && <div className="rs-error">{error}</div>}
      </div>
    </aside>
  )
}

function StatusControl({
  state,
  onSetStatus,
}: {
  state?: ReviewDocumentState
  onSetStatus: (status: ReviewStatus) => Promise<void>
}) {
  const [pending, setPending] = useState<ReviewStatus | undefined>(undefined)
  const [error, setError] = useState<string | undefined>(undefined)
  const current = pending ?? state?.status ?? 'pending'

  async function setStatus(status: ReviewStatus) {
    setPending(status)
    setError(undefined)
    try {
      await onSetStatus(status)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setPending(undefined)
    }
  }

  return (
    <div className="rs-status">
      <div className="rs-status-label">status</div>
      <div className="rs-status-buttons">
        {(['pending', 'reviewed', 'blocked'] as const).map((status) => (
          <button
            key={status}
            type="button"
            className={current === status ? 'active' : ''}
            onClick={() => void setStatus(status)}
            disabled={pending != null}
          >
            {status}
          </button>
        ))}
      </div>
      {state?.updatedAt && <div className="rs-status-time">{new Date(state.updatedAt).toLocaleString()}</div>}
      {error && <div className="rs-error">{error}</div>}
    </div>
  )
}
