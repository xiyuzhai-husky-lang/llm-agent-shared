import {DocumentSurfaceView, type CandidateKey, type DocumentSurface} from '@proposal/kathy-document'
import './effect.css'
export interface EffectTabView {candidate: CandidateKey; effect: DocumentSurface}
export function ProposalEffectTab({view}: {view: EffectTabView}) {
 return <div className="proposal-effect-tab"><DocumentSurfaceView surface={view.effect} candidate={view.candidate} component="EffectTab" absentClassName="app-state selection-state"/></div>
}
