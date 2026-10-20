import type { LeanBenchMacro, LeanBenchMicro } from './data.ts'

export interface LeanBenchComparison {
  delta?: number
  slowdown: boolean
}

// A zero baseline has no finite relative delta; never manufacture Infinity or a percentage.
function delta(current: number, baseline: number | undefined): number | undefined {
  return baseline != null && baseline > 0 ? (current - baseline) / baseline : undefined
}

export function microComparison(row: LeanBenchMicro, baseline?: LeanBenchMicro): LeanBenchComparison {
  if (row.status !== 'ok' || (baseline != null && baseline.status !== 'ok')) return { slowdown: false }
  if (baseline == null) return {
    delta: row.delta_ratio ?? delta(row.ns_median, row.baseline_ns_median),
    slowdown: row.slowdown ?? false,
  }
  const ratio = delta(row.ns_median, baseline.ns_median)
  return {
    delta: ratio,
    slowdown: row.ns_median > baseline.ns_median + 3 * Math.max(row.error_ns, baseline.error_ns)
      && row.ns_median > baseline.ns_median * 1.2,
  }
}

export function macroComparison(row: LeanBenchMacro, baseline?: LeanBenchMacro): LeanBenchComparison {
  return {
    delta: baseline == null ? row.delta_ratio ?? delta(row.ms_per_use, row.baseline_ms_per_use)
      : delta(row.ms_per_use, baseline.ms_per_use),
    // Macro timings have no error bound. Preserve the runner's >20% slowdown flag.
    slowdown: row.slowdown ?? false,
  }
}
