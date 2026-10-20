import { Client } from '../../../packages/proposal-client/src/client'
import { button, element, input, select } from '../../../packages/proposal-client/src/element'
import type { Proposal } from './proposal'
import type { Ledger } from './ledger'

// App owns operator drafts and navigation; the workflow server decides transitions.
export class App {
  private client = new Client('proposal-workflow-command')
  private selected = ''
  private decisionURL = ''
  private drafts: Record<string, string> = {}

  constructor(private readonly root: HTMLElement) {
    this.client.onChange = () => this.render()
    this.subscribe()
    void fetch('/api/config').then(response => response.json()).then((config: { decision_url: string }) => { this.decisionURL = config.decision_url; this.render() }).catch(() => {})
  }

  private subscribe() {
    this.client.subscribe(['proposal/', 'ledger', 'feed', ...(this.selected ? [`event/${this.selected}/`, `decision/${this.selected}/`] : [])])
  }

  private field(name: string, placeholder: string, fallback = '') {
    const node = input(placeholder, this.drafts[name] ?? fallback)
    node.oninput = () => { this.drafts[name] = node.value }
    return node
  }

  private async send(command: Record<string, unknown>) {
    try { await this.client.send(command) } catch { /* Transport renders errors. */ }
  }

  private render() {
    const shell = element('main')
    shell.append(element('h1', 'Proposal workflow'), element('p', 'Role loop · gates · batches · slots', 'muted'))
    if (this.client.error) shell.append(element('p', this.client.error, 'error'))
    if (this.client.hasPending) shell.append(button('Retry pending command', () => { void this.client.retry().catch(() => {}) }, this.client.busy))
    const feed = this.client.get<{ journal: string; cursor: number }>('feed')
    shell.append(element('p', feed ? `Decision feed connected · cursor ${feed.cursor}` : 'Connecting to the decision feed…', 'muted'))
    const ledger = this.client.get<Ledger>('ledger')
    this.renderLedger(shell, ledger)
    const proposals = Object.entries(this.client.values).filter(([key]) => key.startsWith('proposal/')).map(([, value]) => value as Proposal)
    shell.append(element('h2', 'Proposals'))
    const table = element('table')
    for (const proposal of proposals) {
      const row = element('tr')
      row.append(element('td', proposal.uid), element('td', proposal.slot), element('td', `${proposal.status} ${proposal.phase}`), element('td', `Revision ${proposal.revision} · Worker ${proposal.worker_revisions}/${proposal.policy.worker_revisions}`))
      const actions = element('td'); actions.append(button('Open', () => { this.selected = proposal.uid; this.subscribe() })); row.append(actions); table.append(row)
    }
    shell.append(table)
    this.renderStart(shell)
    const current = proposals.find(proposal => proposal.uid === this.selected)
    if (current) {
      shell.append(element('h2', current.uid), element('p', `${current.status} · ${current.phase} · ${current.reason}`))
      if (current.deadline_exceeded) shell.append(element('p', 'Advisory deadline has passed.'))
      for (const action of ['stop', 'resume', 'end_early', 'republish', 'land']) shell.append(button(action, () => { void this.send({ action, uid: current.uid }) }, this.client.busy))
      if (current.candidate && this.decisionURL) {
        const link = element('a', `Open candidate ${current.candidate.id} in proposal-decision`)
        link.href = `${this.decisionURL}/?proposal=${encodeURIComponent(current.uid)}&candidate=${encodeURIComponent(current.candidate.id)}`; link.target = '_blank'; link.rel = 'noopener'; shell.append(link)
      }
      shell.append(element('h3', 'Received user decisions'))
      for (const [key, value] of Object.entries(this.client.values)) if (key.startsWith(`decision/${current.uid}/`)) shell.append(element('pre', JSON.stringify(value, null, 2)))
      shell.append(element('h3', 'Task history'))
      for (const [key, value] of Object.entries(this.client.values)) if (key.startsWith(`event/${current.uid}/`)) {
        const detail = element('details'); detail.append(element('summary', key.split('/').at(-1)), element('pre', JSON.stringify(value, null, 2))); shell.append(detail)
      }
    }
    this.root.replaceChildren(shell)
  }

  private renderLedger(shell: HTMLElement, ledger?: Ledger) {
    shell.append(element('h2', 'Batch ledger'))
    const batch = this.field('batch', 'Batch ID'), title = this.field('title', 'Batch title')
    shell.append(batch, title, button('Open batch', () => { void this.send({ action: 'ledger', ledger: { kind: 'open', batch: batch.value, title: title.value } }) }, this.client.busy))
    if (!ledger) return
    const source = this.field('source', 'Approval or settlement source')
    shell.append(source)
    for (const id of ledger.stack) {
      const value = ledger.batches[id]!
      const article = element('article')
      article.append(element('h3', `${id} · ${value.title}`), element('p', value.approval ? `Approved: ${value.approval}` : 'Awaiting approval'))
      article.append(button('Approve', () => { void this.send({ action: 'ledger', ledger: { kind: 'approve', batch: id, source: source.value } }) }, this.client.busy), button('Close settled batch', () => { void this.send({ action: 'ledger', ledger: { kind: 'close', batch: id } }) }, this.client.busy))
      const uid = this.field(`member-${id}`, 'Proposal ID'), scope = this.field(`scope-${id}`, 'Scope')
      article.append(uid, scope, button('Add member', () => { void this.send({ action: 'ledger', ledger: { kind: 'add', batch: id, uid: uid.value, title: uid.value, scope: scope.value } }) }, this.client.busy))
      for (const memberID of value.members) {
        const member = ledger.members[memberID]!
        const row = element('p', `${member.uid} · ${member.scope} · ${member.settlement || 'open'} `)
        if (!member.settlement) {
          for (const settlement of ['landed', 'rejected', 'abandoned', 'retired']) row.append(button(settlement, () => { void this.send({ action: 'ledger', ledger: { kind: 'settle', uid: member.uid, value: settlement, source: source.value } }) }, this.client.busy))
          row.append(button('Withdraw from batch', () => { void this.send({ action: 'ledger', ledger: { kind: 'withdraw', uid: member.uid, source: source.value } }) }, this.client.busy))
        }
        article.append(row)
      }
      shell.append(article)
    }
  }

  private renderStart(shell: HTMLElement) {
    const form = element('section')
    form.append(element('h3', 'Start an approved proposal'))
    const uid = this.field('uid', 'Proposal ID'), slot = this.field('slot', 'Slot'), directory = this.field('directory', 'Worktree directory'), assignment = this.field('assignment', 'Assignment')
    const rank = select(['D', 'C', 'B', 'A', 'S', 'SS', 'SSS'], this.drafts['rank'] ?? 'D', value => { this.drafts['rank'] = value })
    form.append(uid, slot, directory, assignment, rank, button('Start', () => { void this.send({ action: 'start', uid: uid.value, slot: slot.value, directory: directory.value, assignment: assignment.value, rank: rank.value }) }, this.client.busy))
    shell.append(form)
  }
}
