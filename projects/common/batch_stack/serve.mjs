#!/usr/bin/env node
// usage: serve.mjs <main> <project-key> [port]
//
// The batch stack, on its own page. Every project keeps its
// own stack; this is one piece of code that serves any of them.
// It shows the frames and nothing else: each unsettled batch top-first, its state, and the
// proposals inside it. Read-only — the actions arrive when the scheduler that performs them exists.
import { createServer } from 'node:http'
import { resolve } from 'node:path'
import { pathToFileURL } from 'node:url'

export async function readStack(main, projectKey) {
  const address = process.env.PROPOSAL_WORKFLOW_URL ?? 'http://127.0.0.1:8800'
  const get = async (path) => {
    const response = await fetch(address + path, { signal: AbortSignal.timeout(30000) })
    if (!response.ok) throw new Error(`${address}${path}: ${response.status} ${await response.text()}`)
    return response.json()
  }
  const health = await get('/api/health')
  if (health.project !== `projects/${projectKey}` || resolve(health.main) !== resolve(main)) throw new Error('workflow resident project or main differs')
  const frame = await get('/api/views?prefix=ledger&prefix=proposal%2F')
  const ledger = frame.puts.ledger
  const convert = (batch, index) => ({
    id: batch.id, title: batch.title, description: batch.description, startedAt: batch.started_at,
    state: index === 0 ? 'active' : index > 0 ? 'suspended' : 'settled',
    approval: batch.approval ? { source: batch.approval, at: batch.approved_at, text: batch.approval_text } : null,
    proposals: batch.members.map((uid, i) => {
      const member = ledger.members[uid]
      return { ...member, ordinal: member.ordinal, branch: member.branch || null,
        settlement: member.settlement ? { kind: member.settlement, source: member.source } : null,
        workflowStatus: frame.puts[`proposal/${uid}`]?.status ?? 'not started' }
    }),
  })
  return { stacked: ledger.stack.map((id, i) => convert(ledger.batches[id], i)),
    settled: Object.values(ledger.batches).filter(batch => !ledger.stack.includes(batch.id)).map(batch => convert(batch, -1)),
    next: ledger.next }
}

const escape = (text) => String(text ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]))

function renderProposal(proposal) {
  const settlement = proposal.settlement
  const state = escape(settlement?.kind ?? proposal.workflowStatus)
  // The scope line is the thing the user rules on before a proposal is dispatched, so it belongs here rather than in
  // a chat message that scrolls away.
  const scope = proposal.scope ? `<p class="scope">${escape(proposal.scope)}</p>` : ''
  return `<li class="proposal" data-state="${escape(state)}">
    <div class="proposal-line">
      <span class="ordinal">#${escape(proposal.ordinal)}</span>
      <span class="title">${escape(proposal.title)}</span>
      <span class="uid">${escape(proposal.uid)}</span>
      <span class="state">${state}</span>
    </div>
    ${scope}
  </li>`
}

function renderBatch(batch, position) {
  const label = position === 0 ? 'top' : position == null ? 'settled' : `under ${position}`
  const started = batch.startedAt ? `started ${escape(batch.startedAt)}` : 'not started'
  const approval = batch.approval
    ? `approved ${escape(batch.approval.at ?? '')}`
    : 'not approved yet'
  // A frame is only useful if it says what the batch is for; the title alone never survives a week on the stack.
  const description = batch.description
    ? `<p class="description">${escape(batch.description)}</p>`
    : '<p class="description missing">no description written</p>'
  return `<section class="batch" data-state="${escape(batch.state)}">
    <h2><span class="id">${escape(batch.id)}</span> ${escape(batch.title)}
      <span class="state">${escape(batch.state)}</span><span class="position">${escape(label)}</span></h2>
    ${description}
    <p class="approval">${started} · ${approval}</p>
    <ul class="proposals">${(batch.proposals ?? []).map(renderProposal).join('')}</ul>
  </section>`
}

export function renderPage({ stacked, settled }, stateUrl = '/stack.json') {
  const live = stacked.length === 0
    ? '<p class="empty">The stack is empty: there is no current work.</p>'
    : stacked.map((batch, index) => renderBatch(batch, index)).join('')
  // Everything the project has ever batched stays listed, so the number, what it was and when it began remain
  // answerable long after it settled.
  const history = settled.length === 0 ? '' :
    `<h1 class="past">settled</h1>${settled.map((batch) => renderBatch(batch, null)).join('')}`
  const body = live + history
  return `<!doctype html><html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Batch stack</title><style>
:root { color-scheme: light; --ink: #1a1a1a; --dim: #6b6b6b; --line: #d8d8d4; --ground: #f7f7f5; --panel: #fff; }
body { margin: 0; padding: 24px; background: var(--ground); color: var(--ink);
  font: 14px/1.5 ui-sans-serif, -apple-system, "Helvetica Neue", sans-serif; }
h1 { font-size: 15px; margin: 0 0 16px; letter-spacing: .08em; text-transform: uppercase; color: var(--dim); }
.batch { background: var(--panel); border: 1px solid var(--line); margin-bottom: 10px; }
.batch h2 { display: flex; align-items: baseline; gap: 10px; margin: 0; padding: 10px 14px; font-size: 15px;
  border-bottom: 1px solid var(--line); }
.batch .id { font: 600 13px ui-monospace, SFMono-Regular, Menlo, monospace; color: var(--dim); }
.batch .state, .proposal .state { font: 12px ui-monospace, SFMono-Regular, Menlo, monospace; color: var(--dim); }
.batch .position { margin-left: auto; font-size: 12px; color: var(--dim); }
.batch[data-state="active"] .state { color: #1d6f42; }
.batch[data-state="suspended"] .state { color: #9a6700; }
ul.proposals { list-style: none; margin: 0; padding: 0; }
.proposal { padding: 8px 14px; border-top: 1px solid #efefec; }
.proposal-line { display: flex; align-items: baseline; gap: 10px; }
.scope { margin: 8px 0 2px; max-width: 62ch; color: #333; border-left: 2px solid var(--line); padding-left: 10px;
  overflow-wrap: anywhere; }
.proposal:first-child { border-top: 0; }
.proposal .ordinal { font: 600 12px ui-monospace, SFMono-Regular, Menlo, monospace; color: var(--dim); }
.proposal .uid { font: 12px ui-monospace, SFMono-Regular, Menlo, monospace; color: var(--dim); }
.proposal .state { margin-left: auto; }
.proposal[data-state="landed"] .state { color: #1d6f42; }
.proposal[data-state="running"] .state { color: #1d4ed8; }
.description { margin: 10px 14px 6px; max-width: 62ch; }
.description.missing { color: #9a6700; font-style: italic; }
.approval { margin: 0 14px 10px; font-size: 12px; color: var(--dim); }
.empty { color: var(--dim); }
h1.past { margin: 26px 0 12px; }
.batch[data-state="done"] { opacity: .72; }
</style></head><body>
<h1>Batch stack</h1>
${body}
<script>setInterval(() => fetch(${JSON.stringify(stateUrl)}).then((r) => r.json()).then((now) => {
  if (now.etag !== ${JSON.stringify(JSON.stringify({ stacked, settled }))}) location.reload()
}).catch(() => {}), 5000)</script>
</body></html>`
}

/** Same standalone page mounted by each reviewer host; it does not start the loop. */
export function batchStackMiddleware(main, projectKey, base = '/batch-stack') {
  return async (request, response, next) => {
    const path = new URL(request.url ?? '/', 'http://localhost').pathname
    const stateUrl = base ? `${base}/stack.json` : '/stack.json'
    if (path !== (base || '/') && path !== `${base}/` && path !== stateUrl) { next(); return }
    try {
      const stack = await readStack(main, projectKey)
      const etag = JSON.stringify({ stacked: stack.stacked, settled: stack.settled })
      if (path === stateUrl) {
        response.writeHead(200, { 'content-type': 'application/json', 'cache-control': 'no-store' })
        response.end(JSON.stringify({ ...stack, etag }))
      } else {
        response.writeHead(200, { 'content-type': 'text/html; charset=utf-8', 'cache-control': 'no-store' })
        response.end(renderPage(stack, stateUrl))
      }
    } catch (error) {
      response.writeHead(500, { 'content-type': 'text/plain; charset=utf-8' })
      response.end(`batch stack unreadable from the workflow resident for ${projectKey}: ${error.message}\n`)
    }
  }
}

if (process.argv[1] && pathToFileURL(resolve(process.argv[1])).href === import.meta.url) {
  const [main, projectKey, port = '8795'] = process.argv.slice(2)
  if (!main || !projectKey || process.argv.length > 5) { console.error('usage: serve.mjs <main> <project-key> [port]'); process.exit(64) }
  const handler = batchStackMiddleware(main, projectKey, '')
  createServer((request, response) => handler(request, response, () => {
    response.writeHead(404); response.end('Not found')
  })).listen(Number(port), '127.0.0.1', () => console.log(`batch stack of ${projectKey} under ${main} on http://127.0.0.1:${port}`))
}
