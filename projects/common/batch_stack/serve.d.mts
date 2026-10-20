import type { IncomingMessage, ServerResponse } from 'node:http'

export function batchStackMiddleware(main: string, projectKey: string, base?: string):
  (request: IncomingMessage, response: ServerResponse, next: () => void) => Promise<void>
export interface BatchFrame {
 id: string; title: string; state: string
 proposals: { uid: string; branch: string | null; ordinal: number; settlement: {kind:string;source:string} | null; workflowStatus: string }[]
}
export function readStack(main: string, projectKey: string): Promise<{stacked: BatchFrame[]; settled: BatchFrame[]; next: number}>
export function renderPage(stack: Awaited<ReturnType<typeof readStack>>, stateUrl?: string): string
