import { useMemo } from 'react'
import ReactMarkdown, { type Components } from 'react-markdown'
import remarkGfm from 'remark-gfm'
import remarkMath from 'remark-math'
import rehypeKatex from 'rehype-katex'
import 'katex/dist/katex.min.css'

export function MarkdownView({
  text,
  onLink,
  linkTarget,
}: {
  text: string
  onLink?: (href: string) => boolean
  linkTarget?: '_blank'
}) {
  const components = useMemo<Components>(() => ({
    table: ({ children }) => <div className="rs-markdown-table"><table>{children}</table></div>,
    ...(onLink || linkTarget ? {
      a: ({ href, children }) => <a
        href={href}
        target={linkTarget}
        rel={linkTarget === '_blank' ? 'noopener noreferrer' : undefined}
        onClick={(event) => {
          if (href != null && onLink?.(href)) event.preventDefault()
        }}
        onAuxClick={(event) => {
          if (event.button === 1 && href != null && onLink?.(href)) event.preventDefault()
        }}
      >{children}</a>,
    } : {}),
  }), [onLink, linkTarget])
  return (
    <div className="rs-markdown">
      <ReactMarkdown
        remarkPlugins={[remarkMath, remarkGfm]}
        rehypePlugins={[rehypeKatex]}
        components={components}
      >
        {text}
      </ReactMarkdown>
    </div>
  )
}
