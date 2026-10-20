/**
 * The batch page: a page of its own, read-only, unsettled batches top first.
 */

import type { BatchView, BatchPageView } from './types'

import './batch.css'
export type { BatchPageView } from './types'
export const batchStackSubject = (location: URLSearchParams) => location.get('page') === 'batch' ? null : undefined
export function ProposalBatchStackPage({ view: batch, navigate }: { view: BatchPageView; navigate: (href: string) => void }) {
  const close = () => { const query = new URLSearchParams(window.location.search); query.delete('page'); navigate(`?${query}`) }
  return (
    <main className="proposal-batch-page">
      <header className="proposal-batch-header">
        <h1>{batch.heading}</h1>
        {batch.currentBatchLabel === null ? null : <span>{batch.currentBatchLabel}</span>}
        <button type="button" onClick={close}>
          {batch.closeLabel}
        </button>
      </header>
      {batch.failure === null ? null : (
        <p className="proposal-review-error" role="alert">
          {batch.failure}
        </p>
      )}
      {batch.absenceNotice === null ? null : (
        <p className="proposal-review-note">{batch.absenceNotice}</p>
      )}
      <section>
        <h2>Stacked</h2>
        {batch.stacked.map((one) => (
          <Batch key={one.id} batch={one} />
        ))}
      </section>
      <section>
        <h2>Settled</h2>
        {batch.settled.map((one) => (
          <Batch key={one.id} batch={one} />
        ))}
      </section>
    </main>
  )
}

function Batch({ batch }: { batch: BatchView }) {
  return (
    <article className="proposal-batch">
      <header>
        <strong>{batch.title}</strong>
        <code>{batch.id}</code>
        <span>{batch.stateLabel}</span>
        <span>{batch.positionLabel}</span>
        {batch.startedAt === null ? null : <span>{batch.startedAt}</span>}
      </header>
      {batch.description === null ? null : <p>{batch.description}</p>}
      {batch.approvalNotice === null ? null : (
        <p className="proposal-review-note">{batch.approvalNotice}</p>
      )}
      <ol className="proposal-batch-members">
        {batch.members.map((member) => (
          <li key={member.uid}>
            <span className="proposal-ordinal">{member.ordinalLabel}</span>
            <strong>{member.title}</strong>
            <code>{member.uid}</code>
            <span>{member.stateLabel}</span>
            <span>{member.workflowLabel}</span>
            {member.branch === null ? null : <code>{member.branch}</code>}
            <small>{member.scope}</small>
            {member.settlementNotice === null ? null : <small>{member.settlementNotice}</small>}
          </li>
        ))}
      </ol>
    </article>
  )
}
