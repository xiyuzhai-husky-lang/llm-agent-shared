import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, expect, it } from 'vitest'
import { StrictMode } from 'react'
import example from '../fixtures/lean-bench.json'
import { BenchmarkCaseDetails } from '../src/LeanBenchView'
import { buildBenchmarkTree } from '../src/benchExplorerModel'
import { parseLeanBenchData } from '../src/data'

const sample = parseLeanBenchData(example)
const data = { ...sample, macro: [], micro: [
  { ...sample.micro[0], id: 'ok', case: 'OK case', module: 'A', budget_ns: 1000, budget_level: 'ok' as const },
  { ...sample.micro[0], id: 'warning', case: 'Warning case', module: 'B', budget_ns: 500, budget_level: 'warn' as const },
  { ...sample.micro[0], id: 'alarm', case: 'Alarm case', module: 'C', budget_ns: 100, budget_level: 'alarm' as const },
] }
afterEach(cleanup)

it('exposes a module/case subtree ordered by budget severity for embedding in one explorer', () => {
  const tree = buildBenchmarkTree(data)
  expect(tree.map(node => node.label)).toEqual(['C', 'B', 'A'])
  const cases = tree.flatMap(node => node.children!)
  expect(cases.map(node => node.label)).toEqual(['Alarm case', 'Warning case', 'OK case'])
  expect(cases.map(node => node.tone)).toEqual(['error', 'warning', 'success'])
  expect(cases[0].meta).toContain('alarm')
  expect(cases[1].meta).toContain('warn')
  expect(tree.map(node => node.count)).toEqual([1, 1, 1])
})

it('renders the reusable case detail without adding another explorer or columns', () => {
  const entry = buildBenchmarkTree(data)[0].children![0].value!
  const { container } = render(<StrictMode><BenchmarkCaseDetails data={data} entry={entry} /></StrictMode>)
  const detail = screen.getByRole('region', { name: 'Selected benchmark details' })
  for (const label of ['ns_median', 'Min / max', 'Iterations × batches', 'Budget', 'Budget level', 'Nodes / steps']) {
    expect(within(detail).getByText(label)).toBeTruthy()
  }
  expect(within(detail).getByText('alarm', { selector: 'span' })).toBeTruthy()
  expect(container.querySelector('nav, .report-panels, .lean-bench-columns, table, input')).toBeNull()
})

it('expands the process tree in place inside the selected case', () => {
  const process = [{ depth: 0, label: 'Root call', us: 1 }, { depth: 1, label: 'Child call', us: 0.5 }]
  const measured = { ...data, micro: [{ ...data.micro[2], process }] }
  const entry = buildBenchmarkTree(measured)[0].children![0].value!
  render(<BenchmarkCaseDetails data={measured} entry={entry} />)
  fireEvent.click(screen.getByRole('link', { name: /process/ }))
  const tree = screen.getByRole('list', { name: 'Process call tree' })
  fireEvent.click(within(tree).getByRole('link', { name: /Root call/ }))
  expect(within(tree).getByText('Child call')).toBeTruthy()
  expect(within(tree).getByText('0.5 µs')).toBeTruthy()
  expect(screen.queryByRole('separator')).toBeNull()
})

it('groups macro measurements by Lean module and labels the case without a file extension', () => {
  const tree = buildBenchmarkTree({ ...sample, micro: [] })
  const macro = sample.macro[0]
  expect(tree[0].label).toBe(macro.file.replace(/^lean-src\//, '').replace(/\.lean$/, '').replaceAll('/', '.'))
  expect(tree[0].children![0].label).toBe(macro.file.split('/').pop()!.replace(/\.lean$/, ''))
  expect(tree[0].children![0].value?.row).toBe(macro)
})
