import { tokenizeLines } from './highlight'
import type { SourceFile } from './source'

/**
 * Fetch a repo source file from the dev server and return the whole highlighted
 * file. The dev server serves repo files read-only under `base`.
 */
export async function loadSourceFile(base: string, path: string, line: number): Promise<SourceFile> {
  const res = await fetch(base + path)
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
  const text = await res.text()
  return { focus: line, lines: tokenizeLines(text, path) }
}
