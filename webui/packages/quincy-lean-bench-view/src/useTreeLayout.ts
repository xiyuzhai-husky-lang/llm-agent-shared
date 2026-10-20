import { useEffect, useState } from 'react'

/** Follow the phone breakpoint, including changes while the viewer is mounted. */
export function useTreeLayout(layout?: 'tree') {
  const [phone, setPhone] = useState(() => typeof window !== 'undefined'
    && typeof window.matchMedia === 'function' && window.matchMedia('(max-width: 760px)').matches)
  useEffect(() => {
    if (typeof window.matchMedia !== 'function') return
    const media = window.matchMedia('(max-width: 760px)')
    const sync = () => setPhone(media.matches)
    sync()
    media.addEventListener('change', sync)
    return () => media.removeEventListener('change', sync)
  }, [])
  return layout === 'tree' || phone
}
