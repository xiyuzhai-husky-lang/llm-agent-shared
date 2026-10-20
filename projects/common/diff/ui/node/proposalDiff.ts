import { execFile } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import type { DiffPresentation, SnapshotDiffClassification } from '../src/fileProtocol.ts'
import type { LoadedDiffFile } from '../src/filePatch.ts'
import { SnapshotError, requiredPath } from './snapshotGuards.ts'

export type DiffCandidate = { kind: 'commit'; commit: string } | { kind: 'working-tree' }
export interface DiffIndex { base: string; candidate: string; classifications: SnapshotDiffClassification[] }
export interface DiffByRefs extends DiffIndex {
  candidateSide: DiffCandidate
  presentation: DiffPresentation
  patch: string
  baseShort: string
  candidateShort: string
  files: Record<string, LoadedDiffFile>
  contents: Record<string, string>
}

/** Go prepares a pinned read model once. File requests are cache reads, with no subprocess or file watching. */
export function diffByRefs(repo: string, base: string, candidate: DiffCandidate): Promise<DiffByRefs> {
  const script = fileURLToPath(new URL('../../../../../scripts/proposal-tool.sh', import.meta.url))
  const binary = process.env.PROPOSAL_DIFF_BINARY
  return new Promise((resolve, reject) => {
    const child = execFile(binary ?? script, binary ? [] : ['diff'], { maxBuffer: 128 * 1024 * 1024 }, (error, stdout, stderr) => {
      if (error) { reject(new SnapshotError(422, stderr.trim() || error.message)); return }
      try { resolve(JSON.parse(stdout) as DiffByRefs) } catch (error) { reject(error) }
    })
    child.stdin!.end(JSON.stringify({ repo, base, candidate }))
  })
}
export async function readDiffFileEntry(_repo: string, data: DiffByRefs, path: string): Promise<LoadedDiffFile> {
  const file = data.files[requiredPath(path)]
  if (file == null) throw new SnapshotError(404, 'File is not in the selected diff.')
  return file
}
export async function readDiffFilePatch(repo: string, data: DiffByRefs, path: string): Promise<string> {
  return (await readDiffFileEntry(repo, data, path)).patch
}
export async function readDiffFileContent(_repo: string, data: DiffByRefs, path: string): Promise<string> {
  const content = data.contents[requiredPath(path)]
  if (content == null) throw new SnapshotError(404, 'File is not in the selected diff.')
  return content
}
