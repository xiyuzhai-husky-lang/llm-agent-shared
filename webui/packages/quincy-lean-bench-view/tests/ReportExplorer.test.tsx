import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { ReportExplorer } from '../src/ReportExplorer'
import { ExplorerPanels } from '../src/ExplorerPanels'
import type { ExplorerNode } from '../src/explorerModel'

afterEach(() => { cleanup(); vi.restoreAllMocks() })
const nodes: ExplorerNode<string>[] = [
  { key: 'errors', label: 'failed', count: 1, defaultExpanded: true, children: [{ key: 'a', label: 'A', count: 1, value: 'detail A' }] },
  { key: 'passes', label: 'passed', count: 1, children: [{ key: 'b', label: 'B', count: 1, value: 'detail B' }] },
]
const view = (tree = false) => <ReportExplorer nodes={nodes} tree={tree} label="Results" filterLabel="Filter results" renderDetails={value => <pre>{value}</pre>} />

it('mounts only expanded rows, supports search and collapse, and restores the unfiltered tree', () => {
  render(view())
  expect(screen.getByText('detail A')).toBeTruthy()
  expect(screen.queryByRole('button', { name: 'B' })).toBeNull()
  fireEvent.change(screen.getByRole('searchbox'), { target: { value: 'B' } })
  expect(screen.getByRole('button', { name: 'B' })).toBeTruthy()
  fireEvent.click(screen.getByRole('button', { name: 'passed 1' }))
  expect(screen.queryByRole('button', { name: 'B' })).toBeNull()
  fireEvent.change(screen.getByRole('searchbox'), { target: { value: '' } })
  expect(screen.getByRole('button', { name: 'failed 1' }).getAttribute('aria-expanded')).toBe('true')
  expect(screen.getByRole('button', { name: 'passed 1' }).getAttribute('aria-expanded')).toBe('false')
})

it('moves selection into phone leaves without a separator, preserving it on a layout change', () => {
  const mounted = render(view())
  fireEvent.click(screen.getByRole('button', { name: 'passed 1' }))
  fireEvent.click(screen.getByRole('button', { name: 'B' }))
  mounted.rerender(view(true))
  expect(screen.getByRole('button', { name: 'B' }).closest('li')!.contains(screen.getByText('detail B'))).toBe(true)
  expect(screen.queryByRole('separator')).toBeNull()
  fireEvent.click(screen.getByRole('button', { name: 'B' }))
  expect(screen.queryByText('detail B')).toBeNull()
})

it('resizes only its own panels by keyboard and clamps the minimum width', () => {
  vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(800)
  const bounds = vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue({ width: 320 } as DOMRect)
  const { container } = render(<><ExplorerPanels label="First" left="left" right="right" /><ExplorerPanels label="Second" left="other" right="detail" /></>)
  const panels = container.querySelectorAll<HTMLElement>('.report-panel-left')
  fireEvent.keyDown(screen.getByRole('separator', { name: 'First' }), { key: 'ArrowRight' })
  expect(panels[0].style.flexBasis).toBe('344px')
  expect(panels[1].style.flexBasis).toBe('320px')
  bounds.mockReturnValue({ width: 145 } as DOMRect)
  act(() => fireEvent.keyDown(screen.getByRole('separator', { name: 'First' }), { key: 'ArrowLeft' }))
  expect(panels[0].style.flexBasis).toBe('140px')
})
