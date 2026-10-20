import type { PublishedDoctrines } from './doctrineReportModel'
import './review-tabs.css'

/** The commissar's report for the candidate on screen: every doctrine that ran with what it found, then the
convictions. Read-only on purpose. A conviction the Worker declined reaches the user as a doubt labelled
`[conviction]` and is ruled on in the Doubts tab; this tab exists so that what the commissar examined, found
clean or could not finish is visible at all, which no other surface shows. */
export function DoctrinesTab({ published }: { published?: PublishedDoctrines }) {
  if (published == null) {
    return <p className="review-doctrines-empty">No commissar report for this candidate. The doctrines it ran,
      what each found, and any it could not finish would appear here.</p>
  }
  return (
    <div className="review-doctrines">
      {published.halt != null && (
        <p className="review-doctrines-halt" role="alert"><strong>Halt</strong> {published.halt}</p>
      )}
      {published.doctrines.length === 0
        ? <p className="review-doctrines-clean">The report names no doctrines one by one.</p>
        : (
          <ol className="review-doctrines-list" aria-label="Doctrines run">
            {published.doctrines.map((doctrine) => (
              <li key={doctrine.name} className={`review-doctrines-item review-doctrines-${doctrine.status}`}>
                <p className="review-doctrines-headline">
                  <span className="review-doctrines-name">{doctrine.name}</span>
                  <span className={`review-doctrines-status review-doctrines-status-${doctrine.status}`}>
                    {doctrine.status === 'convicted' ? `convicted · ${doctrine.convictions.join(', ')}` : doctrine.status}
                  </span>
                </p>
                {doctrine.reason != null && <p className="review-doctrines-detail">{doctrine.reason}</p>}
                {doctrine.note.length > 0 && <p className="review-doctrines-detail">{doctrine.note}</p>}
              </li>
            ))}
          </ol>
        )}
      {published.convictions.length === 0
        ? <p className="review-doctrines-clean">No convictions.</p>
        : (
          <ol className="review-doctrines-list" aria-label="Convictions">
            {published.convictions.map((conviction) => (
              <li key={conviction.number} className="review-doctrines-item">
                <p className="review-doctrines-headline">{conviction.title}</p>
                <p className="review-doctrines-meta">
                  {conviction.location.length > 0 && <span className="review-doctrines-location">{conviction.location}</span>}
                  {conviction.doctrine.length > 0 && <span className="review-doctrines-doctrine">{conviction.doctrine}</span>}
                </p>
                {conviction.detail.length > 0 && <p className="review-doctrines-detail">{conviction.detail}</p>}
              </li>
            ))}
          </ol>
        )}
      {(published.starting_files != null || published.coverage.length > 0) && (
        <div className="review-doctrines-coverage">
          {published.starting_files != null && <p>Starting files: {published.starting_files}</p>}
          {published.coverage.map((line, position) => <p key={position}>{line}</p>)}
        </div>
      )}
    </div>
  )
}
