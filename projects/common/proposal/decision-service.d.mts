import type { IncomingMessage, ServerResponse } from 'node:http'
import type { Plugin } from 'vite'
export function decisionMiddleware(routes: string[]): (request: IncomingMessage, response: ServerResponse, next: () => void) => void
export function decisionPlugin(name: string, routes: string[]): Plugin
export function decisionGet<T>(route: string, parameters: Record<string, string | undefined>): Promise<T>
