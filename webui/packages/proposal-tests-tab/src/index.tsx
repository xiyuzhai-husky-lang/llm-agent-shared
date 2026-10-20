import { ProposalQuoteNotebookSurface, type QuoteNotebookTabView } from '@proposal/quote-notebook'
export type TestsTabView = QuoteNotebookTabView
export function ProposalTestsTab({view, navigate}: { view: TestsTabView; navigate: (href: string) => void }) {
 return <ProposalQuoteNotebookSurface view={view} component="TestsTab" words={{surface: 'tests', heading: 'Selected tests', zeroState: 'No selected tests.', withDiscussion: false}} selectionKey="tests-item" navigate={navigate}/>
}
