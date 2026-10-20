/** Git C-quotes non-ASCII paths in patch headers (`"a/…/AshisogiJiz\305\215/x.lean"`), escaping UTF-8 *bytes* as octal.
 *  gitdiff-parser does not understand that form, so headers are rewritten to plain UTF-8 paths before parsing. */
export function decodeGitQuoted(body: string): string {
  const bytes: number[] = []
  const encoder = new TextEncoder()
  for (let i = 0; i < body.length; i += 1) {
    const ch = body[i]
    if (ch !== '\\') { bytes.push(...encoder.encode(ch)); continue }
    const octal = body.slice(i + 1, i + 4)
    if (/^[0-7]{3}$/.test(octal)) { bytes.push(Number.parseInt(octal, 8)); i += 3; continue }
    const escape = body[i + 1]; i += 1
    if (escape === 't') bytes.push(9)
    else if (escape === 'n') bytes.push(10)
    else if (escape === 'r') bytes.push(13)
    else bytes.push(...encoder.encode(escape ?? ''))
  }
  return new TextDecoder().decode(new Uint8Array(bytes))
}

const QUOTED = /"((?:\\.|[^"\\])*)"/g
const HEADER = /^(diff --git |--- |\+\+\+ |rename from |rename to |copy from |copy to )/

/** Rewrite every quoted path in patch header lines to its decoded UTF-8 form; hunk content is untouched. */
export function unquoteGitHeaderPaths(patch: string): string {
  return patch.split('\n').map((line) => (HEADER.test(line) && line.includes('"')
    ? line.replace(QUOTED, (_match, body: string) => decodeGitQuoted(body))
    : line)).join('\n')
}
