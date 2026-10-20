import { request as httpRequest } from 'node:http'
import { request as httpsRequest } from 'node:https'

function endpoint(route) {
  return new URL(route, process.env.PROPOSAL_DECISION_URL ?? 'http://127.0.0.1:8801')
}

/** Thin transport: the Go resident owns snapshots, decisions and event waiting. */
export function decisionMiddleware(routes) {
  const allowed = new Set(routes)
  return (request, response, next) => {
    const incoming = new URL(request.url ?? '/', 'http://localhost')
    if (!allowed.has(incoming.pathname)) { next(); return }
    const target = endpoint(incoming.pathname + incoming.search)
    const forward = target.protocol === 'https:' ? httpsRequest : httpRequest
    const upstream = forward(target, { method: request.method, headers: request.headers }, result => {
      response.writeHead(result.statusCode ?? 502, result.headers)
      result.pipe(response)
    })
    response.on('close', () => upstream.destroy())
    upstream.on('error', error => {
      if (response.destroyed) return
      if (!response.headersSent) response.writeHead(502, { 'Content-Type': 'application/json' })
      response.end(JSON.stringify({ error: `proposal-decision: ${error.message}` }))
    })
    request.pipe(upstream)
  }
}
export function decisionPlugin(name, routes) {
  const middleware = decisionMiddleware(routes)
  return { name, configureServer(server) { server.middlewares.use(middleware) }, configurePreviewServer(server) { server.middlewares.use(middleware) } }
}
export async function decisionGet(route, parameters) {
  const url = endpoint(route)
  for (const [key, value] of Object.entries(parameters)) if (value != null) url.searchParams.set(key, value)
  const response = await fetch(url)
  const value = await response.json()
  if (!response.ok) throw new Error(value.error ?? `proposal-decision HTTP ${response.status}`)
  return value
}
