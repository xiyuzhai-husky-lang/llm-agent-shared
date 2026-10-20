import { expect, test } from 'vitest'

import { parseKyDocumentJson } from '@kathy/ast'

import { slice1Wire } from '../tests/fixture.ts'
import { kyRendererProfileCatalog } from './profile.generated.ts'
import { validateRendererSupport } from './render.ts'

test('generated renderer profiles cover the exhaustive Slice 4 catalogs', () => {
  expect(kyRendererProfileCatalog.commands).toHaveLength(973)
  expect(kyRendererProfileCatalog.environments).toHaveLength(12)
  expect(kyRendererProfileCatalog.rose).toHaveLength(30)
  expect(kyRendererProfileCatalog.commands.filter((row) => row.katex_safe.status === 'supported')).toHaveLength(924)
  expect(kyRendererProfileCatalog.commands.filter((row) => row.katex_safe.status === 'unsupported')).toHaveLength(49)
  expect(kyRendererProfileCatalog.commands.filter((row) => row.rose_html.status === 'unsupported')).toHaveLength(376)
  expect(new Set(kyRendererProfileCatalog.commands.map((row) => row.command_name)).size).toBe(973)
})

test('web-v1 admits the exact Slice 1 command catalog', () => {
  const document = parseKyDocumentJson(slice1Wire())
  expect([...new Set(document.arena.math.flatMap((node) => {
    if (node.kind === 'complete_command') return [node.command_name]
    if (node.kind === 'lefted' || node.kind === 'righted') return [node.command]
    return []
  }))].sort()).toEqual(['lVert', 'left', 'rVert', 'right', 'sin', 'sqrt'])
  expect(() => validateRendererSupport(document)).not.toThrow()
})

test.each([
  ['tikzpicture', 'rose'],
  ['tikzcd', 'rose'],
  ['tikzcd', 'math'],
] as const)('%s in %s has no renderer: rejected with a named error before any DOM is written', (environment, arena) => {
  const wire = slice1Wire()
  wire.arena.tikz = [{ kind: 'complete_command', command_name: 'draw' }]
  const node = { kind: 'environment' as const, starred: false, environment_name: environment, arguments: [], asts: { kind: 'tikz' as const, range: { start: 0, end: 1 } } }
  if (arena === 'rose') {
    wire.arena.math = []
    wire.arena.rose = [node]
    wire.body = { start: 0, end: 1 }
  } else {
    wire.arena.math = [node]
    wire.arena.rose = [{ kind: 'math', left_delimiter: 'dollar', right_delimiter: 'dollar', math_asts: { start: 0, end: 1 } }]
    wire.body = { start: 0, end: 1 }
  }
  expect(() => validateRendererSupport(parseKyDocumentJson(wire))).toThrowError(
    expect.objectContaining({ code: 'UnsupportedByRenderer', backend: null, path: `arena.${arena}[0]`, reason: 'tikz_has_no_renderer' }),
  )
})

test('rose-html reports the exact generated rejection reason', () => {
  const wire = slice1Wire()
  wire.arena.math = []
  wire.arena.rose = [{ kind: 'complete_command', command_name: 'AA', arguments: [] }]
  wire.body = { start: 0, end: 1 }
  expect(() => validateRendererSupport(parseKyDocumentJson(wire))).toThrowError(
    expect.objectContaining({
      code: 'UnsupportedByRenderer',
      backend: 'rose-html',
      path: 'arena.rose[0]',
      reason: 'generic_latex_text_command_has_no_rose_html_handler',
    }),
  )
})
