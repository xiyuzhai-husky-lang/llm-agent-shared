import type { MeasuredCommitView, OutcomeFragmentView, OutcomeItemView, OutcomeItemsView, OutcomeTabView } from './types'
import './tab.css'
export type { OutcomeTabView } from './types'
export function ProposalOutcomeTab({view, navigate}: {view: OutcomeTabView; navigate: (href: string) => void}) {
  const query = new URLSearchParams(window.location.search)
  const selectedKey = query.get('report')
  const selected = selectedKey === null ? view.reports[0] : view.reports.find(report => report.key === selectedKey)
  if (selectedKey !== null && selected === undefined) throw new Error(`Outcome candidate ${view.candidate}: report=${JSON.stringify(selectedKey)} does not exist; available=${JSON.stringify(view.reports.map(report => report.key))}`)
  return (
    <div className="outcome-browser">
      <nav aria-label="Outcome fragments" className="outcome-fragment-list">
        {view.reports.map(report => (
          <button key={report.key} type="button" aria-pressed={selected?.key === report.key} onClick={() => {query.set('report', report.key); navigate(`?${query}`)}}>
            <span>{report.label}</span>
            <small className="outcome-fragment-badge" data-status={report.status.toLowerCase()}>{report.status}{report.elapsed === null ? null : <span className="outcome-fragment-elapsed"> · {report.elapsed}</span>}</small>
          </button>
        ))}
      </nav>
      <section aria-label="Outcome results" className="outcome-fragment-detail">
        {view.measured_commit === null ? null : <MeasuredCommit measured={view.measured_commit} />}
        {selected === undefined ? <p className="outcome-state">No reports.</p> : <OutcomeFragment key={selected.key} fragment={selected} />}
      </section>
    </div>
  )
}

function OutcomeFragment({ fragment }: { fragment: OutcomeFragmentView }) {
  const report = fragment.report
  return (
    <>
      {fragment.measured_commit === null ? null : <MeasuredCommit measured={fragment.measured_commit} />}
      {fragment.disagreement_notice === null ? null : (
        <p className="outcome-state" role="alert">
          {fragment.disagreement_notice}
        </p>
      )}
      {fragment.stamped_message === null ? null : <p className="outcome-state">{fragment.stamped_message}</p>}
      {report.alert !== null ? (
        <p role="alert" className="outcome-state">
          {report.alert}
        </p>
      ) : (
        <div className="outcome-sets">
          <section aria-label="Selected test set" className="outcome-set-body">
            <div className="outcome-report">
              {report.title === null ? null : <h3>{report.title}</h3>}
              <dl className="outcome-figures">
                {report.figures.map((figure) => (
                  <div key={figure.label}>
                    <dt>{figure.label}</dt>
                    <dd className={figure.tone === null ? undefined : `outcome-token-${figure.tone}`}>
                      {figure.value}
                    </dd>
                  </div>
                ))}
              </dl>
              {report.table === null || report.table.columns.length === 0 ? null : (
                <table className="outcome-table">
                  <thead><tr>{report.table.columns.map(column => <th key={column} scope="col">{column}</th>)}</tr></thead>
                  <tbody>{report.table.rows.map((row, index) => <tr key={index}>{row.map((cell, column) => <td key={column}>{cell}</td>)}</tr>)}</tbody>
                </table>
              )}
              {report.lists.map((list) =>
                list.folded ? (
                  <FoldedItems key={list.title} list={list} />
                ) : (
                  <Items key={list.title} list={list} />
                ),
              )}
            </div>
          </section>
        </div>
      )}
      {report.note === null ? null : <p className="outcome-state">{report.note}</p>}
    </>
  )
}

function MeasuredCommit({ measured }: { measured: MeasuredCommitView }) {
  return (
    <p className={`outcome-state outcome-measured outcome-measured-${measured.tone}`} data-kind={measured.kind}>
      {measured.label}
    </p>
  )
}

function Items({ list }: { list: OutcomeItemsView }) {
  return (
    <section className="outcome-items" aria-label={list.title}>
      <h4>
        {list.title}
        {list.count === null ? null : <span className="outcome-counts"> {list.count}</span>}
      </h4>
      {list.emptyText !== null ? (
        <p className="outcome-state outcome-empty">{list.emptyText}</p>
      ) : (
        <ItemList items={list.items} />
      )}
    </section>
  )
}

/** Everything that ran, folded: the count is the headline, the list is for checking coverage. */
function FoldedItems({ list }: { list: OutcomeItemsView }) {
  return (
    <section className="outcome-items" aria-label={list.title}>
      <details>
        <summary>
          <h4 className="outcome-inline-title">
            {list.title}
            <span className="outcome-counts"> {list.count}</span>
          </h4>
        </summary>
        {list.emptyText !== null ? (
          <p className="outcome-state outcome-empty">{list.emptyText}</p>
        ) : (
          <ItemList items={list.items} />
        )}
      </details>
    </section>
  )
}

function ItemList({ items }: { items: OutcomeItemView[] }) {
  return (
    <ul>
      {items.map((item) => (
        <li key={item.key}>
          {item.detail === null ? (
            <Row item={item} />
          ) : (
            <details>
              <summary>
                <Row item={item} />
              </summary>
              <pre>{item.detail}</pre>
            </details>
          )}
          {item.children.length === 0 ? null : (
            <div className="outcome-item-children">
              <ItemList items={item.children} />
            </div>
          )}
        </li>
      ))}
    </ul>
  )
}

function Row({ item }: { item: OutcomeItemView }) {
  return (
    <span className="outcome-item">
      <span className={`outcome-item-label outcome-token-${item.tone}`}>{item.label}</span>
      {item.note === null ? null : <span className="outcome-item-note">{item.note}</span>}
    </span>
  )
}
