import type { FileData } from 'react-diff-view'
import type { DiffKind, DiffClass, DiffSubclass } from '@projects/diff-ui/classification'

export interface IndexedDiffFile {
  file: FileData
  index: number
  path: string
  stats: DiffStats
}

export interface DiffStats {
  hunkCount: number
  signedHunkCount?: number
  fileCount: number
  additions: number
  deletions: number
}

export interface DiffSubclassGroup {
  subclass: DiffSubclass
  label: string
  files: IndexedDiffFile[]
  stats: DiffStats
  tree: DiffTreeNode[]
}

export type DiffTreeNode =
  | { kind: 'file'; path: string; label: string; entry: IndexedDiffFile }
  | { kind: 'directory'; path: string; label: string; children: DiffTreeNode[]; stats: DiffStats }

export interface DiffClassGroup {
  class: DiffClass
  label: string
  subclassGroups: DiffSubclassGroup[]
  stats: DiffStats
}

export interface DiffKindGroup {
  kind: DiffKind
  label: string
  classGroups: DiffClassGroup[]
  stats: DiffStats
}

/** Traverses display order and naturally skips empty subclasses. */
export function groupedDiffFiles(groups: DiffKindGroup[]): IndexedDiffFile[] {
  return groups.flatMap(kind => kind.classGroups.flatMap(fileClass =>
    fileClass.subclassGroups.flatMap(subclass => subclass.files)))
}
