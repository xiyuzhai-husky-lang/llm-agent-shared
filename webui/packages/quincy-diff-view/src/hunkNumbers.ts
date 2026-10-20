import type { HunkData } from 'react-diff-view'

/** Non-textual candidate hunks retain the journal identity and have no source range. */
export function hunkMetadata(hunk: Pick<HunkData, 'content'>): { id: string; text: string } | undefined {
  return (hunk as HunkData & { metadata?: { id: string; text: string } }).metadata
}

export interface NumberedHunk { path: string; range: string; ordinal: number; total: number }

/** The canonical identity keeps explicit zero counts and omits trailing function context. */
export function diffHunkRange(hunk: Pick<HunkData, 'content'>): string {
  if (hunkMetadata(hunk) != null) return hunk.content
  const range = /^@@\s*-(\d+)(?:,(\d+))?\s*\+(\d+)(?:,(\d+))?\s*@@/.exec(hunk.content)
  if (range == null) throw new Error(`Invalid hunk header: ${hunk.content}`)
  return `@@ -${range[1]},${range[2] ?? '1'} +${range[3]},${range[4] ?? '1'} @@`
}

export function hunkNumberLabel(hunk: NumberedHunk): string { return `#${hunk.ordinal}` }

export function diffHunkKey(hunk: HunkData): string {
  if (hunkMetadata(hunk) != null) return hunkMetadata(hunk)!.id
  return `${hunk.oldStart}:${hunk.oldLines}:${hunk.newStart}:${hunk.newLines}:${hunk.content}`
}
