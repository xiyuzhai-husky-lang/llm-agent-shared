import { useRef, useState } from 'react'
/** The narrowest the question source may be dragged and the narrowest the question beside it may become, and the default width, in pixels. */
const NARROWEST = 140, NARROWEST_BESIDE = 140, DEFAULT_WIDTH = 480
/** The fraction of the window the reader dragged the question source to; it outlives the pane, as the old page kept it. */
let dragged: number | undefined
function clamped(fraction: number): number {
 const width = Math.max(window.innerWidth, 1)
 return Math.min(Math.max(fraction, NARROWEST / width), Math.max((width - NARROWEST_BESIDE) / width, NARROWEST / width))
}
/** The question source's left edge: a drag reports a fraction of the window, an arrow key a step in pixels; the width reaches the stylesheet through `--question-source-size`. */
export function QuestionSourceResizeHandle() {
 const [fraction, setFraction] = useState(() => dragged ?? DEFAULT_WIDTH / Math.max(window.innerWidth, 1))
 const dragging = useRef(false)
 const resize = (next: number) => {
  dragged = clamped(next)
  document.documentElement.style.setProperty('--question-source-size', `${(dragged * 100).toFixed(2)}vw`)
  setFraction(dragged)
 }
 return <div
  className="proposal-resize-handle"
  role="separator"
  aria-orientation="vertical"
  aria-label="Resize question and source"
  aria-valuemin={0}
  aria-valuemax={100}
  aria-valuenow={Math.round(fraction * 100)}
  tabIndex={0}
  onPointerDown={event => {
   if (event.button !== 0) return
   event.preventDefault()
   event.stopPropagation()
   event.currentTarget.focus({ preventScroll: true })
   dragging.current = true
   event.currentTarget.setPointerCapture(event.pointerId)
  }}
  onPointerMove={event => {
   const width = window.innerWidth
   if (dragging.current && width !== 0) resize((width - event.clientX) / width)
  }}
  onPointerUp={event => {
   dragging.current = false
   if (event.currentTarget.hasPointerCapture(event.pointerId)) event.currentTarget.releasePointerCapture(event.pointerId)
  }}
  onLostPointerCapture={() => { dragging.current = false }}
  onPointerCancel={() => { dragging.current = false }}
  onKeyDown={event => {
   if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return
   event.preventDefault()
   event.stopPropagation()
   const step = (event.key === 'ArrowRight' ? 1 : -1) * (event.shiftKey ? 8 : 24)
   resize(fraction - step / Math.max(window.innerWidth, 1))
  }}
 />
}
