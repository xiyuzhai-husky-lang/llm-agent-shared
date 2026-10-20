import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import example from '../fixtures/lean-bench.json'
import { LeanBenchView } from '../src/LeanBenchView'
import { parseLeanBenchData } from '../src/data'

const sample = parseLeanBenchData(example)
const data = { ...sample, micro: [
  { ...sample.micro[0], id: 'case / α', case: 'First case', module: 'Module.A', process: [
    { depth: 0, label: 'Root', us: 1 }, { depth: 1, label: 'Child', us: 0.8 },
    { depth: 2, label: 'Grandchild', us: 0.5 }, { depth: 1, label: 'Sibling', us: 0.1 },
    { depth: 0, label: 'Next root', us: 2 },
  ] },
  { ...sample.micro[0], id: 'second', case: 'Second case', module: 'Module.B', process: undefined },
] }
const link = (name: string | RegExp) => screen.getByRole('link', { name })
const click = (name: string | RegExp) => fireEvent.click(link(name))
const details = () => screen.getByRole('region', { name: 'Selected benchmark details' })
const field = (name: string) => within(details()).getByText(name, { selector: 'dt' }).nextElementSibling!.textContent
beforeEach(() => window.history.replaceState(null, '', '/'))
afterEach(() => { cleanup(); vi.unstubAllGlobals() })

it('nests case numbers, uncertainty and baseline delta inside its module and case', () => {
  const { container } = render(<LeanBenchView layout="tree" data={data} />)
  const row = link('First case').closest('li')!
  expect(row.contains(details())).toBe(true)
  expect(row.parentElement?.closest('li')).toBe(link('Module.A').closest('li'))
  expect(field('ns_median')).toBe('920 ± 40 ns/call')
  expect(field('Baseline')).toBe('2,687 ns/call')
  expect(field('Delta')).toBe('-66%')
  expect(link('First case').getAttribute('aria-expanded')).toBe('true')
  expect(link('Second case').getAttribute('aria-expanded')).toBe('false')
  expect(window.location.hash).toBe('#case%20%2F%20%CE%B1')
  expect(container.querySelector('.lean-bench-columns, .pane, .col, aside, [role="dialog"]')).toBeNull()
})

it('expands process and call levels as nested lists, collapsing without hiding siblings', () => {
  render(<LeanBenchView layout="tree" data={data} />)
  expect(screen.queryByRole('list', { name: 'Process call tree' })).toBeNull()
  click('process')
  expect(screen.getByRole('list', { name: 'Process call tree' }).closest('li')).toBe(link('process').closest('li'))
  expect(screen.queryByText('Child')).toBeNull()
  click(/Root 1 µs/)
  const root = link(/Root 1 µs/).closest('li')!
  expect(within(root).getByText('Child')).toBeTruthy()
  expect(screen.queryByText('Grandchild')).toBeNull()
  click(/Child 0.8 µs/)
  expect(within(link(/Child 0.8 µs/).closest('li')!).getByText('Grandchild')).toBeTruthy()
  click(/Child 0.8 µs/)
  expect(screen.queryByText('Grandchild')).toBeNull()
  expect(within(root).getByText('Sibling')).toBeTruthy()
  click(/Root 1 µs/)
  expect(screen.queryByText('Child')).toBeNull()
  expect(screen.getByText('Next root')).toBeTruthy()
  click('process')
  expect(screen.queryByRole('list', { name: 'Process call tree' })).toBeNull()
  expect(window.location.hash).toBe('#case%20%2F%20%CE%B1')
  expect(screen.queryByRole('region', { name: /Process details:/ })).toBeNull()
})

it('collapses modules and cases, keeping only the selected case expanded and syncing its hash', () => {
  render(<LeanBenchView layout="tree" data={data} />)
  click('Module.A')
  expect(screen.queryByRole('link', { name: 'First case' })).toBeNull()
  expect(screen.queryByRole('region', { name: 'Selected benchmark details' })).toBeNull()
  click('Module.A')
  expect(field('Id')).toBe('case / α')
  click('First case')
  expect(link('First case').getAttribute('aria-expanded')).toBe('false')
  expect(screen.queryByRole('region', { name: 'Selected benchmark details' })).toBeNull()
  expect(window.location.hash).toBe('')
  click('Second case')
  expect(field('Id')).toBe('second')
  expect(window.location.hash).toBe('#second')
  click('First case')
  expect(link('Second case').getAttribute('aria-expanded')).toBe('false')
  expect(field('Id')).toBe('case / α')
})

it('nests macro measurements and supports both macro group and file collapse', () => {
  render(<LeanBenchView layout="tree" data={data} />)
  click('Macro1000.lean')
  expect(field('File')).toBe(sample.macro[0].file)
  expect(field('ms/use')).toBe('12.35 ms/use')
  expect(field('Baseline')).toBe('13 ms/use')
  expect(field('Delta')).toBe('-5%')
  expect(details().textContent).not.toContain('±')
  expect(link('Macro1000.lean').closest('li')!.contains(details())).toBe(true)
  expect(window.location.hash).toBe(`#${sample.macro[0].id}`)
  click('Macro')
  expect(screen.queryByRole('link', { name: 'Macro1000.lean' })).toBeNull()
  click('Macro')
  expect(field('Id')).toBe(sample.macro[0].id)
  click('Macro1000.lean')
  expect(screen.queryByRole('region', { name: 'Selected benchmark details' })).toBeNull()
})

it('restores encoded hashes and reveals ancestors on external hash and history changes', () => {
  window.history.replaceState(null, '', '#second')
  render(<LeanBenchView layout="tree" data={data} />)
  expect(field('Id')).toBe('second')
  click('Module.A')
  window.history.replaceState(null, '', '#case%20%2F%20%CE%B1')
  fireEvent(window, new HashChangeEvent('hashchange'))
  expect(link('Module.A').getAttribute('aria-expanded')).toBe('true')
  expect(field('Id')).toBe('case / α')
  click('Macro')
  window.history.replaceState(null, '', `#${sample.macro[0].id}`)
  fireEvent(window, new PopStateEvent('popstate'))
  expect(link('Macro').getAttribute('aria-expanded')).toBe('true')
  expect(field('Id')).toBe(sample.macro[0].id)
  window.history.replaceState(null, '', '/')
  fireEvent(window, new PopStateEvent('popstate'))
  expect(screen.queryByRole('region', { name: 'Selected benchmark details' })).toBeNull()
})

it('filters and navigates cases while leaving keys in inline process details alone', () => {
  render(<LeanBenchView layout="tree" data={data} />)
  fireEvent.keyDown(link('First case'), { key: 'ArrowDown' })
  expect(field('Id')).toBe('second')
  expect(document.activeElement).toBe(link('Second case'))
  fireEvent.keyDown(document.body, { key: '/' })
  expect(document.activeElement).toBe(screen.getByRole('searchbox'))
  fireEvent.change(screen.getByRole('searchbox'), { target: { value: 'First case' } })
  expect(screen.queryByRole('link', { name: 'Second case' })).toBeNull()
  fireEvent.keyDown(screen.getByRole('searchbox'), { key: 'ArrowDown' })
  expect(field('Id')).toBe('case / α')
  fireEvent.keyDown(link('process'), { key: 'ArrowDown' })
  expect(field('Id')).toBe('case / α')
})

it('switches on live media changes without losing selection and supports a forced tree', () => {
  let matches = true
  const listeners = new Set<() => void>()
  vi.stubGlobal('matchMedia', vi.fn(() => ({ get matches() { return matches },
    addEventListener: (_type: string, listener: () => void) => listeners.add(listener),
    removeEventListener: (_type: string, listener: () => void) => listeners.delete(listener),
  })))
  const view = render(<LeanBenchView data={data} compact />)
  expect(view.container.querySelector('.lean-bench-tree-layout')).toBeTruthy()
  click('Second case')
  act(() => { matches = false; listeners.forEach((listener) => listener()) })
  expect(view.container.querySelector('.lean-bench-columns')).toBeTruthy()
  expect(field('Id')).toBe('second')
  act(() => { matches = true; listeners.forEach((listener) => listener()) })
  expect(link('Second case').getAttribute('aria-expanded')).toBe('true')
  act(() => { matches = false; listeners.forEach((listener) => listener()) })
  view.rerender(<LeanBenchView layout="tree" data={data} />)
  expect(view.container.querySelector('.lean-bench-columns')).toBeNull()
  expect(window.matchMedia).toHaveBeenCalledWith('(max-width: 760px)')
  view.unmount()
  expect(listeners.size).toBe(0)
})
