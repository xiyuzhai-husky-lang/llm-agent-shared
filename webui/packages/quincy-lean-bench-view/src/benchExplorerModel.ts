import type { LeanBenchData, LeanBenchMacro, LeanBenchMicro } from './data.ts'
import { budgetLevel } from './data.ts'
import { macroComparison, microComparison, type LeanBenchComparison } from './comparison.ts'
import type { ExplorerNode, ExplorerTone } from './explorerModel.ts'
const numbers = new Intl.NumberFormat('en-US', { maximumFractionDigits: 3 })

export type Entry = { key: string; hash: string; comparison: LeanBenchComparison } & (
  | { kind: 'micro'; row: LeanBenchMicro; baseline?: LeanBenchMicro }
  | { kind: 'macro'; row: LeanBenchMacro; baseline?: LeanBenchMacro }
)
const microKey = (row: LeanBenchMicro) => JSON.stringify([row.module, row.id])
export const entryName = (entry: Entry) => entry.kind === 'micro' ? entry.row.case : entry.row.file.split('/').pop()!
export const entryGroup = (entry: Entry) => entry.kind === 'micro' ? entry.row.module : 'Macro'
export const entryStatus = (entry: Entry) => entry.row.status != null && entry.row.status !== 'ok'
  ? entry.row.status : entry.comparison.slowdown ? 'slow' : 'ok'

const outcomePriority = (entry: Entry) => entryStatus(entry) === 'fail' ? 0 : budgetLevel(entry.row) === 'alarm' ? 1
  : entryStatus(entry) === 'slow' ? 2 : budgetLevel(entry.row) === 'warn' ? 3 : entryStatus(entry) === 'unsupported' ? 4 : 5
const outcomeTone = (entry: Entry): ExplorerTone => outcomePriority(entry) < 2 ? 'error' : outcomePriority(entry) < 5 ? 'warning' : 'success'

const outcomeGroup = (entry: Entry) => entry.kind === 'micro' ? entry.row.module
  : entry.row.file.replace(/^lean-src\//, '').replace(/\.lean$/, '').replaceAll('/', '.')

function outcomeNodes(entries: Entry[]): ExplorerNode<Entry>[] {
  const groups = new Map<string, Entry[]>()
  for (const entry of entries) {
    const key = JSON.stringify([entry.kind, outcomeGroup(entry)])
    if (!groups.has(key)) groups.set(key, [])
    groups.get(key)!.push(entry)
  }
  return [...groups].map(([key, items]) => ({ key, label: outcomeGroup(items[0]), count: items.length,
    tone: outcomeTone(items[0]), defaultExpanded: true,
    children: items.map(entry => ({ key: entry.key, label: entry.kind === 'micro' ? entryName(entry) : entryName(entry).replace(/\.lean$/, ''), count: 1, value: entry,
      tone: outcomeTone(entry), search: `${entry.row.id} ${outcomeGroup(entry)}`,
      meta: `${entryStatus(entry)} · ${budgetLevel(entry.row) ?? 'no budget'} · ${numbers.format(entry.kind === 'micro'
        ? entry.row.ns_median : entry.row.ms_per_use)} ${entry.kind === 'micro' ? 'ns/call' : 'ms/use'}` })) }))
}

export function benchmarkEntries(data: LeanBenchData, baseline?: LeanBenchData): Entry[] {
  const modesDiffer = baseline != null && baseline.mode !== data.mode
  const microBaseline = new Map(baseline?.micro.map((row) => [microKey(row), row]))
  const macroBaseline = new Map(baseline?.macro.map((row) => [row.id, row]))
  const counts = new Map<string, number>()
  for (const row of [...data.micro, ...data.macro]) counts.set(row.id, (counts.get(row.id) ?? 0) + 1)
  const items: Entry[] = [
    ...data.micro.map((row): Entry => {
      const match = microBaseline.get(microKey(row))
      const key = JSON.stringify(['micro', row.module, row.id])
      return { kind: 'micro', key, hash: counts.get(row.id) === 1 ? row.id : key, row, baseline: match,
        comparison: modesDiffer ? { slowdown: false } : microComparison(row, match) }
    }),
    ...data.macro.map((row): Entry => {
      const match = macroBaseline.get(row.id)
      const key = JSON.stringify(['macro', row.id])
      return { kind: 'macro', key, hash: counts.get(row.id) === 1 ? row.id : key, row, baseline: match,
        comparison: modesDiffer ? { slowdown: false } : macroComparison(row, match) }
    }),
  ]
  return items
}

export function buildBenchmarkTree(data: LeanBenchData): ExplorerNode<Entry>[] {
  return outcomeNodes(benchmarkEntries(data).sort((a, b) => outcomePriority(a) - outcomePriority(b)))
}
