/** Token classes the lexer assigns; each maps to a colour below. */
export type TokenKind =
  | 'keyword'
  | 'type'
  | 'function'
  | 'macro'
  | 'string'
  | 'number'
  | 'comment'
  | 'lifetime'
  | 'attribute'

/** GitHub-light-ish palette, tuned to read on the report's light surfaces. */
export const COLOR: Record<TokenKind, string> = {
  keyword: '#cf222e',
  type: '#953800',
  function: '#8250df',
  macro: '#8250df',
  string: '#0a3069',
  number: '#0550ae',
  comment: '#6e7781',
  lifetime: '#1f6feb',
  attribute: '#6e7781',
}
