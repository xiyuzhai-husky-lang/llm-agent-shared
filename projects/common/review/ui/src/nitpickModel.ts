/** The nitpicker's `Nitpicks:` section (older lists were headed `Nitpickings:`): a numbered list, each item tagged `[must-fix]` or `[note]`. */
/** `declined` is written by the loop driver: a must-fix item the Worker declined, with its reason in `details`. */
export type NitpickTag = 'must-fix' | 'note' | 'declined'
/** `headline` is the item's one bold sentence; `details` the paragraph under it. */
export interface Nitpick { index: number; tag: NitpickTag; headline: string; details: string }

const ITEM_START = /^\s*(\d+)[.)]\s+\[(must-fix|note|declined)\]\s*(.*)$/

export function parseNitpicks(markdown: string): Nitpick[] {
  const items: Nitpick[] = []
  for (const raw of markdown.split('\n')) {
    const line = raw.trim()
    const start = ITEM_START.exec(raw)
    if (start) { items.push({ index: Number(start[1]), tag: start[2] as NitpickTag, headline: unbold(start[3]), details: '' }); continue }
    if (items.length === 0 || line.length === 0 || /^Nitpick(ing)?s\b/i.test(line) || /^\d+[.)]\s/.test(line)) continue
    const item = items[items.length - 1]
    if (item.headline.length === 0) item.headline = unbold(line)
    else item.details = item.details.length === 0 ? line : `${item.details} ${line}`
  }
  return items
}

function unbold(text: string): string {
  return text.trim().replace(/^\*\*(.*)\*\*$/s, '$1').trim()
}

/** The must-fix items as a numbered list, headline first and details after it. */
export function mustFixList(items: Nitpick[]): string {
  return items.filter(item => item.tag === 'must-fix')
    .map((item, position) => `${position + 1}. ${item.headline}${item.details.length === 0 ? '' : ` ${item.details}`}`).join('\n')
}
