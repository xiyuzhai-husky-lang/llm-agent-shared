import type { KyDocument } from '@kathy/ast'

import { KyRendererError, type KyRendererProfileId } from './errors.ts'
import { kyRendererProfileCatalog } from './profile.generated.ts'
import { kyKatexSafeProfile } from './profiles/katex-0.17.0-safe.ts'
import { kyRoseHtmlProfile } from './profiles/rose-html.ts'
import { kyWebV1Profile } from './profiles/web-v1.ts'

type BackendId = 'rose-html' | 'katex-0.17.0-safe'

const katexCommands = new Map<string, (typeof kyKatexSafeProfile.commands)[number]>(kyKatexSafeProfile.commands.map((row) => [row.name, row]))
const katexEnvironments = new Map<string, (typeof kyKatexSafeProfile.environments)[number]>(kyKatexSafeProfile.environments.map((row) => [`${row.name}\0${row.starred}`, row]))
const roseCommands = new Set<string>(kyRoseHtmlProfile.builtins.filter((row) => row.kind === 'command').map((row) => row.name))
const roseCommandProfiles = new Map<string, (typeof kyRendererProfileCatalog.commands)[number]['rose_html']>(
  kyRendererProfileCatalog.commands.map((row) => [row.command_name, row.rose_html]),
)
const roseEnvironments = new Set<string>(kyRoseHtmlProfile.builtins.filter((row) => row.kind === 'environment').map((row) => `${row.name}\0${row.starred}`))

export function validateRendererSupport(document: KyDocument, profile: KyRendererProfileId = 'web-v1'): void {
  if (kyWebV1Profile.backends.length !== 2) throw new KyRendererError('InvalidProfile', 'profile', { profile })
  validateRose(document, profile)
  validateKatex(document, profile)
  validateTikz(document, profile)
}

function unsupported(profile: KyRendererProfileId, backend: BackendId | null, path: string, reason: string): never {
  throw new KyRendererError('UnsupportedByRenderer', path, { profile, backend, reason })
}

function validateRose(document: KyDocument, profile: KyRendererProfileId): void {
  const backend = 'rose-html'
  for (const [index, node] of document.arena.rose.entries()) {
    const path = `arena.rose[${index}]`
    if (node.kind === 'rendered_span') validateRendererSupport(node.content as KyDocument, profile)
    if (node.kind === 'codespan' && profile !== 'web-v1-source') {
      unsupported(profile, backend, path, 'codespan_requires_kySource_profile')
    }
    if (node.kind === 'complete_command' && !roseCommands.has(node.command_name + (node.starred ? '*' : ''))
      && roseCommandProfiles.get(node.command_name)?.status !== 'supported'
      && !(profile === 'web-v1-source' && ['textbf', 'texttt'].includes(node.command_name))) {
      unsupported(
        profile,
        backend,
        path,
        roseCommandProfiles.get(node.command_name)?.reason ?? 'command_not_in_rose_html_profile',
      )
    }
    if (node.kind === 'verbatim_command' && node.command_name !== 'verb') {
      unsupported(profile, backend, path, 'rose_verbatim_command_has_no_html_handler')
    }
    if (node.kind === 'environment') {
      const key = `${node.environment_name}\0${node.starred}`
      if (!roseEnvironments.has(key) && node.asts.kind !== 'math') {
        unsupported(profile, backend, path, 'rose_environment_has_no_html_handler')
      }
    }
  }
}

function validateKatex(document: KyDocument, profile: KyRendererProfileId): void {
  const backend = 'katex-0.17.0-safe'
  for (const [index, node] of document.arena.math.entries()) {
    const path = `arena.math[${index}]`
    if (node.kind === 'complete_command' || node.kind === 'verbatim_command') {
      const row = katexCommands.get(node.command_name)
      if (row?.status !== 'supported') unsupported(profile, backend, path, row?.reason ?? 'command_not_in_katex_safe_profile')
    }
    if (node.kind === 'lefted' || node.kind === 'righted') {
      const row = katexCommands.get(node.command)
      if (row?.status !== 'supported') unsupported(profile, backend, path, row?.reason ?? 'delimiter_command_not_in_katex_safe_profile')
    }
    if (node.kind === 'environment' && node.asts.kind !== 'tikz') {
      const row = katexEnvironments.get(`${node.environment_name}\0${node.starred}`)
      if (row?.status !== 'supported') unsupported(profile, backend, path, row?.reason ?? 'environment_not_in_katex_safe_profile')
    }
  }
  for (const [index, node] of document.arena.rose.entries()) {
    if (node.kind !== 'environment' || node.asts.kind !== 'math') continue
    const row = katexEnvironments.get(`${node.environment_name}\0${node.starred}`)
    if (row?.status !== 'supported') {
      unsupported(profile, backend, `arena.rose[${index}]`, row?.reason ?? 'environment_not_in_katex_safe_profile')
    }
  }
}

/** TikZ has no renderer yet: every TikZ environment, in Rose or Math, is rejected before any DOM is written. */
function validateTikz(document: KyDocument, profile: KyRendererProfileId): void {
  for (const [index, node] of document.arena.rose.entries()) {
    if (node.kind === 'environment' && node.asts.kind === 'tikz') unsupported(profile, null, `arena.rose[${index}]`, KY_TIKZ_HAS_NO_RENDERER)
  }
  for (const [index, node] of document.arena.math.entries()) {
    if (node.kind === 'environment' && node.asts.kind === 'tikz') unsupported(profile, null, `arena.math[${index}]`, KY_TIKZ_HAS_NO_RENDERER)
  }
}

export const KY_TIKZ_HAS_NO_RENDERER = 'tikz_has_no_renderer'
