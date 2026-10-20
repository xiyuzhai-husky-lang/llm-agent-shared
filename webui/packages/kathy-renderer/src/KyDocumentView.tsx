import { useEffect, useRef, useState } from 'react'

import type { KyDocument } from '@kathy/ast'

import {
  renderKyDocument,
  type KyDocumentRenderOptions,
} from './render.ts'
import { KyRendererError } from './errors.ts'

export interface KyDocumentViewProps extends KyDocumentRenderOptions {
  document: KyDocument
  className?: string
  onError?: (error: KyRendererError) => void
}

export function KyDocumentView({ document, profile, vfs, oid, className, onError }: KyDocumentViewProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const onErrorRef = useRef(onError)
  const [error, setError] = useState<KyRendererError | null>(null)

  useEffect(() => {
    onErrorRef.current = onError
  }, [onError])

  useEffect(() => {
    const container = containerRef.current
    if (container === null) return
    let current = true
    setError(null)
    try {
      const handle = renderKyDocument(container, document, { profile, vfs, oid })
      void handle.finished.catch((cause: unknown) => {
        if (!current) return
        const normalized = normalizeRenderError(cause)
        setError(normalized)
        onErrorRef.current?.(normalized)
      })
      return () => {
        current = false
        handle.dispose()
      }
    } catch (cause) {
      const normalized = normalizeRenderError(cause)
      setError(normalized)
      onErrorRef.current?.(normalized)
      return
    }
  }, [document, profile, vfs, oid])

  return (
    <div className={className}>
      <div ref={containerRef} />
      {error === null ? null : <div role="alert">{error.message}</div>}
    </div>
  )
}

function normalizeRenderError(cause: unknown): KyRendererError {
  return cause instanceof KyRendererError
    ? cause
    : new KyRendererError('RenderFailed', 'body', { message: String(cause) })
}
