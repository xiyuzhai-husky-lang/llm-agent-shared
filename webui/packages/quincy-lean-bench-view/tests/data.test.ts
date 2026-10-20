// @vitest-environment node
import { describe, expect, it } from 'vitest'
import example from '../fixtures/lean-bench.json'
import { isLeanBenchData, parseLeanBenchData } from '../src/data'

describe('schema lean-bench/1', () => {
  it('accepts the complete fixture example', () => {
    expect(parseLeanBenchData(example)).toBe(example)
    expect(isLeanBenchData(example)).toBe(true)
  })

  it.each(Object.keys(example))('rejects a missing root field: %s', (key) => {
    const invalid: Record<string, unknown> = { ...example }
    delete invalid[key]
    expect(() => parseLeanBenchData(invalid)).toThrow(`leanBench.${key}`)
    expect(isLeanBenchData(invalid)).toBe(false)
  })

  it('rejects a missing nested field with its precise path', () => {
    const invalid = structuredClone(example) as unknown as { micro: Record<string, unknown>[] }
    delete invalid.micro[0].error_ns
    expect(() => parseLeanBenchData(invalid)).toThrow('leanBench.micro[0].error_ns')
  })

  it.each([
    ['iterations', 1.5], ['batches', -1], ['ns_median', Infinity], ['ns_min', NaN],
    ['ns_max', '960'], ['status', 'passed'], ['slowdown', 'false'], ['delta_ratio', 'invalid'],
    ['baseline_ns_median', -1], ['process', [{ depth: -1, label: 'decode', us: 1 }]],
    ['process', [{ depth: 0, label: 'decode', us: '1' }]],
    ['nodes', 0], ['nodes', 1.5], ['nodes', true], ['budget_ns', 0], ['budget_ns', -1],
    ['budget_ns', Infinity], ['budget_level', 'yellow'], ['budget_level', []], ['over_budget', 'true'],
    ['alarm_factor', 0.5], ['alarm_factor', NaN],
  ])('rejects invalid micro %s (%j)', (key, value) => {
    const invalid = { ...example, micro: [{ ...example.micro[0], [key]: value }] }
    expect(isLeanBenchData(invalid)).toBe(false)
  })

  it('validates macro and summary numbers, modes and timestamps', () => {
    expect(isLeanBenchData({ ...example, macro: [{ ...example.macro[0], uses: 0.5 }] })).toBe(false)
    expect(isLeanBenchData({ ...example, summary: { ...example.summary, micro_fail: -1 } })).toBe(false)
    expect(isLeanBenchData({ ...example, mode: 'native' })).toBe(false)
    expect(isLeanBenchData({ ...example, generated_at: 'not a timestamp' })).toBe(false)
    expect(isLeanBenchData({ ...example, schema: 1 })).toBe(false)
    expect(isLeanBenchData({ ...example, schema: 2 })).toBe(false)
  })

  it('accepts null deltas from a zero baseline', () => {
    expect(isLeanBenchData({ ...example, micro: [{ ...example.micro[0], baseline_ns_median: 0, delta_ratio: null }] })).toBe(true)
  })

  it('validates additive budgets, severity consistency and summary counts in both kinds', () => {
    for (const kind of ['micro', 'macro'] as const) {
      for (const level of ['ok', 'warn', 'alarm']) {
        const row = { ...example[kind][0], budget_ns: 300, budget_level: level, over_budget: level !== 'ok', alarm_factor: 2 }
        expect(isLeanBenchData({ ...example, [kind]: [row] })).toBe(true)
        expect(isLeanBenchData({ ...example, [kind]: [{ ...row, over_budget: level === 'ok' }] })).toBe(false)
        const { budget_ns, ...withoutBudget } = row
        expect(isLeanBenchData({ ...example, [kind]: [withoutBudget] })).toBe(false)
      }
    }
    expect(isLeanBenchData({ ...example, summary: { ...example.summary, warnings: 1, alarms: 2 } })).toBe(true)
    for (const key of ['warnings', 'alarms']) {
      for (const value of [-1, 1.5, '1']) {
        expect(isLeanBenchData({ ...example, summary: { ...example.summary, [key]: value } })).toBe(false)
      }
    }
    // Old committed baselines still parse with no additive fields.
    const old = structuredClone(example) as {
      micro: Record<string, unknown>[]; macro: Record<string, unknown>[]; summary: Record<string, unknown>
    }
    for (const row of [...old.micro, ...old.macro]) {
      for (const key of ['nodes', 'budget_ns', 'budget_level', 'over_budget', 'alarm_factor']) delete row[key]
    }
    delete old.summary.warnings
    delete old.summary.alarms
    expect(isLeanBenchData(old)).toBe(true)
  })

  it('accepts absent optional comparisons and processes, empty runs and zero timings', () => {
    const data = parseLeanBenchData(structuredClone(example))
    for (const row of data.micro) {
      delete row.process
      delete row.baseline_ns_median
      delete row.delta_ratio
      delete row.slowdown
      row.iterations = 0
      row.batches = 0
      row.ns_median = 0
      row.status = 'unsupported'
    }
    expect(isLeanBenchData(data)).toBe(true)
    expect(isLeanBenchData({ ...data, micro: [], macro: [] })).toBe(true)
    expect(isLeanBenchData(null)).toBe(false)
  })
})
