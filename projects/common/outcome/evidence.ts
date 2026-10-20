import { createContext } from 'react'

export interface FragmentProps { report: unknown; reportText: string; folder: string }
export type EvidenceLoader = (path: string, signal?: AbortSignal) => Promise<string>

// The owning reviewer supplies its selected-revision reader; fragments never
// substitute current worktree files for committed test evidence.
export const FragmentEvidenceContext = createContext<EvidenceLoader>(async () => {
  throw new Error('No evidence reader supplied by this reviewer.')
})
