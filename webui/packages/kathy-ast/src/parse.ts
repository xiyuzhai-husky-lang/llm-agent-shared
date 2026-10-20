import Ajv2020 from 'ajv/dist/2020.js'

import type {
  KyAstIdxRangeWire,
  KyDocumentWire,
  KyEnvironmentArgumentDataWire,
  KyMathAstDataWire,
  KyMathAstIdxRangeWire,
  KyMathCommandArgumentDataWire,
  KyRoseAstDataWire,
  KyRoseAstIdxRangeWire,
  KyTikzAstDataWire,
  KyTikzAstIdxRangeWire,
} from './generated.ts'
import { kyDocumentWireSchema } from './schema.generated.ts'
import { kyBuiltinCommandSpecs, kyBuiltinEnvironmentSpecs, kyTikzBuiltinCommandSpecs } from './builtin.generated.ts'

const kyDocumentBrand: unique symbol = Symbol('KyDocument')

export type KyDocument = Readonly<KyDocumentWire> & {
  readonly [kyDocumentBrand]: true
}

export type KyWebDecodePhase = 'Schema' | 'Structure'

export class KyWebDecodeError extends Error {
  readonly phase: KyWebDecodePhase
  readonly code: string
  readonly path: string

  constructor(phase: KyWebDecodePhase, code: string, path: string) {
    super(`invalid Kathy document at ${path}: ${code}`)
    this.name = 'KyWebDecodeError'
    this.phase = phase
    this.code = code
    this.path = path
  }
}

const ajv = new Ajv2020({ allErrors: true, strict: false })
ajv.addFormat('uint32', {
  type: 'number',
  validate: (value: number) => Number.isInteger(value) && value >= 0 && value <= 0xffff_ffff,
})
ajv.addFormat('uint8', {
  type: 'number',
  validate: (value: number) => Number.isInteger(value) && value >= 0 && value <= 0xff,
})
const validateSchema = ajv.compile(kyDocumentWireSchema)

export function parseKyDocumentJson(value: unknown): KyDocument {
  if (!validateSchema(value)) {
    const error = validateSchema.errors?.[0]
    throw new KyWebDecodeError('Schema', error?.keyword ?? 'InvalidSchema', error?.instancePath || '$')
  }
  return decodeKyDocumentWire(value as KyDocumentWire)
}

export function decodeKyDocumentWire(wire: KyDocumentWire): KyDocument {
  const owned = structuredClone(wire)
  validateStructure(owned)
  Object.defineProperty(owned, kyDocumentBrand, {
    value: true,
    enumerable: false,
    configurable: false,
    writable: false,
  })
  return deepFreeze(owned) as KyDocument
}

function deepFreeze<T>(value: T): T {
  if (typeof value === 'object' && value !== null && !Object.isFrozen(value)) {
    for (const child of Object.values(value)) deepFreeze(child)
    Object.freeze(value)
  }
  return value
}

type VisitState = 0 | 1 | 2
type RoseRangeContext = 'normal' | 'list_body'

function validateStructure(document: KyDocumentWire): void {
  const { arena } = document
  checkArenaLength(arena.math.length, 'arena.math')
  checkArenaLength(arena.rose.length, 'arena.rose')
  checkArenaLength(arena.tikz.length, 'arena.tikz')
  const mathState: VisitState[] = arena.math.map(() => 0)
  const roseState: VisitState[] = arena.rose.map(() => 0)
  const inputNames = new Set<string>()
  const tikzState: VisitState[] = arena.tikz.map(() => 0)

  const visitMathRange = (range: KyMathAstIdxRangeWire, path: string): void => {
    checkRange(range, arena.math.length, path)
    const leftStack: string[] = []
    for (let index = range.start; index < range.end; index += 1) {
      visitMath(index, `${path}[${index}]`)
      const boundary = mathLeftRightBoundary(index)
      if (boundary?.side === 'left') leftStack.push(boundary.command)
      else if (boundary?.side === 'right') {
        const opener = leftStack.pop()
        if (opener === undefined || matchingRighter(opener) !== boundary.command) {
          fail('DelimiterMismatch', `${path}[${index}]`)
        }
      }
    }
    if (leftStack.length > 0) fail('DelimiterMismatch', path)
  }
  const visitRoseRange = (
    range: KyRoseAstIdxRangeWire,
    path: string,
    context: RoseRangeContext = 'normal',
  ): void => {
    checkRange(range, arena.rose.length, path)
    for (let index = range.start; index < range.end; index += 1) {
      if (arena.rose[index]?.kind === 'item' && context !== 'list_body') {
        fail('ItemOutsideList', `${path}[${index}]`)
      }
      visitRose(index, `${path}[${index}]`)
    }
  }
  const visitTikzRange = (range: KyTikzAstIdxRangeWire, path: string): void => {
    checkRange(range, arena.tikz.length, path)
    for (let index = range.start; index < range.end; index += 1) visitTikz(index, `${path}[${index}]`)
  }
  const visitAnyRange = (range: KyAstIdxRangeWire, path: string): void => {
    switch (range.kind) {
      case 'math': visitMathRange(range.range, `${path}.range`); break
      case 'rose': visitRoseRange(range.range, `${path}.range`); break
      case 'tikz': visitTikzRange(range.range, `${path}.range`); break
      case 'verbatim':
        checkPlainText(range.text, `${path}.text`)
        if (!Array.isArray(range.tokens) || range.tokens.some((token) =>
          !['plain', 'keyword', 'type', 'number', 'string', 'character', 'comment', 'preprocessor', 'identifier', 'punctuation', 'whitespace', 'diff_deletion', 'diff_addition', 'diff_hunk', 'diff_file_header'].includes(token.category)
          || typeof token.text !== 'string') || range.tokens.map((token) => token.text).join('') !== range.text) {
          fail('InvalidCodeTokens', `${path}.tokens`)
        }
        break
    }
  }
  const visitEnvironmentArgument = (data: KyEnvironmentArgumentDataWire, path: string): void => {
    switch (data.kind) {
      case 'math': visitMathRange(data.range, `${path}.range`); break
      case 'rose': visitRoseRange(data.range, `${path}.range`); break
      case 'name': checkName(data.value, `${path}.value`); break
      case 'wild': checkPlainText(data.value, `${path}.value`); break
    }
  }
  const visitMathArgument = (data: KyMathCommandArgumentDataWire, path: string): void => {
    switch (data.kind) {
      case 'math': visitMathRange(data.range, `${path}.range`); break
      case 'rose': visitRoseRange(data.range, `${path}.range`); break
      case 'letter': checkMathLetter(data.letter, `${path}.letter`); break
      case 'name': checkName(data.value, `${path}.value`); break
      case 'wild': checkPlainText(data.value, `${path}.value`); break
    }
  }
  const visitRose = (index: number, path: string): void => {
    checkIndex(index, arena.rose.length, path)
    if (roseState[index] === 1) fail('Cycle', path)
    if (roseState[index] === 2) return
    roseState[index] = 1
    const node: KyRoseAstDataWire = arena.rose[index]!
    switch (node.kind) {
      case 'table': {
        const boundaries = node.column_boundaries
        if (!node.columns || boundaries.length < 2
          || node.rows.some((row) => row.length + 1 !== boundaries.length)
          || node.horizontal_rules.some((index) => !Number.isInteger(index) || index < 0 || index > node.rows.length)
          || boundaries.some((kind, index) => kind === 'label' ? index !== 1
            : kind === 'star' && (index < 2 || index + 1 === boundaries.length))) fail('InvalidTable', path)
        node.rows.forEach((row, rowIndex) => row.forEach((cell, columnIndex) => {
          visitRoseRange(cell, `${path}.rows[${rowIndex}][${columnIndex}]`)
        }))
        break
      }
      case 'rendered_span': {
        const s = node.source
        if (!/^[A-Za-z0-9_./\-\u0080-\uFFFF]+$/.test(s.path) || s.path.split('/').some((p) => ['', '.', '..'].includes(p))
          || s.line < 1 || s.end_line < s.line || (s.oid !== undefined && !/^[a-fA-F0-9]{7,64}$/.test(s.oid))) fail('InvalidRenderedSpanSource', path)
        node.content = decodeKyDocumentWire(node.content)
        for (const [i, a] of node.annotations.entries()) {
          if (a.start >= a.end || a.targets.length === 0 || a.targets.some((t) => t.start >= t.end || t.end <= a.start || t.start >= a.end
            || t.index >= node.content.arena[t.kind].length)) fail('InvalidRenderedSpanRange', `${path}.annotations[${i}]`)
          visitRoseRange(a.label, `${path}.annotations[${i}].label`)
        }
        break
      }
      case 'codespan': {
        const s = node.source
        if (!/^[A-Za-z0-9_./\-\u0080-\uFFFF]+$/.test(s.path) || s.path.split('/').some((p) => ['', '.', '..'].includes(p))
          || !Number.isInteger(s.line) || !Number.isInteger(s.end_line) || s.line < 1 || s.end_line < s.line
          || (s.oid !== undefined && !/^[a-fA-F0-9]{7,64}$/.test(s.oid))) fail('InvalidCodespanSource', path)
        for (const [i, a] of node.annotations.entries()) {
          if (![a.line, a.start, a.end].every(Number.isInteger) || a.line < s.line || a.line > s.end_line
            || a.start < 1 || a.end < a.start || !['error', 'warning', 'note'].includes(a.severity ?? 'note')) fail('InvalidCodespanRange', `${path}.annotations[${i}]`)
          visitRoseRange(a.label, `${path}.annotations[${i}].label`)
        }
        break
      }
      case 'input': {
        if (!/^[A-Za-z0-9_-]{1,64}$/.test(node.name)) fail('InvalidInputName', `${path}.name`)
        if (inputNames.has(node.name)) fail('DuplicateInputName', `${path}.name`)
        inputNames.add(node.name)
        if (node.control.kind === 'choice') {
          const options = node.control.options
          if (options.length < 2 || options.some((o) => o.trim().length === 0 || o.trim() !== o || o.includes(','))
            || new Set(options).size !== options.length) fail('InvalidInputControl', `${path}.control`)
        }
        visitRoseRange(node.prompt, `${path}.prompt`)
        if (node.control.kind === 'choice_items') {
          if (node.control.options.length < 2) fail('InvalidInputControl', `${path}.control`)
          for (const [i, option] of node.control.options.entries()) {
            const optionPath = `${path}.control.options[${i}]`
            visitRoseRange(option, optionPath)
            if (!arena.rose.slice(option.start, option.end).some((node) => node.kind !== 'new_paragraph')) {
              fail('InvalidInputControl', optionPath)
            }
          }
        }
        break
      }
      case 'word': checkWord(node.value, `${path}.value`); break
      case 'punctuation': checkRosePunctuation(node.value, `${path}.value`); break
      case 'math':
        checkEmbeddedMathDelimiters(node.left_delimiter, node.right_delimiter, path)
        visitMathRange(node.math_asts, `${path}.math_asts`)
        break
      case 'delimited':
        if (node.left_delimiter !== 'curl' || node.right_delimiter !== 'curl') fail('DelimiterMismatch', path)
        visitRoseRange(node.asts, `${path}.asts`)
        break
      case 'complete_command':
        validateCommand(node.command_name + (node.starred ? '*' : ''), 'complete_command', 'rose', node.arguments, path)
        for (const [position, argument] of node.arguments.entries()) {
          visitRoseArgument(argument.data, `${path}.arguments[${position}].data`)
        }
        break
      case 'verbatim_command':
        validateCommand(node.command_name, 'verbatim_command', 'rose', [], path)
        validateVerbatim(node.command_name, node.delimiter, node.body, path)
        break
      case 'environment':
        validateEnvironment(node, 'rose', path)
        break
      case 'new_paragraph':
        break
      case 'item':
        validateCommand('item', 'item', 'rose', [], path)
        break
      case 'nat32':
        checkU32(node.value, `${path}.value`)
        break
    }
    roseState[index] = 2
  }
  const visitMath = (index: number, path: string): void => {
    checkIndex(index, arena.math.length, path)
    if (mathState[index] === 1) fail('Cycle', path)
    if (mathState[index] === 2) return
    mathState[index] = 1
    const node: KyMathAstDataWire = arena.math[index]!
    switch (node.kind) {
      case 'plain_letter': checkMathLetter(node.letter, `${path}.letter`); break
      case 'styled_letter': checkStyledLetter(node.style, node.styled_letter, path); break
      case 'styled_word':
        checkMathStyle(node.style, `${path}.style`)
        checkMathStyledWord(node.word, `${path}.word`)
        break
      case 'punctuation': checkMathPunctuation(node.value, `${path}.value`); break
      case 'digit': if (!/^[0-9]$/.test(node.value)) fail('InvalidLexicalValue', `${path}.value`); break
      case 'attach': {
        visitMath(node.base, `${path}.base`)
        let previousScript: { kind: string; quotes: boolean } | undefined
        for (const [position, script] of node.scripts.entries()) {
          checkScriptKind(script.script_kind, `${path}.scripts[${position}].script_kind`)
          if (script.script.kind === 'quotes') {
            if (script.script.number_of_quotes === 0) fail('ZeroQuotes', `${path}.scripts[${position}].script.number_of_quotes`)
            if (script.script_kind !== 'superscript') fail('InvalidScriptSequence', `${path}.scripts[${position}]`)
          } else {
            const validDataKind = script.script.kind === 'subscript'
              ? ['subscript', 'underscript', 'presubscript'].includes(script.script_kind)
              : ['superscript', 'overscript', 'presuperscript'].includes(script.script_kind)
            if (!validDataKind) fail('InvalidScriptSequence', `${path}.scripts[${position}]`)
            visitMath(script.script.ast, `${path}.scripts[${position}].script.ast`)
          }
          if (
            previousScript?.kind === script.script_kind
            && !(previousScript.quotes && script.script_kind === 'superscript')
          ) fail('InvalidScriptSequence', `${path}.scripts[${position}]`)
          previousScript = { kind: script.script_kind, quotes: script.script.kind === 'quotes' }
        }
        break
      }
      case 'delimited':
        if (node.left_delimiter !== 'curl' || node.right_delimiter !== 'curl') fail('DelimiterMismatch', path)
        visitMathRange(node.asts, `${path}.asts`)
        break
      case 'complete_command':
        validateCommand(node.command_name, 'complete_command', 'math', node.arguments, path)
        for (const [position, argument] of node.arguments.entries()) {
          if (argument.kind === 'math_ast') visitMath(argument.ast, `${path}.arguments[${position}].ast`)
          else if (argument.kind === 'rose_ast') visitRose(argument.ast, `${path}.arguments[${position}].ast`)
          else visitMathArgument(argument.data, `${path}.arguments[${position}].data`)
        }
        break
      case 'lefted':
        validateDelimitedCommand(node.command, 'lefter', path)
        visitMath(node.argument, `${path}.argument`)
        validateDelimiterOperand(node.argument, `${path}.argument`)
        break
      case 'righted':
        validateDelimitedCommand(node.command, 'righter', path)
        visitMath(node.argument, `${path}.argument`)
        validateDelimiterOperand(node.argument, `${path}.argument`)
        break
      case 'verbatim_command':
        validateCommand(node.command_name, 'verbatim_command', 'math', [], path)
        validateVerbatim(node.command_name, node.delimiter, node.body, path)
        break
      case 'environment':
        validateEnvironment(node, 'math', path)
        break
      case 'alignment_marker': if (node.alignment_marker !== 'ampersand') fail('InvalidLexicalValue', `${path}.alignment_marker`); break
      case 'line_break': break
      case 'punc_spacing': checkMathPunctuationSpacing(node.spacing, `${path}.spacing`); break
    }
    mathState[index] = 2
  }

  const visitRoseArgument = (
    data: Extract<KyRoseAstDataWire, { kind: 'complete_command' }>['arguments'][number]['data'],
    path: string,
  ): void => {
    switch (data.kind) {
      case 'math': visitMathRange(data.range, `${path}.range`); break
      case 'rose': visitRoseRange(data.range, `${path}.range`); break
      case 'name': checkName(data.value, `${path}.value`); break
      case 'wild': checkPlainText(data.value, `${path}.value`); break
    }
  }

  const validateEnvironment = (
    node: Extract<KyRoseAstDataWire | KyMathAstDataWire, { kind: 'environment' }>,
    parentMode: 'rose' | 'math',
    path: string,
  ): void => {
    const spec = kyBuiltinEnvironmentSpecs.find(
      (candidate) => candidate.name === node.environment_name && candidate.starred === node.starred,
    )
    if (spec === undefined) fail('UnknownEnvironment', `${path}.environment_name`)
    if (!(spec.allowed_modes as readonly string[]).includes(parentMode)) fail('ModeMismatch', path)
    const matched = matchArguments(spec.arguments, node.arguments)
    if (matched === undefined) fail('ArgumentCountMismatch', `${path}.arguments`)
    for (const [position, argument] of node.arguments.entries()) {
      if (!argumentKindMatches(argumentDeclarationKind(matched[position]!), wireArgumentKind(argument.data))) {
        fail('ArgumentMismatch', `${path}.arguments[${position}]`)
      }
      visitEnvironmentArgument(argument.data, `${path}.arguments[${position}].data`)
    }
    if (node.asts.kind !== spec.body_mode) fail('ModeMismatch', `${path}.asts`)
    if ((node.environment_name === 'itemize' || node.environment_name === 'enumerate') && !node.starred) {
      if (node.asts.kind !== 'rose') fail('ModeMismatch', `${path}.asts`)
      validateListBody(node.asts.range, `${path}.asts.range`)
      visitRoseRange(node.asts.range, `${path}.asts.range`, 'list_body')
    } else visitAnyRange(node.asts, `${path}.asts`)
  }

  const validateListBody = (range: KyRoseAstIdxRangeWire, path: string): void => {
    checkRange(range, arena.rose.length, path)
    if (range.start === range.end) fail('EmptyList', path)
    if (arena.rose[range.start]?.kind !== 'item') fail('ListMustStartWithItem', `${path}[${range.start}]`)
    let itemHasContent = false
    for (let index = range.start + 1; index < range.end; index += 1) {
      const node = arena.rose[index]
      if (node?.kind === 'item') {
        if (!itemHasContent) fail('EmptyItem', `${path}[${index}]`)
        itemHasContent = false
      } else if (node?.kind !== 'new_paragraph') itemHasContent = true
    }
    if (!itemHasContent) fail('EmptyItem', path)
  }
  const visitTikz = (index: number, path: string): void => {
    checkIndex(index, arena.tikz.length, path)
    if (tikzState[index] === 1) fail('Cycle', path)
    if (tikzState[index] === 2) return
    tikzState[index] = 1
    const node: KyTikzAstDataWire = arena.tikz[index]!
    switch (node.kind) {
      case 'math_cell': visitMathRange(node.range, `${path}.range`); break
      case 'complete_command':
        if (!kyTikzBuiltinCommandSpecs.some((candidate) => candidate.name === node.command_name)) {
          fail('UnknownCommand', `${path}.command_name`)
        }
        break
      case 'word': checkTikzWord(node.value, `${path}.value`); break
      case 'digit': if (!/^[0-9]$/.test(node.value)) fail('InvalidLexicalValue', `${path}.value`); break
      case 'letter': checkMathLetter(node.letter, `${path}.letter`); break
      case 'punctuation': checkMathPunctuation(node.value, `${path}.value`); break
      case 'delimited':
        checkTikzDelimiter(node.left_delimiter, `${path}.left_delimiter`)
        checkTikzDelimiter(node.right_delimiter, `${path}.right_delimiter`)
        visitTikzRange(node.asts, `${path}.asts`)
        break
      case 'other': checkTikzOther(node.value, `${path}.value`); break
      case 'subscript':
      case 'superscript':
      case 'alignment_marker':
      case 'line_break':
      case 'semicolon':
      case 'comma':
      case 'equals':
      case 'single_quote':
        break
    }
    tikzState[index] = 2
  }

  visitRoseRange(document.body, 'body')
  requireReachable(mathState, 'arena.math')
  requireReachable(roseState, 'arena.rose')
  requireReachable(tikzState, 'arena.tikz')

  function mathLeftRightBoundary(index: number): { side: 'left' | 'right'; command: string } | undefined {
    const node = arena.math[index]
    if (node?.kind === 'lefted') return { side: 'left', command: node.command }
    if (node?.kind === 'righted') return { side: 'right', command: node.command }
    if (node?.kind === 'attach') return mathLeftRightBoundary(node.base)
    return undefined
  }

  function validateDelimiterOperand(index: number, path: string): void {
    const node = arena.math[index]
    const valid = node?.kind === 'punctuation'
      ? MATH_DELIMITER_PUNCTUATION.has(node.value)
      : node?.kind === 'complete_command'
        && node.arguments.length === 0
        && kyBuiltinCommandSpecs.some(
          (candidate) => candidate.name === node.command_name && candidate.left_right_delimiter,
        )
    if (!valid) fail('DelimiterMismatch', path)
  }
}

function validateVerbatim(command: string, delimiter: string, body: string, path: string): void {
  if (command !== 'verb' || [...delimiter].length !== 1 || /\s/u.test(delimiter) || body.includes(delimiter)) {
    fail('InvalidLexicalValue', path)
  }
}

function validateDelimitedCommand(commandName: string, representation: 'lefter' | 'righter', path: string): void {
  const spec = kyBuiltinCommandSpecs.find((candidate) => candidate.name === commandName)
  if (spec === undefined) fail('UnknownCommand', `${path}.command`)
  if (spec.representation !== representation) fail('BuiltinRepresentationMismatch', `${path}.command`)
  if (!(spec.allowed_modes as readonly string[]).includes('math')) fail('ModeMismatch', path)
  if (spec.arguments.length !== 1 || spec.arguments[0] !== 'required_primitive') {
    fail('BuiltinRepresentationMismatch', `${path}.command`)
  }
}

function validateCommand(
  commandName: string,
  representation: string,
  mode: 'rose' | 'math',
  actualArguments: readonly { kind: string; data?: { kind: string } }[],
  path: string,
): void {
  const spec = kyBuiltinCommandSpecs.find((candidate) => candidate.name === commandName)
  if (spec === undefined) fail('UnknownCommand', `${path}.command_name`)
  if (spec.representation !== representation) fail('BuiltinRepresentationMismatch', `${path}.command_name`)
  if (!(spec.allowed_modes as readonly string[]).includes(mode)) fail('ModeMismatch', path)
  const matched = matchArguments(spec.arguments, actualArguments)
  if (matched === undefined) fail('ArgumentCountMismatch', `${path}.arguments`)
  for (const [position, argument] of actualArguments.entries()) {
    const actualKind = argument.kind === 'math_ast'
      ? 'math'
      : argument.kind === 'rose_ast'
        ? 'rose'
        : argument.data === undefined
          ? undefined
          : wireArgumentKind(argument.data)
    if (actualKind === undefined || !argumentKindMatches(argumentDeclarationKind(matched[position]!), actualKind)) {
      fail('ArgumentMismatch', `${path}.arguments[${position}]`)
    }
  }
}

function matchArguments(
  declarations: readonly string[],
  actualArguments: readonly { kind: string }[],
): string[] | undefined {
  const matched: string[] = []
  let actual = 0
  for (const declaration of declarations) {
    const optional = declaration.startsWith('optional_')
    if (actual >= actualArguments.length) {
      if (optional) continue
      return undefined
    }
    const actualOptional = actualArguments[actual]?.kind === 'optional'
    if (optional !== actualOptional) {
      if (optional) continue
      return undefined
    }
    matched.push(declaration)
    actual += 1
  }
  return actual === actualArguments.length ? matched : undefined
}

function argumentDeclarationKind(declaration: string): string {
  return declaration.replace(/^(optional|required)_/, '')
}

function wireArgumentKind(data: { kind: string }): string {
  return data.kind === 'wild' ? 'raw' : data.kind
}

function argumentKindMatches(expected: string, actual: string): boolean {
  return expected === actual
    || (expected === 'primitive' && actual === 'math')
    || (expected === 'original' && (actual === 'math' || actual === 'rose'))
    || (expected === 'hbox' && actual === 'rose')
    || (['color', 'size', 'url'].includes(expected) && actual === 'raw')
}

function checkEmbeddedMathDelimiters(left: string, right: string, path: string): void {
  const matching =
    (left === 'dollar' && right === 'dollar')
    || (left === 'dollar_dollar' && right === 'dollar_dollar')
    || (left === 'escaped_lpar' && right === 'escaped_rpar')
    || (left === 'escaped_lbox' && right === 'escaped_rbox')
  if (!matching) fail('DelimiterMismatch', path)
}

function checkRange(range: { start: number; end: number }, length: number, path: string): void {
  checkU32(range.start, `${path}.start`)
  checkU32(range.end, `${path}.end`)
  if (range.start > range.end || range.end > length) fail('RangeOutOfBounds', path)
}

function checkIndex(index: number, length: number, path: string): void {
  checkU32(index, path)
  if (index >= length) fail('IndexOutOfBounds', path)
}

function checkArenaLength(length: number, path: string): void {
  if (length >= 0xffff_ffff) fail('ArenaTooLarge', path)
}

function checkU32(value: number, path: string): void {
  if (!Number.isInteger(value) || value < 0 || value > 0xffff_ffff) fail('InvalidU32', path)
}

function checkPlainText(value: string, path: string): void {
  if (value.includes('\0')) fail('InvalidLexicalValue', path)
}

function checkName(value: string, path: string): void {
  if (!/^[\p{L}][\p{L}\p{N}:_.-]*$/u.test(value)) fail('InvalidLexicalValue', path)
}

function checkWord(value: string, path: string): void {
  if (!/^[\p{L}\p{N}][\p{L}\p{N}'-]*$/u.test(value)) fail('InvalidLexicalValue', path)
}

function checkMathStyledWord(value: string, path: string): void {
  if (!/^[A-Za-z][\p{L}\p{N}]+$/u.test(value)) fail('InvalidLexicalValue', path)
}

function checkTikzWord(value: string, path: string): void {
  if (!/^[A-Za-z]+$/.test(value)) fail('InvalidLexicalValue', path)
}

function checkMathStyle(value: string, path: string): void {
  if (!MATH_STYLES.has(value)) fail('InvalidLexicalValue', path)
}

function checkStyledLetter(style: string, value: string, path: string): void {
  checkMathStyle(style, `${path}.style`)
  if (value !== `\\${style}{${value.slice(-2, -1)}}` || !/^\\[A-Za-z]+\{[A-Za-z]\}$/.test(value)) {
    fail('StyledLetterMismatch', `${path}.styled_letter`)
  }
}

function checkMathLetter(value: string, path: string): void {
  if (/^[A-Za-z]$/.test(value) || GREEK_MATH_LETTERS.has(value) || GREEK_UNICODE_LETTERS.has(value)) return
  fail('InvalidLexicalValue', path)
}

function checkMathPunctuation(value: string, path: string): void {
  if (!MATH_PUNCTUATION.has(value)) fail('InvalidLexicalValue', path)
}

function checkMathPunctuationSpacing(value: string, path: string): void {
  if (!MATH_PUNCTUATION_SPACING.has(value)) fail('InvalidLexicalValue', path)
}

function checkScriptKind(value: string, path: string): void {
  if (!SCRIPT_KINDS.has(value)) fail('InvalidLexicalValue', path)
}

function checkTikzDelimiter(value: string, path: string): void {
  if (!TIKZ_DELIMITERS.has(value)) fail('InvalidLexicalValue', path)
}

function checkTikzOther(value: string, path: string): void {
  const chars = [...value]
  if (
    chars.length !== 1
    || /^[A-Za-z0-9]$/.test(value)
    || "\\{}[]()_^&;,='".includes(value)
  ) fail('InvalidLexicalValue', path)
}

function checkRosePunctuation(value: string, path: string): void {
  const admitted = new Set([
    'comma', 'period', 'colon', 'semicolon', 'exclamation', 'question',
    'left_curl', 'right_curl', 'left_box', 'right_box', 'hyphen',
    'escaped_backslash', 'escaped_lcurl', 'escaped_rcurl',
    'fullwidth_colon', 'ideographic_comma', 'fullwidth_comma', 'ideographic_full_stop', 'fullwidth_semicolon', 'fullwidth_left_paren', 'fullwidth_right_paren', 'left_corner_bracket', 'right_corner_bracket', 'left_double_quote', 'right_double_quote', 'em_dash', 'slash', 'plus', 'escaped_underscore',
  ])
  if (!admitted.has(value)) fail('InvalidLexicalValue', path)
}

function requireReachable(states: readonly VisitState[], path: string): void {
  const index = states.findIndex((state) => state !== 2)
  if (index >= 0) fail('UnreachableEntry', `${path}[${index}]`)
}

function fail(code: string, path: string): never {
  throw new KyWebDecodeError('Structure', code, path)
}

const MATH_STYLES = new Set(['mathbb', 'mathcal', 'mathfrak', 'mathit', 'mathrm', 'mathscr', 'mathsf'])
const SCRIPT_KINDS = new Set([
  'subscript', 'underscript', 'presubscript', 'superscript', 'overscript', 'presuperscript',
])
const MATH_PUNCTUATION_SPACING = new Set([
  'thin_space', 'medium_space', 'thick_space', 'negative_thin_space', 'normal_space',
])
const TIKZ_DELIMITERS = new Set(['curl', 'par', 'box'])
const MATH_PUNCTUATION = new Set([
  '+', '-', '·', '/', '∈', '∉', '∋', '∌', '⊂', '⊃', '⊆', '⊇', '∀', '∃', '∄', '∞',
  '=', '≠', '<', '>', '≤', '≥', '±', '×', '(', ')', '[', ']', '\\{', '\\}', '.', ',', ':', '|', '!', '#', '\\_', '*',
])
const MATH_DELIMITER_PUNCTUATION = new Set(['(', ')', '[', ']', '\\{', '\\}', '<', '>', '/', '|', '.'])
const GREEK_MATH_LETTERS = new Set([
  '\\Gamma', '\\Delta', '\\Theta', '\\Lambda', '\\Xi', '\\Pi', '\\Sigma', '\\Upsilon', '\\Phi', '\\Psi', '\\Omega',
  '\\alpha', '\\beta', '\\gamma', '\\delta', '\\epsilon', '\\zeta', '\\eta', '\\theta', '\\iota', '\\kappa', '\\lambda',
  '\\mu', '\\nu', '\\xi', '\\omicron', '\\pi', '\\rho', '\\sigma', '\\tau', '\\upsilon', '\\phi', '\\chi', '\\psi', '\\omega',
  '\\varepsilon', '\\vartheta', '\\varpi', '\\varrho', '\\varsigma', '\\varphi',
])
const GREEK_UNICODE_LETTERS = new Set([
  'Γ', 'Δ', 'Θ', 'Λ', 'Ξ', 'Π', 'Σ', 'Φ', 'Ψ', 'Ω',
  'α', 'β', 'γ', 'δ', 'ε', 'ζ', 'η', 'θ', 'ι', 'κ', 'λ', 'μ', 'ν', 'ξ', 'ο', 'π', 'ρ', 'σ', 'τ', 'υ', 'φ', 'χ', 'ψ', 'ω',
])

function matchingRighter(lefter: string): string | undefined {
  return ({ left: 'right', bigl: 'bigr', Bigl: 'Bigr', biggl: 'biggr', Biggl: 'Biggr' } as Record<string, string>)[lefter]
}
