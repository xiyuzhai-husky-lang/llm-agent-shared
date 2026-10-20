import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, expect, it } from 'vitest'
import example from '../fixtures/lean-bench.json'
import { LeanBenchView } from '../src/LeanBenchView'
import { parseLeanBenchData, type LeanBenchMicro } from '../src/data'
import { microComparison } from '../src/comparison'

afterEach(() => { cleanup(); window.history.replaceState(null, '', window.location.pathname) })
const sample = parseLeanBenchData(example)
const micro = (id: string, overrides: Partial<LeanBenchMicro> = {}): LeanBenchMicro => ({
  ...sample.micro[0], id, case: id, process: undefined,
  budget_ns: undefined, budget_level: undefined, over_budget: undefined, alarm_factor: undefined, nodes: undefined,
  baseline_ns_median: undefined, delta_ratio: undefined, slowdown: undefined, ...overrides,
})
const explorer = () => screen.getByRole('complementary', { name: 'Benchmark explorer' })
const details = () => screen.getByRole('region', { name: 'Selected benchmark details' })
const rows = () => [...explorer().querySelectorAll('[data-lean-bench-id]')]
const selected = () => within(explorer()).getByRole('button', { current: 'page' })
const select = (name: string) => fireEvent.click(within(explorer()).getByRole('button', { name }))
const field = (name: string) => within(details()).getByText(name, { selector: 'dt' }).nextElementSibling!.textContent

it('renders module → case groups and macro files with timings and status marks in the explorer', () => {
  const data = { ...sample, micro: [
    micro('slow', { module: 'Module.A', slowdown: true, delta_ratio: 0.3 }),
    micro('unsupported', { module: 'Module.B', status: 'unsupported' }),
    micro('failed', { module: 'Module.A', status: 'fail' }),
  ], macro: [{ ...sample.macro[0], slowdown: true }] }
  render(<LeanBenchView data={data} />)
  expect(within(explorer()).getByRole('button', { name: 'Module.A' })).toBeTruthy()
  expect(within(explorer()).getByRole('button', { name: 'Module.B' })).toBeTruthy()
  expect(within(explorer()).getByRole('button', { name: 'Macro' })).toBeTruthy()
  expect(rows().map((row) => row.getAttribute('data-lean-bench-id'))).toEqual(['slow', 'failed', 'unsupported', sample.macro[0].id])
  expect(within(explorer()).getAllByText('920')).toHaveLength(3)
  expect(within(explorer()).getByText('12.35')).toBeTruthy()
  expect(within(explorer()).getAllByRole('img', { name: 'Slowdown' })).toHaveLength(2)
  expect(within(explorer()).getByRole('img', { name: 'Failed' }).classList.contains('lean-bench-fail')).toBe(true)
  expect(within(explorer()).getByRole('img', { name: 'Unsupported' }).classList.contains('lean-bench-unsupported')).toBe(true)
  expect(selected().textContent).toBe('slow')
  expect(window.location.hash).toBe('#slow')
  expect(field('Delta')).toBe('+30% · Slowdown')
})

it('shows the first case details and run header, then replaces details on case selection', () => {
  render(<LeanBenchView data={{ ...sample, micro: [sample.micro[0], micro('second', { input: 'x', output: 'Var x', batches: 7 })] }} />)
  expect(field('Id')).toBe(sample.micro[0].id)
  expect(field('Module')).toBe(sample.micro[0].module)
  expect(field('Input → output')).toBe('a\n→\nVar a')
  expect(field('ns_median')).toBe('920 ± 40 ns/call')
  expect(field('Min / max')).toBe('905 / 960 ns/call')
  expect(field('Iterations × batches')).toBe('10,000 × 5')
  expect(field('Mode')).toBe('interpreted')
  expect(field('Baseline')).toBe('2,687 ns/call')
  expect(within(details()).getByText(/>20% growth and >3 × the larger error/)).toBeTruthy()
  expect(within(details()).getByText(/median of 5 batches/)).toBeTruthy()
  const header = screen.getByRole('region', { name: 'Lean bench' }).querySelector('.lean-bench-header')!
  expect(header.textContent).toContain(sample.commit)
  expect(header.textContent).toContain(sample.generated_at)
  expect(header.textContent).toContain(sample.host)
  expect(header.textContent).toContain('Micro cases1')
  select('second')
  expect(field('Id')).toBe('second')
  expect(field('Input → output')).toBe('x\n→\nVar x')
  expect(field('Iterations × batches')).toBe('10,000 × 7')
  expect(within(details()).queryByText(sample.micro[0].id)).toBeNull()
  expect(window.location.hash).toBe('#second')
  expect(selected().textContent).toBe('second')
})

it('opens process selections and their children in successive columns to the right', () => {
  render(<LeanBenchView data={{ ...sample, micro: [micro('decode', { process: [
    { depth: 0, label: 'Root call', us: 1 },
    { depth: 1, label: 'Child call', us: 0.8 },
    { depth: 2, label: 'Grandchild call', us: 0.5 },
    { depth: 1, label: 'Sibling call', us: 0.1 },
    { depth: 0, label: 'Next root', us: 2 },
  ] }), micro('other')] }} />)
  const tree = screen.getByRole('list', { name: 'Process call tree' })
  expect(screen.queryByRole('region', { name: 'Process details: Root call' })).toBeNull()
  fireEvent.click(within(tree).getByRole('button', { name: /Root call 1 µs/ }))
  const root = screen.getByRole('region', { name: 'Process details: Root call' })
  expect(root.previousElementSibling).toBe(details())
  expect(within(root).getByText('1 µs')).toBeTruthy()
  expect(within(root).queryByText('Grandchild call')).toBeNull()
  fireEvent.click(within(root).getByRole('button', { name: /Child call/ }))
  const child = screen.getByRole('region', { name: 'Process details: Child call' })
  expect(child.previousElementSibling).toBe(root)
  fireEvent.click(within(child).getByRole('button', { name: /Grandchild call/ }))
  const grandchild = screen.getByRole('region', { name: 'Process details: Grandchild call' })
  expect(grandchild.previousElementSibling).toBe(child)
  expect(within(grandchild).getByText('No child calls.')).toBeTruthy()
  fireEvent.click(within(root).getByRole('button', { name: /Sibling call/ }))
  expect(screen.queryByRole('region', { name: 'Process details: Child call' })).toBeNull()
  expect(screen.queryByRole('region', { name: 'Process details: Grandchild call' })).toBeNull()
  expect(screen.getByRole('region', { name: 'Process details: Sibling call' }).previousElementSibling).toBe(root)
  fireEvent.click(screen.getByRole('button', { name: 'Close process column 2' }))
  expect(screen.queryByRole('region', { name: 'Process details: Sibling call' })).toBeNull()
  expect(window.location.hash).toBe('#decode')
  select('other')
  expect(screen.queryByRole('region', { name: 'Process details: Root call' })).toBeNull()
  expect(screen.queryByRole('dialog')).toBeNull()
})

it('collapses process branches without hiding siblings or opening details inline', () => {
  render(<LeanBenchView data={{ ...sample, micro: [micro('decode', { process: [
    { depth: 0, label: 'Root', us: 1 }, { depth: 1, label: 'Child', us: 0.8 },
    { depth: 2, label: 'Grandchild', us: 0.5 }, { depth: 1, label: 'Sibling', us: 0.1 }, { depth: 0, label: 'Next root', us: 2 },
  ] })] }} />)
  const tree = screen.getByRole('list', { name: 'Process call tree' })
  fireEvent.click(within(tree).getByRole('button', { name: 'Collapse Child' }))
  expect(within(tree).queryByText('Grandchild')).toBeNull()
  expect(within(tree).getByText('Sibling')).toBeTruthy()
  expect(within(tree).getByRole('button', { name: 'Expand Child' }).getAttribute('aria-expanded')).toBe('false')
  fireEvent.click(within(tree).getByRole('button', { name: 'Collapse Root' }))
  expect(within(tree).queryByText('Sibling')).toBeNull()
  expect(within(tree).getByText('Next root')).toBeTruthy()
  fireEvent.click(within(tree).getByRole('button', { name: 'Expand Root' }))
  expect(within(tree).queryByText('Grandchild')).toBeNull()
  fireEvent.click(within(tree).getByRole('button', { name: 'Expand Child' }))
  fireEvent.click(within(tree).getByRole('button', { name: /Grandchild 0.5/ }))
  expect(screen.getByRole('region', { name: 'Process details: Grandchild' })).toBeTruthy()
  fireEvent.click(within(tree).getByRole('button', { name: 'Collapse Root' }))
  expect(screen.queryByRole('region', { name: 'Process details: Grandchild' })).toBeNull()
})

it('filters modules, case names, ids and macro paths, retaining the selected details', () => {
  render(<LeanBenchView data={{ ...sample, micro: [micro('alpha', { module: 'Module.A' }), micro('beta', { module: 'Module.B', case: 'Decode a variable' })] }} />)
  const search = screen.getByRole('searchbox')
  const ids = () => rows().map((row) => row.getAttribute('data-lean-bench-id'))
  fireEvent.change(search, { target: { value: '  ALPha  ' } })
  expect(ids()).toEqual(['alpha'])
  fireEvent.change(search, { target: { value: 'variable' } })
  expect(ids()).toEqual(['beta'])
  fireEvent.change(search, { target: { value: 'module.b' } })
  expect(ids()).toEqual(['beta'])
  fireEvent.change(search, { target: { value: 'Tests/Term' } })
  expect(ids()).toEqual([sample.macro[0].id])
  fireEvent.change(search, { target: { value: 'missing' } })
  expect(screen.getByText('No matching cases or files.')).toBeTruthy()
  expect(field('Id')).toBe('alpha')
})

it('navigates visible cases and macro files with arrows, Home/End and the slash shortcut', () => {
  render(<LeanBenchView data={{ ...sample, micro: [micro('alpha', { module: 'A' }), micro('beta', { module: 'B' }), micro('gamma', { module: 'C' })] }} />)
  fireEvent.keyDown(document.body, { key: 'ArrowDown' })
  expect(field('Id')).toBe('beta')
  fireEvent.keyDown(selected(), { key: 'ArrowUp' })
  selected().focus()
  fireEvent.keyDown(selected(), { key: 'ArrowUp' })
  expect(selected().textContent).toBe('alpha')
  fireEvent.keyDown(selected(), { key: 'ArrowDown' })
  expect(document.activeElement).toBe(selected())
  expect(field('Id')).toBe('beta')
  fireEvent.keyDown(selected(), { key: 'End' })
  expect(field('Id')).toBe(sample.macro[0].id)
  fireEvent.keyDown(selected(), { key: 'ArrowDown' })
  expect(field('Id')).toBe(sample.macro[0].id)
  fireEvent.keyDown(selected(), { key: 'Home' })
  expect(field('Id')).toBe('alpha')
  fireEvent.click(within(explorer()).getByRole('button', { name: 'B' }))
  fireEvent.keyDown(selected(), { key: 'ArrowDown' })
  expect(field('Id')).toBe('gamma')
  fireEvent.keyDown(selected(), { key: '/' })
  expect(document.activeElement).toBe(screen.getByRole('searchbox'))
  fireEvent.change(screen.getByRole('searchbox'), { target: { value: 'beta' } })
  fireEvent.keyDown(screen.getByRole('searchbox'), { key: 'ArrowDown' })
  expect(field('Id')).toBe('beta')
  expect(window.location.hash).toBe('#beta')
  const typing = new KeyboardEvent('keydown', { key: '/', bubbles: true, cancelable: true })
  screen.getByRole('searchbox').dispatchEvent(typing)
  expect(typing.defaultPrevented).toBe(false)
})

it('reads encoded case hashes and follows hash changes and browser history', () => {
  window.history.replaceState({ test: true }, '', '#case%20%2F%20%CE%B1')
  render(<LeanBenchView data={{ ...sample, micro: [micro('first'), micro('case / α')] }} />)
  expect(field('Id')).toBe('case / α')
  select('first')
  expect(window.location.hash).toBe('#first')
  expect(window.history.state).toEqual({ test: true })
  window.history.replaceState(null, '', '#case%20%2F%20%CE%B1')
  fireEvent(window, new PopStateEvent('popstate'))
  expect(field('Id')).toBe('case / α')
  window.history.replaceState(null, '', `#${sample.macro[0].id}`)
  fireEvent(window, new HashChangeEvent('hashchange'))
  expect(field('Id')).toBe(sample.macro[0].id)
})

it.each(['#missing', '#%E0%A4%A'])('falls back to the first case for an invalid hash %s', (hash) => {
  window.history.replaceState(null, '', hash)
  render(<LeanBenchView data={sample} />)
  expect(field('Id')).toBe(sample.micro[0].id)
  expect(window.location.hash).toBe(`#${sample.micro[0].id}`)
})

it('gives duplicate ids distinct deep links and selects the first case after data replacement', () => {
  const data = { ...sample, micro: [micro('same', { module: 'A', case: 'Case A' }), micro('same', { module: 'B', case: 'Case B' })] }
  const view = render(<LeanBenchView data={data} />)
  select('Case B')
  const hash = window.location.hash
  expect(hash).not.toBe('#same')
  view.unmount()
  const reopened = render(<LeanBenchView data={data} />)
  expect(field('Module')).toBe('B')
  expect(window.location.hash).toBe(hash)
  reopened.rerender(<LeanBenchView data={{ ...sample, micro: [micro('new')] }} />)
  expect(field('Id')).toBe('new')
  expect(window.location.hash).toBe('#new')
})

it('selects macro files and shows file, uses, wall time, per-use timing, baseline and delta', () => {
  render(<LeanBenchView data={sample} />)
  fireEvent.click(rows().at(-1)!.querySelector('td')!)
  expect(field('File')).toBe(sample.macro[0].file)
  expect(field('Uses')).toBe('1,000')
  expect(field('wall_ms')).toBe('12,345.6 ms')
  expect(field('ms/use')).toBe('12.35 ms/use')
  expect(field('Baseline')).toBe('13 ms/use')
  expect(field('Delta')).toBe('-5%')
  expect(field('Mode')).toBe('interpreted')
  expect(within(details()).queryByText('ns_median')).toBeNull()
  expect(details().textContent).not.toContain('±')
  expect(window.location.hash).toBe(`#${sample.macro[0].id}`)
})

it.each([
  [120, 0, 0, false], // Exactly 20%, so not a slowdown.
  [121, 7, 0, false], // Exactly 3 × error, so not a slowdown.
  [121, 0, 7, false], // Baseline uncertainty matters too.
  [122, 7, 1, true],
  [110, 1, 1, false],
])('requires both strict slowdown thresholds: %i ns, errors %i/%i', (current, error, baselineError, slowdown) => {
  expect(microComparison(micro('a', { ns_median: current, error_ns: error }),
    micro('a', { ns_median: 100, error_ns: baselineError })).slowdown).toBe(slowdown)
})

it('matches baselines by module and id, suppresses unsupported comparisons, and computes macro deltas', () => {
  const data = { ...sample, micro: [
    micro('same', { case: 'Case A', module: 'A', ns_median: 150, error_ns: 1 }),
    micro('same', { case: 'Case B', module: 'B', ns_median: 80, error_ns: 1 }),
    micro('unsupported', { status: 'unsupported', slowdown: true, delta_ratio: 9 }),
  ], macro: [{ ...sample.macro[0], ms_per_use: 15, delta_ratio: -1, slowdown: true }] }
  const baseline = { ...sample, micro: [
    micro('same', { module: 'B', ns_median: 100, error_ns: 1 }),
    micro('same', { module: 'A', ns_median: 100, error_ns: 1 }),
  ], macro: [{ ...sample.macro[0], ms_per_use: 10 }] }
  render(<LeanBenchView data={data} baseline={baseline} />)
  expect(field('Delta')).toBe('+50% · Slowdown')
  expect(field('Baseline')).toContain('100 ns/call')
  expect(rows()[0].classList.contains('lean-bench-slow')).toBe(true)
  select('Case B')
  expect(field('Delta')).toBe('-20%')
  expect(rows()[1].classList.contains('lean-bench-slow')).toBe(false)
  select('unsupported')
  expect(field('Delta')).toBe('—')
  expect(rows()[2].classList.contains('lean-bench-slow')).toBe(false)
  select('Macro1000.lean')
  expect(field('Baseline')).toContain('10 ms/use')
  expect(field('Delta')).toBe('+50% · Slowdown')
})

it('never compares different modes, even when the file contains comparison fields', () => {
  render(<LeanBenchView data={{ ...sample, micro: [micro('a', { slowdown: true, delta_ratio: 1 })] }} baseline={{ ...sample, mode: 'precompiled' }} />)
  expect(screen.getByRole('status').textContent).toContain('measurement modes differ')
  expect(within(explorer()).queryByRole('img', { name: 'Slowdown' })).toBeNull()
  expect(field('Delta')).toBe('—')
  select('Macro1000.lean')
  expect(field('Delta')).toBe('—')
  expect(field('Baseline')).not.toContain('13 ms/use')
})

it('handles zero, missing and failed baselines, empty runs and macro-only runs', () => {
  expect(microComparison(micro('a', { ns_median: 0 }), micro('a', { ns_median: 0 }))).toEqual({ delta: undefined, slowdown: false })
  expect(microComparison(micro('a'), micro('a', { status: 'fail' }))).toEqual({ slowdown: false })
  expect(microComparison(micro('a', { ns_median: 80, baseline_ns_median: 100 }))).toEqual({ delta: -0.2, slowdown: false })
  const view = render(<LeanBenchView data={{ ...sample, micro: [], macro: [] }} />)
  expect(screen.getByText('No Lean bench measurements.')).toBeTruthy()
  expect(screen.queryByRole('region', { name: 'Selected benchmark details' })).toBeNull()
  fireEvent.keyDown(explorer(), { key: 'ArrowDown' })
  view.rerender(<LeanBenchView data={{ ...sample, micro: [] }} />)
  expect(field('Id')).toBe(sample.macro[0].id)
})

it('supports a compact embed and a collapsible phone explorer without discarding selection', () => {
  render(<LeanBenchView data={sample} compact />)
  expect(screen.getByRole('region', { name: 'Lean bench' }).classList.contains('compact')).toBe(true)
  // The control is hidden by CSS on desktop and available at the phone breakpoint.
  const toggle = screen.getByRole('button', { name: 'Hide explorer' })
  fireEvent.click(toggle)
  expect(toggle.getAttribute('aria-expanded')).toBe('false')
  expect(explorer().classList.contains('is-open')).toBe(false)
  expect(field('Id')).toBe(sample.micro[0].id)
  fireEvent.click(toggle)
  expect(toggle.getAttribute('aria-expanded')).toBe('true')
  expect(explorer().classList.contains('is-open')).toBe(true)
})

it.each([false, true])('shows independent yellow and red budget marks and details (compact=%s)', (compact) => {
  const data = { ...sample, micro: [
    micro('yellow', { ns_median: 6000, nodes: 2, budget_ns: 4000, budget_level: 'warn', over_budget: true, slowdown: true }),
    micro('red', { ns_median: 9000, nodes: 2, budget_ns: 4000, budget_level: 'alarm', over_budget: true, alarm_factor: 2 }),
  ], macro: [{ ...sample.macro[0], ms_per_use: 151, budget_ns: 50_000_000, budget_level: 'alarm' as const, over_budget: true }] }
  render(<LeanBenchView data={data} compact={compact} />)
  const marks = (name: string) => within(explorer()).getAllByRole('img', { name })
  expect(marks('Over budget (warning)')).toHaveLength(1)
  expect(marks('Over budget (alarm)')).toHaveLength(2)
  expect(marks('Over budget (warning)')[0].classList.contains('lean-bench-budget-warn')).toBe(true)
  expect(marks('Over budget (alarm)')[0].classList.contains('lean-bench-budget-alarm')).toBe(true)
  expect(marks('Slowdown')).toHaveLength(1) // A yellow warning must not hide a relative failure.
  expect(within(details()).getByText('over budget · warning')).toBeTruthy()
  expect(field('Budget')).toBe('4,000 ns/call')
  expect(field('Nodes / steps')).toBe('2')
  expect(field('Per node / step')).toBe('3,000 ns / 2,000 ns budget')
  expect(field('Budget level')).toContain('Yellow: above budget through 3×. Red: above 3×.')
  const header = screen.getByRole('region', { name: 'Lean bench' }).querySelector('.lean-bench-header')!
  expect(header.textContent).toContain('Warnings1')
  expect(header.textContent).toContain('Alarms2')
  select('red')
  expect(within(details()).getByText('over budget · alarm')).toBeTruthy()
  expect(field('Budget level')).toContain('Red: above 2×.')
  // Phone explorer uses the same marks and details through collapse/reopen.
  fireEvent.click(screen.getByRole('button', { name: 'Hide explorer' }))
  expect(within(details()).getByText('over budget · alarm')).toBeTruthy()
  fireEvent.click(screen.getByRole('button', { name: 'Show explorer' }))
  expect(marks('Over budget (alarm)')).toHaveLength(2)
  select('Macro1000.lean')
  expect(field('Budget')).toBe('50 ms/use')
  expect(within(details()).queryByText('Nodes / steps')).toBeNull()
  expect(within(details()).getByText('over budget · alarm')).toBeTruthy()
})

it('combines budget and text filters, expands matches, and navigates only matching entries', () => {
  render(<LeanBenchView data={{ ...sample, micro: [micro('ok'),
    micro('yellow', { module: 'Yellow', budget_level: 'warn', over_budget: true, budget_ns: 500 }),
    micro('red', { module: 'Red', budget_level: 'alarm', over_budget: true, budget_ns: 200 }),
  ], macro: [{ ...sample.macro[0], budget_level: 'alarm', over_budget: true, budget_ns: 1_000_000 }] }} />)
  const ids = () => rows().map((row) => row.getAttribute('data-lean-bench-id'))
  const filter = (name: string) => within(screen.getByRole('group', { name: 'Budget filter' })).getByRole('button', { name })
  fireEvent.click(within(explorer()).getByRole('button', { name: 'Yellow' }))
  fireEvent.click(filter('warn+alarm'))
  expect(filter('warn+alarm').getAttribute('aria-pressed')).toBe('true')
  expect(ids()).toEqual(['yellow', 'red', sample.macro[0].id])
  expect(field('Id')).toBe('ok') // Filtering retains detail selection.
  fireEvent.keyDown(document.body, { key: 'ArrowDown' })
  expect(field('Id')).toBe('yellow')
  fireEvent.click(filter('alarm only'))
  expect(ids()).toEqual(['red', sample.macro[0].id])
  fireEvent.keyDown(document.body, { key: 'ArrowDown' })
  expect(field('Id')).toBe('red')
  fireEvent.change(screen.getByRole('searchbox'), { target: { value: 'Macro1000' } })
  expect(ids()).toEqual([sample.macro[0].id])
  fireEvent.change(screen.getByRole('searchbox'), { target: { value: 'yellow' } })
  expect(screen.getByText('No matching cases or files.')).toBeTruthy()
  fireEvent.click(filter('warn+alarm'))
  expect(ids()).toEqual(['yellow'])
  fireEvent.change(screen.getByRole('searchbox'), { target: { value: '' } })
  fireEvent.click(filter('All'))
  expect(ids()).toEqual(['ok', 'red', sample.macro[0].id]) // Restore the user's collapsed group.
})

it('keeps absolute alarms across baseline changes and marks old data as unbudgeted', () => {
  const data = { ...sample, micro: [micro('red', { budget_ns: 200, over_budget: true, budget_level: 'alarm' })], macro: [] }
  const view = render(<LeanBenchView data={data} baseline={{ ...sample, mode: 'precompiled' }} />)
  expect(within(details()).getByText('over budget · alarm')).toBeTruthy()
  view.rerender(<LeanBenchView data={data} baseline={{ ...sample, micro: [micro('red', { ns_median: 2000 })] }} />)
  expect(within(details()).getByText('over budget · alarm')).toBeTruthy()
  expect(field('Delta')).toBe('-54%')
  view.rerender(<LeanBenchView data={{ ...sample, micro: [micro('legacy')], macro: [] }} />)
  expect(field('Budget')).toBe('Not recorded')
  expect(field('Nodes / steps')).toBe('1')
  expect(screen.getByText('Some measurements have no recorded absolute budget.')).toBeTruthy()
  expect(within(explorer()).queryByRole('img', { name: /Over budget/ })).toBeNull()
})
