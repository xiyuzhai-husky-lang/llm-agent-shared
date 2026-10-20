import type { FragmentProps } from '../../../evidence'
import { parseDebuggerReport } from '../report'
export default function FragmentView({ reportText }: FragmentProps) {
  try {
    const report = parseDebuggerReport(reportText)
    return <section aria-label="Debugger test results">
      <p>{report.checks.filter(row => row.status === 'passed').length} passed / {report.checks.length} commands · {(report.duration_ms / 1000).toFixed(1)} s</p>
      {report.checks.map(row => <details key={row.name} open={row.status === 'failed'}>
        <summary>{row.name}: {row.status} · exit {row.exit_code} · {(row.duration_ms / 1000).toFixed(1)} s</summary>
        <p>cwd: {row.cwd}</p><pre style={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>{row.command.join(' ')}</pre>
        {row.diagnostic && <pre style={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>{row.diagnostic}</pre>}
      </details>)}
    </section>
  } catch (error) { return <p role="alert">{String(error)}</p> }
}
