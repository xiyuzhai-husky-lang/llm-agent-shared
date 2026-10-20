export interface DebuggerCheck {
  name: string; cwd: string; command: string[]; exit_code: number;
  status: 'passed' | 'failed'; duration_ms: number; diagnostic: string
}
export interface DebuggerReport { format: 1; status: 'passed' | 'failed'; duration_ms: number; checks: DebuggerCheck[] }
export function parseDebuggerReport(text: string): DebuggerReport {
  const value = JSON.parse(text)
  if (value?.format !== 1 || !Array.isArray(value.checks) || value.checks.length === 0
    || !Number.isFinite(value.duration_ms) || value.duration_ms < 0
    || !value.checks.every((row: DebuggerCheck) => row && typeof row.name === 'string'
      && typeof row.cwd === 'string' && Array.isArray(row.command) && row.command.every(part => typeof part === 'string')
      && Number.isInteger(row.exit_code) && Number.isFinite(row.duration_ms) && row.duration_ms >= 0
      && typeof row.diagnostic === 'string' && row.status === (row.exit_code === 0 ? 'passed' : 'failed'))
    || value.status !== (value.checks.some((row: DebuggerCheck) => row.exit_code !== 0) ? 'failed' : 'passed')) {
    throw new Error('Invalid debugger test report')
  }
  return value
}
