import { COLOR, type TokenKind } from './theme'

/** A contiguous run of source text with an optional highlight colour. Spans
 *  tile the input exactly (every byte covered, in order), so the renderer can
 *  reconstruct each line verbatim. */
export interface Span {
  value: string
  color?: string
}

// prettier-ignore
const KEYWORDS = new Set([
  'as', 'async', 'await', 'break', 'const', 'continue', 'crate', 'dyn', 'else',
  'enum', 'extern', 'false', 'fn', 'for', 'if', 'impl', 'in', 'let', 'loop',
  'match', 'mod', 'move', 'mut', 'pub', 'ref', 'return', 'self', 'Self',
  'static', 'struct', 'super', 'trait', 'true', 'type', 'union', 'unsafe',
  'use', 'where', 'while',
])

// prettier-ignore
const PRIMITIVES = new Set([
  'i8', 'i16', 'i32', 'i64', 'i128', 'isize',
  'u8', 'u16', 'u32', 'u64', 'u128', 'usize',
  'f32', 'f64', 'bool', 'char', 'str',
])

const isIdentStart = (c: string) => /[A-Za-z_]/.test(c)
const isIdent = (c: string) => /[A-Za-z0-9_]/.test(c)
const isDigit = (c: string) => c >= '0' && c <= '9'

const NUMBER = /^(?:0[xX][0-9a-fA-F_]+|0[oO][0-7_]+|0[bB][01_]+|[0-9][0-9_]*(?:\.[0-9][0-9_]*)?(?:[eE][+-]?[0-9]+)?)/
const NUM_SUFFIX = /^(?:u8|u16|u32|u64|u128|usize|i8|i16|i32|i64|i128|isize|f32|f64)/
const CHAR_LIT = /^'(?:\\.|[^'\\\n])'/
const LIFETIME = /^'[A-Za-z_]\w*/
const RAW_STRING = /^(?:b?r)(#*)"/

/**
 * Lex Rust source into coloured spans for highlighting. Hand-rolled (no parser /
 * grammar / wasm): keywords, primitive & CamelCase types, function calls, macros,
 * strings (incl. raw & byte), chars, lifetimes, numbers, comments, attributes.
 * Best-effort — a snippet highlighter, not a compiler.
 */
export function lexRust(code: string): Span[] {
  const spans: Span[] = []
  const n = code.length
  let i = 0
  const push = (value: string, kind?: TokenKind) =>
    spans.push(kind != null ? { value, color: COLOR[kind] } : { value })

  while (i < n) {
    const c = code[i]
    const rest = code.slice(i)

    // line comment
    if (c === '/' && code[i + 1] === '/') {
      let j = i + 2
      while (j < n && code[j] !== '\n') j++
      push(code.slice(i, j), 'comment')
      i = j
      continue
    }
    // block comment (nested, as Rust allows)
    if (c === '/' && code[i + 1] === '*') {
      let j = i + 2
      let depth = 1
      while (j < n && depth > 0) {
        if (code[j] === '/' && code[j + 1] === '*') {
          depth++
          j += 2
        } else if (code[j] === '*' && code[j + 1] === '/') {
          depth--
          j += 2
        } else {
          j++
        }
      }
      push(code.slice(i, j), 'comment')
      i = j
      continue
    }
    // raw / byte-raw string: r"…", r#"…"#, br#"…"#
    const raw = RAW_STRING.exec(rest)
    if (raw != null) {
      const close = '"' + raw[1]
      const bodyStart = i + raw[0].length
      const end = code.indexOf(close, bodyStart)
      const stop = end < 0 ? n : end + close.length
      push(code.slice(i, stop), 'string')
      i = stop
      continue
    }
    // string (incl. byte string b"…"), with escapes
    if (c === '"' || (c === 'b' && code[i + 1] === '"')) {
      let j = c === '"' ? i + 1 : i + 2
      while (j < n) {
        if (code[j] === '\\') {
          j += 2
          continue
        }
        if (code[j] === '"') {
          j++
          break
        }
        j++
      }
      push(code.slice(i, j), 'string')
      i = j
      continue
    }
    // char literal vs lifetime
    if (c === "'") {
      const ch = CHAR_LIT.exec(rest)
      if (ch != null) {
        push(ch[0], 'string')
        i += ch[0].length
        continue
      }
      const lt = LIFETIME.exec(rest)
      if (lt != null) {
        push(lt[0], 'lifetime')
        i += lt[0].length
        continue
      }
      push(c)
      i++
      continue
    }
    // number
    if (isDigit(c)) {
      const m = NUMBER.exec(rest)
      if (m != null) {
        let len = m[0].length
        const suf = NUM_SUFFIX.exec(code.slice(i + len))
        if (suf != null) len += suf[0].length
        push(code.slice(i, i + len), 'number')
        i += len
        continue
      }
    }
    // identifier → keyword / type / macro / function / plain
    if (isIdentStart(c)) {
      let j = i + 1
      while (j < n && isIdent(code[j])) j++
      const word = code.slice(i, j)
      // macro invocation: ident! (but not the `!=` operator)
      if (code[j] === '!' && code[j + 1] !== '=') {
        push(word, 'macro')
        push('!', 'macro')
        i = j + 1
        continue
      }
      if (KEYWORDS.has(word)) {
        push(word, 'keyword')
        i = j
        continue
      }
      if (PRIMITIVES.has(word) || /^[A-Z]/.test(word)) {
        push(word, 'type')
        i = j
        continue
      }
      // function call/def: ident immediately before `(`
      let k = j
      while (k < n && (code[k] === ' ' || code[k] === '\t')) k++
      if (code[k] === '(') {
        push(word, 'function')
        i = j
        continue
      }
      push(word)
      i = j
      continue
    }
    // attribute marker: #[ … ] / #![ … ]
    if (c === '#' && (code[i + 1] === '[' || (code[i + 1] === '!' && code[i + 2] === '['))) {
      push('#', 'attribute')
      i++
      continue
    }
    // anything else (punctuation, whitespace, newline): default colour
    push(c)
    i++
  }
  return spans
}
