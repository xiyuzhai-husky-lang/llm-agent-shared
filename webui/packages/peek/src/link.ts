export interface SourceReference { path: string; startLine: number; endLine: number }

export function peekFileReference(href: string, origin: string): SourceReference | undefined {
  let path = href
  if (/^(?:https?:)?\/\//i.test(path)) {
    const url = new URL(path, origin)
    if (url.origin !== origin) return undefined
    path = url.pathname + url.hash
  } else if (/^[a-z][a-z\d+.-]*:/i.test(path) && !/:\d+(?::\d+)?$/.test(path)) {
    return undefined
  }
  path = decodeURIComponent(path)
  const location = path.match(/(?::([1-9]\d*)(?::\d+)?|#L([1-9]\d*)(?:-L?([1-9]\d*))?)$/)
  const startLine = Number(location?.[1] ?? location?.[2] ?? 1)
  const endLine = Number(location?.[3] ?? startLine)
  if (!Number.isSafeInteger(endLine) || endLine < startLine) throw new Error('Invalid source line range.')
  if (location != null) path = path.slice(0, location.index)
  path = path.replace(/^\.\//, '')
  const absolute = path.startsWith('/')
  // Preserve existing absolute citations; the workspace matches them against
  // the files already present in its Diff rather than navigating to this path.
  path = (absolute ? '/' : '') + validateRepoRelativePath(encodeURIComponent(absolute ? path.slice(1) : path))
  return { path, startLine, endLine }
}

export function validateRepoRelativePath(raw: string, label = 'Repository path'): string {
  const decoded = decodePath(raw, label).trim()
  if (decoded.length === 0) throw new Error(`${label} is empty.`)
  if (decoded.startsWith('/') || decoded.startsWith('\\')) {
    throw new Error(`${label} must be repository-relative.`)
  }
  if (/^[A-Za-z]:/.test(decoded) || /^[A-Za-z][A-Za-z\d+.-]*:/.test(decoded)) {
    throw new Error(`${label} must not be an absolute path or URL.`)
  }
  if (decoded.includes('\\') || decoded.includes('\0')) {
    throw new Error(`${label} contains unsupported characters.`)
  }
  const segments = decoded.split('/')
  if (segments.some((segment) => segment.length === 0 || segment === '.' || segment === '..')) {
    throw new Error(`${label} must not contain empty, current, or parent segments.`)
  }
  return segments.join('/')
}

function decodePath(raw: string, label: string): string {
  try {
    return decodeURIComponent(raw)
  } catch {
    throw new Error(`${label} contains invalid percent encoding.`)
  }
}

