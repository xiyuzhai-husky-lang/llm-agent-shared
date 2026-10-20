// Source frames inside a Kathy document. The frame is `@kathy/source`'s SourceFrame, rendered once to static markup
// with its source already loaded; the other state of its expand/collapse toggle is rendered alongside into an inert
// <template>, so the browser toggles by swapping the two (hydrate.ts) and never loads or renders source itself.

import { createElement, type ReactNode } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { SourceFrame, type SourceFrameProps } from '@kathy/source'

export interface StaticSourceFrame {
  path: string
  line: number
  endLine: number
  oid: string
  source: string
  open: boolean
  compactCollapsed?: boolean
  annotations?: Array<{ line: number; start: number; end: number; severity?: 'error' | 'warning' | 'note'; label: HTMLElement }>
}

/** Fill `host` with the frame in its initial state and a `<template data-ks-alternate>` holding the other state. */
export function renderStaticSourceFrame(host: HTMLElement, frame: StaticSourceFrame): void {
  // A label is Kathy-rendered DOM; the frame wraps it in one span, as SourceFrame does for any React label.
  const annotations = frame.annotations?.map(({ label, ...rest }) => ({
    ...rest, label: createElement('span', { dangerouslySetInnerHTML: { __html: label.outerHTML } }) as ReactNode,
  }))
  const markup = (open: boolean) => renderToStaticMarkup(createElement(SourceFrame, {
    path: frame.path, line: frame.line, endLine: frame.endLine, oid: frame.oid, source: frame.source,
    defaultOpen: open, compactCollapsed: frame.compactCollapsed, annotations,
  } satisfies SourceFrameProps))
  host.innerHTML = markup(frame.open)
  const alternate = host.ownerDocument.createElement('template')
  alternate.dataset.ksAlternate = ''
  alternate.innerHTML = markup(!frame.open)
  host.append(alternate)
}
