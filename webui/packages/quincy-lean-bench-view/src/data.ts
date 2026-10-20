/** The lean-bench/1 report. Times are per call unless noted. */
export interface LeanBenchProcess {
  depth: number
  label: string
  us: number
}

export interface LeanBenchBudget {
  /** Absolute allowance in ns/call (micro) or ns/use (macro). */
  budget_ns?: number
  over_budget?: boolean
  budget_level?: 'ok' | 'warn' | 'alarm'
  alarm_factor?: number
}

export interface LeanBenchMicro extends LeanBenchBudget {
  id: string
  module: string
  case: string
  input: string
  output: string
  iterations: number
  batches: number
  ns_median: number
  ns_min: number
  ns_max: number
  error_ns: number
  status: 'ok' | 'unsupported' | 'fail'
  /** Application nodes, or reduction steps; legacy records default to one. */
  nodes?: number
  baseline_ns_median?: number
  delta_ratio?: number | null
  slowdown?: boolean
  process?: LeanBenchProcess[]
}

export interface LeanBenchMacro extends LeanBenchBudget {
  id: string
  file: string
  uses: number
  wall_ms: number
  ms_per_use: number
  status?: 'ok' | 'fail'
  error?: string
  baseline_ms_per_use?: number
  delta_ratio?: number | null
  slowdown?: boolean
}

export interface LeanBenchSummary {
  micro_cases: number
  micro_unsupported: number
  micro_fail: number
  micro_slowdowns: number
  macro_files: number
  macro_slowdowns: number
  warnings?: number
  alarms?: number
}

export interface LeanBenchData {
  schema: 'lean-bench/1'
  generated_at: string
  commit: string
  mode: 'interpreted' | 'precompiled'
  host: string
  micro: LeanBenchMicro[]
  macro: LeanBenchMacro[]
  summary: LeanBenchSummary
}

/** Validate untrusted JSON, preserving runner-provided values. Throws a field-specific TypeError. */
export function parseLeanBenchData(value: unknown): LeanBenchData {
  const data = record(value, 'leanBench')
  if (data.schema !== 'lean-bench/1') invalid('leanBench.schema', 'lean-bench/1')
  oneOf(data.mode, 'leanBench.mode', ['interpreted', 'precompiled'])
  for (const key of ['generated_at', 'commit', 'host']) string(data[key], `leanBench.${key}`, true)
  if (!Number.isFinite(Date.parse(data.generated_at as string))) invalid('leanBench.generated_at', 'a timestamp')
  array(data.micro, 'leanBench.micro').forEach((value, index) => {
    const path = `leanBench.micro[${index}]`
    const row = record(value, path)
    for (const key of ['id', 'module', 'case']) string(row[key], `${path}.${key}`, true)
    for (const key of ['input', 'output']) string(row[key], `${path}.${key}`)
    for (const key of ['iterations', 'batches']) number(row[key], `${path}.${key}`, true)
    for (const key of ['ns_median', 'ns_min', 'ns_max', 'error_ns']) number(row[key], `${path}.${key}`)
    oneOf(row.status, `${path}.status`, ['ok', 'unsupported', 'fail'])
    if ('nodes' in row) {
      number(row.nodes, `${path}.nodes`, true)
      if (row.nodes === 0) invalid(`${path}.nodes`, 'a positive integer')
    }
    budget(row, path)
    comparison(row, path, 'baseline_ns_median')
    if ('process' in row) array(row.process, `${path}.process`).forEach((value, index) => {
      const childPath = `${path}.process[${index}]`
      const child = record(value, childPath)
      number(child.depth, `${childPath}.depth`, true)
      string(child.label, `${childPath}.label`)
      number(child.us, `${childPath}.us`)
    })
  })
  array(data.macro, 'leanBench.macro').forEach((value, index) => {
    const path = `leanBench.macro[${index}]`
    const row = record(value, path)
    for (const key of ['id', 'file']) string(row[key], `${path}.${key}`, true)
    number(row.uses, `${path}.uses`, true)
    for (const key of ['wall_ms', 'ms_per_use']) number(row[key], `${path}.${key}`)
    if ('status' in row) oneOf(row.status, `${path}.status`, ['ok', 'fail'])
    if ('error' in row) string(row.error, `${path}.error`)
    budget(row, path)
    comparison(row, path, 'baseline_ms_per_use')
  })
  const summary = record(data.summary, 'leanBench.summary')
  for (const key of ['micro_cases', 'micro_unsupported', 'micro_fail', 'micro_slowdowns', 'macro_files', 'macro_slowdowns']) {
    number(summary[key], `leanBench.summary.${key}`, true)
  }
  for (const key of ['warnings', 'alarms']) {
    if (key in summary) number(summary[key], `leanBench.summary.${key}`, true)
  }
  return value as LeanBenchData
}

export function isLeanBenchData(value: unknown): value is LeanBenchData {
  try {
    parseLeanBenchData(value)
    return true
  } catch {
    return false
  }
}

function invalid(path: string, expected: string): never {
  throw new TypeError(`${path}: expected ${expected}.`)
}

function record(value: unknown, path: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) invalid(path, 'an object')
  return value as Record<string, unknown>
}

function array(value: unknown, path: string): unknown[] {
  if (!Array.isArray(value)) invalid(path, 'an array')
  return value
}

function string(value: unknown, path: string, nonempty = false): void {
  if (typeof value !== 'string' || (nonempty && value.trim().length === 0)) invalid(path, nonempty ? 'a nonempty string' : 'a string')
}

function number(value: unknown, path: string, integer = false, signed = false): void {
  if (typeof value !== 'number' || !Number.isFinite(value) || (!signed && value < 0)
    || (integer && !Number.isSafeInteger(value))) invalid(path, integer ? 'a nonnegative integer' : signed ? 'a finite number' : 'a nonnegative finite number')
}

function oneOf(value: unknown, path: string, values: string[]): void {
  if (typeof value !== 'string' || !values.includes(value)) invalid(path, values.join(' | '))
}

function comparison(row: Record<string, unknown>, path: string, baselineKey: string): void {
  if (baselineKey in row) number(row[baselineKey], `${path}.${baselineKey}`)
  if ('delta_ratio' in row && row.delta_ratio !== null) number(row.delta_ratio, `${path}.delta_ratio`, false, true)
  if ('slowdown' in row && typeof row.slowdown !== 'boolean') invalid(`${path}.slowdown`, 'a boolean')
}

function budget(row: Record<string, unknown>, path: string): void {
  if ('budget_ns' in row) {
    number(row.budget_ns, `${path}.budget_ns`)
    if (row.budget_ns === 0) invalid(`${path}.budget_ns`, 'a positive finite number')
  }
  if ('alarm_factor' in row) {
    number(row.alarm_factor, `${path}.alarm_factor`)
    if ((row.alarm_factor as number) < 1) invalid(`${path}.alarm_factor`, 'a number >= 1')
  }
  if ('over_budget' in row && typeof row.over_budget !== 'boolean') invalid(`${path}.over_budget`, 'a boolean')
  if ('budget_level' in row) {
    oneOf(row.budget_level, `${path}.budget_level`, ['ok', 'warn', 'alarm'])
    if (!('budget_ns' in row)) invalid(`${path}.budget_ns`, 'a positive finite number')
    if (row.over_budget !== (row.budget_level !== 'ok')) invalid(`${path}.over_budget`, 'consistent with budget_level')
  }
}

/** Preserve absolute severity independently of relative baseline comparisons. */
export function budgetLevel(row: LeanBenchBudget): 'ok' | 'warn' | 'alarm' | undefined {
  return row.budget_level ?? (row.over_budget === true ? 'warn' : row.over_budget === false ? 'ok' : undefined)
}
