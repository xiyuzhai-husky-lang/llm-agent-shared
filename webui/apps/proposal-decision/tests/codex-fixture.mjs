#!/usr/bin/env node
// Explicit local app-server protocol fixture; never connects to a model provider.
import { createInterface } from 'node:readline'
import { appendFileSync } from 'node:fs'
const send = message => process.stdout.write(JSON.stringify(message) + '\n')
let turn = 0
createInterface({ input: process.stdin }).on('line', line => {
  const request = JSON.parse(line)
  appendFileSync(process.env.PEEK_FIXTURE_LOG, JSON.stringify({ argv: process.argv.slice(2), request }) + '\n')
  if (request.method === 'initialize') send({ id: request.id, result: {} })
  if (request.method === 'model/list') send({ id: request.id, result: { data: [{ model: 'gpt-6-astra', displayName: 'Fixture Astra', defaultReasoningEffort: 'medium', supportedReasoningEfforts: [{ reasoningEffort: 'medium', description: 'Fixture' }], isDefault: true }], nextCursor: null } })
  if (request.method === 'thread/start') send({ id: request.id, result: { model: request.params.model, thread: { id: 'fixture-thread' } } })
  if (request.method === 'turn/start') {
    const id = 'fixture-turn-' + ++turn
    send({ id: request.id, result: { turn: { id } } })
    const prompt = request.params.input[0].text
    send({ method: 'item/agentMessage/delta', params: { threadId: 'fixture-thread', turnId: id, delta: 'Fixture answer: see [address_of.c](address_of.c#L1). ' } })
    if (prompt.includes('Navigate with the tool')) {
      send({ id: 'navigate', method: 'item/tool/call', params: { threadId: 'fixture-thread', turnId: id, tool: 'ui', arguments: { command: 'diff dereference.c' } } })
      return
    }
    if (prompt.includes('Wait for cancellation')) return
    send({ method: 'turn/completed', params: { threadId: 'fixture-thread', turn: { id, status: 'completed' } } })
  }
  if (request.id === 'navigate') send({ method: 'turn/completed', params: { threadId: 'fixture-thread', turn: { id: 'fixture-turn-' + turn, status: 'completed' } } })
})
