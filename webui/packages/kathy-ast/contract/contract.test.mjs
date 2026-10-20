import { readFileSync } from 'node:fs'
import { describe, expect, test } from 'bun:test'

import {
  KyWebDecodeError,
  decodeKyDocumentWire,
  kyDocumentWireSchema,
  parseKyDocumentJson,
} from '../src/index.ts'

const valid = {
  arena: {
    math: [],
    rose: [{ kind: 'word', value: 'hello' }],
    tikz: [],
  },
  body: { start: 0, end: 1 },
}

describe('@kathy/ast checked boundary', () => {
  test('decodes the generated Chow producer bytes', () => {
    const fixture = readFileSync(
      new URL('../../kathy-renderer/fixtures/producers/chow/plain-text-math.kathy.json', import.meta.url),
      'utf8',
    )
    const document = parseKyDocumentJson(JSON.parse(fixture))
    expect(document.body).toEqual({ start: 0, end: 24 })
    expect(document.arena.math).toHaveLength(15)
  })

  test('generated runtime schema equals the checked JSON artifact', () => {
    const artifact = JSON.parse(readFileSync(new URL('../schema/kathy-document.schema.json', import.meta.url), 'utf8'))
    expect(kyDocumentWireSchema).toEqual(artifact)
    expect(artifact.$id).toBe('urn:kathy:document')
  })

  test('decode owns and deeply freezes the branded graph', () => {
    const input = structuredClone(valid)
    const document = decodeKyDocumentWire(input)
    input.arena.rose[0].value = 'changed'
    input.arena.rose.push({ kind: 'word', value: 'extra' })
    expect(document.arena.rose).toEqual([{ kind: 'word', value: 'hello' }])
    expect(Object.isFrozen(document)).toBe(true)
    expect(Object.isFrozen(document.arena.rose)).toBe(true)
    expect(() => document.arena.rose.push({ kind: 'word', value: 'blocked' })).toThrow()
  })

  test('schema validation precedes structural validation', () => {
    expect(() => parseKyDocumentJson({ arena: {}, body: {} })).toThrow(
      expect.objectContaining({ phase: 'Schema' }),
    )
  })

  for (const [name, code, path] of [
    ['range-out-of-bounds', 'RangeOutOfBounds', 'body'],
    ['cycle', 'Cycle', 'body[0].asts[0]'],
    ['unreachable', 'UnreachableEntry', 'arena.rose[1]'],
  ]) {
    test(`rejects the shared ${name} graph`, () => {
      const value = JSON.parse(readFileSync(new URL(`./malformed/${name}.kathy.json`, import.meta.url), 'utf8'))
      try {
        parseKyDocumentJson(value)
        throw new Error('expected decode failure')
      } catch (error) {
        expect(error).toBeInstanceOf(KyWebDecodeError)
        expect(error).toMatchObject({ phase: 'Structure', code, path })
      }
    })
  }

  test('accepts generated-catalog document structure', () => {
    const document = decodeKyDocumentWire({
      arena: {
        math: [],
        rose: [
          { kind: 'word', value: 'Main' },
          { kind: 'item' },
          { kind: 'word', value: 'first' },
          { kind: 'item' },
          { kind: 'word', value: 'second' },
          {
            kind: 'complete_command',
            command_name: 'label',
            arguments: [{ kind: 'required', data: { kind: 'name', value: 'thm:main' } }],
          },
          {
            kind: 'environment',
            starred: false,
            environment_name: 'itemize',
            arguments: [],
            asts: { kind: 'rose', range: { start: 1, end: 5 } },
          },
          {
            kind: 'environment',
            starred: false,
            environment_name: 'theorem',
            arguments: [{ kind: 'optional', data: { kind: 'rose', range: { start: 0, end: 1 } } }],
            asts: { kind: 'rose', range: { start: 5, end: 7 } },
          },
        ],
        tikz: [],
      },
      body: { start: 7, end: 8 },
    })
    expect(document.arena.rose).toHaveLength(8)
  })

  for (const [name, wire, code] of itemizeFailures()) {
    test(`rejects ${name}`, () => {
      expect(() => decodeKyDocumentWire(wire)).toThrow(expect.objectContaining({ code }))
    })
  }

  test('checks a shared Item range in every occurrence context', () => {
    const shared = { start: 0, end: 2 }
    const wire = {
      arena: {
        math: [],
        rose: [
          { kind: 'item' },
          { kind: 'word', value: 'content' },
          {
            kind: 'environment', starred: false, environment_name: 'itemize', arguments: [],
            asts: { kind: 'rose', range: shared },
          },
          {
            kind: 'environment', starred: false, environment_name: 'theorem', arguments: [],
            asts: { kind: 'rose', range: shared },
          },
        ],
        tikz: [],
      },
      body: { start: 2, end: 4 },
    }
    expect(() => decodeKyDocumentWire(wire)).toThrow(
      expect.objectContaining({ code: 'ItemOutsideList' }),
    )
  })

  test('matches Rust builtin argument-kind admission exactly', () => {
    for (const command_name of ['fbox', 'Bra']) {
      const document = decodeKyDocumentWire({
        arena: {
          math: [],
          rose: [
            { kind: 'word', value: 'content' },
            {
              kind: 'complete_command',
              command_name,
              arguments: [{ kind: 'required', data: { kind: 'rose', range: { start: 0, end: 1 } } }],
            },
          ],
          tikz: [],
        },
        body: { start: 1, end: 2 },
      })
      expect(document.arena.rose[1].command_name).toBe(command_name)
    }

    expect(() => decodeKyDocumentWire(mathDocument([
      { kind: 'complete_command', command_name: 'fbox', arguments: [{ kind: 'curled', data: { kind: 'wild', value: 'raw' } }] },
    ]))).toThrow(expect.objectContaining({ code: 'ArgumentMismatch' }))
  })

  test('checks math_ast and rose_ast argument kinds', () => {
    expect(() => decodeKyDocumentWire(mathDocument([
      { kind: 'plain_letter', letter: 'x' },
      { kind: 'complete_command', command_name: 'sqrt', arguments: [{ kind: 'math_ast', ast: 0 }] },
    ], { start: 1, end: 2 }))).not.toThrow()

    expect(() => decodeKyDocumentWire({
      arena: {
        math: [{ kind: 'complete_command', command_name: 'sqrt', arguments: [{ kind: 'rose_ast', ast: 0 }] }],
        rose: [
          { kind: 'word', value: 'wrong' },
          { kind: 'math', left_delimiter: 'dollar', math_asts: { start: 0, end: 1 }, right_delimiter: 'dollar' },
        ],
        tikz: [],
      },
      body: { start: 1, end: 2 },
    })).toThrow(expect.objectContaining({ code: 'ArgumentMismatch' }))
  })

  test('accepts every Rust-valid verb delimiter', () => {
    for (const delimiter of ['*', '\\']) {
      expect(() => decodeKyDocumentWire(mathDocument([
        { kind: 'verbatim_command', command_name: 'verb', starred: false, delimiter, body: 'literal' },
      ]))).not.toThrow()
    }
  })

  for (const [name, wire, code] of strictStructureFailures()) {
    test(`rejects Rust-invalid ${name}`, () => {
      expect(() => decodeKyDocumentWire(wire)).toThrow(expect.objectContaining({ code }))
    })
  }
})

function itemizeFailures() {
  const itemize = (body) => ({
    arena: {
      math: [],
      rose: [
        ...body,
        {
          kind: 'environment', starred: false, environment_name: 'itemize', arguments: [],
          asts: { kind: 'rose', range: { start: 0, end: body.length } },
        },
      ],
      tikz: [],
    },
    body: { start: body.length, end: body.length + 1 },
  })
  return [
    ['standalone item', {
      arena: { math: [], rose: [{ kind: 'item' }], tikz: [] },
      body: { start: 0, end: 1 },
    }, 'ItemOutsideList'],
    ['empty itemize', itemize([]), 'EmptyList'],
    ['itemize missing first Item', itemize([{ kind: 'word', value: 'content' }]), 'ListMustStartWithItem'],
    ['consecutive Items', itemize([{ kind: 'item' }, { kind: 'item' }, { kind: 'word', value: 'content' }]), 'EmptyItem'],
    ['paragraph-only Item', itemize([{ kind: 'item' }, { kind: 'new_paragraph' }, { kind: 'item' }, { kind: 'word', value: 'content' }]), 'EmptyItem'],
    ['trailing empty Item', itemize([{ kind: 'item' }, { kind: 'word', value: 'content' }, { kind: 'item' }]), 'EmptyItem'],
  ]
}

function mathDocument(math, mathRange = { start: 0, end: math.length }) {
  return {
    arena: {
      math,
      rose: [
        { kind: 'math', left_delimiter: 'dollar', math_asts: mathRange, right_delimiter: 'dollar' },
      ],
      tikz: [],
    },
    body: { start: 0, end: 1 },
  }
}

function strictStructureFailures() {
  return [
    ['unpaired left command', mathDocument([
      { kind: 'punctuation', value: '(' },
      { kind: 'lefted', command: 'left', argument: 0 },
    ], { start: 1, end: 2 }), 'DelimiterMismatch'],
    ['mismatched left/right commands', mathDocument([
      { kind: 'punctuation', value: '(' },
      { kind: 'lefted', command: 'left', argument: 0 },
      { kind: 'punctuation', value: ')' },
      { kind: 'righted', command: 'bigr', argument: 2 },
    ], { start: 1, end: 4 }), 'DelimiterMismatch'],
    ['non-delimiter operand', mathDocument([
      { kind: 'plain_letter', letter: 'x' },
      { kind: 'lefted', command: 'left', argument: 0 },
    ], { start: 1, end: 2 }), 'DelimiterMismatch'],
    ['script data-kind mismatch', mathDocument([
      { kind: 'plain_letter', letter: 'x' },
      { kind: 'plain_letter', letter: 'n' },
      { kind: 'attach', base: 0, scripts: [{ script_kind: 'superscript', script: { kind: 'subscript', ast: 1 } }] },
    ], { start: 2, end: 3 }), 'InvalidScriptSequence'],
    ['duplicate script kind', mathDocument([
      { kind: 'plain_letter', letter: 'x' },
      { kind: 'plain_letter', letter: 'i' },
      { kind: 'plain_letter', letter: 'j' },
      { kind: 'attach', base: 0, scripts: [
        { script_kind: 'subscript', script: { kind: 'subscript', ast: 1 } },
        { script_kind: 'subscript', script: { kind: 'subscript', ast: 2 } },
      ] },
    ], { start: 3, end: 4 }), 'InvalidScriptSequence'],
    ['quotes outside superscript', mathDocument([
      { kind: 'plain_letter', letter: 'x' },
      { kind: 'attach', base: 0, scripts: [{ script_kind: 'overscript', script: { kind: 'quotes', number_of_quotes: 1 } }] },
    ], { start: 1, end: 2 }), 'InvalidScriptSequence'],
    ['styled letter mismatch', mathDocument([
      { kind: 'styled_letter', style: 'mathbb', styled_letter: '\\mathcal{A}' },
    ]), 'StyledLetterMismatch'],
    ['invalid styled word', mathDocument([
      { kind: 'styled_word', style: 'mathbb', word: 'x' },
    ]), 'InvalidLexicalValue'],
    ['invalid math punctuation', mathDocument([
      { kind: 'punctuation', value: '?' },
    ]), 'InvalidLexicalValue'],
    ['invalid punctuation spacing', mathDocument([
      { kind: 'punc_spacing', spacing: 'wide' },
    ]), 'InvalidLexicalValue'],
    ['verbatim body containing its delimiter', mathDocument([
      { kind: 'verbatim_command', command_name: 'verb', starred: false, delimiter: '|', body: 'a|b' },
    ]), 'InvalidLexicalValue'],
    ['invalid Rose word', {
      arena: { math: [], rose: [{ kind: 'word', value: '-bad' }], tikz: [] },
      body: { start: 0, end: 1 },
    }, 'InvalidLexicalValue'],
    ['invalid TikZ word', {
      arena: {
        math: [],
        rose: [{
          kind: 'environment', starred: false, environment_name: 'tikzpicture', arguments: [],
          asts: { kind: 'tikz', range: { start: 0, end: 1 } },
        }],
        tikz: [{ kind: 'word', value: 'bad-word' }],
      },
      body: { start: 0, end: 1 },
    }, 'InvalidLexicalValue'],
  ]
}

test('choice item ranges validate content, bounds, cycles and minimum item count', () => {
  const wire = {
    arena: { math: [], tikz: [], rose: [
      { kind: 'word', value: 'Choose' },
      { kind: 'verbatim_command', command_name: 'verb', starred: false, delimiter: '|', body: 'lw_001' },
      { kind: 'word', value: 'Later' },
      { kind: 'input', name: 'ruling_1', prompt: { start: 0, end: 1 }, control: {
        kind: 'choice_items', options: [{ start: 1, end: 2 }, { start: 2, end: 3 }],
      } },
    ] }, body: { start: 3, end: 4 },
  }
  expect(parseKyDocumentJson(wire).arena.rose[3].control.options).toEqual(wire.arena.rose[3].control.options)
  for (const options of [
    [{ start: 1, end: 2 }],
    [{ start: 1, end: 1 }, { start: 2, end: 3 }],
    [{ start: 1, end: 99 }, { start: 2, end: 3 }],
    [{ start: 3, end: 4 }, { start: 2, end: 3 }],
    ['first', 'second'],
  ]) {
    const invalid = structuredClone(wire)
    invalid.arena.rose[3].control.options = options
    expect(() => parseKyDocumentJson(invalid)).toThrow()
  }
  const empty = structuredClone(wire)
  empty.arena.rose[1] = { kind: 'new_paragraph' }
  expect(() => parseKyDocumentJson(empty)).toThrow()
})

test('table cells are checked graph edges and horizontal rules address row boundaries', () => {
  const wire = {
    arena: { math: [], tikz: [], rose: [
      { kind: 'word', value: 'cell' },
      { kind: 'table', columns: 'l|c', column_boundaries: ['none', 'label', 'none'], rows: [[{ start: 0, end: 1 }, { start: 0, end: 0 }]], horizontal_rules: [0, 1] },
    ] }, body: { start: 1, end: 2 },
  }
  expect(parseKyDocumentJson(wire).arena.rose[1].columns).toBe('l|c')
  for (const [field, value] of [['columns', ''], ['horizontal_rules', [2]], ['rows', [[{ start: 1, end: 2 }, { start: 0, end: 0 }]]], ['rows', [[{ start: 0, end: 3 }, { start: 0, end: 0 }]]]]) {
    const invalid = structuredClone(wire)
    invalid.arena.rose[1][field] = value
    expect(() => parseKyDocumentJson(invalid)).toThrow(KyWebDecodeError)
  }
})
