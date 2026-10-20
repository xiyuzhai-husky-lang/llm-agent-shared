import {DocumentSurfaceView, type CandidateKey, type DocumentSurface, type DocumentStop} from '@proposal/kathy-document'
import './main.css'
export interface MainTabView { candidate: CandidateKey; columns: DocumentSurface[][]; bypass: { bypasses: { fragment: string; reason: string }[] } | null }
export function ProposalMainTab({view}: {view: MainTabView}) {
 return <div className="proposal-main-tab"><div className="proposal-main-grid">{view.columns.map((column,index)=><div className="proposal-main-column" key={index}>{column.map(surface=><section className="proposal-main-section" key={surface.identifier} aria-label={surface.heading} data-tab={surface.identifier} data-content={surface.content.kind} data-stop={surface.stop??undefined}>
  <h2 className="proposal-main-section-heading"><span className={surface.stop===null?undefined:`candidate-stop-ground candidate-stop-ground-${surface.stop}`}>{surface.heading}{surface.stop!==null&&<StopMark stop={surface.stop}/>}</span></h2>
  <div className="proposal-main-section-body"><DocumentSurfaceView surface={surface} candidate={view.candidate} component="MainTab" absentClassName="proposal-main-section-absent"/></div>
 </section>)}{index === view.columns.length - 1 && <section className="proposal-main-section" aria-label="Bypass">
  <h2 className="proposal-main-section-heading">Bypass</h2>
  <div className="proposal-main-section-body" style={{padding: 16}}>{view.bypass === null ? <p>No bypasses.</p> : <pre style={{whiteSpace: 'pre-wrap', overflowWrap: 'anywhere'}}>{JSON.stringify(view.bypass, null, 2)}</pre>}</div>
 </section>}</div>)}</div></div>
}
/** A ❓ on the surface holding an incomplete candidate's question, a ⚠ on the one holding its Chow problems. */
function StopMark({stop}: {stop: DocumentStop}) {
 if(stop!=='incomplete'&&stop!=='chow-problems') throw new Error(`Main surface stop expected incomplete/chow-problems, found ${JSON.stringify(stop)}`)
 const name = stop === 'incomplete' ? 'incomplete' : 'Chow problems'
 return <span className="candidate-stop-mark" data-stop={stop} role="img" aria-label={name} title={name}>{stop === 'incomplete' ? '❓' : '⚠'}</span>
}
