import { ProposalPeekPanel, peekSubject, type PeekPanelView } from '@proposal/peek-panel'
import { ProposalOutcomeTab, type OutcomeTabView } from '@proposal/outcome-tab'
import { ProposalDiffTab, type DiffTabView } from '@proposal/diff-tab'
import { ProposalDoubtsTab, type DoubtsTabView } from '@proposal/doubts-tab'
import { ProposalReviewPanel, type ReviewPanelView } from '@proposal/review-panel'
import { ProposalTestsTab, type TestsTabView } from '@proposal/tests-tab'
import { createRoot } from 'react-dom/client'
import { ProposalPage, component, candidateSubject } from '@proposal/connection'
import { ProposalTitleBar, titleBarSubject, type TitleBarView } from '@proposal/title-bar'
import { ProposalOverviewSidebar, overviewSubject, type SidebarView } from '@proposal/overview-sidebar'
import { ProposalBatchStackPage, batchStackSubject, type BatchPageView } from '@proposal/batch-stack-page'
import { ProposalMainTab, type MainTabView } from '@proposal/main-tab'
import { ProposalEffectTab, type EffectTabView } from '@proposal/effect-tab'
import { DecisionConnection, resolveDecisionLocation } from '@proposal/connection/decision'

const components = [
  component<MainTabView>('MainTab', 'tab', candidateSubject, view => <ProposalMainTab view={view}/>),
  component<TestsTabView>('TestsTab', 'tab', candidateSubject, (view, navigate) => <ProposalTestsTab view={view} navigate={navigate}/>),
  component<DoubtsTabView>('DoubtsTab', 'tab', candidateSubject, (view, navigate, act) => <ProposalDoubtsTab key={view.candidate.candidate_commit} view={view} navigate={navigate} act={act}/>),
  component<DiffTabView>('DiffTab', 'tab', candidateSubject, (view, navigate, act) => <ProposalDiffTab key={view.candidate.candidate_commit} view={view} navigate={navigate} act={act}/>),
  component<EffectTabView>('EffectTab', 'tab', candidateSubject, view => <ProposalEffectTab view={view}/>),
  component<OutcomeTabView>('OutcomeTab', 'tab', candidateSubject, (view, navigate) => <ProposalOutcomeTab view={view} navigate={navigate}/>),
  component<ReviewPanelView>('ReviewPanel', 'panel', candidateSubject, (view, navigate, act) => <ProposalReviewPanel key={view.candidate.proposal_number} view={view} navigate={navigate} act={act}/>),
  component<PeekPanelView>('PeekPanel', 'panel', peekSubject, (view, navigate, act) => <ProposalPeekPanel key={view.candidate.candidate_commit} view={view} navigate={navigate} act={act}/>),
  component<SidebarView>('OverviewSidebar', 'sidebar', overviewSubject, (view, navigate) => <ProposalOverviewSidebar view={view} navigate={navigate}/>),
  component<TitleBarView>('TitleBar', 'header', titleBarSubject, (view, navigate, _act, shell) => <ProposalTitleBar view={view} navigate={navigate} reviewTopline={shell.reviewTopline}/>),
  component<BatchPageView>('BatchStackPage', 'main', batchStackSubject, (view, navigate) => <ProposalBatchStackPage view={view} navigate={navigate}/>),
]
const initialize = () => Promise.resolve()
async function start() {
  await resolveDecisionLocation()
  createRoot(document.getElementById('root')!).render(<ProposalPage components={components} initialize={initialize} connection={DecisionConnection} baseUrl="/"/>)
}
void start().catch(error => { document.getElementById('root')!.textContent = String(error) })