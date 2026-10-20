import { execFile } from 'node:child_process'
import type { KyDocumentWire } from '@kathy/ast'

export class KathyConversionError extends Error {
  readonly status = 422
}

/** Run the producer in the source checkout, including when serving a fixture repository. */
export function convertKyTex(text: string, converterRoot: string): Promise<KyDocumentWire> {
  return new Promise((resolve, reject) => {
    const child = execFile('cargo', ['run', '--quiet', '-p', 'latex-to-kathy', '--bin', 'latex-to-kathy', '--', '-'],
      { cwd: converterRoot, maxBuffer: 16 * 1024 * 1024 }, (error, stdout, stderr) => {
        if (error != null) { reject(new KathyConversionError(`Kathy conversion failed: ${stderr.trim() || error.message}`)); return }
        try { resolve(JSON.parse(stdout) as KyDocumentWire) }
        catch (reason) { reject(new KathyConversionError(`Kathy conversion failed: ${String(reason)}`)) }
      })
    child.stdin?.end(text)
  })
}

/** References interpreted by web-v1-source, including embedded rendered documents. */
export function kathySourcePaths(document: KyDocumentWire): Array<[path: string, oid?: string]> {
  const references: Array<[string, string?]> = []
  for (const node of document.arena.rose) {
    if (node.kind === 'rendered_span') references.push(...kathySourcePaths(node.content))
    if (node.kind === 'codespan') references.push([node.source.path, node.source.oid ?? undefined])
    if (node.kind !== 'verbatim_command') continue
    const match = node.body.match(/^([A-Za-z0-9_./-]+)#L([1-9][0-9]*)-L([1-9][0-9]*)(?:@([0-9a-fA-F]{7,64}))?$/)
    if (match == null || match[1].split('/').some(part => part === '' || part === '.' || part === '..')) continue
    const start = Number(match[2]), end = Number(match[3])
    if (!Number.isSafeInteger(start) || !Number.isSafeInteger(end) || end < start) continue
    references.push([match[1], match[4]])
  }
  return references
}
