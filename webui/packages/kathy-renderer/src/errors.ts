export type KyRendererProfileId = 'web-v1' | 'web-v1-source'
type KyRendererBackendId = 'rose-html' | 'katex-0.17.0-safe'

export class KyRendererError extends Error {
  readonly code: string
  readonly path: string
  readonly profile: KyRendererProfileId
  readonly backend: KyRendererBackendId | null
  readonly reason: string | null

  constructor(
    code: string,
    path: string,
    options: {
      profile?: KyRendererProfileId
      backend?: KyRendererBackendId | null
      reason?: string | null
      message?: string
    } = {},
  ) {
    const profile = options.profile ?? 'web-v1'
    const backend = options.backend ?? null
    const reason = options.reason ?? null
    super(options.message ?? [code, profile, backend, path, reason].filter((part) => part !== null).join(': '))
    this.name = 'KyRendererError'
    this.code = code
    this.path = path
    this.profile = profile
    this.backend = backend
    this.reason = reason
  }
}
