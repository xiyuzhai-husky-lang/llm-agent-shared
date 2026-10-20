import type {
  KyDocument,
  KyEnvironmentArgumentDataWire,
  KyMathAstDataWire,
  KyMathAstIdxRangeWire,
  KyMathCommandArgumentDataWire,
  KyRoseBoundaryClass,
  KyRoseCompleteCommandArgumentDataWire,
  KyRoseAstIdxRangeWire,
  KyTexLexicalClass,
  KyTikzAstIdxRangeWire,
} from '@kathy/ast'
import {
  classifyKyRosePunctuation,
  classifyKyTexFragment,
  kyTexNeedsSeparator,
  roseBoundaryNeedsSpace,
  spellKyMathDelimiter,
  spellKyMathPunctuation,
  spellKyMathPunctuationSpacing,
  spellKyMathScriptPrefix,
  spellKyRoseMathDelimiter,
  spellKyRosePunctuation,
  spellKyTikzDelimiter,
} from '@kathy/ast'

interface TexFragment { text: string; class: KyTexLexicalClass }
export type MathDecoration = (index: number, tex: string) => string
interface TexDecoration { math: MathDecoration; rose: (index: number, tex: string) => string }
interface MathFragments extends Array<TexFragment> { decorate?: MathDecoration }

export function serializeMathRange(document: KyDocument, range: KyMathAstIdxRangeWire, decorate?: MathDecoration): string {
  const fragments: MathFragments = Object.assign([], { decorate })
  for (let index = range.start; index < range.end; index += 1) {
    emitMathNode(document, index, fragments)
  }
  return joinFragments(fragments)
}

function emitMathNode(document: KyDocument, index: number, output: MathFragments): void {
  if (output.decorate) {
    const fragments: MathFragments = Object.assign([], { decorate: output.decorate })
    emitMathNodeBody(document, index, fragments)
    push(output, output.decorate(index, joinFragments(fragments)))
  } else emitMathNodeBody(document, index, output)
}

function emitMathNodeBody(document: KyDocument, index: number, output: MathFragments): void {
  const node = document.arena.math[index]
  if (node === undefined) throw new Error(`missing checked Math AST ${index}`)
  switch (node.kind) {
    case 'plain_letter': push(output, node.letter); break
    case 'styled_letter': push(output, node.styled_letter); break
    case 'styled_word': push(output, `\\${node.style}`); push(output, '{'); push(output, node.word); push(output, '}'); break
    case 'punctuation': push(output, spellKyMathPunctuation(node.value)); break
    case 'digit': push(output, node.value); break
    case 'attach':
      emitMathNode(document, node.base, output)
      for (const script of node.scripts) {
        if (script.script.kind === 'quotes') {
          push(output, "'".repeat(script.script.number_of_quotes))
        } else {
          const prefix = spellKyMathScriptPrefix(script.script_kind)
          push(output, prefix)
          const scriptNode = document.arena.math[script.script.ast]
          if (scriptNode?.kind === 'delimited') {
            emitMathNode(document, script.script.ast, output)
          } else {
            push(output, '{')
            emitMathNode(document, script.script.ast, output)
            push(output, '}')
          }
        }
      }
      break
    case 'delimited':
      push(output, spellKyMathDelimiter(node.left_delimiter, 'left'))
      emitMathRange(document, node.asts, output)
      push(output, spellKyMathDelimiter(node.right_delimiter, 'right'))
      break
    case 'complete_command':
      push(output, `\\${node.command_name}`)
      for (const argument of node.arguments) {
        switch (argument.kind) {
          case 'optional': push(output, '['); emitMathArgument(document, argument.data, output); push(output, ']'); break
          case 'curled': push(output, '{'); emitMathArgument(document, argument.data, output); push(output, '}'); break
          case 'math_ast': push(output, '{'); emitMathNode(document, argument.ast, output); push(output, '}'); break
          case 'rose_ast': push(output, '{'); push(output, serializeRoseRangeToTex(document, { start: argument.ast, end: argument.ast + 1 })); push(output, '}'); break
        }
      }
      break
    case 'lefted': push(output, `\\${node.command}`); emitMathNode(document, node.argument, output); break
    case 'righted': push(output, `\\${node.command}`); emitMathNode(document, node.argument, output); break
    case 'alignment_marker': push(output, '&'); break
    case 'line_break': push(output, '\\\\'); break
    case 'punc_spacing': push(output, spellKyMathPunctuationSpacing(node.spacing)); break
    case 'environment': push(output, serializeEnvironment(document, node)); break
    case 'verbatim_command':
      push(output, `\\${node.command_name}${node.starred ? '*' : ''}${node.delimiter}${node.body}${node.delimiter}`)
      break
  }
}

export function serializeEnvironment(
  document: KyDocument,
  node: Extract<KyMathAstDataWire | KyDocument['arena']['rose'][number], { kind: 'environment' }>,
  decoration?: TexDecoration,
): string {
  const star = node.starred ? '*' : ''
  const environmentName = `${node.environment_name}${star}`
  const body = node.asts.kind === 'math'
    ? serializeMathRange(document, node.asts.range, decoration?.math)
    : node.asts.kind === 'rose'
      ? serializeRoseRangeToTex(document, node.asts.range, decoration)
      : node.asts.kind === 'tikz'
        ? serializeTikzRange(document, node.asts.range)
        : node.asts.text
  return `\\begin{${environmentName}}${node.arguments.map((argument) => {
    const delimiters = argument.kind === 'optional' ? ['[', ']'] : ['{', '}']
    return `${delimiters[0]}${serializeEnvironmentArgument(document, argument.data, decoration)}${delimiters[1]}`
  }).join('')}${body}\\end{${environmentName}}`
}

export function serializeTikzRange(document: KyDocument, range: KyTikzAstIdxRangeWire): string {
  const fragments: TexFragment[] = []
  for (let index = range.start; index < range.end; index += 1) {
    const node = document.arena.tikz[index]
    if (node === undefined) throw new Error(`missing checked TikZ AST ${index}`)
    switch (node.kind) {
      case 'math_cell': push(fragments, serializeMathRange(document, node.range)); break
      case 'complete_command': push(fragments, `\\${node.command_name}`); break
      case 'word': pushClass(fragments, node.value, node.value === 'and' ? 'tikz_separator_word' : 'tikz_word'); break
      case 'digit': pushClass(fragments, node.value, 'ascii_digit'); break
      case 'letter': push(fragments, node.letter); break
      case 'punctuation': push(fragments, spellKyMathPunctuation(node.value)); break
      case 'subscript': push(fragments, '_'); break
      case 'superscript': push(fragments, '^'); break
      case 'alignment_marker': push(fragments, '&'); break
      case 'line_break': push(fragments, '\\\\'); break
      case 'semicolon': push(fragments, ';'); break
      case 'comma': push(fragments, ','); break
      case 'equals': push(fragments, '='); break
      case 'single_quote': push(fragments, "'"); break
      case 'delimited':
        push(fragments, spellKyTikzDelimiter(node.left_delimiter, 'left'))
        push(fragments, serializeTikzRange(document, node.asts))
        push(fragments, spellKyTikzDelimiter(node.right_delimiter, 'right'))
        break
      case 'other': push(fragments, node.value); break
    }
  }
  return joinFragments(fragments)
}

function serializeEnvironmentArgument(
  document: KyDocument,
  data: KyEnvironmentArgumentDataWire,
  decoration?: TexDecoration,
): string {
  switch (data.kind) {
    case 'math': return serializeMathRange(document, data.range, decoration?.math)
    case 'rose': return serializeRoseRangeToTex(document, data.range, decoration)
    case 'name':
    case 'wild': return data.value
  }
}

function emitMathRange(document: KyDocument, range: KyMathAstIdxRangeWire, output: TexFragment[]): void {
  for (let index = range.start; index < range.end; index += 1) emitMathNode(document, index, output)
}

function emitMathArgument(document: KyDocument, data: KyMathCommandArgumentDataWire, output: TexFragment[]): void {
  switch (data.kind) {
    case 'math': emitMathRange(document, data.range, output); break
    case 'rose': push(output, serializeRoseRangeToTex(document, data.range)); break
    case 'letter': push(output, data.letter); break
    case 'name':
    case 'wild': push(output, data.value); break
  }
}

export function serializeRoseRangeToTex(document: KyDocument, range: KyRoseAstIdxRangeWire, decoration?: TexDecoration): string {
  let result = ''
  let previous: KyRoseBoundaryClass | undefined
  for (let index = range.start; index < range.end; index += 1) {
    const node = document.arena.rose[index]
    if (node === undefined) throw new Error(`missing checked Rose AST ${index}`)
    const emitted = serializeRoseNode(document, node, decoration)
    if (decoration) emitted.text = decoration.rose(index, emitted.text)
    if (
      previous !== undefined
      && result.length > 0
      && !/\s$/.test(result)
      && roseBoundaryNeedsSpace(previous, emitted.boundary)
    ) {
      result += ' '
    }
    result += emitted.text
    previous = emitted.boundary
  }
  return result
}

function serializeRoseNode(
  document: KyDocument,
  node: KyDocument['arena']['rose'][number],
  decoration?: TexDecoration,
): { text: string; boundary: KyRoseBoundaryClass } {
  switch (node.kind) {
    case 'word': return { text: node.value, boundary: 'ordinary' }
    case 'punctuation': return { text: spellKyRosePunctuation(node.value), boundary: classifyKyRosePunctuation(node.value) }
    case 'nat32': return { text: String(node.value), boundary: 'ordinary' }
    case 'math': return {
      text: `${spellKyRoseMathDelimiter(node.left_delimiter)}${serializeMathRange(document, node.math_asts, decoration?.math)}${spellKyRoseMathDelimiter(node.right_delimiter)}`,
      boundary: 'ordinary',
    }
    case 'delimited': return { text: `{${serializeRoseRangeToTex(document, node.asts, decoration)}}`, boundary: 'ordinary' }
    case 'complete_command': return {
      text: `\\${node.command_name}${'starred' in node && node.starred ? '*' : ''}${node.arguments.map((argument) => {
        const [left, right] = argument.kind === 'optional' ? ['[', ']'] : ['{', '}']
        return `${left}${serializeRoseArgument(document, argument.data, decoration)}${right}`
      }).join('')}`,
      boundary: 'ordinary',
    }
    case 'verbatim_command': return {
      text: `\\${node.command_name}${node.starred ? '*' : ''}${node.delimiter}${node.body}${node.delimiter}`,
      boundary: 'ordinary',
    }
    case 'codespan': return {
      text: `\\begin{kyCodespan}{${node.source.path}}{${node.source.line}-${node.source.end_line}}${node.source.oid ? `[${node.source.oid}]` : ''}\n${node.annotations.map((a) => `\\kySpan{${a.line}}{${a.start}}{${a.end}}{${serializeRoseRangeToTex(document, a.label)}}[${a.severity ?? 'note'}]`).join('\n')}\n\\end{kyCodespan}`,
      boundary: 'hard_boundary',
    }
    case 'input': return {
      text: node.control.kind === 'text'
        ? `\\kyInput{${node.name}}{${serializeRoseRangeToTex(document, node.prompt)}}`
        : node.control.kind === 'verdict'
          ? `\\kyVerdict{${node.name}}{${serializeRoseRangeToTex(document, node.prompt)}}`
          : `\\kyChoice{${node.name}}{${serializeRoseRangeToTex(document, node.prompt)}}{${node.control.kind === 'choice_items'
            ? `\\begin{itemize}\n${node.control.options.map((option) => `\\item ${serializeRoseRangeToTex(document, option)}`).join('\n')}\n\\end{itemize}`
            : node.control.options.join(',')}}`,
      boundary: 'hard_boundary',
    }
    case 'rendered_span': return {
      text: `\\begin{kyRenderedContent}
${serializeRenderedContentToTex(node.content as KyDocument, node.annotations)}
${node.annotations.map((a) => `\\kyRenderedLabel{${a.severity ?? 'note'}}{${serializeRoseRangeToTex(document, a.label)}}`).join('\n')}
\\end{kyRenderedContent}`,
      boundary: 'hard_boundary',
    }
    case 'table': return {
      text: `\\begin{tabular}{${node.columns}}
${node.rows.map((row, index) =>
        `${node.horizontal_rules.filter((boundary) => boundary === index).map(() => '\\hline ').join('')}${row.map((cell) => serializeRoseRangeToTex(document, cell, decoration)).join(' & ')} \\\\`,
      ).join('\n')}
${node.horizontal_rules.filter((boundary) => boundary === node.rows.length).map(() => '\\hline ').join('')}\\end{tabular}`,
      boundary: 'hard_boundary',
    }
    case 'environment': return { text: serializeEnvironment(document, node, decoration), boundary: 'ordinary' }
    case 'new_paragraph': return { text: '\n\n', boundary: 'hard_boundary' }
    case 'item': return { text: '\\item', boundary: 'ordinary' }
  }
}

function serializeRoseArgument(document: KyDocument, data: KyRoseCompleteCommandArgumentDataWire, decoration?: TexDecoration): string {
  switch (data.kind) {
    case 'name':
    case 'wild': return data.value
    case 'rose': return serializeRoseRangeToTex(document, data.range, decoration)
    case 'math': return serializeMathRange(document, data.range, decoration?.math)
  }
}

export function serializeDocumentToTex(document: KyDocument): string {
  return serializeRoseRangeToTex(document, document.body)
}

function push(output: TexFragment[], text: string): void {
  output.push({ text, class: classifyKyTexFragment(text) })
}

function pushClass(output: TexFragment[], text: string, lexicalClass: KyTexLexicalClass): void {
  output.push({ text, class: lexicalClass })
}

function joinFragments(fragments: readonly TexFragment[]): string {
  let result = ''
  let previous: KyTexLexicalClass | undefined
  for (const fragment of fragments) {
    if (previous !== undefined && kyTexNeedsSeparator(previous, fragment.class)) result += ' '
    result += fragment.text
    previous = fragment.class
  }
  return result
}

export const rosePunctuation = (value: string): string => value === 'escaped_underscore' ? '_' : spellKyRosePunctuation(value)
export const mathDelimiter = spellKyRoseMathDelimiter

export function ensureSlice1MathSupport(node: KyMathAstDataWire, path: string): void {
  if (node.kind === 'complete_command' && !['sin', 'sqrt', 'lVert', 'rVert'].includes(node.command_name)) {
    throw new Error(`${path}: unsupported command ${node.command_name}`)
  }
  if (node.kind === 'environment' && node.environment_name !== 'alignat') {
    throw new Error(`${path}: unsupported Math environment ${node.environment_name}`)
  }
  if (node.kind === 'verbatim_command') {
    throw new Error(`${path}: unsupported Math node ${node.kind}`)
  }
}

function serializeRenderedContentToTex(document: KyDocument, annotations: import('@kathy/ast').KyRenderedSpanAnnotation<KyRoseAstIdxRangeWire>[]): string {
  const mathIds = new Set(annotations.flatMap((a) => a.targets.filter((t) => t.kind === 'math').map((t) => t.index)))
  const roseIds = new Set(annotations.flatMap((a) => a.targets.filter((t) => t.kind === 'rose').map((t) => t.index)))
  const decorate: MathDecoration = (index, tex) => mathIds.has(index)
    ? document.arena.math[index]?.kind === 'delimited' && tex.startsWith('{')
      ? `{\\underline{${tex.slice(1, -1)}}}` : `\\underline{${tex}}` : tex
  return serializeRoseRangeToTex(document, document.body, {
    math: decorate,
    rose: (index, tex) => roseIds.has(index) ? `\\underline{${tex}}` : tex,
  })
}
