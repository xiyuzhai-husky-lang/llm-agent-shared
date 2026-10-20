import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import type { KyDocumentWire } from '@kathy/ast'
import { createKathySurfaceVfs, KathySurface } from '../index.tsx'
import { kathySourcePaths } from '../node.ts'

const minimal: KyDocumentWire = {
  arena: { math: [], tikz: [], rose: [{ kind: 'word', value: 'Hello' }] }, body: { start: 0, end: 1 },
}
afterEach(cleanup)
it('renders a minimal Kathy document', async () => {
  const { waitFor } = await import('@testing-library/react')
  render(<KathySurface document={minimal} revision="abc1234" label="Problem" vfs={createKathySurfaceVfs({ folder: 'proposal', revision: 'abc1234', loadSource: async () => '' })} />)
  await waitFor(() => expect(screen.getByRole('region', { name: 'Problem document' }).textContent).toBe('Hello'))
  expect(screen.queryByRole('alert')).toBeNull()
})
it('labels an unavailable revision', () => {
  render(<KathySurface revision="abc1234" label="Design" vfs={createKathySurfaceVfs({ folder: 'proposal', revision: 'abc1234', loadSource: async () => '' })} />)
  expect(screen.getByText('Design is not available in this revision.')).toBeTruthy()
})
it('collects verbatim and codespan references including nested documents', () => {
  const fixture: KyDocumentWire = { arena: { math: [], tikz: [], rose: [
    { kind: 'verbatim_command', command_name: 'verb', starred: false, delimiter: '|', body: 'src/a.rs#L1-L2@abcdef0' },
    { kind: 'codespan', source: { path: 'src/b.rs', line: 1, end_line: 2 }, annotations: [] },
    { kind: 'rendered_span', source: { path: 'embedded.tex', line: 1, end_line: 1 }, annotations: [],
      content: { arena: { math: [], tikz: [], rose: [
        { kind: 'codespan', source: { path: 'src/c.rs', line: 1, end_line: 1, oid: '1234567' }, annotations: [] },
      ] }, body: { start: 0, end: 1 } } },
    { kind: 'verbatim_command', command_name: 'verb', starred: false, delimiter: '|', body: '../secret#L1-L2' },
    { kind: 'verbatim_command', command_name: 'verb', starred: false, delimiter: '|', body: 'src/invalid.rs#L3-L1' },
  ] }, body: { start: 0, end: 5 } }
  expect(kathySourcePaths(fixture)).toEqual([['src/a.rs', 'abcdef0'], ['src/b.rs', undefined], ['src/c.rs', '1234567']])
})

it('shares one endpoint-backed VFS across surfaces', async () => {
  const { waitFor } = await import('@testing-library/react')
  const loadSource = vi.fn(async () => 'shared source')
  const vfs = createKathySurfaceVfs({ folder: 'proposal', revision: 'abc1234', loadSource })
  const document: KyDocumentWire = {
    arena: { math: [], tikz: [], rose: [
      { kind: 'codespan', source: { path: 'shared.txt', line: 1, end_line: 1 }, annotations: [] },
    ] }, body: { start: 0, end: 1 },
  }
  const { container } = render(<>
    <KathySurface document={document} revision="abc1234" label="Problem" vfs={vfs} />
    <KathySurface document={document} revision="abc1234" label="Design" vfs={vfs} />
  </>)
  await waitFor(() => expect(container.querySelectorAll('.ks-row-focus')).toHaveLength(2))
  expect(loadSource).toHaveBeenCalledExactlyOnceWith('shared.txt', 'proposal', 'abc1234', 'abc1234')
})
