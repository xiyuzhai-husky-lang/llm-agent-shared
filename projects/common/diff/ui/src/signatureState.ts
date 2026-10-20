export interface DiffHunkSignature { path: string; hunk: string; signer: string; at: string; carried?: boolean; signedAtCandidate?: string }
export interface SignatureState {
  signatures: readonly DiffHunkSignature[]
  totalHunks: number; signedHunks: number; unsignedHunks: number
  signedFiles: number; totalFiles: number; pendingFiles: number
}
/** Bind display lookups to the signature projection returned by Go. */
export function diffSignatureState(state: SignatureState) {
  return { ...state, signature: (path: string, hunk: string) => state.signatures.find(entry => entry.path === path && entry.hunk === hunk) }
}
