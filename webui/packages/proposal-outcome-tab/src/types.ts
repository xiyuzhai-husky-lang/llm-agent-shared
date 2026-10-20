export interface MeasuredCommitView { kind: 'matches' | 'mismatch'; label: string; tone: 'passed' | 'failed' | 'warn' }
export interface OutcomeItemView { key: string; label: string; note: string | null; tone: string; detail: string | null; children: OutcomeItemView[] }
export interface OutcomeItemsView { title: string; count: number | null; items: OutcomeItemView[]; emptyText: string | null; folded: boolean }
export interface OutcomeFragmentView {
  key: string; kind: string; label: string; file: string | null; status: string; elapsed: string | null; failure_count: number;
  measured_commit: MeasuredCommitView | null; disagreement_notice: string | null; stamped_message: string | null;
  report: { kind: string; title: string | null; table: { columns: string[]; rows: string[][] } | null; figures: {label: string; value: string; tone: string | null}[]; lists: OutcomeItemsView[]; alert: string | null; note: string | null }
}
export interface OutcomeTabView { candidate: string; alert: boolean; measured_commit: MeasuredCommitView | null; reports: OutcomeFragmentView[] }
