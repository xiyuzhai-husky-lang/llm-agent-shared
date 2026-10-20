import { afterEach, expect, test, vi } from 'vitest'
import { cleanup, fireEvent, render, waitFor } from '@testing-library/react'
import { SourceFrame } from './SourceFrame'

afterEach(cleanup)

test.each([undefined, false, true])('compact pill respects defaultOpen=%s and restores the full frame on click', async (defaultOpen) => {
  const loadSource = vi.fn(async () => 'outside\n  let value = 1;\n  value')
  const { container, getByRole } = render(<SourceFrame path="crates/demo.rs" line={2} endLine={3} oid="abcdef0123456"
    compactCollapsed defaultOpen={defaultOpen} loadSource={loadSource} />)
  const button = getByRole('button')
  expect(button.getAttribute('aria-expanded')).toBe(String(defaultOpen ?? false))
  if (!defaultOpen) {
    expect(getByRole('button', { name: 'src' })).toBe(button)
    expect(container.textContent).toBe('▸src')
    expect(container.querySelector('[title], .ks-loc-path, .ks-loc-line, .ks-body')).toBeNull()
    expect(loadSource).not.toHaveBeenCalled()
    fireEvent.click(button)
  }
  await waitFor(() => expect(container.querySelectorAll('.ks-line')).toHaveLength(2))
  expect(button.getAttribute('aria-expanded')).toBe('true')
  expect(container.querySelector('.ks-loc-path')?.textContent).toBe('crates/demo.rs')
  expect(container.querySelector('.ks-loc-line')?.textContent).toBe('L2-L3 @ abcdef0123')
  expect(container.querySelector('.ks-dedent')?.textContent).toBe('dedented 2')
  expect([...container.querySelectorAll('.ks-line')].map((row) => row.textContent)).toEqual(['let value = 1;', 'value'])
  fireEvent.click(button)
  expect(getByRole('button', { name: 'src' })).toBe(button)
  expect(button.getAttribute('aria-expanded')).toBe('false')
  expect(container.textContent).toBe('▸src')
  expect(container.querySelector('.ks-body')).toBeNull()
  fireEvent.click(button)
  expect(container.querySelectorAll('.ks-line')).toHaveLength(2)
  expect(loadSource).toHaveBeenCalledExactlyOnceWith('crates/demo.rs', 'abcdef0123456')
})

test('marks exact inline characters and stacks labels in input order after tabs and dedent', async () => {
  const loadSource = vi.fn(async () => 'unused\n\tα😀abc\n\tend')
  const { container } = render(<SourceFrame path="demo.rs" line={2} endLine={3} oid="abcdef0123" defaultOpen loadSource={loadSource} annotations={[
    { line: 2, start: 4, end: 6, severity: 'error', label: <strong>first</strong> },
    { line: 2, start: 2, end: 2, severity: 'warning', label: 'second' },
  ]} />)
  await waitFor(() => expect(container.querySelectorAll('.cs-annotation')).toHaveLength(2))
  expect(loadSource).toHaveBeenCalledWith('demo.rs', 'abcdef0123')
  expect(container.querySelector('.ks-dedent')?.textContent).toBe('dedented 4')
  const rows = [...container.querySelectorAll('.ks-row')]
  expect(rows.map((r) => r.querySelector('.ks-gutter')?.textContent)).toEqual(['2', '│', '│', '3'])
  expect(rows[1].classList.contains('cs-error')).toBe(true)
  expect(rows[2].classList.contains('cs-note')).toBe(true)
  expect(rows[0].querySelector('.ks-line')?.textContent).toBe('α😀abc')
  const marks = [...rows[0].querySelectorAll('mark.cs-span')]
  expect(marks.map((mark) => [mark.textContent, mark.className])).toEqual([
    ['α', 'cs-span cs-note'], ['abc', 'cs-span cs-error'],
  ])
  expect(marks[1].getAttribute('aria-label')).toBe('L2 C4-6')
  expect(rows[1].querySelector('.cs-annotation-content')?.firstElementChild?.textContent).toBe('└')
  expect(rows[2].querySelector('.cs-label')?.textContent).toBe('second')
  expect(rows[1].querySelector('.cs-label strong')?.textContent).toBe('first')
  expect(container.querySelector('.cs-annotation-prefix, .cs-underline')).toBeNull()
  expect(rows[1].querySelector('mark')).toBeNull()
})

test.each(['error', 'warning', 'note'] as const)('inline span and label share primary severity %s', async (severity) => {
  const { container } = render(<SourceFrame path="a.rs" line={1} endLine={1} defaultOpen loadSource={async () => 'abc'} annotations={[
    { line: 1, start: 1, end: 1, severity, label: severity },
  ]} />)
  await waitFor(() => expect(container.querySelector(`.cs-annotation.cs-${severity}`)).not.toBeNull())
  expect(container.querySelector(`.ks-line > mark.cs-span.cs-${severity}`)?.textContent).toBe('a')
})

test('marks across syntax tokens while preserving highlighting and Unicode characters', async () => {
  const { container } = render(<SourceFrame path="syntax.rs" line={1} endLine={2} defaultOpen loadSource={async () => '  let value = "a😀b";\n  end'} annotations={[
    { line: 1, start: 7, end: 19, label: 'expression' },
  ]} />)
  await waitFor(() => expect(container.querySelector('mark')?.textContent).toBe('value = "a😀b"'))
  expect(container.querySelector('.ks-line')?.textContent).toBe('let value = "a😀b";')
  expect(container.querySelectorAll('mark')).toHaveLength(1)
  expect(container.querySelector('mark')?.querySelectorAll('span').length).toBeGreaterThan(1)
  expect(container.querySelector('mark [style]')?.textContent).toBe('"a😀b"')
})

test('expands an annotated tab after dedent without shifting the following Unicode span', async () => {
  const { container } = render(<SourceFrame path="tabs.txt" line={1} endLine={2} defaultOpen loadSource={async () => '\t\t😀abc\n\tend'} annotations={[
    { line: 1, start: 3, end: 4, label: 'emoji and a' },
    { line: 1, start: 2, end: 2, label: 'tab' },
  ]} />)
  await waitFor(() => expect(container.querySelectorAll('mark')).toHaveLength(2))
  expect([...container.querySelectorAll('mark')].map((mark) => mark.textContent)).toEqual(['    ', '😀a'])
  expect(container.querySelector('.ks-line')?.textContent).toBe('    😀abc')
})

test.each([
  { spans: [[2, 3], [4, 5]], marked: ['bc', 'de'] },
  { spans: [[2, 5], [3, 4]], marked: ['bcde', 'cd'] },
  { spans: [[2, 4], [3, 6]], marked: ['bcd', 'cdef'] },
])('retains each adjacent or overlapping annotation without repeating code: $spans', async ({ spans, marked }) => {
  const { container } = render(<SourceFrame path="overlap.txt" line={1} endLine={1} defaultOpen loadSource={async () => 'abcdef'} annotations={spans.map(([start, end], i) => ({
    line: 1, start, end, label: `label ${i}`,
  }))} />)
  await waitFor(() => expect(container.querySelectorAll('.cs-label')).toHaveLength(2))
  expect(container.querySelector('.ks-line')?.textContent).toBe('abcdef')
  spans.forEach(([start, end], i) => {
    expect([...container.querySelectorAll(`mark[data-start="${start}"][data-end="${end}"]`)].map((mark) => mark.textContent).join('')).toBe(marked[i])
  })
  expect([...container.querySelectorAll('.cs-label')].map((label) => label.textContent)).toEqual(['label 0', 'label 1'])
})

test('rejects a column past the Unicode character length rather than clamping, including changed props', async () => {
  const loadSource = async () => '😀a'
  const props = { path: 'unicode.rs', line: 1, endLine: 1, defaultOpen: true, loadSource }
  const { container, rerender } = render(<SourceFrame {...props} annotations={[{ line: 1, start: 1, end: 2, label: 'valid' }]} />)
  await waitFor(() => expect(container.querySelector('mark.cs-span')?.textContent).toBe('😀a'))
  rerender(<SourceFrame {...props} annotations={[{ line: 1, start: 2, end: 3, label: 'invalid' }]} />)
  await waitFor(() => expect(container.querySelector('.ks-msg-error')?.textContent).toContain('unicode.rs:L1 C2-3'))
  expect(container.querySelector('.ks-code')).toBeNull()
})

test('given its source, a frame renders the loaded excerpt at once, on the server too, and never loads', async () => {
  const { renderToStaticMarkup } = await import('react-dom/server')
  const loadSource = vi.fn(async () => 'never')
  const props = { path: 'a.rs', line: 2, endLine: 3, oid: 'abcdef0123', source: 'outside\n  let a = 1;\n  a', defaultOpen: true, loadSource }
  const markup = renderToStaticMarkup(<SourceFrame {...props} />)
  expect(markup).toContain('let')
  expect(markup).not.toContain('loading')
  const { container } = render(<SourceFrame {...props} />)
  expect([...container.querySelectorAll('.ks-line')].map((row) => row.textContent)).toEqual(['let a = 1;', 'a'])
  await new Promise((resolve) => setTimeout(resolve, 10))
  expect(container.querySelectorAll('.ks-line')).toHaveLength(2)
  expect(loadSource).not.toHaveBeenCalled()
})
