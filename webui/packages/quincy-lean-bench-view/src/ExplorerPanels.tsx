import { useRef, useState, type ReactNode } from 'react'
import './report-explorer.css'

/** Two independently scrolling panels. Nest in the right panel to extend rightward. */
export function ExplorerPanels({ left, right, label, initialSize = 320, className = '' }: {
  left: ReactNode; right: ReactNode; label: string; initialSize?: number; className?: string
}) {
  const root = useRef<HTMLDivElement>(null)
  const leftRef = useRef<HTMLDivElement>(null)
  const drag = useRef<{ x: number; size: number } | null>(null)
  const [size, setSize] = useState(initialSize)
  const resize = (next: number) => {
    const available = root.current?.clientWidth ?? 800
    setSize(Math.max(140, Math.min(Math.max(140, available - 180), next)))
  }
  return <div ref={root} className={`report-panels ${className}`}>
    <div ref={leftRef} className="report-panel-left" style={{ flexBasis: size }}>{left}</div>
    <div className="report-panel-resize" role="separator" tabIndex={0} aria-label={label}
      aria-orientation="vertical" aria-valuenow={size} aria-valuemin={140} aria-valuetext={`${size} pixels`}
      onPointerDown={event => {
        if (event.button !== 0) return
        event.preventDefault()
        event.currentTarget.focus({ preventScroll: true })
        event.currentTarget.setPointerCapture(event.pointerId)
        drag.current = { x: event.clientX, size: leftRef.current?.getBoundingClientRect().width ?? size }
      }}
      onPointerMove={event => { if (drag.current) resize(drag.current.size + event.clientX - drag.current.x) }}
      onPointerUp={event => {
        drag.current = null
        if (event.currentTarget.hasPointerCapture(event.pointerId)) event.currentTarget.releasePointerCapture(event.pointerId)
      }}
      onPointerCancel={() => { drag.current = null }} onLostPointerCapture={() => { drag.current = null }}
      onKeyDown={event => {
        if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return
        event.preventDefault()
        event.stopPropagation()
        resize((leftRef.current?.getBoundingClientRect().width ?? size) + (event.key === 'ArrowRight' ? 1 : -1) * (event.shiftKey ? 8 : 24))
      }} />
    <div className="report-panel-right">{right}</div>
  </div>
}
