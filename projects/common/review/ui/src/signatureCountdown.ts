import { useCallback, useEffect, useState } from 'react'

/** A server refusal cools down only that candidate/path/hunk, including while its Diff tab is hidden. */
export function useSignatureCountdown(candidateKey: string) {
  const [deadlines, setDeadlines] = useState<Record<string, number>>({})
  const [now, setNow] = useState(Date.now)
  useEffect(() => {
    const active = Object.values(deadlines).filter(deadline => deadline > now)
    if (active.length === 0) return
    const delay = Math.min(...active.map(deadline => (deadline - now - 1) % 1000 + 1))
    const timer = setTimeout(() => {
      const at = Date.now()
      setNow(at)
      setDeadlines(current => Object.fromEntries(Object.entries(current).filter(([, deadline]) => deadline > at)))
    }, delay)
    return () => clearTimeout(timer)
  }, [deadlines, now])

  const start = useCallback((path: string, hunk: string, remainingMs?: number) => {
    const seconds = Math.max(3, Math.ceil(typeof remainingMs === 'number' && Number.isFinite(remainingMs) ? remainingMs / 1000 : 0))
    const at = Date.now()
    const key = JSON.stringify([candidateKey, path, hunk])
    setNow(at)
    setDeadlines(current => ({ ...current, [key]: at + seconds * 1000 }))
  }, [candidateKey])
  const secondsRemaining = useCallback((path: string, hunk: string) =>
    Math.max(0, Math.ceil(((deadlines[JSON.stringify([candidateKey, path, hunk])] ?? now) - now) / 1000)),
  [candidateKey, deadlines, now])
  return { start, secondsRemaining }
}
