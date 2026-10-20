export interface BatchMemberView {
  uid: string; ordinalLabel: string; title: string; scope: string; branch: string | null
  stateLabel: string; workflowLabel: string; settlementNotice: string | null
}
export interface BatchView {
  id: string; title: string; description: string | null; stateLabel: string; positionLabel: string
  startedAt: string | null; approvalNotice: string | null; members: BatchMemberView[]
}
export interface BatchPageView {
  heading: string; currentBatchLabel: string | null; closeLabel: string
  failure: string | null; absenceNotice: string | null; stacked: BatchView[]; settled: BatchView[]
}
