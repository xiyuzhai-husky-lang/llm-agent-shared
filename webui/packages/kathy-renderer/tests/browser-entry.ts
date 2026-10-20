import { parseKyDocumentJson } from '@kathy/ast'
import { createElement } from 'react'
import { createRoot, type Root } from 'react-dom/client'

import {
  KyDocumentView,
  renderKyDocument,
  type KyDocumentRenderHandle,
} from '../src/index.ts'

let handle: KyDocumentRenderHandle | undefined
let reactRoot: Root | undefined

declare global {
  interface Window {
    renderKathyFixture(value: unknown): { ok: true } | { ok: false; phase?: string; code?: string; path?: string }
    renderKathyFixtureAndWait(value: unknown): Promise<{ ok: true } | { ok: false; phase?: string; code?: string; path?: string; message?: string }>
    resetKathyFixture(): void
    renderKathyReactFixture(value: unknown): void
    unmountKathyReactFixture(): void
    lastKathyReactError(): { code: string; path: string; message: string } | null
  }
}

window.renderKathyFixtureAndWait = async (value) => {
  const root = document.querySelector<HTMLElement>('#root')
  if (root === null) throw new Error('missing test root')
  try {
    handle?.dispose()
    handle = renderKyDocument(root, parseKyDocumentJson(value))
    await handle.finished
    return { ok: true }
  } catch (error) {
    const value = error as { phase?: string; code?: string; path?: string; message?: string }
    return { ok: false, phase: value.phase, code: value.code, path: value.path, message: value.message }
  }
}

window.renderKathyFixture = (value) => {
  const root = document.querySelector<HTMLElement>('#root')
  if (root === null) throw new Error('missing test root')
  try {
    handle?.dispose()
    handle = renderKyDocument(root, parseKyDocumentJson(value))
    return { ok: true }
  } catch (error) {
    const value = error as { phase?: string; code?: string; path?: string }
    return { ok: false, phase: value.phase, code: value.code, path: value.path }
  }
}

window.resetKathyFixture = () => {
  handle?.dispose()
  handle = undefined
}

window.renderKathyReactFixture = (value) => {
  const root = document.querySelector<HTMLElement>('#root')
  if (root === null) throw new Error('missing test root')
  reactRoot ??= createRoot(root)
  reactError = null
  reactRoot.render(createElement(KyDocumentView, {
    document: parseKyDocumentJson(value),
    onError(error) {
      reactError = { code: error.code, path: error.path, message: error.message }
    },
  }))
}

window.unmountKathyReactFixture = () => {
  reactRoot?.unmount()
  reactRoot = undefined
}

let reactError: { code: string; path: string; message: string } | null = null
window.lastKathyReactError = () => reactError
