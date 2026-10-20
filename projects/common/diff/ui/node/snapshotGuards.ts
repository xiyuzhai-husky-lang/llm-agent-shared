import type { ServerResponse } from 'node:http'

const FULL_OID = /^[0-9a-f]{40,64}$/
const SAFE_PATH = /^(?!\/)(?!.*(?:^|\/)\.\.?\/)(?!.*\\)(?!.*\0)[^\n\r]+$/

export class SnapshotError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

export function requiredOid(raw: string | null): string {
  const oid = raw?.trim() ?? ''
  if (!FULL_OID.test(oid)) throw new SnapshotError(400, 'Revision must be a full Git commit OID.')
  return oid
}

export function requiredPath(raw: string | null): string {
  const path = raw?.trim() ?? ''
  if (!SAFE_PATH.test(path) || path.split('/').some((part) => part.length === 0 || part === '.' || part === '..')) {
    throw new SnapshotError(400, 'Path must be a safe repository-relative path.')
  }
  return path
}

export function sendJson(response: ServerResponse, status: number, payload: unknown): void {
  if (response.headersSent) return
  response.statusCode = status
  response.setHeader('Cache-Control', 'no-store')
  response.setHeader('Content-Type', 'application/json; charset=utf-8')
  response.end(JSON.stringify(payload))
}
