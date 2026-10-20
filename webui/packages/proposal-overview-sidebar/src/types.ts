export interface SidebarRowView {
  folder: string; uid: string; title: string; ordinalLabel: string; href: string; dateLabel: string
  branch: string | null; stop: string | null; candidateKind: 'complete' | 'incomplete' | 'chow_problems'; statusLabel: string | null
  workflowLabel: string | null
}
export interface SidebarView {
  title: string; batchPageLabel: string; stackNotice: string | null; emptyNotice: string | null; errors: string[]; defaultHref: string | null
  parts: { heading: string; count: number; foldable: boolean; collapsed: boolean; rows: SidebarRowView[] }[]
}
