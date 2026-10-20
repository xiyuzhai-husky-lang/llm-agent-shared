export interface Proposal {
  uid: string; batch: string; slot: string; directory: string; status: string; phase: string
  purpose: string; reason: string; revision: number; worker_revisions: number
  deadline_exceeded: boolean; candidate?: { id: string; title: string }
  policy: { rank: string; worker_revisions: number }
}
