import { Component, useEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import './shell.css'

export type ComponentAction<View = unknown> = (action: unknown, applied: (view: View) => boolean) => Promise<void>
type Request = Record<string, unknown>
type Frame = { View: [Request, Record<string, unknown>] } | { Refused: [Request, string] }
interface Connection { subscribe?(requests: string[]): void; read(request: string): void; act(request: string, action: string): void; free(): void }
interface ConnectionConstructor { new(address: string, receive: (text: string) => void, fail: (message: string) => void): Connection }
/** What the frame lends a component: the review column's top line, where the revision picker stands while the review is open. */
export interface ProposalShell { reviewTopline: HTMLElement | null }
interface ComponentEntry {
  identifier: string
  slot: 'sidebar' | 'header' | 'main' | 'tab' | 'panel'
  request(location: URLSearchParams): Request | null
  render(view: unknown, navigate: (href: string) => void, act: ComponentAction, shell: ProposalShell): ReactNode
}
class ComponentErrorBoundary extends Component<{ context: string; children: ReactNode }, { failure: string | null }> {
  state: { failure: string | null } = { failure: null }
  static getDerivedStateFromError(reason: unknown) { return { failure: reason instanceof Error ? reason.stack ?? reason.message : String(reason) } }
  render() {
    if (this.state.failure !== null) return <div className="panel-error" role="alert"><pre>{`${this.props.context}: ${this.state.failure}`}</pre></div>
    return this.props.children
  }
}
function ComponentView({ entry, view, navigate, act, shell }: { entry: ComponentEntry; view: unknown; navigate: (href: string) => void; act: ComponentAction; shell: ProposalShell }) {
  return entry.render(view, navigate, act, shell)
}
export function component<View>(identifier: string, slot: ComponentEntry['slot'], subject: (location: URLSearchParams) => unknown | undefined, render: (view: View, navigate: (href: string) => void, act: ComponentAction<View>, shell: ProposalShell) => ReactNode): ComponentEntry {
  return { identifier, slot, request: location => {
    const value = subject(location)
    return value === undefined ? null : { [identifier]: value }
  }, render: (view, navigate, act, shell) => render(view as View, navigate, (action, applied) => act(action, view => applied(view as View)), shell) }
}
interface CandidateKey { proposal_number: number; candidate_commit: string }
type HeadingTone = 'heading' | 'separator' | 'neutral' | 'positive' | 'negative' | 'must_sign'
type CandidateStop = 'incomplete' | 'chow-problems'
interface TabStripView { candidate: CandidateKey; default_tab: string | null; question_tab: string | null; tabs: {identifier: string; heading: {text: string; tone: HeadingTone}[]; component: string; stop: CandidateStop | null}[] }
const TONE_CLASS: Record<HeadingTone, string | undefined> = {
  heading: undefined,
  separator: undefined,
  neutral: 'tab-stat-neutral',
  positive: 'tab-stat-positive',
  negative: 'tab-stat-negative',
  must_sign: 'tab-stat-must-sign',
}
const STOP_NAME: Record<CandidateStop, string> = { incomplete: 'incomplete', 'chow-problems': 'Chow problems' }
type Pane = 'explorer' | 'review' | 'peek'
/** The narrowest a pane may be dragged and the narrowest the pane beside it may become, and the stylesheet's width, in pixels. */
const PANE: Record<Pane, { property: string; narrowest: number; narrowestBeside: number; defaultWidth: number }> = {
  explorer: { property: '--explorer-size', narrowest: 100, narrowestBeside: 280, defaultWidth: 250 },
  review: { property: '--review-size', narrowest: 200, narrowestBeside: 280, defaultWidth: 500 },
  peek: { property: '--peek-size', narrowest: 240, narrowestBeside: 200, defaultWidth: 340 },
}
function clampedFraction(pane: Pane, fraction: number): number {
  const width = Math.max(window.innerWidth, 1)
  const { narrowest, narrowestBeside } = PANE[pane]
  return Math.min(Math.max(fraction, narrowest / width), Math.max((width - narrowestBeside) / width, narrowest / width))
}
/** A draggable pane edge: a drag reports a fraction of the window, an arrow key a step in pixels. */
function ResizeHandle({ pane, fromTheRight, label, fraction, resize }: { pane: Pane; fromTheRight: boolean; label: string; fraction: number; resize(pane: Pane, fraction: number): void }) {
  const dragging = useRef(false)
  const report = (clientX: number) => {
    const width = window.innerWidth
    if (width === 0) return
    resize(pane, fromTheRight ? (width - clientX) / width : clientX / width)
  }
  return <div
    className="proposal-resize-handle"
    role="separator"
    aria-orientation="vertical"
    aria-label={label}
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
    onPointerMove={event => { if (dragging.current) report(event.clientX) }}
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
      resize(pane, fraction + (fromTheRight ? -step : step) / Math.max(window.innerWidth, 1))
    }}
  />
}
export function candidateSubject(location: URLSearchParams): CandidateKey | undefined {
 const number = location.get('proposal')
 if(number === null) return undefined
 if(!/^\d+$/.test(number) || Number(number) > 0xffffffff) throw new Error(`URL ?${location}: proposal expected a u32 number, found ${JSON.stringify(number)}`)
 const candidate = location.get('candidate')
 if(candidate === null || !/^[0-9a-f]{40,64}$/.test(candidate)) throw new Error(`URL ?${location}: proposal ${number}, candidate expected a full commit SHA, found ${JSON.stringify(candidate)}`)
 return {proposal_number:Number(number),candidate_commit:candidate}
}
export function ProposalPage({ components, initialize, connection: ConnectionType, baseUrl }: {
  components: ComponentEntry[]; initialize: () => Promise<unknown>; connection: ConnectionConstructor; baseUrl: string
}) {
  const [location, setLocation] = useState(window.location.search)
  const [views, setViews] = useState<Record<string, unknown>>({})
  const [errors, setErrors] = useState<Record<string, string>>({})
  const connection = useRef<Connection | null>(null)
  const pending = useRef(new Map<string, {resolve(): void; reject(error: Error): void; applied(view: unknown): boolean}>())
  const requested = useRef(new Set<string>())
  const carriedTab = useRef<{ tab: string; question: string | null } | null>(null)
  const [ready, setReady] = useState(false)
  // The review column starts open on a desktop and closed on a phone, where it is a sheet over the page. On a desktop Diff
  // it is a rail unless the reader opened it there.
  const [phone, setPhone] = useState(() => window.innerWidth <= 760)
  const [reviewOpen, setReviewOpen] = useState(() => window.innerWidth > 760)
  const [reviewOpenOnTheDiff, setReviewOpenOnTheDiff] = useState(false)
  const [reviewTopline, setReviewTopline] = useState<HTMLElement | null>(null)
  // The panes the reader dragged, as fractions of the window; an undragged pane keeps the stylesheet's width.
  const [paneFractions, setPaneFractions] = useState<Partial<Record<Pane, number>>>({})
  const resize = (pane: Pane, fraction: number) => setPaneFractions(previous => ({ ...previous, [pane]: clampedFraction(pane, fraction) }))
  const fractionOf = (pane: Pane) => paneFractions[pane] ?? PANE[pane].defaultWidth / Math.max(window.innerWidth, 1)
  const tabList = useRef<HTMLElement>(null)
  useEffect(() => {
    const resize = () => setPhone(window.innerWidth <= 760)
    window.addEventListener('resize', resize)
    return () => window.removeEventListener('resize', resize)
  }, [])
  useEffect(() => {
    let disposed = false
    initialize().then(() => {
      if (disposed) return
      const address = new URL(`${baseUrl}ws`, window.location.href)
      address.protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      connection.current = new ConnectionType(address.href, (text: string) => {
        try {
          const frame = JSON.parse(text) as Frame
          const frameKeys = Object.keys(frame)
          if (frameKeys.length !== 1 || (frameKeys[0] !== 'View' && frameKeys[0] !== 'Refused')) throw new Error(`Wire frame expected exactly View or Refused, found keys ${JSON.stringify(frameKeys)}; raw ${text}`)
          const payload = 'Refused' in frame ? frame.Refused : frame.View
          if (!Array.isArray(payload) || payload.length !== 2) throw new Error(`Wire ${frameKeys[0]} expected [request, response], found ${JSON.stringify(payload)}; raw ${text}`)
          const request = payload[0]
          const requestKeys = Object.keys(request)
          if (requestKeys.length !== 1 || (requestKeys[0] !== 'TabStrip' && !components.some(entry => entry.identifier === requestKeys[0]))) throw new Error(`Wire request expected exactly one registered component, found keys ${JSON.stringify(requestKeys)}; request ${JSON.stringify(request)}; raw ${text}`)
          const key = JSON.stringify(request)
          let view: unknown
          if ('View' in frame) {
            const viewKeys = Object.keys(frame.View[1])
            if (viewKeys.length !== 1 || viewKeys[0] !== requestKeys[0]) throw new Error(`Wire view expected component ${requestKeys[0]}, found keys ${JSON.stringify(viewKeys)}; request ${key}; raw ${text}`)
            view = frame.View[1][requestKeys[0]]
          } else if (typeof frame.Refused[1] !== 'string') throw new Error(`Wire Refused expected string reason, found ${JSON.stringify(frame.Refused[1])}; request ${key}; raw ${text}`)
          const action = pending.current.get(key)
          if (action) {
            if ('Refused' in frame) { pending.current.delete(key); action.reject(new Error(frame.Refused[1])); return }
            // A refresh can deliver a view before the action reply. Only its caller can
            // recognize the applied action in a component's new view.
            if (action.applied(view)) {
              pending.current.delete(key)
              action.resolve()
            }
          }
          if (!requested.current.has(key)) return
          if ('Refused' in frame) {
            setErrors(previous => ({ ...previous, [key]: frame.Refused[1] }))
            setViews(previous => { const next = { ...previous }; delete next[key]; return next })
          } else {
            setErrors(previous => { const next = { ...previous }; delete next[key]; return next })
            setViews(previous => ({ ...previous, [key]: view }))
          }
        } catch (reason) {
          const message = `Connection ${address.href}: ${String(reason)}; raw frame ${text}`
          for (const action of pending.current.values()) action.reject(new Error(message))
          pending.current.clear()
          setReady(false)
          setErrors(previous => ({ ...previous, connection: message }))
        }
      }, message => {
        for (const action of pending.current.values()) action.reject(new Error(message))
        pending.current.clear()
        setErrors(previous => ({ ...previous, connection: message }))
      })
      setReady(true)
    }).catch(reason => setErrors({ connection: String(reason) }))
    return () => { disposed = true; connection.current?.free(); connection.current = null }
  }, [initialize, ConnectionType, baseUrl, components])
  useEffect(() => {
    const back = () => setLocation(window.location.search)
    window.addEventListener('popstate', back)
    return () => window.removeEventListener('popstate', back)
  }, [])
  let subject: CandidateKey | undefined
  let locationFailure: string | null = null
  try { subject = candidateSubject(new URLSearchParams(location)) } catch (reason) { locationFailure = String(reason) }
  const stripRequest = subject ? JSON.stringify({TabStrip: subject}) : null
  const strip = stripRequest ? views[stripRequest] as TabStripView | undefined : undefined
  const requestedTab = new URLSearchParams(location).get('tab')
  // A new candidate of the same proposal keeps the tab on screen only while it still shows that tab and its question stands
  // on the same surface as before; otherwise it opens on its own default tab, as the old reviewer did.
  const carried = carriedTab.current
  const keepsCarried = carried !== null && strip !== undefined && strip.question_tab === carried.question && strip.tabs.some(tab => tab.identifier === carried.tab)
  const openingTab = keepsCarried ? carried!.tab : strip?.default_tab
  const tab = requestedTab !== null ? strip?.tabs.find(tab=>tab.identifier === requestedTab) : strip?.tabs.find(tab => tab.identifier === openingTab)
  const activeComponent = tab ? components.find(entry => entry.slot === 'tab' && entry.identifier === tab.component) : undefined
  if (strip) {
    const context = `URL ?${location}; proposal ${strip.candidate.proposal_number}, candidate ${strip.candidate.candidate_commit}`
    for (const entry of strip.tabs) {
      if (entry.stop !== null && entry.stop !== 'incomplete' && entry.stop !== 'chow-problems') locationFailure = `${context}: tab ${entry.identifier}, component ${entry.component}, stop expected incomplete/chow-problems/null, found ${JSON.stringify(entry.stop)}`
      if (!components.some(component => component.slot === 'tab' && component.identifier === entry.component)) locationFailure = `${context}: tab ${entry.identifier}, unknown component ${JSON.stringify(entry.component)}`
    }
    if (requestedTab !== null && !tab) locationFailure = `${context}: unknown tab ${JSON.stringify(requestedTab)}; available ${JSON.stringify(strip.tabs.map(entry => entry.identifier))}`
    if (requestedTab === null && openingTab !== null && openingTab !== undefined && !tab) locationFailure = `${context}: default tab ${JSON.stringify(openingTab)} absent from ${JSON.stringify(strip.tabs.map(entry => entry.identifier))}`
  }
  const activeRequest = !locationFailure && tab && strip ? {[tab.component]: strip.candidate} : null
  const activeKey = activeRequest ? JSON.stringify(activeRequest) : null
  useEffect(() => {
    if (!ready || locationFailure) return
    const next = new Set<string>()
    try {
      for (const entry of components) {
        if (entry.slot === 'tab') continue
        const request = entry.request(new URLSearchParams(location))
        if (request) next.add(JSON.stringify(request))
      }
      if(stripRequest) next.add(stripRequest)
      if(activeKey) next.add(activeKey)
      setErrors(previous => { const next = { ...previous }; delete next.location; return next })
      const previous = requested.current
      requested.current = next
      connection.current!.subscribe?.([...next])
      for (const request of next) if (!previous.has(request)) connection.current!.read(request)
    } catch (reason) { setErrors({ location: String(reason) }) }
  }, [ready, location, components, stripRequest, activeKey, locationFailure])
  useEffect(() => {
    if(!tab || locationFailure) return
    const query = new URLSearchParams(location)
    carriedTab.current = null
    if(query.get('tab') === tab.identifier) return
    query.set('tab', tab.identifier)
    window.history.replaceState(null, '', `?${query}`)
    setLocation(window.location.search)
  }, [location, tab, locationFailure])
  const navigate = (href: string) => {
    const next = new URL(href, window.location.href)
    const current = new URLSearchParams(location)
    if(next.searchParams.has('candidate') && next.searchParams.get('proposal') === current.get('proposal') && current.has('tab') && !next.searchParams.has('tab')) carriedTab.current = { tab: current.get('tab')!, question: strip?.question_tab ?? null }
    href = next.href
    window.history.pushState(null, '', href)
    setLocation(window.location.search)
  }
  const actFor = (request: Request, identifier: string): ComponentAction => (action, applied) => new Promise((resolve, reject) => {
    const key = JSON.stringify(request)
    if (!connection.current || pending.current.has(key)) { reject(new Error('A write is already pending or the connection is closed')); return }
    pending.current.set(key, {resolve, reject, applied})
    try { connection.current.act(key, JSON.stringify({[identifier]: action})) }
    catch (error) { pending.current.delete(key); reject(error) }
  })
  const shell: ProposalShell = { reviewTopline }
  const renderSlot = (slot: ComponentEntry['slot']) => components.filter(entry => entry.slot === slot).map(entry => {
    let request: Request | null
    try { request = entry.request(new URLSearchParams(location)) } catch (reason) { return <div key={entry.identifier} className="panel-error" role="alert">{`Component ${entry.identifier}, URL ?${location}: ${String(reason)}`}</div> }
    if (!request) return null
    const key = JSON.stringify(request)
    return <section key={entry.identifier} data-component={entry.identifier} className={`proposal-slot proposal-slot-${slot}`}>
      {errors[key] ? <div className="panel-error" role="alert">{errors[key]} <button onClick={() => connection.current?.read(key)}>Retry</button></div>
        : key in views ? <ComponentErrorBoundary key={key} context={`Component ${entry.identifier}, request ${key}`}><ComponentView entry={entry} view={views[key]} navigate={navigate} act={actFor(request, entry.identifier)} shell={shell}/></ComponentErrorBoundary> : <p className="app-state" role="status">Loading…</p>}
    </section>
  })
  const panels = renderSlot('panel').filter(Boolean)
  const desktopDiff = tab?.identifier === 'diff' && tab.stop === null && !phone
  const presentation: 'open' | 'rail' | 'closed' = panels.length === 0 ? 'closed'
    : desktopDiff ? (reviewOpenOnTheDiff ? 'open' : 'rail')
    : reviewOpen ? 'open' : 'closed'
  const reviewIsOpen = presentation === 'open'
  const setTheReviewOpen = (open: boolean) => desktopDiff ? setReviewOpenOnTheDiff(open) : setReviewOpen(open)
  const selectedTab = tab?.identifier
  // Peek stands inside the review column, which the desktop Diff shows only as a rail: asking for Peek opens the column.
  const peekRequested = new URLSearchParams(location).has('peek')
  useEffect(() => { if (peekRequested) { setReviewOpen(true); setReviewOpenOnTheDiff(true) } }, [peekRequested])
  useEffect(() => {
    const list = tabList.current
    const selected = list?.querySelector<HTMLElement>('[role="tab"][aria-selected="true"]')
    if (selected == null) return
    // A tab moved to with the keyboard keeps the keyboard focus.
    if (list?.contains(window.document.activeElement) && window.document.activeElement !== selected) selected.focus()
    const scrollIntoView = () => selected.scrollIntoView({ block: 'nearest', inline: 'nearest' })
    scrollIntoView()
    window.addEventListener('resize', scrollIntoView)
    return () => window.removeEventListener('resize', scrollIntoView)
  }, [selectedTab, strip])
  const selectTab = (identifier: string) => {
    const query = new URLSearchParams(location)
    query.set('tab', identifier)
    navigate(`?${query}`)
  }
  if (errors.connection || locationFailure || errors.location) return <div className="panel-error" role="alert">{errors.connection ?? locationFailure ?? errors.location}</div>
  // A page of its own (the batch page) takes the whole window in place of the review frame.
  const pages = renderSlot('main').filter(Boolean)
  if (pages.length > 0) return <>{pages}</>
  return <div className="proposal-review-app" style={Object.fromEntries(Object.entries(paneFractions).map(([pane, fraction]) => [PANE[pane as Pane].property, `${(fraction * 100).toFixed(2)}vw`]))}>
    {renderSlot('sidebar')}
    <div className="proposal-review-detail">
      <ResizeHandle pane="explorer" fromTheRight={false} label="Resize proposals and content" fraction={fractionOf('explorer')} resize={resize} />
      {errors.connection && <div className="panel-error" role="alert">{errors.connection}</div>}
      {errors.location && <div className="panel-error" role="alert">{errors.location}</div>}
      <div className={['proposal-workspace', tab?.identifier === 'diff' ? 'diff-active' : '', reviewIsOpen ? 'review-open' : '', presentation === 'rail' ? 'review-rail' : '', reviewIsOpen && panels.length > 1 ? 'peek-open' : ''].filter(Boolean).join(' ')}>
        <div className="proposal-workspace-main">
          <div className="proposal-workspace-header">
            {renderSlot('header')}
            {panels.length > 0 && <button
              type="button"
              className="review-toggle"
              aria-label={reviewIsOpen ? 'Close review' : 'Open review'}
              aria-pressed={reviewIsOpen}
              aria-expanded={reviewIsOpen}
              onClick={() => setTheReviewOpen(!reviewIsOpen)}
            ><span aria-hidden="true">{reviewIsOpen ? '›' : '‹'}</span></button>}
          </div>
          {stripRequest && errors[stripRequest] && <div className="panel-error" role="alert">{errors[stripRequest]}</div>}
          {strip ? <nav aria-label="Proposal content" aria-orientation="horizontal" className="workspace-tabs" role="tablist" ref={tabList}>{strip.tabs.map(entry => {
            const selected = entry.identifier === tab?.identifier
            return <button
              key={entry.identifier}
              type="button"
              role="tab"
              id={`proposal-tab-${entry.identifier}`}
              aria-controls={`proposal-panel-${entry.identifier}`}
              aria-selected={selected}
              tabIndex={selected ? 0 : -1}
              className={selected ? 'active' : undefined}
              data-tab={entry.identifier}
              data-stop={entry.stop ?? undefined}
              aria-label={entry.stop ? `${entry.heading.map(part => part.text).join('')} ${STOP_NAME[entry.stop]}` : undefined}
              onKeyDown={event => {
                const keys = ['ArrowLeft', 'ArrowRight', 'Home', 'End']
                if (!keys.includes(event.key)) return
                event.preventDefault()
                const entries = strip.tabs
                const index = entries.findIndex(each => each.identifier === entry.identifier)
                const next = event.key === 'Home' ? 0 : event.key === 'End' ? entries.length - 1 : (index + (event.key === 'ArrowRight' ? 1 : -1) + entries.length) % entries.length
                selectTab(entries[next].identifier)
              }}
              onClick={() => selectTab(entry.identifier)}
            >
              <span className={entry.stop ? `workspace-tab-label candidate-stop-ground candidate-stop-ground-${entry.stop}` : 'workspace-tab-label'}>
                {entry.heading.map((part, position) => <span key={position} className={TONE_CLASS[part.tone]}>{part.text}</span>)}
                {entry.stop && <span className="candidate-stop-mark" data-stop={entry.stop} role="img" aria-label={STOP_NAME[entry.stop]} title={STOP_NAME[entry.stop]}>{entry.stop === 'incomplete' ? '❓' : '⚠'}</span>}
              </span>
            </button>
          })}</nav> : <div />}
          <div className="workspace-body">
            <section
              className="workspace-content"
              role="tabpanel"
              id={`proposal-panel-${tab?.identifier ?? ''}`}
              aria-labelledby={`proposal-tab-${tab?.identifier ?? ''}`}
              data-tab={tab?.identifier ?? ''}
              tabIndex={0}
            >
              {activeKey && tab && <div data-component={tab.component} className="proposal-tab-body">{errors[activeKey] ? <div className="panel-error" role="alert">{errors[activeKey]}</div> : activeKey in views ? <ComponentErrorBoundary key={activeKey} context={`Component ${tab.component}, request ${activeKey}`}><ComponentView entry={activeComponent!} view={views[activeKey]} navigate={navigate} act={actFor(activeRequest!, tab.component)} shell={shell}/></ComponentErrorBoundary> : <p className="app-state" role="status">Loading…</p>}</div>}
            </section>
          </div>
        </div>
        <aside aria-label="Review" className="proposal-review-panel" hidden={presentation === 'closed'}>
          {presentation === 'rail' && <button className="review-rail-toggle" type="button" aria-expanded={false} onClick={() => setTheReviewOpen(true)}>Expand review</button>}
          <header className="proposal-review-sheet-header">
            <div className="review-action" role="button" tabIndex={0} onClick={() => setTheReviewOpen(false)} onKeyDown={event => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); setTheReviewOpen(false) } }}>Close review</div>
          </header>
          {reviewIsOpen && <ResizeHandle pane="review" fromTheRight label="Resize content and Review" fraction={fractionOf('review')} resize={resize} />}
          {reviewIsOpen && <div className="proposal-review-topline" ref={setReviewTopline} />}
          <div className={reviewIsOpen && panels.length > 1 ? 'proposal-review-layout peek-open' : 'proposal-review-layout'}>{presentation === 'rail' ? panels.slice(0, 1) : panels.length > 1
            ? [panels[0], <ResizeHandle key="peek-handle" pane="peek" fromTheRight label="Resize Review and Peek" fraction={fractionOf('peek')} resize={resize} />, ...panels.slice(1)]
            : panels}</div>
        </aside>
      </div>
    </div>
  </div>
}
