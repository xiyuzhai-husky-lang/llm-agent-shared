// The part of jsdom server.ts uses (jsdom ships no types).
declare module 'jsdom' {
  export class JSDOM {
    constructor(html?: string)
    readonly window: Window & typeof globalThis
  }
}
