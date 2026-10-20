// The reviewer prepares archived Kathy documents once, before serving the UI.
import { readFileSync } from 'node:fs'
import { execFileSync } from 'node:child_process'
import { parseKyDocumentJson } from '../../../packages/kathy-ast/src/index.ts'
import { renderKyDocumentHtml } from '../../../packages/kathy-renderer/src/server.ts'
import { createKathyVfs } from '../../../packages/kathy-vfs/src/index.ts'
const input = JSON.parse(readFileSync(0, 'utf8')) as { documents: Record<string, unknown>; sources: Record<string, string>; oid: string; repository: string }
const vfs = createKathyVfs(async (path, oid) => {
  const source = input.sources[`${oid}:${path}`]
  if (source !== undefined) return source
  if (!/^[0-9a-f]{40,64}$/.test(oid) || path.startsWith('/') || path.split('/').includes('..')) throw new Error('Invalid pinned source')
  return execFileSync('git', ['-C', input.repository, 'show', `${oid}:${path}`], { encoding: 'utf8', maxBuffer: 24 * 1024 * 1024 })
})
const output: Record<string, string> = {}
for (const [id, wire] of Object.entries(input.documents)) output[id] = await renderKyDocumentHtml(parseKyDocumentJson(wire), { profile: 'web-v1-source', oid: input.oid, vfs })
process.stdout.write(JSON.stringify(output))
