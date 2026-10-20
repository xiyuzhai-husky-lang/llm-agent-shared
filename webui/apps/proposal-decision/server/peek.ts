import { createServer } from 'node:http'
import { spawn } from 'node:child_process'
import { parseArgs } from 'node:util'
import { createPeekChatStore, peekMiddleware, peekScopeDirectory, type PeekRequest } from '@webui/peek/node'

const { values } = parseArgs({ options: {
  socket: { type: 'string' }, repository: { type: 'string' }, state: { type: 'string' }, 'context-url': { type: 'string' },
}})
const required = (name: keyof typeof values): string => {
  const value = values[name]
  if (!value) throw new Error(`Missing --${name}`)
  return value
}
const repository = required('repository'), state = required('state'), contextURL = required('context-url')
const label = (value: unknown): string => {
  if (typeof value !== 'string' || !/^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$/.test(value)) throw new Error('Invalid proposal ID.')
  return value
}
interface Context { proposal: string; id: string; title: string; base: string; source?: { repository: string }; documents: { title: string; text: string }[] }
async function material(folder: string, candidate: string): Promise<Context | undefined> {
  const response = await fetch(`${contextURL}/api/peek-context?${new URLSearchParams({ proposal: folder, candidate })}`)
  if (response.status === 404) return undefined
  if (!response.ok) throw new Error(`Cannot read candidate context: ${response.status}`)
  return await response.json() as Context
}
const middleware = peekMiddleware({
  repoRoot: repository, route: '/api/peek',
  startProcess: cwd => spawn('codex', ['-c', 'sandbox_mode="read-only"', 'app-server', '--stdio'], { cwd, stdio: ['pipe', 'pipe', 'pipe'] }),
  store: createPeekChatStore(folder => peekScopeDirectory(state, repository, folder)),
  parseFolder: label, tabs: new Set(['main', 'tests', 'doubts', 'diff', 'effect', 'outcome']),
  invalidTab: 'Unknown decision tab.', targetUnavailable: 'The candidate has not been delivered.',
  candidateMismatch: 'This Peek belongs to another candidate.',
  targetExists: async (_, input) => (await material(input.folder, input.candidate)) !== undefined,
  prompt: async (input: PeekRequest) => {
    const context = await material(input.folder, input.candidate)
    if (!context) throw new Error('The candidate has not been delivered.')
    return [
      'You are Peek, the read-only assistant beside the proposal decision view. Answer the user about this immutable candidate.',
      `Proposal: ${context.proposal} — ${context.title}. Candidate: ${context.id}. Base: ${context.base}. Visible tab: ${input.tab}.`,
      `Repository: ${context.source?.repository ?? repository}. Use git show or git diff at the pinned commits when reading files. Never assume the current working tree equals the candidate.`,
      'Do not modify files, Git, signatures, Doubt rulings, verdicts, or the workflow. Decisions belong to the user.',
      'Use repository-relative Markdown links with #L line anchors. To show a changed file, call ui with command "diff REPO_PATH". To change tabs, use "tab TAB_NAME".',
      ...context.documents.map(doc => `${doc.title}:\n${doc.text}`),
      'Conversation:', ...input.messages.map(message => `${message.role}: ${message.text}`),
    ].join('\n\n')
  },
  clientInfo: { name: 'proposal-decision-peek', title: 'Proposal decision Peek', version: '1.0.0' },
  uiTool: { name: 'ui', description: 'Navigate the proposal decision view without making a decision.', inputSchema: {
    type: 'object', properties: { command: { type: 'string' } }, required: ['command'], additionalProperties: false,
  } },
})
const server = createServer((request, response) => middleware(request, response, () => { response.writeHead(404); response.end() }))
server.listen(required('socket'), () => process.stdout.write('ready\n'))
for (const signal of ['SIGTERM', 'SIGINT'] as const) process.on(signal, () => server.close(() => process.exit(0)))
