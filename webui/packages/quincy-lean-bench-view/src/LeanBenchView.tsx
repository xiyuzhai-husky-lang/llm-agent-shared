import { useEffect, useId, useMemo, useRef, useState, type ReactNode } from 'react'
import type { LeanBenchData, LeanBenchProcess } from './data.ts'
import { budgetLevel } from './data.ts'
import { benchmarkEntries, entryName, entryGroup, entryStatus, type Entry } from './benchExplorerModel.ts'
import { useTreeLayout } from './useTreeLayout.ts'
import './lean-bench.css'
import './outcome-bench.css'

export interface LeanBenchViewProps {
  data: LeanBenchData
  baseline?: LeanBenchData
  compact?: boolean
  /** Force the inline tree independently of the phone breakpoint. */
  layout?: 'tree'
}

type NavigationEvent = Pick<globalThis.KeyboardEvent, 'key' | 'altKey' | 'ctrlKey' | 'metaKey' | 'target' | 'preventDefault'>
const numbers = new Intl.NumberFormat('en-US', { maximumFractionDigits: 3 })
const percent = new Intl.NumberFormat('en-US', { style: 'percent', maximumFractionDigits: 1, signDisplay: 'exceptZero' })

function hashSelection(entries: Entry[]) {
  if (typeof window === 'undefined') return entries[0]?.key
  try {
    const hash = decodeURIComponent(window.location.hash.slice(1))
    return (entries.find((entry) => entry.hash === hash || entry.row.id === hash) ?? entries[0])?.key
  } catch {
    return entries[0]?.key
  }
}

function writeHash(entry: Entry, replace = false) {
  const hash = `#${encodeURIComponent(entry.hash)}`
  if (window.location.hash !== hash) {
    window.history[replace ? 'replaceState' : 'pushState'](window.history.state, '', hash)
  }
}

export function LeanBenchView({ data, baseline, compact = false, layout }: LeanBenchViewProps) {
  const treeLayout = useTreeLayout(layout)
  const [filter, setFilter] = useState('')
  const [budgetFilter, setBudgetFilter] = useState<'all' | 'over' | 'alarm'>('all')
  const [explorerOpen, setExplorerOpen] = useState(true)
  const [collapsed, setCollapsed] = useState<Set<string>>(() => new Set())
  const initialized = useRef(false)
  const filterRef = useRef<HTMLInputElement>(null)
  const focusFilterOnOpen = useRef(false)
  const rootRef = useRef<HTMLElement>(null)
  const rowRefs = useRef(new Map<string, HTMLButtonElement | HTMLAnchorElement>())
  const explorerId = useId()
  const modesDiffer = baseline != null && baseline.mode !== data.mode
  const entries = useMemo(() => {
    const items = benchmarkEntries(data, baseline)
    // Keep module children together, in first-seen module order.
    const modules = [...new Set(data.micro.map((row) => row.module))]
    return [...modules.flatMap((module) => items.filter((entry) => entry.kind === 'micro' && entry.row.module === module)),
      ...items.filter((entry) => entry.kind === 'macro')]
  }, [data, baseline])
  const [selectedKey, setSelectedKey] = useState<string | null | undefined>(() => hashSelection(entries))
  const selected = selectedKey === null ? undefined : entries.find((entry) => entry.key === selectedKey) ?? entries[0]
  const groups = useMemo(() => {
    const query = filter.trim().toLocaleLowerCase('en-US')
    const groups = new Map<string, Entry[]>()
    for (const entry of entries) {
      const level = budgetLevel(entry.row)
      if (budgetFilter === 'over' && level !== 'warn' && level !== 'alarm') continue
      if (budgetFilter === 'alarm' && level !== 'alarm') continue
      const text = `${entry.row.id}\n${entryName(entry)}\n${entryGroup(entry)}\n${entry.kind === 'macro' ? entry.row.file : ''}`
      if (!text.toLocaleLowerCase('en-US').includes(query)) continue
      const key = JSON.stringify([entry.kind, entryGroup(entry)])
      if (!groups.has(key)) groups.set(key, [])
      groups.get(key)!.push(entry)
    }
    return [...groups].map(([key, items]) => ({ key, items, open: filter.trim() !== '' || budgetFilter !== 'all' || !collapsed.has(key) }))
  }, [entries, filter, budgetFilter, collapsed])
  const visible = groups.flatMap((group) => group.open ? group.items : [])

  useEffect(() => {
    const sync = () => {
      const key = treeLayout && initialized.current && !window.location.hash ? null : hashSelection(entries)
      if (entries.length) initialized.current = true
      setSelectedKey(key)
      const entry = entries.find((entry) => entry.key === key)
      if (entry != null) {
        writeHash(entry, true)
        setCollapsed((current) => {
          const next = new Set(current)
          next.delete(JSON.stringify([entry.kind, entryGroup(entry)]))
          return next
        })
      }
    }
    sync()
    window.addEventListener('hashchange', sync)
    window.addEventListener('popstate', sync)
    return () => {
      window.removeEventListener('hashchange', sync)
      window.removeEventListener('popstate', sync)
    }
  }, [entries, treeLayout])

  useEffect(() => {
    const focusFilter = (event: globalThis.KeyboardEvent) => {
      const target = event.target as HTMLElement | null
      if (event.key !== '/' || event.ctrlKey || event.metaKey || event.altKey || event.defaultPrevented
        || target?.closest('input, textarea, select, [contenteditable="true"]')) return
      // An embed handles its own focus, and leaves shortcuts in other workspaces alone.
      if (target !== document.body && target != null && !rootRef.current?.contains(target)) return
      event.preventDefault()
      setExplorerOpen(true)
      filterRef.current?.focus()
      focusFilterOnOpen.current = document.activeElement !== filterRef.current
    }
    window.addEventListener('keydown', focusFilter)
    return () => window.removeEventListener('keydown', focusFilter)
  }, [])

  useEffect(() => {
    if (explorerOpen && focusFilterOnOpen.current) {
      filterRef.current?.focus()
      focusFilterOnOpen.current = false
    }
  }, [explorerOpen])

  useEffect(() => {
    if (selected != null) rowRefs.current.get(selected.key)?.scrollIntoView?.({ block: 'nearest', inline: 'nearest' })
  }, [selected?.key])

  const select = (entry: Entry, focus = false) => {
    setSelectedKey(entry.key)
    writeHash(entry)
    if (focus) {
      const row = rowRefs.current.get(entry.key)
      row?.focus({ preventScroll: true })
      row?.scrollIntoView?.({ block: 'nearest', inline: 'nearest' })
    }
  }
  const navigate = (event: NavigationEvent) => {
    if (treeLayout && (event.target as HTMLElement | null)?.closest('[data-benchmark-details]')) return
    if (event.altKey || event.ctrlKey || event.metaKey || !['ArrowUp', 'ArrowDown', 'Home', 'End'].includes(event.key)) return
    if (event.target === filterRef.current && ['Home', 'End'].includes(event.key)) return
    if (visible.length === 0) return
    event.preventDefault()
    const index = visible.findIndex((entry) => entry.key === selected?.key)
    const next = event.key === 'Home' ? 0 : event.key === 'End' ? visible.length - 1
      : index < 0 ? (event.key === 'ArrowUp' ? visible.length - 1 : 0)
        : Math.max(0, Math.min(visible.length - 1, index + (event.key === 'ArrowDown' ? 1 : -1)))
    select(visible[next], true)
  }
  useEffect(() => {
    const navigateFromPage = (event: globalThis.KeyboardEvent) => {
      if (event.target === document.body && !event.defaultPrevented) navigate(event)
    }
    window.addEventListener('keydown', navigateFromPage)
    return () => window.removeEventListener('keydown', navigateFromPage)
  })
  const summary = [
    ['Micro cases', data.summary.micro_cases], ['Unsupported', data.summary.micro_unsupported], ['Failed', data.summary.micro_fail],
    ['Micro slowdowns', baseline == null ? data.summary.micro_slowdowns : entries.filter((entry) => entry.kind === 'micro' && entry.comparison.slowdown).length],
    ['Macro files', data.summary.macro_files],
    ['Macro slowdowns', baseline == null ? data.summary.macro_slowdowns : entries.filter((entry) => entry.kind === 'macro' && entry.comparison.slowdown).length],
    ['Warnings', entries.filter((entry) => budgetLevel(entry.row) === 'warn').length],
    ['Alarms', entries.filter((entry) => budgetLevel(entry.row) === 'alarm').length],
  ] as const

  const runHeader = <header className="lean-bench-header">
        <dl className="lean-bench-metadata">
          <Field label="Commit"><code>{data.commit}</code></Field>
          <Field label="Generated"><time dateTime={data.generated_at}>{data.generated_at}</time></Field>
          <Field label="Host">{data.host}</Field>
        </dl>
        <dl className="lean-bench-summary">{summary.map(([label, count]) => <Field key={label} label={label}>
          <span className={count > 0 ? label === 'Warnings' ? 'lean-bench-budget-warn' : label === 'Alarms' ? 'lean-bench-budget-alarm' : undefined : undefined}>{numbers.format(count)}</span>
        </Field>)}</dl>
        {entries.some((entry) => entry.row.budget_ns == null) && <p className="lean-bench-note">Some measurements have no recorded absolute budget.</p>}
        {modesDiffer && <p role="status">Baseline comparison unavailable: measurement modes differ.</p>}
      </header>

  if (treeLayout) return <section ref={rootRef} className={`lean-bench-view lean-bench-tree-layout${compact ? ' compact' : ''}`} aria-label="Lean bench">
    <header className="lean-bench-tree-header"><h2>Lean bench</h2></header>
    {runHeader}
    <nav aria-label="Benchmark explorer" onKeyDown={navigate}>
      <label className="lean-bench-filter">Filter <span aria-hidden="true">/</span>
        <input ref={filterRef} type="search" aria-label="Filter cases" value={filter}
          onChange={(event) => setFilter(event.target.value)} placeholder="Module, case or file" />
      </label>
      <ul className="lean-bench-tree" aria-label="Benchmark modules">{groups.map(({ key, items, open }) => <li key={key}>
        <a className="lean-bench-tree-toggle" href="#" aria-expanded={open} onClick={(event) => {
          event.preventDefault()
          setCollapsed((current) => { const next = new Set(current); if (next.has(key)) next.delete(key); else next.add(key); return next })
        }}><span aria-hidden="true">{open ? '▾' : '▸'}</span> {entryGroup(items[0])}</a>
        {open && <ul className="lean-bench-tree-children">{items.map((entry) => {
          const expanded = selected?.key === entry.key
          return <li key={entry.key} data-lean-bench-id={entry.row.id} className={`lean-bench-${entryStatus(entry)}`}>
            <a className="lean-bench-tree-toggle" href={`#${encodeURIComponent(entry.hash)}`} aria-expanded={expanded}
              aria-current={expanded ? 'page' : undefined} aria-label={entryName(entry)}
              ref={(node) => { if (node) rowRefs.current.set(entry.key, node); else rowRefs.current.delete(entry.key) }}
              onClick={(event) => {
                event.preventDefault()
                if (expanded) {
                  setSelectedKey(null)
                  window.history.pushState(window.history.state, '', window.location.pathname + window.location.search)
                } else select(entry)
              }}>
              <span aria-hidden="true">{expanded ? '▾' : '▸'}</span> <span>{entryName(entry)}</span> <StatusMark status={entryStatus(entry)} />
            </a>
            {expanded && <BenchmarkDetails key={entry.key} entry={entry} data={data} baseline={baseline} modesDiffer={modesDiffer} tree>
              {entry.kind === 'micro' && !!entry.row.process?.length && <InlineProcessTree process={entry.row.process} />}
            </BenchmarkDetails>}
          </li>
        })}</ul>}
      </li>)}</ul>
      {visible.length === 0 && <p className="lean-bench-empty">{entries.length === 0 ? 'No Lean bench measurements.' : groups.length === 0 ? 'No matching cases or files.' : 'Select a module to show its cases.'}</p>}
    </nav>
  </section>

  return <section ref={rootRef} className={`lean-bench-view debug-shell${compact ? ' compact' : ''}`} aria-label="Lean bench">
    <aside className={`lean-bench-explorer${explorerOpen ? ' is-open' : ''}`} aria-label="Benchmark explorer" onKeyDown={navigate}>
      <header className="lean-bench-explorer-header">
        <div className="lean-bench-explorer-title"><h2>Lean bench</h2>
          <button type="button" className="lean-bench-explorer-toggle" aria-expanded={explorerOpen} aria-controls={explorerId}
            onClick={() => setExplorerOpen((open) => !open)}>{explorerOpen ? 'Hide explorer' : 'Show explorer'}</button>
        </div>
        <label className="lean-bench-filter">Filter <span aria-hidden="true">/</span>
          <input ref={filterRef} type="search" aria-label="Filter cases" value={filter}
            onChange={(event) => setFilter(event.target.value)} placeholder="Module, case or file" />
        </label>
        <div className="lean-bench-budget-filter" role="group" aria-label="Budget filter">
          {([['all', 'All'], ['over', 'warn+alarm'], ['alarm', 'alarm only']] as const).map(([value, label]) =>
            <button key={value} type="button" aria-pressed={budgetFilter === value}
              onClick={() => setBudgetFilter(value)}>{label}</button>)}
        </div>
      </header>
      <div id={explorerId} className="lean-bench-explorer-list">
        {(['micro', 'macro'] as const).map((kind) => <table key={kind} aria-label={`${kind === 'micro' ? 'Micro' : 'Macro'} Lean bench measurements`}>
          <thead><tr><th scope="col">{kind === 'micro' ? 'Micro' : 'Macro'}</th><th scope="col">{kind === 'micro' ? 'ns/call' : 'ms/use'}</th><th scope="col"><span className="lean-bench-sr-only">Status</span></th></tr></thead>
          {groups.filter((group) => group.items[0].kind === kind).map(({ key, items, open }) => <tbody key={key}>
            <tr className="lean-bench-module"><th scope="rowgroup" colSpan={3}>
              <button type="button" aria-expanded={open} title={entryGroup(items[0])}
                onClick={() => setCollapsed((current) => {
                  const next = new Set(current)
                  if (next.has(key)) next.delete(key)
                  else next.add(key)
                  return next
                })}><span aria-hidden="true">{open ? '▾' : '▸'}</span> {entryGroup(items[0])}</button>
            </th></tr>
            {open && items.map((entry) => <tr key={entry.key} data-lean-bench-id={entry.row.id}
              className={`lean-bench-explorer-row lean-bench-${entryStatus(entry)} lean-bench-budget-${budgetLevel(entry.row) ?? 'unknown'}${selected?.key === entry.key ? ' active' : ''}`}
              onClick={() => select(entry, true)}>
              <th scope="row"><button type="button" ref={(node) => { if (node) rowRefs.current.set(entry.key, node); else rowRefs.current.delete(entry.key) }}
                aria-label={entryName(entry)} aria-current={selected?.key === entry.key ? 'page' : undefined}
                tabIndex={selected?.key === entry.key || !visible.some((item) => item.key === selected?.key) && visible[0]?.key === entry.key ? 0 : -1}
                title={`${entryName(entry)}\n${entry.row.id}`} onClick={(event) => { event.stopPropagation(); select(entry) }}>{entryName(entry)}
                {(budgetLevel(entry.row) === 'warn' || budgetLevel(entry.row) === 'alarm') &&
                  <small>over budget · {budgetLevel(entry.row) === 'alarm' ? 'alarm' : 'warning'}</small>}
              </button></th>
              <td className="lean-bench-number">{numbers.format(entry.kind === 'micro' ? entry.row.ns_median : entry.row.ms_per_use)}</td>
              <td><EntryMarks entry={entry} compact /></td>
            </tr>)}
          </tbody>)}
        </table>)}
        {visible.length === 0 && <p className="lean-bench-empty">{entries.length === 0 ? 'No Lean bench measurements.' : groups.length === 0 ? 'No matching cases or files.' : 'Select a module to show its cases.'}</p>}
      </div>
    </aside>
    <div className="lean-bench-detail pane">
      {runHeader}
      {selected ? <DetailColumns key={selected.key} entry={selected} data={data} baseline={baseline} modesDiffer={modesDiffer} />
        : <p className="lean-bench-empty">Select a case or file in the explorer.</p>}
    </div>
  </section>
}

function EntryMarks({ entry, compact = false }: { entry: Entry; compact?: boolean }) {
  const level = budgetLevel(entry.row)
  const over = level === 'warn' || level === 'alarm'
  const label = level === 'alarm' ? 'Over budget (alarm)' : 'Over budget (warning)'
  return <span className="lean-bench-marks">
    {(!over || entryStatus(entry) !== 'ok') && <StatusMark status={entryStatus(entry)} />}
    {over && <span className={`lean-bench-status lean-bench-budget-${level}`} role="img" aria-label={label} title={label}>
      {compact ? level === 'alarm' ? '!!' : '!' : `over budget · ${level === 'alarm' ? 'alarm' : 'warning'}`}
    </span>}
  </span>
}

function StatusMark({ status }: { status: ReturnType<typeof entryStatus> }) {
  const label = status === 'slow' ? 'Slowdown' : status === 'fail' ? 'Failed' : status === 'unsupported' ? 'Unsupported' : 'OK'
  return <span className={`lean-bench-status lean-bench-${status}`} role="img" aria-label={label} title={label}>
    {status === 'slow' ? '↑' : status === 'fail' ? '×' : status === 'unsupported' ? '—' : '·'}
  </span>
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return <div><dt>{label}</dt><dd>{children}</dd></div>
}

function DetailColumns({ entry, data, baseline, modesDiffer }: {
  entry: Entry; data: LeanBenchData; baseline?: LeanBenchData; modesDiffer: boolean
}) {
  const [path, setPath] = useState<number[]>([])
  const columnsRef = useRef<HTMLDivElement>(null)
  const process = entry.kind === 'micro' ? entry.row.process : undefined
  useEffect(() => setPath([]), [process])
  useEffect(() => {
    if (columnsRef.current) columnsRef.current.scrollLeft = path.length === 0 ? 0 : columnsRef.current.scrollWidth
  }, [path])
  const detail = <BenchmarkDetails key="case" entry={entry} data={data} baseline={baseline} modesDiffer={modesDiffer}>
      {process != null && process.length > 0 && <section className="lean-bench-process-section" aria-label={`Process for ${entryName(entry)}`}>
        <h4>Process <span>µs</span></h4>
        <ProcessTree process={process} selected={path[0]} onSelect={(index) => setPath([index])}
          onCollapse={(index) => { if (path[0] > index && path[0] < branchEnd(process, index)) setPath([]) }} />
      </section>}
    </BenchmarkDetails>
  const processColumns = process ? path.map((index, column) => process[index] && <section key={`${column}:${index}`}
      className="lean-bench-process-detail col col-cycle" aria-label={`Process details: ${process[index].label}`}>
      <header className="lean-bench-column-header"><h3>{process[index].label || '(unlabelled)'}</h3>
        <button type="button" aria-label={`Close process column ${column + 1}`} onClick={() => setPath(path.slice(0, column))}>Close</button></header>
      <dl className="lean-bench-fields"><Field label="Time">{numbers.format(process[index].us)} µs</Field><Field label="Depth">{process[index].depth}</Field></dl>
      <h4>Calls</h4>
      <ul className="lean-bench-process">
        {childIndices(process, index).map((child) => <li key={child}>
          <button type="button" className={path[column + 1] === child ? 'active' : undefined}
            aria-current={path[column + 1] === child ? 'true' : undefined}
            onClick={() => setPath([...path.slice(0, column + 1), child])}>
            <span>{process[child].label || '(unlabelled)'}</span>{' '}<span className="lean-bench-number">{numbers.format(process[child].us)} µs <span aria-hidden="true">›</span></span>
          </button>
        </li>)}
      </ul>
      {childIndices(process, index).length === 0 && <p className="lean-bench-note">No child calls.</p>}
    </section>) : []
  return <div className="lean-bench-columns cols" ref={columnsRef}>
    {[detail, ...processColumns]}
  </div>
}

function BenchmarkDetails({ entry, data, baseline, modesDiffer, tree = false, children }: {
  entry: Entry; data: LeanBenchData; baseline?: LeanBenchData; modesDiffer: boolean; tree?: boolean; children?: ReactNode
}) {
  const baselineValue = modesDiffer ? undefined : entry.kind === 'micro'
    ? entry.baseline?.ns_median ?? entry.row.baseline_ns_median : entry.baseline?.ms_per_use ?? entry.row.baseline_ms_per_use
  const unit = entry.kind === 'micro' ? 'ns/call' : 'ms/use'
  return <section data-benchmark-details className={tree ? "lean-bench-tree-details" : "lean-bench-case-detail col col-source"} aria-label="Selected benchmark details">
      {!tree && <header className="lean-bench-column-header"><h3>{entryName(entry)}</h3><EntryMarks entry={entry} /></header>}
      <dl className="lean-bench-fields">
        <Field label="Id"><code>{entry.row.id}</code></Field>
        {entry.kind === 'micro' ? <>
          <Field label="Module"><code>{entry.row.module}</code></Field>
          <Field label="Input → output"><pre>{entry.row.input}{'\n→\n'}{entry.row.output}</pre></Field>
          <Field label="ns_median"><span className="lean-bench-number">{numbers.format(entry.row.ns_median)} ± {numbers.format(entry.row.error_ns)} ns/call</span></Field>
          <Field label="Min / max"><span className="lean-bench-number">{numbers.format(entry.row.ns_min)} / {numbers.format(entry.row.ns_max)} ns/call</span></Field>
          <Field label="Iterations × batches">{numbers.format(entry.row.iterations)} × {numbers.format(entry.row.batches)}</Field>
          <Field label="Status">{entry.row.status}</Field>
        </> : <>
          <Field label="File"><code>{entry.row.file}</code></Field>
          <Field label="Uses">{numbers.format(entry.row.uses)}</Field>
          <Field label="wall_ms">{numbers.format(entry.row.wall_ms)} ms</Field>
          <Field label="ms/use">{numbers.format(entry.row.ms_per_use)} ms/use</Field>
        </>}
        <Field label="Budget">{entry.row.budget_ns == null ? 'Not recorded' :
          `${numbers.format(entry.row.budget_ns / (entry.kind === 'micro' ? 1 : 1_000_000))} ${unit}`}</Field>
        {entry.kind === 'micro' && <>
          <Field label="Nodes / steps">{numbers.format(entry.row.nodes ?? 1)}</Field>
          <Field label="Per node / step">{numbers.format(entry.row.ns_median / (entry.row.nodes ?? 1))} ns
            {entry.row.budget_ns != null && ` / ${numbers.format(entry.row.budget_ns / (entry.row.nodes ?? 1))} ns budget`}</Field>
        </>}
        {entry.row.budget_ns != null && <Field label="Budget level">
          <span className={`lean-bench-budget-${budgetLevel(entry.row) ?? 'unknown'}`}>{budgetLevel(entry.row) ?? 'Not recorded'}</span>
          <small>Yellow: above budget through {numbers.format(entry.row.alarm_factor ?? 3)}×. Red: above {numbers.format(entry.row.alarm_factor ?? 3)}×.</small>
        </Field>}
        <Field label="Mode">{data.mode}</Field>
        <Field label="Baseline">{baselineValue == null ? '—' : `${numbers.format(baselineValue)} ${unit}`}
          {baseline && <small><code>{baseline.commit}</code> · {baseline.mode}</small>}</Field>
        <Field label="Delta"><span className={entry.comparison.slowdown ? 'lean-bench-slow' : undefined}>
          {entry.comparison.delta == null ? '—' : percent.format(entry.comparison.delta)}{entry.comparison.slowdown ? ' · Slowdown' : ''}
        </span></Field>
      </dl>
      {tree && children}
      <p className="lean-bench-note">{entry.kind === 'micro'
        ? 'Micro slowdown: >20% growth and >3 × the larger error bound above baseline. Error bound = max(median − min, max − median).'
        : 'Macro: whole-file wall time ÷ uses; >20% slowdown flags come from the runner. Macro timings have no error bound.'}</p>
      <p className="lean-bench-note">Interpreted and precompiled measurements are recorded separately; only the same mode is compared.</p>
      {entry.kind === 'micro' && <p className="lean-bench-note">10 warm-up calls; ≥1,000 iterations and ≥100 ms; median of 5 batches using a nanosecond clock.</p>}
    {!tree && children}
  </section>
}

function InlineProcessTree({ process }: { process: LeanBenchProcess[] }) {
  const [open, setOpen] = useState(false)
  const roots: number[] = []
  for (let index = 0; index < process.length; index = branchEnd(process, index)) roots.push(index)
  return <ul className="lean-bench-tree-children"><li>
    <a className="lean-bench-tree-toggle" href="#" aria-expanded={open} onClick={(event) => { event.preventDefault(); setOpen(!open) }}>
      <span aria-hidden="true">{open ? '▾' : '▸'}</span> process
    </a>
    {open && <ul className="lean-bench-tree-children" aria-label="Process call tree">
      {roots.map((index) => <InlineProcessNode key={index} process={process} index={index} />)}
    </ul>}
  </li></ul>
}

function InlineProcessNode({ process, index }: { process: LeanBenchProcess[]; index: number }) {
  const [open, setOpen] = useState(false)
  const children = childIndices(process, index)
  const entry = process[index]
  const line = <><span>{entry.label || '(unlabelled)'}</span> <span className="lean-bench-number">{numbers.format(entry.us)} µs</span></>
  return <li>
    {children.length ? <a className="lean-bench-tree-toggle lean-bench-tree-call" href="#" aria-expanded={open}
      onClick={(event) => { event.preventDefault(); setOpen(!open) }}><span aria-hidden="true">{open ? '▾' : '▸'}</span> {line}</a>
      : <span className="lean-bench-tree-call lean-bench-tree-leaf">{line}</span>}
    {open && <ul className="lean-bench-tree-children">{children.map((child) => <InlineProcessNode key={child} process={process} index={child} />)}</ul>}
  </li>
}

function branchEnd(process: LeanBenchProcess[], index: number) {
  let end = index + 1
  while (end < process.length && process[end].depth > process[index].depth) end++
  return end
}

function childIndices(process: LeanBenchProcess[], index: number) {
  const result: number[] = []
  const end = branchEnd(process, index)
  for (let child = index + 1; child < end; child = branchEnd(process, child)) result.push(child)
  return result
}

function ProcessTree({ process, selected, onSelect, onCollapse }: {
  process: LeanBenchProcess[]; selected?: number; onSelect: (index: number) => void; onCollapse: (index: number) => void
}) {
  const [collapsed, setCollapsed] = useState<Set<number>>(() => new Set())
  let hiddenBelow: number | undefined
  return <ul className="lean-bench-process" aria-label="Process call tree">
    {process.map((entry, index) => {
      if (hiddenBelow != null && entry.depth > hiddenBelow) return null
      hiddenBelow = undefined
      const hasChildren = (process[index + 1]?.depth ?? -1) > entry.depth
      const closed = collapsed.has(index)
      if (closed && hasChildren) hiddenBelow = entry.depth
      return <li key={index} style={{ paddingInlineStart: `${entry.depth * 12}px` }}>
        {hasChildren ? <button type="button" className="lean-bench-disclosure" aria-label={`${closed ? 'Expand' : 'Collapse'} ${entry.label}`} aria-expanded={!closed}
          onClick={() => {
            setCollapsed((current) => { const next = new Set(current); if (closed) next.delete(index); else next.add(index); return next })
            if (!closed) onCollapse(index)
          }}><span aria-hidden="true">{closed ? '▸' : '▾'}</span></button> : <span className="lean-bench-disclosure" />}
        <button type="button" className={selected === index ? 'active' : undefined} aria-current={selected === index ? 'true' : undefined}
          onClick={() => onSelect(index)}><span>{entry.label || '(unlabelled)'}</span>{' '}<span className="lean-bench-number">{numbers.format(entry.us)} µs <span aria-hidden="true">›</span></span></button>
      </li>
    })}
  </ul>
}

/** A case detail for the shared Outcome tree, without its own explorer or columns. */
export function BenchmarkCaseDetails({ entry, data }: { entry: Entry; data: LeanBenchData }) {
  return <div className="lean-bench-view lean-bench-outcome outcome-case">
    <h3>{entryName(entry)}</h3>
    <BenchmarkDetails entry={entry} data={data} modesDiffer={false} tree>
      {entry.kind === 'micro' && !!entry.row.process?.length && <InlineProcessTree process={entry.row.process} />}
    </BenchmarkDetails>
  </div>
}
